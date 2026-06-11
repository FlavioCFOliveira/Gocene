// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import (
	"testing"
)

// TestLucene102RWBinaryFlatVectorsScorer_Constructor verifies the format
// constructor fields.
func TestLucene102RWBinaryFlatVectorsScorer_Constructor(t *testing.T) {
	s := NewLucene102BinaryFlatVectorsScorer("10.2")
	if s.Name != "Lucene102BinaryFlatVectorsScorer" {
		t.Errorf("Name = %q, want %q", s.Name, "Lucene102BinaryFlatVectorsScorer")
	}
	if s.Version != "10.2" {
		t.Errorf("Version = %q, want %q", s.Version, "10.2")
	}
}
