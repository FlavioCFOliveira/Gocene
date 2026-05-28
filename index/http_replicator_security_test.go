// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"strings"
	"testing"
)

// TestIsSafeReplicaFileName is the rmp #4720 regression test for the
// HTTPReplicator filename guard: a server-supplied file name must be a plain
// local base name, never a path-traversal vector.
func TestIsSafeReplicaFileName(t *testing.T) {
	safe := []string{"_0.cfs", "_12.si", "segments_3", "pending_segments_4"}
	for _, n := range safe {
		if !isSafeReplicaFileName(n) {
			t.Errorf("isSafeReplicaFileName(%q) = false, want true", n)
		}
	}
	unsafe := []string{
		"", ".", "..", "../evil", "../../etc/passwd",
		"sub/_0.cfs", "/abs/_0.cfs", `..\evil`, "a/../../b",
	}
	for _, n := range unsafe {
		if isSafeReplicaFileName(n) {
			t.Errorf("isSafeReplicaFileName(%q) = true, want false (traversal must be rejected)", n)
		}
	}
}

// TestBuildFileURLForwardSlashes is the rmp #4720 regression test for
// buildFileURL using path.Join (URL semantics) instead of filepath.Join, so the
// URL is well-formed on Windows (no backslashes) as well as Unix.
func TestBuildFileURLForwardSlashes(t *testing.T) {
	hr, err := NewHTTPReplicator("http://example.com/base", t.TempDir())
	if err != nil {
		t.Fatalf("NewHTTPReplicator: %v", err)
	}

	got, err := hr.buildFileURL("http://example.com/base", "_0.cfs")
	if err != nil {
		t.Fatalf("buildFileURL: %v", err)
	}
	want := "http://example.com/base/files/_0.cfs"
	if got != want {
		t.Errorf("buildFileURL = %q, want %q", got, want)
	}
	if strings.Contains(got, `\`) {
		t.Errorf("buildFileURL produced a backslash in the URL: %q", got)
	}
}
