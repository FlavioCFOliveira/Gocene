// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestParentBlockJoinByteKnnVectorQuery.
package join

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestParentBlockJoinByteKnnVectorQuery_VectorEncodingMismatch corresponds to
// TestParentBlockJoinByteKnnVectorQuery.testVectorEncodingMismatch and adds the
// positive byte-vector block-join round-trip: a byte-encoded child vector field
// (sparse, since parents carry no vector) round-trips through IndexWriter and
// the diversifying byte query returns the nearest child per parent; a float
// query over the byte field errors.
func TestParentBlockJoinByteKnnVectorQuery_VectorEncodingMismatch(t *testing.T) {
	dir, w := newBlockWriter(t)
	// Two parent blocks, each with two byte-vector children.
	addBlock(t, w,
		pbjMakeByteChild(t, "field", []byte{1, 2, 3}, "c0"),
		pbjMakeByteChild(t, "field", []byte{3, 3, 3}, "c1"),
		pbjMakeParent(t, ""),
	)
	addBlock(t, w,
		pbjMakeByteChild(t, "field", []byte{0, 0, 1}, "c2"),
		pbjMakeByteChild(t, "field", []byte{10, 10, 10}, "c3"),
		pbjMakeParent(t, ""),
	)
	r, s := commitAndOpen(t, dir, w)
	parentFilter := pbjParentsFilter()
	if err := Check(r, parentFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	// Byte query near {9,9,9}: best child of block 1 is {3,3,3} (c1), best of
	// block 2 is {10,10,10} (c3, the nearest overall).
	bq := NewDiversifyingChildrenByteKnnVectorQuery("field", []byte{9, 9, 9}, 3, nil, parentFilter)
	td, err := s.Search(bq, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(td.ScoreDocs) != 2 {
		t.Fatalf("got %d hits, want 2 (one per parent)", len(td.ScoreDocs))
	}
	topDoc, err := s.Doc(td.ScoreDocs[0].Doc)
	if err != nil {
		t.Fatalf("Doc: %v", err)
	}
	if got := storedString(topDoc, "id"); got != "c3" {
		t.Errorf("top child id = %q, want c3", got)
	}

	// A float query over the byte-encoded field must error.
	fq := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{1, 2, 3}, 2, nil, parentFilter)
	if _, err := s.Search(fq, 3); err == nil {
		t.Errorf("float query over byte field: want error, got nil")
	}
}

// pbjMakeByteChild builds a child document carrying a byte vector for field and
// a stored "id".
func pbjMakeByteChild(t *testing.T, field string, vector []byte, id string) index.Document {
	t.Helper()
	d := document.NewDocument()
	vf, err := document.NewKnnByteVectorField(field, vector, index.VectorSimilarityFunctionEuclidean)
	if err != nil {
		t.Fatalf("NewKnnByteVectorField(%q): %v", field, err)
	}
	d.Add(vf)
	d.Add(mustStringField(t, "id", id, true))
	return d
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
