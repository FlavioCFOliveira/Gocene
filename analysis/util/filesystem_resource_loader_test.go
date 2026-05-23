// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestFilesystemResourceLoader_OpenResource verifies that files in the base
// directory can be opened.
func TestFilesystemResourceLoader_OpenResource(t *testing.T) {
	dir := t.TempDir()
	content := "hello resource"
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	loader, err := NewFilesystemResourceLoader(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	rc, err := loader.OpenResource("test.txt")
	if err != nil {
		t.Fatalf("OpenResource: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("got %q, want %q", string(data), content)
	}
}

// TestFilesystemResourceLoader_Delegate verifies fallback to delegate.
func TestFilesystemResourceLoader_Delegate(t *testing.T) {
	dir := t.TempDir()

	delegateContent := "from delegate"
	delegateDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(delegateDir, "fallback.txt"), []byte(delegateContent), 0o600); err != nil {
		t.Fatal(err)
	}
	delegate, err := NewFilesystemResourceLoader(delegateDir, nil)
	if err != nil {
		t.Fatal(err)
	}

	loader, err := NewFilesystemResourceLoader(dir, delegate)
	if err != nil {
		t.Fatal(err)
	}

	rc, err := loader.OpenResource("fallback.txt")
	if err != nil {
		t.Fatalf("OpenResource via delegate: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != delegateContent {
		t.Errorf("got %q, want %q", string(data), delegateContent)
	}
}

// TestFilesystemResourceLoader_NotDirectory verifies that a non-directory path
// is rejected.
func TestFilesystemResourceLoader_NotDirectory(t *testing.T) {
	f, err := os.CreateTemp("", "frl-test-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	_, err = NewFilesystemResourceLoader(f.Name(), nil)
	if err == nil {
		t.Error("expected error for non-directory path, got nil")
	}
}
