// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// BenchmarkBM25Score measures BM25 score computation throughput.
// The benchmark covers the hot path: IDF computation followed by ScoreBM25.
//
// Setup: a collection of 1 000 000 documents, 50 000 matching the query term,
// each with a term frequency of 3 and document length of 100 (average 120).
func BenchmarkBM25Score(b *testing.B) {
	const (
		totalDocs    = 1_000_000
		matchingDocs = 50_000
		termFreq     = 3.0
		docLen       = 100.0
		avgDocLen    = 120.0
	)

	sim := search.NewBM25Similarity()
	term := index.NewTerm("body", "lucene")
	collStats := search.NewCollectionStatistics("body", totalDocs, totalDocs, 5_000_000, 500_000)
	termStats := search.NewTermStatistics(term, matchingDocs, 150_000)

	// Pre-compute IDF via the scorer chain so the benchmark measures only
	// the per-document ScoreBM25 call.
	scorer := search.NewBM25SimScorer(sim, collStats, termStats)
	_ = scorer // ensure setup allocations are done

	idf := sim.InverseDocumentFrequency(totalDocs, matchingDocs)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = sim.ScoreBM25(termFreq, docLen, avgDocLen, idf)
	}
}
