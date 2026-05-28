// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
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

// TestFilesystemResourceLoader_Traversal verifies that path-traversal resource
// names (absolute paths and ".." escapes) are rejected with
// ErrUnsafeResourceName and are never forwarded to the delegate.
func TestFilesystemResourceLoader_Traversal(t *testing.T) {
	dir := t.TempDir()

	// Place a sentinel file outside the base directory to prove that a
	// successful traversal would have leaked it.
	outerDir := t.TempDir()
	secret := filepath.Join(outerDir, "secret.txt")
	if err := os.WriteFile(secret, []byte("top secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	// A delegate that must never be reached for an unsafe name.
	delegate := &recordingLoader{}

	loader, err := NewFilesystemResourceLoader(dir, delegate)
	if err != nil {
		t.Fatal(err)
	}

	malicious := []string{
		"../secret.txt",
		"../../secret.txt",
		"subdir/../../secret.txt",
		"..",
		"../",
		filepath.Join("..", filepath.Base(outerDir), "secret.txt"),
		secret, // absolute path to the sentinel
		"/etc/passwd",
	}

	for _, name := range malicious {
		t.Run(name, func(t *testing.T) {
			rc, err := loader.OpenResource(name)
			if err == nil {
				rc.Close()
				t.Fatalf("OpenResource(%q): expected rejection, got nil error", name)
			}
			if !errors.Is(err, ErrUnsafeResourceName) {
				t.Fatalf("OpenResource(%q): got error %v, want ErrUnsafeResourceName", name, err)
			}
			if delegate.calls != 0 {
				t.Fatalf("OpenResource(%q): delegate was called for an unsafe name", name)
			}
		})
	}
}

// TestFilesystemResourceLoader_Subdirectory verifies that legitimate names
// pointing at a nested file inside the base directory still resolve.
func TestFilesystemResourceLoader_Subdirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "nested")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	content := "nested resource"
	if err := os.WriteFile(filepath.Join(sub, "child.txt"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	loader, err := NewFilesystemResourceLoader(dir, nil)
	if err != nil {
		t.Fatal(err)
	}

	rc, err := loader.OpenResource(filepath.Join("nested", "child.txt"))
	if err != nil {
		t.Fatalf("OpenResource on nested file: %v", err)
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

// recordingLoader is a ResourceLoader that counts how many times it is invoked,
// used to assert the delegate is never reached for rejected names.
type recordingLoader struct {
	calls int
}

func (r *recordingLoader) OpenResource(string) (io.ReadCloser, error) {
	r.calls++
	return nil, errors.New("recordingLoader: should not be called")
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
