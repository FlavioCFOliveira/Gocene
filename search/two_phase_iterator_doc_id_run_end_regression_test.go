// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// This file pins two TwoPhaseIterator.docIDRunEnd() renderings of Lucene
// 10.5.0: (1) the DocIdSetIterator view returned by asDocIdSetIterator inherits
// DocIdSetIterator.docIDRunEnd() (docID() + 1) — it used to delegate to the
// approximation, reporting unverified candidates as a run of matches; (2) a
// TwoPhaseIterator subclass may override docIDRunEnd(), which the verifier
// renders through TwoPhaseDocIDRunEndOverride.

import "testing"

type runEndVerifier struct {
	approx   DocIdSetIterator
	override bool
}

func (v *runEndVerifier) Matches() (bool, error) { return v.approx.DocID()%2 == 0, nil }
func (v *runEndVerifier) MatchCost() float32     { return 1 }

type runEndOverrideVerifier struct{ runEndVerifier }

func (v *runEndOverrideVerifier) DocIDRunEnd() (int, error) { return 42, nil }

func TestTwoPhaseIterator_DocIDRunEndRegression(t *testing.T) {
	approx := NewRangeDocIdSetIterator(0, 100) // docIDRunEnd() == maxDoc == 100
	tpi := NewTwoPhaseIterator(approx, &runEndVerifier{approx: approx})
	disi := AsDocIdSetIterator(tpi)
	doc, err := disi.NextDoc()
	if err != nil || doc != 0 {
		t.Fatalf("nextDoc = %d, %v", doc, err)
	}
	end, err := disi.DocIDRunEnd()
	if err != nil || end != 1 {
		t.Fatalf("asDocIdSetIterator.docIDRunEnd() = %d, %v; want docID()+1 = 1", end, err)
	}
	if end, _ := tpi.DocIDRunEnd(); end != 0 {
		t.Fatalf("TwoPhaseIterator.docIDRunEnd() default = %d, want approximation().docID() = 0", end)
	}

	approx2 := NewRangeDocIdSetIterator(0, 100)
	tpi2 := NewTwoPhaseIterator(approx2, &runEndOverrideVerifier{runEndVerifier{approx: approx2}})
	if end, _ := tpi2.DocIDRunEnd(); end != 42 {
		t.Fatalf("overridden docIDRunEnd() = %d, want 42", end)
	}
}
