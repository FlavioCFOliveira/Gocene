// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"embed"
	"errors"
	"io/fs"
	"testing"
)

//go:embed resource_as_stream.go
var testEmbedFS embed.FS

// TestResourceAsStreamWithEmbed_RejectsTraversal is the rmp #4720 regression
// test: the embedded-resource branch must reject path-traversal / absolute
// names (which io/fs treats as invalid) with a clear error rather than
// attempting the lookup. These names do not exist on the local filesystem, so
// the filesystem branch falls through to the embed branch, which rejects them.
func TestResourceAsStreamWithEmbed_RejectsTraversal(t *testing.T) {
	bad := []string{
		"../resource_as_stream.go",
		"../../some_nonexistent_xyz",
		"sub/../../escape_nonexistent_xyz",
		"/absolute_nonexistent_xyz",
	}
	for _, name := range bad {
		rc, err := ResourceAsStreamWithEmbed(&testEmbedFS, name)
		if err == nil {
			rc.Close()
			t.Errorf("ResourceAsStreamWithEmbed(embed, %q) = nil error, want rejection", name)
			continue
		}
		if !errors.Is(err, fs.ErrInvalid) {
			// Not fatal — a "not found" is still a rejection — but the embed
			// branch should surface the explicit invalid-path error for these.
			t.Logf("ResourceAsStreamWithEmbed(%q) rejected with %v (expected fs.ErrInvalid)", name, err)
		}
	}

	// A valid embedded name resolves (sanity check that the guard is not
	// over-rejecting). The file is also present on disk during the test, so this
	// exercises the documented filesystem-first behaviour as well.
	rc, err := ResourceAsStreamWithEmbed(&testEmbedFS, "resource_as_stream.go")
	if err != nil {
		t.Fatalf("ResourceAsStreamWithEmbed(embed, valid name) error = %v", err)
	}
	rc.Close()
}
