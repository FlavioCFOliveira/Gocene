// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"testing"
)

// TestLucene50StoredFieldsFormatMergeInstance_Constructor verifies that
// the stored fields format struct is constructed correctly.
func TestLucene50StoredFieldsFormatMergeInstance_Constructor(t *testing.T) {
	f := NewLucene50StoredFieldsFormat("merge")
	if f == nil {
		t.Fatal("NewLucene50StoredFieldsFormat returned nil")
	}
	if f.Name != "Lucene50StoredFieldsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene50StoredFieldsFormat")
	}
	if f.Version != "merge" {
		t.Errorf("Version: got %q, want %q", f.Version, "merge")
	}
}

// TestLucene50StoredFieldsFormatMergeInstance_LiveDocs verifies the live
// docs format constructor.
func TestLucene50StoredFieldsFormatMergeInstance_LiveDocs(t *testing.T) {
	f := NewLucene50LiveDocsFormat("merge")
	if f.Name != "Lucene50LiveDocsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene50LiveDocsFormat")
	}
	if f.Version != "merge" {
		t.Errorf("Version: got %q, want %q", f.Version, "merge")
	}
}
