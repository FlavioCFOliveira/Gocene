package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewLocalReplicator(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	lr, err := NewLocalReplicator(sourceDir, targetDir)
	if err != nil {
		t.Fatalf("failed to create LocalReplicator: %v", err)
	}
	defer lr.Close()

	if lr == nil {
		t.Fatal("expected LocalReplicator to not be nil")
	}

	if lr.GetSourcePath() != sourceDir {
		t.Errorf("expected source %s, got %s", sourceDir, lr.GetSourcePath())
	}

	if lr.GetTargetPath() != targetDir {
		t.Errorf("expected target %s, got %s", targetDir, lr.GetTargetPath())
	}

	if !lr.IsOpen() {
		t.Error("expected replicator to be open")
	}
}

func TestNewLocalReplicator_EmptySourcePath(t *testing.T) {
	_, err := NewLocalReplicator("", "/tmp/target")
	if err == nil {
		t.Error("expected error for empty source path")
	}
}

func TestNewLocalReplicator_EmptyTargetPath(t *testing.T) {
	_, err := NewLocalReplicator("/tmp/source", "")
	if err == nil {
		t.Error("expected error for empty target path")
	}
}

func TestLocalReplicator_StartStop(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	defer lr.Close()

	// Start
	err := lr.Start()
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if !lr.IsRunning() {
		t.Error("expected replicator to be running")
	}

	// Stop
	err = lr.Stop()
	if err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	if lr.IsRunning() {
		t.Error("expected replicator to be stopped")
	}
}

func TestLocalReplicator_CheckNow(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	// Create a test file in source
	testFile := filepath.Join(sourceDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	defer lr.Close()

	lr.Start()
	defer lr.Stop()

	ctx := context.Background()
	err := lr.CheckNow(ctx)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}

	// Verify file was copied
	targetFile := filepath.Join(targetDir, "test.txt")
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Error("expected file to be copied to target")
	}

	// Verify stats
	stats := lr.GetStats()
	if stats.TotalChecks != 1 {
		t.Errorf("expected 1 check, got %d", stats.TotalChecks)
	}
}

func TestLocalReplicator_CheckNow_Closed(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	lr.Close()

	ctx := context.Background()
	err := lr.CheckNow(ctx)
	if err == nil {
		t.Error("expected error when checking closed replicator")
	}
}

func TestLocalReplicator_SetSourcePath(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	newSourceDir := t.TempDir()

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	defer lr.Close()

	err := lr.SetSourcePath(newSourceDir)
	if err != nil {
		t.Fatalf("failed to set source path: %v", err)
	}

	if lr.GetSourcePath() != newSourceDir {
		t.Errorf("expected source %s, got %s", newSourceDir, lr.GetSourcePath())
	}
}

func TestLocalReplicator_SetSourcePath_Empty(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	defer lr.Close()

	err := lr.SetSourcePath("")
	if err == nil {
		t.Error("expected error for empty source path")
	}
}

func TestLocalReplicator_SetSourcePath_Closed(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	lr.Close()

	err := lr.SetSourcePath("/new/source")
	if err == nil {
		t.Error("expected error when setting source path on closed replicator")
	}
}

func TestLocalReplicator_SetTargetPath(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	newTargetDir := t.TempDir()

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	defer lr.Close()

	err := lr.SetTargetPath(newTargetDir)
	if err != nil {
		t.Fatalf("failed to set target path: %v", err)
	}

	if lr.GetTargetPath() != newTargetDir {
		t.Errorf("expected target %s, got %s", newTargetDir, lr.GetTargetPath())
	}
}

func TestLocalReplicator_SetTargetPath_Empty(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	defer lr.Close()

	err := lr.SetTargetPath("")
	if err == nil {
		t.Error("expected error for empty target path")
	}
}

func TestLocalReplicator_SetTargetPath_Closed(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	lr.Close()

	err := lr.SetTargetPath("/new/target")
	if err == nil {
		t.Error("expected error when setting target path on closed replicator")
	}
}

func TestLocalReplicator_GetStats(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	defer lr.Close()

	stats := lr.GetStats()
	if stats.TotalChecks != 0 {
		t.Errorf("expected 0 checks initially, got %d", stats.TotalChecks)
	}
}

func TestLocalReplicator_ResetStats(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(sourceDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	defer lr.Close()

	lr.Start()
	ctx := context.Background()
	lr.CheckNow(ctx)
	lr.Stop()

	// Reset stats
	lr.ResetStats()

	stats := lr.GetStats()
	if stats.TotalChecks != 0 {
		t.Errorf("expected 0 checks after reset, got %d", stats.TotalChecks)
	}
}

func TestLocalReplicator_GetCurrentRevision(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	// Create a test file
	testFile := filepath.Join(sourceDir, "test.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	defer lr.Close()

	// Initially no revision
	revision := lr.GetCurrentRevision()
	if revision != nil {
		t.Error("expected nil revision initially")
	}

	lr.Start()
	ctx := context.Background()
	lr.CheckNow(ctx)
	lr.Stop()

	// Should have revision after replication
	revision = lr.GetCurrentRevision()
	if revision == nil {
		t.Fatal("expected revision after replication")
	}

	if revision.Generation != 1 {
		t.Errorf("expected generation 1, got %d", revision.Generation)
	}

	if len(revision.Files) != 1 {
		t.Errorf("expected 1 file, got %d", len(revision.Files))
	}
}

func TestLocalReplicator_Close(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	lr, _ := NewLocalReplicator(sourceDir, targetDir)

	err := lr.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	if lr.IsOpen() {
		t.Error("expected replicator to be closed")
	}

	// Close again should not error
	err = lr.Close()
	if err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

func TestLocalReplicator_String(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	defer lr.Close()

	str := lr.String()
	if str == "" {
		t.Error("expected non-empty string")
	}
}

func TestLocalReplicator_SubdirectoryReplication(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()

	// Create subdirectory structure
	subDir := filepath.Join(sourceDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Create files in subdirectory
	testFile := filepath.Join(subDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	lr, _ := NewLocalReplicator(sourceDir, targetDir)
	defer lr.Close()

	lr.Start()
	ctx := context.Background()
	err := lr.CheckNow(ctx)
	lr.Stop()

	if err != nil {
		t.Fatalf("check failed: %v", err)
	}

	// Verify subdirectory was created and file was copied
	targetSubDir := filepath.Join(targetDir, "subdir")
	targetFile := filepath.Join(targetSubDir, "test.txt")

	if _, err := os.Stat(targetSubDir); os.IsNotExist(err) {
		t.Error("expected subdirectory to be created in target")
	}

	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Error("expected file to be copied to subdirectory")
	}
}
