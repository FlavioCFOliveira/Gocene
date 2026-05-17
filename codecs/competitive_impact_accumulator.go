// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"sort"
)

// CompetitiveImpactAccumulator collects per-document Impact values and keeps
// only the impacts that may produce the maximum BM25-like score within the
// observed block. It is the Go port of
// org.apache.lucene.codecs.CompetitiveImpactAccumulator from Apache Lucene 10.4.0.
//
// Two Impacts (f1, n1) and (f2, n2) are ordered by score so that an impact
// (f, n) dominates (fOther, nOther) when f >= fOther AND
// uint64(n) <= uint64(nOther). The unsigned comparison on Norm matches the
// Java implementation, which uses Long.compareUnsigned to handle the encoded
// byte's sign bit. Dominated impacts cannot beat the dominating one for any
// monotonic similarity function, so they are pruned at insertion time.
//
// The accumulator is not safe for concurrent use; callers are expected to
// reuse a single instance per posting block while a writer iterates the
// document stream sequentially.
type CompetitiveImpactAccumulator struct {
	// maxFreqs[norm] = best (largest) Freq observed for that exact encoded norm.
	// Using a per-norm map is byte-cheap and mirrors the Java code's
	// initial bucket-by-byte step before the global merge.
	maxFreqs map[int64]int
}

// NewCompetitiveImpactAccumulator creates an empty accumulator.
func NewCompetitiveImpactAccumulator() *CompetitiveImpactAccumulator {
	return &CompetitiveImpactAccumulator{
		maxFreqs: make(map[int64]int),
	}
}

// Clear resets the accumulator so it can be reused for the next block.
func (a *CompetitiveImpactAccumulator) Clear() {
	// Reset by value, keeping the map allocation when small.
	for k := range a.maxFreqs {
		delete(a.maxFreqs, k)
	}
}

// Add records an (freq, norm) impact, keeping the highest freq per norm.
// Lucene 10.4.0 deduplicates by norm at insertion to keep the working set
// bounded by 256 entries (the encoded norm fits in a single signed byte for
// the default similarity).
func (a *CompetitiveImpactAccumulator) Add(freq int, norm int64) {
	if existing, ok := a.maxFreqs[norm]; !ok || freq > existing {
		a.maxFreqs[norm] = freq
	}
}

// AddAll merges every impact from other into this accumulator, applying the
// same per-norm max-freq deduplication.
func (a *CompetitiveImpactAccumulator) AddAll(other *CompetitiveImpactAccumulator) {
	if other == nil {
		return
	}
	for norm, freq := range other.maxFreqs {
		a.Add(freq, norm)
	}
}

// GetCompetitiveFreqNormPairs returns the pruned set of competitive impacts
// sorted by ascending freq, matching Lucene's
// CompetitiveImpactAccumulator#getCompetitiveFreqNormPairs() contract.
//
// An impact (f, n) is competitive when no other impact (f', n') satisfies
// f' >= f AND uint64(n') <= uint64(n). The returned slice is a fresh copy
// owned by the caller.
func (a *CompetitiveImpactAccumulator) GetCompetitiveFreqNormPairs() []Impact {
	if len(a.maxFreqs) == 0 {
		return nil
	}

	// Materialize all candidate impacts.
	candidates := make([]Impact, 0, len(a.maxFreqs))
	for norm, freq := range a.maxFreqs {
		candidates = append(candidates, Impact{Freq: freq, Norm: norm})
	}

	// Sort by freq descending, then by unsigned norm ascending so that the
	// most-competitive impact for each freq tier is visited first.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Freq != candidates[j].Freq {
			return candidates[i].Freq > candidates[j].Freq
		}
		return uint64(candidates[i].Norm) < uint64(candidates[j].Norm)
	})

	// Sweep: an impact survives only if no earlier (higher-freq) impact has
	// an equal-or-smaller unsigned norm.
	competitive := candidates[:0]
	var bestNormSoFar uint64
	first := true
	for _, imp := range candidates {
		n := uint64(imp.Norm)
		if first || n < bestNormSoFar {
			competitive = append(competitive, imp)
			bestNormSoFar = n
			first = false
		}
	}

	// Return in ascending freq order to match Java's TreeMap iteration.
	result := make([]Impact, len(competitive))
	for i, imp := range competitive {
		result[len(competitive)-1-i] = imp
	}
	return result
}
