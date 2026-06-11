// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"testing"
)

// TestLucene50PostingsWriter_Constants verifies the exported format constants.
func TestLucene50PostingsWriter_Constants(t *testing.T) {
	if BlockSize != 128 {
		t.Errorf("BlockSize: got %d, want 128", BlockSize)
	}
	if VersionStart != 0 {
		t.Errorf("VersionStart: got %d, want 0", VersionStart)
	}
	if VersionImpactSkipData != 1 {
		t.Errorf("VersionImpactSkipData: got %d, want 1", VersionImpactSkipData)
	}
	if VersionCurrent != VersionImpactSkipData {
		t.Errorf("VersionCurrent: got %d, want VersionImpactSkipData", VersionCurrent)
	}
}

// TestLucene50PostingsWriter_FormatConstructors verifies that the format
// structs are constructed with the expected Name and Version.
func TestLucene50PostingsWriter_FormatConstructors(t *testing.T) {
	pf := NewLucene50PostingsFormat("v1")
	if pf.Name != "Lucene50PostingsFormat" || pf.Version != "v1" {
		t.Errorf("Lucene50PostingsFormat: got (%q, %q)", pf.Name, pf.Version)
	}
	pr := NewLucene50PostingsReader("v2")
	if pr.Name != "Lucene50PostingsReader" || pr.Version != "v2" {
		t.Errorf("Lucene50PostingsReader: got (%q, %q)", pr.Name, pr.Version)
	}
}
