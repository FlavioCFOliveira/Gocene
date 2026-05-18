// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import "math/rand"

// RandomSamplingFacetsCollector is a FacetsCollector variant that records a
// uniformly random sample of the matched documents instead of every hit.
// Mirrors org.apache.lucene.facet.RandomSamplingFacetsCollector.
type RandomSamplingFacetsCollector struct {
	sampleSize int
	rng        *rand.Rand
	matchedSet []int
	seen       int
}

// NewRandomSamplingFacetsCollector builds a collector that keeps at most
// sampleSize documents using reservoir sampling.
func NewRandomSamplingFacetsCollector(sampleSize int, seed int64) *RandomSamplingFacetsCollector {
	if sampleSize < 1 {
		sampleSize = 1
	}
	return &RandomSamplingFacetsCollector{
		sampleSize: sampleSize,
		rng:        rand.New(rand.NewSource(seed)),
		matchedSet: make([]int, 0, sampleSize),
	}
}

// Collect observes docID with the reservoir sampling rule: the first
// sampleSize docs are kept verbatim; subsequent docs replace a random slot
// with decreasing probability.
func (c *RandomSamplingFacetsCollector) Collect(docID int) {
	c.seen++
	if len(c.matchedSet) < c.sampleSize {
		c.matchedSet = append(c.matchedSet, docID)
		return
	}
	idx := c.rng.Intn(c.seen)
	if idx < c.sampleSize {
		c.matchedSet[idx] = docID
	}
}

// SampleSize returns the requested sample budget.
func (c *RandomSamplingFacetsCollector) SampleSize() int { return c.sampleSize }

// Seen returns the number of Collect calls made.
func (c *RandomSamplingFacetsCollector) Seen() int { return c.seen }

// Matches returns a copy of the sampled doc IDs.
func (c *RandomSamplingFacetsCollector) Matches() []int {
	out := make([]int, len(c.matchedSet))
	copy(out, c.matchedSet)
	return out
}
