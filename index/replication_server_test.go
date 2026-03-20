package index

import (
	"context"
	"testing"
	"time"
)

func TestNewReplicationServer(t *testing.T) {
	address := "0.0.0.0"
	port := 8080
	indexPath := "/tmp/index"

	rs, err := NewReplicationServer(address, port, indexPath)
	if err != nil {
		t.Fatalf("failed to create ReplicationServer: %v", err)
	}

	if rs == nil {
		t.Fatal("expected ReplicationServer to not be nil")
	}

	if rs.GetAddress() != address {
		t.Errorf("expected address %s, got %s", address, rs.GetAddress())
	}

	if rs.GetPort() != port {
		t.Errorf("expected port %d, got %d", port, rs.GetPort())
	}

	if rs.GetIndexPath() != indexPath {
		t.Errorf("expected index path %s, got %s", indexPath, rs.GetIndexPath())
	}

	if rs.IsRunning() {
		t.Error("expected server to not be running initially")
	}
}

func TestNewReplicationServer_EmptyAddress(t *testing.T) {
	_, err := NewReplicationServer("", 8080, "/tmp/index")
	if err == nil {
		t.Error("expected error for empty address")
	}
}

func TestNewReplicationServer_InvalidPort(t *testing.T) {
	_, err := NewReplicationServer("0.0.0.0", 0, "/tmp/index")
	if err == nil {
		t.Error("expected error for invalid port")
	}

	_, err = NewReplicationServer("0.0.0.0", -1, "/tmp/index")
	if err == nil {
		t.Error("expected error for negative port")
	}
}

func TestNewReplicationServer_EmptyIndexPath(t *testing.T) {
	_, err := NewReplicationServer("0.0.0.0", 8080, "")
	if err == nil {
		t.Error("expected error for empty index path")
	}
}

func TestReplicationServer_StartStop(t *testing.T) {
	address := "127.0.0.1"
	port := 18080 // Use different port to avoid conflicts
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)

	// Start
	err := rs.Start()
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if !rs.IsRunning() {
		t.Error("expected server to be running")
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Stop
	ctx := context.Background()
	err = rs.Stop(ctx)
	if err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	if rs.IsRunning() {
		t.Error("expected server to be stopped")
	}
}

func TestReplicationServer_Start_AlreadyRunning(t *testing.T) {
	address := "127.0.0.1"
	port := 18081
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)

	rs.Start()
	defer rs.Stop(context.Background())

	time.Sleep(50 * time.Millisecond)

	err := rs.Start()
	if err == nil {
		t.Error("expected error when starting already running server")
	}
}

func TestReplicationServer_SetIndexPath(t *testing.T) {
	address := "0.0.0.0"
	port := 8080
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)

	newPath := "/tmp/newindex"
	err := rs.SetIndexPath(newPath)
	if err != nil {
		t.Fatalf("failed to set index path: %v", err)
	}

	if rs.GetIndexPath() != newPath {
		t.Errorf("expected index path %s, got %s", newPath, rs.GetIndexPath())
	}
}

func TestReplicationServer_SetIndexPath_Empty(t *testing.T) {
	address := "0.0.0.0"
	port := 8080
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)

	err := rs.SetIndexPath("")
	if err == nil {
		t.Error("expected error for empty index path")
	}
}

func TestReplicationServer_SetIndexPath_Running(t *testing.T) {
	address := "127.0.0.1"
	port := 18082
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)
	rs.Start()
	defer rs.Stop(context.Background())

	time.Sleep(50 * time.Millisecond)

	err := rs.SetIndexPath("/tmp/newindex")
	if err == nil {
		t.Error("expected error when setting index path while running")
	}
}

func TestReplicationServer_GetCurrentRevision(t *testing.T) {
	address := "0.0.0.0"
	port := 8080
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)

	// Initially empty revision
	revision := rs.GetCurrentRevision()
	if revision == nil {
		t.Fatal("expected revision to not be nil")
	}
}

func TestReplicationServer_SetCurrentRevision(t *testing.T) {
	address := "0.0.0.0"
	port := 8080
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)

	revision := &IndexRevision{
		Generation: 1,
		Version:    1,
		Files:      []string{"file1.txt"},
	}

	err := rs.SetCurrentRevision(revision)
	if err != nil {
		t.Fatalf("failed to set current revision: %v", err)
	}

	current := rs.GetCurrentRevision()
	if current == nil {
		t.Fatal("expected current revision to be set")
	}

	if current.Generation != 1 {
		t.Errorf("expected generation 1, got %d", current.Generation)
	}
}

func TestReplicationServer_SetCurrentRevision_Nil(t *testing.T) {
	address := "0.0.0.0"
	port := 8080
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)

	err := rs.SetCurrentRevision(nil)
	if err == nil {
		t.Error("expected error for nil revision")
	}
}

func TestReplicationServer_CreateSession(t *testing.T) {
	address := "127.0.0.1"
	port := 18083
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)
	rs.Start()
	defer rs.Stop(context.Background())

	time.Sleep(50 * time.Millisecond)

	session, err := rs.CreateSession("")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if session == nil {
		t.Fatal("expected session to not be nil")
	}

	if session.ID == "" {
		t.Error("expected session ID to be set")
	}

	if rs.GetSessionCount() != 1 {
		t.Errorf("expected 1 session, got %d", rs.GetSessionCount())
	}
}

func TestReplicationServer_CreateSession_NotRunning(t *testing.T) {
	address := "0.0.0.0"
	port := 8080
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)

	_, err := rs.CreateSession("")
	if err == nil {
		t.Error("expected error when creating session on stopped server")
	}
}

func TestReplicationServer_GetSession(t *testing.T) {
	address := "127.0.0.1"
	port := 18084
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)
	rs.Start()
	defer rs.Stop(context.Background())

	time.Sleep(50 * time.Millisecond)

	session, _ := rs.CreateSession("")

	retrieved, err := rs.GetSession(session.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if retrieved.ID != session.ID {
		t.Error("expected retrieved session to have same ID")
	}
}

func TestReplicationServer_GetSession_NotFound(t *testing.T) {
	address := "127.0.0.1"
	port := 18085
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)
	rs.Start()
	defer rs.Stop(context.Background())

	time.Sleep(50 * time.Millisecond)

	_, err := rs.GetSession("non-existent-session")
	if err == nil {
		t.Error("expected error for non-existent session")
	}
}

func TestReplicationServer_CloseSession(t *testing.T) {
	address := "127.0.0.1"
	port := 18086
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)
	rs.Start()
	defer rs.Stop(context.Background())

	time.Sleep(50 * time.Millisecond)

	session, _ := rs.CreateSession("")

	err := rs.CloseSession(session.ID)
	if err != nil {
		t.Fatalf("failed to close session: %v", err)
	}

	if rs.GetSessionCount() != 0 {
		t.Errorf("expected 0 sessions, got %d", rs.GetSessionCount())
	}
}

func TestReplicationServer_CleanupSessions(t *testing.T) {
	address := "127.0.0.1"
	port := 18087
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)
	rs.Start()
	defer rs.Stop(context.Background())

	time.Sleep(50 * time.Millisecond)

	// Create a session
	session, _ := rs.CreateSession("")

	// Manually expire the session
	rs.mu.Lock()
	session.ExpiresAt = time.Now().Add(-1 * time.Hour)
	rs.mu.Unlock()

	// Cleanup should remove expired sessions
	rs.CleanupSessions()

	if rs.GetSessionCount() != 0 {
		t.Errorf("expected 0 sessions after cleanup, got %d", rs.GetSessionCount())
	}
}

func TestReplicationServer_String(t *testing.T) {
	address := "0.0.0.0"
	port := 8080
	indexPath := "/tmp/index"

	rs, _ := NewReplicationServer(address, port, indexPath)

	str := rs.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}
