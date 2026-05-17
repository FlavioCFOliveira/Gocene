// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// TestIndexOptions_OrdinalsMatchLucene104 locks in the on-disk ordinal
// contract with org.apache.lucene.index.IndexOptions (Lucene 10.4.0).
func TestIndexOptions_OrdinalsMatchLucene104(t *testing.T) {
	tests := []struct {
		ord  int
		name string
		io   IndexOptions
	}{
		{0, "NONE", IndexOptionsNone},
		{1, "DOCS", IndexOptionsDocs},
		{2, "DOCS_AND_FREQS", IndexOptionsDocsAndFreqs},
		{3, "DOCS_AND_FREQS_AND_POSITIONS", IndexOptionsDocsAndFreqsAndPositions},
		{4, "DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS", IndexOptionsDocsAndFreqsAndPositionsAndOffsets},
	}
	for _, tc := range tests {
		if int(tc.io) != tc.ord {
			t.Errorf("%s: ordinal=%d want %d", tc.name, int(tc.io), tc.ord)
		}
		if tc.io.String() != tc.name {
			t.Errorf("ordinal %d: String=%q want %q", tc.ord, tc.io.String(), tc.name)
		}
	}
}

func TestIndexOptions_Predicates(t *testing.T) {
	if IndexOptionsNone.IsIndexed() {
		t.Errorf("NONE.IsIndexed() = true")
	}
	if !IndexOptionsDocs.IsIndexed() {
		t.Errorf("DOCS.IsIndexed() = false")
	}
	if IndexOptionsDocs.HasFreqs() {
		t.Errorf("DOCS.HasFreqs() = true")
	}
	if !IndexOptionsDocsAndFreqs.HasFreqs() {
		t.Errorf("DOCS_AND_FREQS.HasFreqs() = false")
	}
	if IndexOptionsDocsAndFreqs.HasPositions() {
		t.Errorf("DOCS_AND_FREQS.HasPositions() = true")
	}
	if !IndexOptionsDocsAndFreqsAndPositions.HasPositions() {
		t.Errorf("DOCS_AND_FREQS_AND_POSITIONS.HasPositions() = false")
	}
	if IndexOptionsDocsAndFreqsAndPositions.HasOffsets() {
		t.Errorf("DOCS_AND_FREQS_AND_POSITIONS.HasOffsets() = true")
	}
	if !IndexOptionsDocsAndFreqsAndPositionsAndOffsets.HasOffsets() {
		t.Errorf("DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS.HasOffsets() = false")
	}
}
