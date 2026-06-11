// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"testing"
)

// TestBlockPostingsFormat2_Constants verifies the postings block constants.
func TestBlockPostingsFormat2_Constants(t *testing.T) {
	if BlockSize != 128 {
		t.Errorf("BlockSize: got %d, want 128", BlockSize)
	}
}

// TestBlockPostingsFormat2_Name verifies that format types carry the
// expected human-readable name.
func TestBlockPostingsFormat2_Name(t *testing.T) {
	pf := NewLucene50PostingsFormat("")
	if pf.Name != "Lucene50PostingsFormat" {
		t.Errorf("PostingsFormat.Name: got %q", pf.Name)
	}
	pr := NewLucene50PostingsReader("")
	if pr.Name != "Lucene50PostingsReader" {
		t.Errorf("PostingsReader.Name: got %q", pr.Name)
	}
	cf := NewLucene50CompoundFormat("")
	if cf.Name != "Lucene50CompoundFormat" {
		t.Errorf("CompoundFormat.Name: got %q", cf.Name)
	}
	sf := NewLucene50StoredFieldsFormat("")
	if sf.Name != "Lucene50StoredFieldsFormat" {
		t.Errorf("StoredFieldsFormat.Name: got %q", sf.Name)
	}
	tf := NewLucene50TermVectorsFormat("")
	if tf.Name != "Lucene50TermVectorsFormat" {
		t.Errorf("TermVectorsFormat.Name: got %q", tf.Name)
	}
}
