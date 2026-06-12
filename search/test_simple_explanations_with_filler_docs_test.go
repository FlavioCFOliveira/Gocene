// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestSimpleExplanationsWithFillerDocs.java
//
// Subclass of TestSimpleExplanations that injects filler docs between every
// real doc. The filler docs carry an EXTRA field so queries can exclude them,
// and the expected doc numbers are remapped to account for the injected docs.
// This emphasises the DocFreq factor in scoring while keeping the same expected
// matching sets.
//
// Faithful divergence: Lucene randomly chooses between empty filler docs
// (EXTRA == null) and content-bearing filler docs (EXTRA != null), and uses
// JUnit Assume to skip testMA1/testMA2 in the empty-filler case. Gocene's
// no-skip policy forbids skipping, so this port always uses content-bearing
// filler docs (EXTRA != null): every scenario — MA1/MA2 included — is viable
// and is exercised, never skipped. The filler count is fixed (deterministic),
// matching the day-build NUM_FILLER_DOCS of 4.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/search/testutil"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	// fillerNumFillerDocs is the number of filler docs injected after every
	// real doc (the deterministic day-build value).
	fillerNumFillerDocs = 4
	// fillerPreFillerDocs is the number of filler docs injected before the
	// first real doc (a fixed value in [0, NUM_FILLER_DOCS/2]).
	fillerPreFillerDocs = 2
	// fillerExtra is the EXTRA field name and value used to exclude filler docs.
	// It is always non-null in this port so MA1/MA2 stay viable.
	fillerExtra = "extra"
)

// fillerExplanationTestCase wraps the explanation harness with the filler-doc
// remapping and EXTRA exclusion semantics of TestSimpleExplanationsWithFillerDocs.
type fillerExplanationTestCase struct {
	t        *testing.T
	searcher *search.IndexSearcher
	cleanup  func()
}

// newFillerExplanationTestCase rebuilds the explanation index with filler docs,
// mirroring TestSimpleExplanationsWithFillerDocs.replaceIndex.
func newFillerExplanationTestCase(t *testing.T) *fillerExplanationTestCase {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	makeFiller := func(seed int) *document.Document {
		doc := createExplDoc(t, seed%len(explDocFields))
		extra, fErr := document.NewStringField(fillerExtra, fillerExtra, false)
		if fErr != nil {
			t.Fatalf("NewStringField(EXTRA): %v", fErr)
		}
		doc.Add(extra)
		return doc
	}
	fillerSeed := 0
	for filler := 0; filler < fillerPreFillerDocs; filler++ {
		if addErr := w.AddDocument(makeFiller(fillerSeed)); addErr != nil {
			t.Fatalf("AddDocument(pre-filler): %v", addErr)
		}
		fillerSeed++
	}
	for i := range explDocFields {
		if addErr := w.AddDocument(createExplDoc(t, i)); addErr != nil {
			t.Fatalf("AddDocument(%d): %v", i, addErr)
		}
		for filler := 0; filler < fillerNumFillerDocs; filler++ {
			if addErr := w.AddDocument(makeFiller(fillerSeed)); addErr != nil {
				t.Fatalf("AddDocument(filler): %v", addErr)
			}
			fillerSeed++
		}
	}
	if err = w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return &fillerExplanationTestCase{
		t:        t,
		searcher: search.NewIndexSearcher(reader),
		cleanup: func() {
			_ = reader.Close()
			_ = dir.Close()
		},
	}
}

// qtest remaps the expected doc numbers for the injected filler docs and wraps
// q so the filler docs are excluded, then runs the matching-doc + explanation
// checks. Mirrors TestSimpleExplanationsWithFillerDocs.qtest.
func (tc *fillerExplanationTestCase) qtest(q search.Query, expDocNrs []int) {
	tc.t.Helper()
	remapped := make([]int, len(expDocNrs))
	for i, d := range expDocNrs {
		remapped[i] = fillerPreFillerDocs + (fillerNumFillerDocs+1)*d
	}
	bq := search.NewBooleanQuery()
	bq.Add(q, search.MUST)
	bq.Add(search.NewTermQuery(index.NewTerm(fillerExtra, fillerExtra)), search.MUST_NOT)
	wrapped := search.Query(bq)

	testutil.CheckHitCollector(tc.t, wrapped, explField, tc.searcher, remapped)
	testutil.CheckExplanations(tc.t, wrapped, explField, tc.searcher, true)
}

func TestSimpleExplanationsWithFillerDocs_MA1(t *testing.T) {
	tc := newFillerExplanationTestCase(t)
	defer tc.cleanup()
	tc.qtest(search.NewMatchAllDocsQuery(), []int{0, 1, 2, 3})
}

func TestSimpleExplanationsWithFillerDocs_MA2(t *testing.T) {
	tc := newFillerExplanationTestCase(t)
	defer tc.cleanup()
	tc.qtest(search.NewBoostQuery(search.NewMatchAllDocsQuery(), 1000), []int{0, 1, 2, 3})
}

// TestSimpleExplanationsWithFillerDocs_All replays every TestSimpleExplanations
// scenario against the filler-doc index, mirroring the inherited test methods
// (Go has no method override, so the shared scenario table is replayed here).
func TestSimpleExplanationsWithFillerDocs_All(t *testing.T) {
	tc := newFillerExplanationTestCase(t)
	defer tc.cleanup()
	// simpleExplanationScenarios needs an explanationTestCase only for the
	// matchTheseItems helper, which depends solely on the KEY field constants
	// and not on the searcher, so a bare value suffices here.
	scenarios := simpleExplanationScenarios(&explanationTestCase{t: t})
	for _, sc := range scenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			sub := &fillerExplanationTestCase{t: t, searcher: tc.searcher, cleanup: func() {}}
			sub.qtest(sc.q, sc.exp)
		})
	}
}