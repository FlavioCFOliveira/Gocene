// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"testing"
)

// TestBlockPostingsFormat3_VersionConstants verifies the version constants.
func TestBlockPostingsFormat3_VersionConstants(t *testing.T) {
	if VersionStart != 0 {
		t.Errorf("VersionStart: got %d, want 0", VersionStart)
	}
	if VersionImpactSkipData != 1 {
		t.Errorf("VersionImpactSkipData: got %d, want 1", VersionImpactSkipData)
	}
	if VersionCurrent != VersionImpactSkipData {
		t.Errorf("VersionCurrent != VersionImpactSkipData")
	}
	if BlockSize != 128 {
		t.Errorf("BlockSize: got %d, want 128", BlockSize)
	}
}

// TestBlockPostingsFormat3_LiveDocsFormat verifies the
// Lucene50LiveDocsFormat constructor.
func TestBlockPostingsFormat3_LiveDocsFormat(t *testing.T) {
	f := NewLucene50LiveDocsFormat("v1")
	if f.Name != "Lucene50LiveDocsFormat" {
		t.Errorf("Name: got %q, want %q", f.Name, "Lucene50LiveDocsFormat")
	}
	if f.Version != "v1" {
		t.Errorf("Version: got %q, want %q", f.Version, "v1")
	}
}

// TestBlockPostingsFormat3_StoredFieldsFormat verifies the stored fields
// format constructor.
func TestBlockPostingsFormat3_StoredFieldsFormat(t *testing.T) {
	f := NewLucene50StoredFieldsFormat("v2")
	if f.Name != "Lucene50StoredFieldsFormat" {
		t.Errorf("Name: got %q", f.Name)
	}
	if f.Version != "v2" {
		t.Errorf("Version: got %q, want %q", f.Version, "v2")
	}
}
