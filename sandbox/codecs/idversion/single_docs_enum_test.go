// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.codecs.idversion.SingleDocsEnum tests.
package idversion

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// --- SingleDocsEnum tests ---

// TestSingleDocsEnum_NextDocMatchesSingleDoc verifies that NextDoc returns the
// configured docID on the first call.
func TestSingleDocsEnum_NextDocMatchesSingleDoc(t *testing.T) {
	e := &SingleDocsEnum{}
	e.Reset(42)

	doc, err := e.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if doc != 42 {
		t.Errorf("NextDoc() = %d; want 42", doc)
	}
}

// TestSingleDocsEnum_ExhaustedAfterFirstDoc verifies NO_MORE_DOCS on second call.
func TestSingleDocsEnum_ExhaustedAfterFirstDoc(t *testing.T) {
	e := &SingleDocsEnum{}
	e.Reset(5)

	if _, err := e.NextDoc(); err != nil {
		t.Fatal(err)
	}
	doc, err := e.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if doc != index.NO_MORE_DOCS {
		t.Errorf("second NextDoc() = %d; want NO_MORE_DOCS", doc)
	}
}

// TestSingleDocsEnum_AdvanceToDocID verifies Advance succeeds when target <= docID.
func TestSingleDocsEnum_AdvanceToDocID(t *testing.T) {
	e := &SingleDocsEnum{}
	e.Reset(10)

	doc, err := e.Advance(10)
	if err != nil {
		t.Fatal(err)
	}
	if doc != 10 {
		t.Errorf("Advance(10) = %d; want 10", doc)
	}
}

// TestSingleDocsEnum_AdvancePastDocIDReturnsNoMoreDocs verifies that targeting
// beyond the single doc produces NO_MORE_DOCS.
func TestSingleDocsEnum_AdvancePastDocIDReturnsNoMoreDocs(t *testing.T) {
	e := &SingleDocsEnum{}
	e.Reset(10)

	doc, err := e.Advance(11)
	if err != nil {
		t.Fatal(err)
	}
	if doc != index.NO_MORE_DOCS {
		t.Errorf("Advance(11) = %d; want NO_MORE_DOCS", doc)
	}
}

// TestSingleDocsEnum_CostIsOne verifies Cost() == 1.
func TestSingleDocsEnum_CostIsOne(t *testing.T) {
	e := &SingleDocsEnum{}
	e.Reset(0)
	if got := e.Cost(); got != 1 {
		t.Errorf("Cost() = %d; want 1", got)
	}
}

// TestSingleDocsEnum_GetPayloadErrors verifies GetPayload returns an error.
func TestSingleDocsEnum_GetPayloadErrors(t *testing.T) {
	e := &SingleDocsEnum{}
	e.Reset(0)
	_, err := e.GetPayload()
	if err == nil {
		t.Fatal("expected error from GetPayload, got nil")
	}
}

// --- SinglePostingsEnum tests ---

// TestSinglePostingsEnum_NextDocAndPosition verifies the standard iteration
// sequence: NextDoc → NextPosition → GetPayload.
func TestSinglePostingsEnum_NextDocAndPosition(t *testing.T) {
	e := &SinglePostingsEnum{}
	const docID = 7
	const version int64 = 99999
	e.Reset(docID, version)

	doc, err := e.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if doc != docID {
		t.Errorf("NextDoc() = %d; want %d", doc, docID)
	}

	pos, err := e.NextPosition()
	if err != nil {
		t.Fatal(err)
	}
	if pos != 0 {
		t.Errorf("NextPosition() = %d; want 0", pos)
	}

	payload, err := e.GetPayload()
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) != 8 {
		t.Fatalf("payload length = %d; want 8", len(payload))
	}
	got := BytesToLong(payload)
	if got != version {
		t.Errorf("decoded version = %d; want %d", got, version)
	}
}

// TestSinglePostingsEnum_ExhaustsAfterSingleDoc verifies NO_MORE_DOCS on second
// NextDoc call.
func TestSinglePostingsEnum_ExhaustsAfterSingleDoc(t *testing.T) {
	e := &SinglePostingsEnum{}
	e.Reset(3, 1)

	if _, err := e.NextDoc(); err != nil {
		t.Fatal(err)
	}
	doc, err := e.NextDoc()
	if err != nil {
		t.Fatal(err)
	}
	if doc != index.NO_MORE_DOCS {
		t.Errorf("second NextDoc() = %d; want NO_MORE_DOCS", doc)
	}
}

// TestSinglePostingsEnum_AdvanceHitsDoc verifies Advance reaches the doc when
// target <= docID.
func TestSinglePostingsEnum_AdvanceHitsDoc(t *testing.T) {
	e := &SinglePostingsEnum{}
	e.Reset(20, 5)

	doc, err := e.Advance(5)
	if err != nil {
		t.Fatal(err)
	}
	if doc != 20 {
		t.Errorf("Advance(5) = %d; want 20", doc)
	}
}

// TestSinglePostingsEnum_FreqIsOne verifies Freq() returns 1.
func TestSinglePostingsEnum_FreqIsOne(t *testing.T) {
	e := &SinglePostingsEnum{}
	e.Reset(0, 0)
	freq, err := e.Freq()
	if err != nil {
		t.Fatal(err)
	}
	if freq != 1 {
		t.Errorf("Freq() = %d; want 1", freq)
	}
}
