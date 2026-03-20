package index

import (
	"context"
	"testing"
	"time"
)

func TestNewHTTPReplicator(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, err := NewHTTPReplicator(serverURL, targetPath)
	if err != nil {
		t.Fatalf("failed to create HTTPReplicator: %v", err)
	}
	defer hr.Close()

	if hr == nil {
		t.Fatal("expected HTTPReplicator to not be nil")
	}

	if hr.GetServerURL() != serverURL {
		t.Errorf("expected server URL %s, got %s", serverURL, hr.GetServerURL())
	}

	if hr.GetTargetPath() != targetPath {
		t.Errorf("expected target path %s, got %s", targetPath, hr.GetTargetPath())
	}

	if !hr.IsOpen() {
		t.Error("expected replicator to be open")
	}

	if hr.GetTimeout() != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", hr.GetTimeout())
	}
}

func TestNewHTTPReplicator_EmptyServerURL(t *testing.T) {
	_, err := NewHTTPReplicator("", "/tmp/target")
	if err == nil {
		t.Error("expected error for empty server URL")
	}
}

func TestNewHTTPReplicator_EmptyTargetPath(t *testing.T) {
	_, err := NewHTTPReplicator("http://localhost:8080", "")
	if err == nil {
		t.Error("expected error for empty target path")
	}
}

func TestNewHTTPReplicator_InvalidURL(t *testing.T) {
	_, err := NewHTTPReplicator("://invalid-url", "/tmp/target")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestHTTPReplicator_StartStop(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	defer hr.Close()

	// Start
	err := hr.Start()
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if !hr.IsRunning() {
		t.Error("expected replicator to be running")
	}

	// Stop
	err = hr.Stop()
	if err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	if hr.IsRunning() {
		t.Error("expected replicator to be stopped")
	}
}

func TestHTTPReplicator_SetServerURL(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	defer hr.Close()

	newURL := "http://localhost:9090"
	err := hr.SetServerURL(newURL)
	if err != nil {
		t.Fatalf("failed to set server URL: %v", err)
	}

	if hr.GetServerURL() != newURL {
		t.Errorf("expected server URL %s, got %s", newURL, hr.GetServerURL())
	}
}

func TestHTTPReplicator_SetServerURL_Empty(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	defer hr.Close()

	err := hr.SetServerURL("")
	if err == nil {
		t.Error("expected error for empty server URL")
	}
}

func TestHTTPReplicator_SetServerURL_Invalid(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	defer hr.Close()

	err := hr.SetServerURL("://invalid")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestHTTPReplicator_SetServerURL_Closed(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	hr.Close()

	err := hr.SetServerURL("http://localhost:9090")
	if err == nil {
		t.Error("expected error when setting URL on closed replicator")
	}
}

func TestHTTPReplicator_SetTargetPath(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	defer hr.Close()

	newPath := "/tmp/newtarget"
	err := hr.SetTargetPath(newPath)
	if err != nil {
		t.Fatalf("failed to set target path: %v", err)
	}

	if hr.GetTargetPath() != newPath {
		t.Errorf("expected target path %s, got %s", newPath, hr.GetTargetPath())
	}
}

func TestHTTPReplicator_SetTargetPath_Empty(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	defer hr.Close()

	err := hr.SetTargetPath("")
	if err == nil {
		t.Error("expected error for empty target path")
	}
}

func TestHTTPReplicator_SetTargetPath_Closed(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	hr.Close()

	err := hr.SetTargetPath("/tmp/newtarget")
	if err == nil {
		t.Error("expected error when setting path on closed replicator")
	}
}

func TestHTTPReplicator_SetHeader(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	defer hr.Close()

	hr.SetHeader("Authorization", "Bearer token123")

	if hr.GetHeader("Authorization") != "Bearer token123" {
		t.Error("expected header to be set")
	}

	// Non-existent header
	if hr.GetHeader("NonExistent") != "" {
		t.Error("expected empty value for non-existent header")
	}
}

func TestHTTPReplicator_SetTimeout(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	defer hr.Close()

	hr.SetTimeout(60 * time.Second)

	if hr.GetTimeout() != 60*time.Second {
		t.Errorf("expected timeout 60s, got %v", hr.GetTimeout())
	}
}

func TestHTTPReplicator_GetStats(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	defer hr.Close()

	stats := hr.GetStats()
	if stats.TotalChecks != 0 {
		t.Errorf("expected 0 checks initially, got %d", stats.TotalChecks)
	}
}

func TestHTTPReplicator_ResetStats(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	defer hr.Close()

	hr.ResetStats()

	stats := hr.GetStats()
	if stats.TotalChecks != 0 {
		t.Errorf("expected 0 checks after reset, got %d", stats.TotalChecks)
	}
}

func TestHTTPReplicator_GetCurrentRevision(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	defer hr.Close()

	// Initially no revision
	revision := hr.GetCurrentRevision()
	if revision != nil {
		t.Error("expected nil revision initially")
	}
}

func TestHTTPReplicator_Close(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)

	err := hr.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if hr.IsOpen() {
		t.Error("expected replicator to be closed")
	}

	// Close again should not error
	err = hr.Close()
	if err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

func TestHTTPReplicator_String(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	defer hr.Close()

	str := hr.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}

func TestHTTPReplicator_CheckNow_Closed(t *testing.T) {
	serverURL := "http://localhost:8080"
	targetPath := "/tmp/target"

	hr, _ := NewHTTPReplicator(serverURL, targetPath)
	hr.Close()

	ctx := context.Background()
	err := hr.CheckNow(ctx)
	if err == nil {
		t.Error("expected error when checking closed replicator")
	}
}
