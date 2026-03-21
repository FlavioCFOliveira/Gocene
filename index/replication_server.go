package index

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ReplicationServer handles the server-side of index replication.
// It serves index files to replication clients.
type ReplicationServer struct {
	mu sync.RWMutex

	// address is the server listen address
	address string

	// port is the server listen port
	port int

	// indexPath is the path to the index directory
	indexPath string

	// httpServer is the HTTP server
	httpServer *http.Server

	// listener is the network listener
	listener net.Listener

	// isRunning indicates if the server is running
	isRunning atomic.Bool

	// sessions holds active replication sessions
	sessions map[string]*ReplicationSession

	// currentRevision is the current index revision
	currentRevision *IndexRevision

	// handler is the HTTP handler
	handler http.Handler
}

// NewReplicationServer creates a new ReplicationServer.
func NewReplicationServer(address string, port int, indexPath string) (*ReplicationServer, error) {
	if address == "" {
		return nil, fmt.Errorf("address cannot be empty")
	}

	if port <= 0 {
		return nil, fmt.Errorf("port must be positive")
	}

	if indexPath == "" {
		return nil, fmt.Errorf("index path cannot be empty")
	}

	rs := &ReplicationServer{
		address:         address,
		port:            port,
		indexPath:       indexPath,
		sessions:        make(map[string]*ReplicationSession),
		currentRevision: &IndexRevision{},
	}

	// Create HTTP handler
	mux := http.NewServeMux()
	mux.HandleFunc("/revision", rs.handleRevision)
	mux.HandleFunc("/files/", rs.handleFile)
	mux.HandleFunc("/session", rs.handleSession)
	rs.handler = mux

	return rs, nil
}

// Start starts the replication server.
func (rs *ReplicationServer) Start() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.isRunning.Load() {
		return fmt.Errorf("replication server is already running")
	}

	// Create listener
	addr := fmt.Sprintf("%s:%d", rs.address, rs.port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("creating listener: %w", err)
	}
	rs.listener = listener

	// Create HTTP server
	rs.httpServer = &http.Server{
		Handler:      rs.handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	rs.isRunning.Store(true)

	// Start serving in a goroutine
	go func() {
		if err := rs.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Log error
		}
	}()

	return nil
}

// Stop stops the replication server.
func (rs *ReplicationServer) Stop(ctx context.Context) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if !rs.isRunning.Load() {
		return nil
	}

	rs.isRunning.Store(false)

	// Close all sessions
	rs.sessions = make(map[string]*ReplicationSession)

	// Shutdown HTTP server
	if rs.httpServer != nil {
		if err := rs.httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutting down server: %w", err)
		}
	}

	return nil
}

// IsRunning returns true if the server is running.
func (rs *ReplicationServer) IsRunning() bool {
	return rs.isRunning.Load()
}

// GetAddress returns the server address.
func (rs *ReplicationServer) GetAddress() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	return rs.address
}

// GetPort returns the server port.
func (rs *ReplicationServer) GetPort() int {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	return rs.port
}

// GetIndexPath returns the index path.
func (rs *ReplicationServer) GetIndexPath() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	return rs.indexPath
}

// SetIndexPath sets the index path.
func (rs *ReplicationServer) SetIndexPath(path string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.isRunning.Load() {
		return fmt.Errorf("cannot change index path while server is running")
	}

	if path == "" {
		return fmt.Errorf("index path cannot be empty")
	}

	rs.indexPath = path
	return nil
}

// GetCurrentRevision returns the current index revision.
func (rs *ReplicationServer) GetCurrentRevision() *IndexRevision {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	if rs.currentRevision == nil {
		return nil
	}

	return rs.currentRevision.Clone()
}

// SetCurrentRevision sets the current index revision.
func (rs *ReplicationServer) SetCurrentRevision(revision *IndexRevision) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if revision == nil {
		return fmt.Errorf("revision cannot be nil")
	}

	rs.currentRevision = revision
	return nil
}

// CreateSession creates a new replication session.
func (rs *ReplicationServer) CreateSession(clientID string) (*ReplicationSession, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if !rs.isRunning.Load() {
		return nil, fmt.Errorf("replication server is not running")
	}

	session := &ReplicationSession{
		ID:        generateReplicationServerSessionID(),
		Revision:  rs.currentRevision,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	rs.sessions[session.ID] = session

	return session, nil
}

// GetSession returns a replication session by ID.
func (rs *ReplicationServer) GetSession(sessionID string) (*ReplicationSession, error) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	session, ok := rs.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Check if expired
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired: %s", sessionID)
	}

	return session, nil
}

// CloseSession closes a replication session.
func (rs *ReplicationServer) CloseSession(sessionID string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	delete(rs.sessions, sessionID)
	return nil
}

// GetSessionCount returns the number of active sessions.
func (rs *ReplicationServer) GetSessionCount() int {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	return len(rs.sessions)
}

// CleanupSessions removes expired sessions.
func (rs *ReplicationServer) CleanupSessions() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	now := time.Now()
	for id, session := range rs.sessions {
		if now.After(session.ExpiresAt) {
			delete(rs.sessions, id)
		}
	}
}

// handleRevision handles requests for the current revision.
func (rs *ReplicationServer) handleRevision(w http.ResponseWriter, r *http.Request) {
	rs.mu.RLock()
	revision := rs.currentRevision
	rs.mu.RUnlock()

	// Return revision info as JSON
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"generation":%d,"version":%d,"files":%d}`,
		revision.Generation, revision.Version, len(revision.Files))
}

// handleFile handles requests for specific files.
func (rs *ReplicationServer) handleFile(w http.ResponseWriter, r *http.Request) {
	// Extract filename from URL
	filename := r.URL.Path[len("/files/"):]
	if filename == "" {
		http.Error(w, "filename required", http.StatusBadRequest)
		return
	}

	rs.mu.RLock()
	indexPath := rs.indexPath
	rs.mu.RUnlock()

	// Serve file
	filePath := indexPath + "/" + filename
	http.ServeFile(w, r, filePath)
}

// handleSession handles session management requests.
func (rs *ReplicationServer) handleSession(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		// Create new session
		session, err := rs.CreateSession("")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, `{"session_id":"%s"}`, session.ID)

	case http.MethodDelete:
		// Close session
		sessionID := r.URL.Query().Get("id")
		if sessionID == "" {
			http.Error(w, "session ID required", http.StatusBadRequest)
			return
		}
		rs.CloseSession(sessionID)
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// String returns a string representation of the ReplicationServer.
func (rs *ReplicationServer) String() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	return fmt.Sprintf("ReplicationServer{running=%v, address=%s:%d, sessions=%d}",
		rs.isRunning.Load(), rs.address, rs.port, len(rs.sessions))
}

// generateReplicationServerSessionID generates a unique session ID.
func generateReplicationServerSessionID() string {
	return fmt.Sprintf("server-session-%d", time.Now().UnixNano())
}
