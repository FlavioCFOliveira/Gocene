// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResourceAsStream_EmptyName(t *testing.T) {
	_, err := ResourceAsStream("")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestResourceAsStream_FileNotFound(t *testing.T) {
	_, err := ResourceAsStream("/nonexistent/path/to/file.txt")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestResourceAsStream_ExistingFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(tmpFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Read the file
	rc, err := ResourceAsStream(tmpFile)
	if err != nil {
		t.Fatalf("failed to open resource: %v", err)
	}
	defer rc.Close()

	// Read contents
	buf := make([]byte, 11)
	n, err := rc.Read(buf)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}

	if n != 11 {
		t.Errorf("expected 11 bytes, got %d", n)
	}

	if string(buf) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(buf))
	}
}

func TestResourceAsBytes(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(tmpFile, []byte("test data"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Read as bytes
	bytes, err := ResourceAsBytes(tmpFile)
	if err != nil {
		t.Fatalf("failed to read resource: %v", err)
	}

	if string(bytes) != "test data" {
		t.Errorf("expected 'test data', got %q", string(bytes))
	}
}

func TestResourceAsString(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	if err := os.WriteFile(tmpFile, []byte("string content"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Read as string
	str, err := ResourceAsString(tmpFile)
	if err != nil {
		t.Fatalf("failed to read resource: %v", err)
	}

	if str != "string content" {
		t.Errorf("expected 'string content', got %q", str)
	}
}

func TestResourceExists(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "exists.txt")

	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Check existing file
	if !ResourceExists(tmpFile) {
		t.Error("expected ResourceExists to return true for existing file")
	}

	// Check non-existing file
	if ResourceExists("/nonexistent/file.txt") {
		t.Error("expected ResourceExists to return false for non-existing file")
	}
}

func TestListResources(t *testing.T) {
	// Create a temporary directory with files
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("1"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("2"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// List resources
	resources, err := ListResources(tmpDir, nil)
	if err != nil {
		t.Fatalf("failed to list resources: %v", err)
	}

	if len(resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(resources))
	}
}

func TestListResources_NonExistent(t *testing.T) {
	_, err := ListResources("/nonexistent/directory", nil)
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}
