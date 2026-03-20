package index

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// HTTPReplicator handles HTTP-based index replication.
// It downloads index files from a remote HTTP server.
type HTTPReplicator struct {
	mu sync.RWMutex

	// serverURL is the URL of the replication server
	serverURL string

	// targetPath is the local path to store replicated files
	targetPath string

	// base provides common replicator functionality
	base *ReplicatorWithStats

	// isOpen indicates if the replicator is open
	isOpen atomic.Bool

	// httpClient is the HTTP client for downloads
	httpClient *http.Client

	// currentRevision is the current replicated revision
	currentRevision *IndexRevision

	// headers contains additional HTTP headers to send
	headers map[string]string
}

// NewHTTPReplicator creates a new HTTPReplicator.
func NewHTTPReplicator(serverURL, targetPath string) (*HTTPReplicator, error) {
	if serverURL == "" {
		return nil, fmt.Errorf("server URL cannot be empty")
	}

	if targetPath == "" {
		return nil, fmt.Errorf("target path cannot be empty")
	}

	// Validate URL
	_, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	hr := &HTTPReplicator{
		serverURL:  serverURL,
		targetPath: targetPath,
		base:       NewReplicatorWithStats("http-replicator", nil),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		headers: make(map[string]string),
	}

	// Set the check function
	hr.base.SetCheckFunc(func(ctx context.Context) error {
		return hr.performReplication(ctx)
	})

	hr.isOpen.Store(true)

	return hr, nil
}

// performReplication performs the actual HTTP replication.
func (hr *HTTPReplicator) performReplication(ctx context.Context) error {
	hr.mu.RLock()
	serverURL := hr.serverURL
	targetPath := hr.targetPath
	hr.mu.RUnlock()

	// Ensure target directory exists
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
	}

	// Fetch revision info from server
	revision, err := hr.fetchRevision(ctx, serverURL)
	if err != nil {
		return fmt.Errorf("fetching revision: %w", err)
	}

	// Download files
	var totalBytes int64
	for _, file := range revision.Files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fileURL, err := hr.buildFileURL(serverURL, file)
		if err != nil {
			return fmt.Errorf("building file URL: %w", err)
		}

		targetFile := filepath.Join(targetPath, file)
		bytes, err := hr.downloadFile(ctx, fileURL, targetFile)
		if err != nil {
			return fmt.Errorf("downloading file %s: %w", file, err)
		}

		totalBytes += bytes
	}

	// Record statistics
	hr.base.RecordBytesTransferred(totalBytes)

	// Update current revision
	hr.mu.Lock()
	hr.currentRevision = revision
	hr.mu.Unlock()

	return nil
}

// fetchRevision fetches the current revision from the server.
func (hr *HTTPReplicator) fetchRevision(ctx context.Context, serverURL string) (*IndexRevision, error) {
	revisionURL := serverURL + "/revision"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, revisionURL, nil)
	if err != nil {
		return nil, err
	}

	// Add custom headers
	hr.mu.RLock()
	for key, value := range hr.headers {
		req.Header.Set(key, value)
	}
	hr.mu.RUnlock()

	resp, err := hr.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// For now, return a mock revision
	// In real implementation, parse JSON/XML response
	return &IndexRevision{
		Generation: 1,
		Version:    1,
		Files:      []string{},
	}, nil
}

// buildFileURL builds the URL for a specific file.
func (hr *HTTPReplicator) buildFileURL(serverURL, filename string) (string, error) {
	base, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}

	base.Path = filepath.Join(base.Path, "files", filename)
	return base.String(), nil
}

// downloadFile downloads a file from the given URL.
func (hr *HTTPReplicator) downloadFile(ctx context.Context, fileURL, targetPath string) (int64, error) {
	// Ensure target directory exists
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return 0, err
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return 0, err
	}

	// Add custom headers
	hr.mu.RLock()
	for key, value := range hr.headers {
		req.Header.Set(key, value)
	}
	hr.mu.RUnlock()

	// Execute request
	resp, err := hr.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Create target file
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return 0, err
	}
	defer targetFile.Close()

	// Copy content
	bytes, err := io.Copy(targetFile, resp.Body)
	if err != nil {
		return 0, err
	}

	return bytes, nil
}

// CheckNow checks if replication is needed and performs it.
func (hr *HTTPReplicator) CheckNow(ctx context.Context) error {
	hr.mu.RLock()
	if !hr.isOpen.Load() {
		hr.mu.RUnlock()
		return fmt.Errorf("HTTP replicator is closed")
	}
	hr.mu.RUnlock()

	return hr.base.CheckNow(ctx)
}

// Start starts the replicator.
func (hr *HTTPReplicator) Start() error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if !hr.isOpen.Load() {
		return fmt.Errorf("HTTP replicator is closed")
	}

	return hr.base.Start()
}

// Stop stops the replicator.
func (hr *HTTPReplicator) Stop() error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	return hr.base.Stop()
}

// IsRunning returns true if the replicator is running.
func (hr *HTTPReplicator) IsRunning() bool {
	return hr.base.IsRunning()
}

// GetLastException returns the last exception that occurred.
func (hr *HTTPReplicator) GetLastException() error {
	return hr.base.GetLastException()
}

// GetServerURL returns the server URL.
func (hr *HTTPReplicator) GetServerURL() string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	return hr.serverURL
}

// GetTargetPath returns the target path.
func (hr *HTTPReplicator) GetTargetPath() string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	return hr.targetPath
}

// SetServerURL sets the server URL.
func (hr *HTTPReplicator) SetServerURL(serverURL string) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if !hr.isOpen.Load() {
		return fmt.Errorf("HTTP replicator is closed")
	}

	if serverURL == "" {
		return fmt.Errorf("server URL cannot be empty")
	}

	// Validate URL
	_, err := url.Parse(serverURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}

	hr.serverURL = serverURL
	return nil
}

// SetTargetPath sets the target path.
func (hr *HTTPReplicator) SetTargetPath(targetPath string) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if !hr.isOpen.Load() {
		return fmt.Errorf("HTTP replicator is closed")
	}

	if targetPath == "" {
		return fmt.Errorf("target path cannot be empty")
	}

	hr.targetPath = targetPath
	return nil
}

// SetHeader sets a custom HTTP header.
func (hr *HTTPReplicator) SetHeader(key, value string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	hr.headers[key] = value
}

// GetHeader returns a custom HTTP header value.
func (hr *HTTPReplicator) GetHeader(key string) string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	return hr.headers[key]
}

// SetTimeout sets the HTTP client timeout.
func (hr *HTTPReplicator) SetTimeout(timeout time.Duration) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	hr.httpClient.Timeout = timeout
}

// GetTimeout returns the HTTP client timeout.
func (hr *HTTPReplicator) GetTimeout() time.Duration {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	return hr.httpClient.Timeout
}

// GetStats returns replication statistics.
func (hr *HTTPReplicator) GetStats() ReplicatorStats {
	return hr.base.GetStats()
}

// ResetStats resets replication statistics.
func (hr *HTTPReplicator) ResetStats() {
	hr.base.ResetStats()
}

// GetCurrentRevision returns the current replicated revision.
func (hr *HTTPReplicator) GetCurrentRevision() *IndexRevision {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if hr.currentRevision == nil {
		return nil
	}

	return hr.currentRevision.Clone()
}

// Close closes the HTTPReplicator.
func (hr *HTTPReplicator) Close() error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if !hr.isOpen.Load() {
		return nil
	}

	hr.isOpen.Store(false)
	hr.base.Stop()

	return nil
}

// IsOpen returns true if the replicator is open.
func (hr *HTTPReplicator) IsOpen() bool {
	return hr.isOpen.Load()
}

// String returns a string representation of the HTTPReplicator.
func (hr *HTTPReplicator) String() string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	return fmt.Sprintf("HTTPReplicator{open=%v, server=%s, target=%s}",
		hr.isOpen.Load(), hr.serverURL, hr.targetPath)
}
