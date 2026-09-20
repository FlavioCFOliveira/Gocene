// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene102

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
)

// TestLucene102RWBinaryFlatVectorsScorer_Constructor verifies the scorer's
// toString over the default flat vector scorer.
func TestLucene102RWBinaryFlatVectorsScorer_Constructor(t *testing.T) {
	s := NewLucene102BinaryFlatVectorsScorer(hnsw.DefaultFlatVectorScorerInstance)
	want := "Lucene102BinaryFlatVectorsScorer(nonQuantizedDelegate=DefaultFlatVectorScorer())"
	if got := s.String(); got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}
