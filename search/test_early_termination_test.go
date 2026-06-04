// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/search/TestEarlyTermination.java
//
// This test drives a MatchAllDocsQuery over a multi-segment index with a
// collector that may signal early termination in two ways, exactly as Lucene's
// SimpleCollector does: by raising a CollectionTerminatedException either from
// the per-leaf hook (GetLeafCollector, the analogue of doSetNextReader) or from
// collect(). The invariant under test is that once a leaf collector has
// terminated, Collect is never called on it again — the search loop must swallow
// the signal and move on to the next leaf. In Go the exception is modelled as
// the CollectionTerminatedException error value (see IsCollectionTerminated).
package search_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestEarlyTermination_TestEarlyTermination mirrors
// TestEarlyTermination.testEarlyTermination.
func TestEarlyTermination_TestEarlyTermination(t *testing.T) {
	ix := newIntegrationIndex(t)
	const numDocs = 120
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < numDocs; i++ {
		ix.addDoc(document.NewDocument())
		// "rarely" commit, building a multi-segment index so several leaves are
		// visited and the per-leaf termination path is actually exercised.
		if rng.Intn(10) == 0 {
			ix.commit()
		}
	}
	searcher, cleanup := ix.searcher()
	defer cleanup()

	const iters = 5
	for i := 0; i < iters; i++ {
		collector := &earlyTerminationCollector{rng: rng, t: t}
		if err := searcher.SearchWithCollector(search.NewMatchAllDocsQuery(), collector); err != nil {
			t.Fatalf("SearchWithCollector: %v", err)
		}
	}
}

// earlyTerminationCollector mirrors the anonymous SimpleCollector created per leaf in
// the upstream test. ScoreMode is COMPLETE_NO_SCORES.
type earlyTerminationCollector struct {
	rng *rand.Rand
	t   *testing.T
}

func (c *earlyTerminationCollector) ScoreMode() search.ScoreMode { return search.COMPLETE_NO_SCORES }

// GetLeafCollector is the analogue of doSetNextReader: it randomly decides to
// terminate the leaf immediately (returning a CollectionTerminatedException) or
// to collect it.
func (c *earlyTerminationCollector) GetLeafCollector(_ *index.LeafReaderContext) (search.LeafCollector, error) {
	if c.rng.Intn(2) == 0 {
		return nil, search.NewCollectionTerminatedException()
	}
	return &earlyTerminationLeafCollector{rng: c.rng, t: c.t}, nil
}

// earlyTerminationLeafCollector asserts that Collect is never invoked after the leaf
// has terminated, and "rarely" terminates from collect itself.
type earlyTerminationLeafCollector struct {
	rng        *rand.Rand
	t          *testing.T
	terminated bool
}

func (lc *earlyTerminationLeafCollector) SetScorer(_ search.Scorer) error { return nil }

func (lc *earlyTerminationLeafCollector) Collect(_ int) error {
	if lc.terminated {
		lc.t.Errorf("Collect called after the leaf collector terminated")
		return nil
	}
	if lc.rng.Intn(10) == 0 {
		lc.terminated = true
		return search.NewCollectionTerminatedException()
	}
	return nil
}
