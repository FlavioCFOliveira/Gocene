// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestParentBlockJoinByteKnnVectorQuery.
package join

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestParentBlockJoinByteKnnVectorQuery_VectorEncodingMismatch corresponds to
// TestParentBlockJoinByteKnnVectorQuery.testVectorEncodingMismatch.
// Skipped: requires DirectoryReader + IndexSearcher with byte-vector field.
func TestParentBlockJoinByteKnnVectorQuery_VectorEncodingMismatch(t *testing.T) {
	t.Skip("requires DirectoryReader + IndexSearcher: SegmentReader coreReaders not yet wired")
}

// TestParentBlockJoinByteKnnVectorQuery_ToString corresponds to
// TestParentBlockJoinByteKnnVectorQuery.testToString.
func TestParentBlockJoinByteKnnVectorQuery_ToString(t *testing.T) {
	// Without filter: fromFloat({0,1}) → {0,1} (byte).
	q := NewDiversifyingChildrenByteKnnVectorQuery("field", []byte{0, 1}, 10, nil, nil)
	s := q.String()
	if !strings.HasPrefix(s, "DiversifyingChildrenByteKnnVectorQuery:field") {
		t.Errorf("String() prefix wrong: %q", s)
	}
	if !strings.Contains(s, "[10]") {
		t.Errorf("String() should contain [10]: %q", s)
	}
	// Should contain the first byte value.
	if !strings.Contains(s, "[0,") {
		t.Errorf("String() should start vector with [0,: %q", s)
	}

	// With filter.
	filter := search.NewTermQuery(index.NewTerm("id", "text"))
	q2 := NewDiversifyingChildrenByteKnnVectorQuery("field", []byte{0, 1}, 10, filter, nil)
	s2 := q2.String()
	if !strings.Contains(s2, "[id:text]") {
		t.Errorf("String() with filter should contain [id:text]: %q", s2)
	}
}

// TestParentBlockJoinByteKnnVectorQuery_TargetCopy verifies the target vector
// is defensively copied on construction.
func TestParentBlockJoinByteKnnVectorQuery_TargetCopy(t *testing.T) {
	orig := []byte{1, 2, 3}
	q := NewDiversifyingChildrenByteKnnVectorQuery("f", orig, 5, nil, nil)
	orig[0] = 0
	if q.Target[0] == 0 {
		t.Error("target was not defensively copied")
	}
}
