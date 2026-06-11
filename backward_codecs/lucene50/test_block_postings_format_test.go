// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"testing"
)

// TestBlockPostingsFormat_Constants verifies the Lucene50 postings format
// constants used by block-based postings.
func TestBlockPostingsFormat_Constants(t *testing.T) {
	if BlockSize != 128 {
		t.Errorf("BlockSize: got %d, want 128", BlockSize)
	}
	if VersionStart != 0 {
		t.Errorf("VersionStart: got %d, want 0", VersionStart)
	}
	if VersionCurrent != 1 {
		t.Errorf("VersionCurrent: got %d, want 1", VersionCurrent)
	}
}

// TestBlockPostingsFormat_FormatConstructors verifies that the format types
// carry the expected Name and Version values.
func TestBlockPostingsFormat_FormatConstructors(t *testing.T) {
	pf := NewLucene50PostingsFormat("test")
	if pf.Name != "Lucene50PostingsFormat" {
		t.Errorf("Name: got %q, want %q", pf.Name, "Lucene50PostingsFormat")
	}
	pr := NewLucene50PostingsReader("test")
	if pr.Name != "Lucene50PostingsReader" {
		t.Errorf("Name: got %q, want %q", pr.Name, "Lucene50PostingsReader")
	}
}

// TestBlockPostingsFormat_Trim verifies the trim helper used by skip writers.
func TestBlockPostingsFormat_Trim(t *testing.T) {
	if got := trim(128); got != 127 {
		t.Errorf("trim(128): got %d, want 127", got)
	}
	if got := trim(100); got != 100 {
		t.Errorf("trim(100): got %d, want 100", got)
	}
}
