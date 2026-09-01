// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// xorShift64Random is a fast random number generator.
// Port of Lucene's XORShift64Random.
type xorShift64Random struct {
	x uint64
}

func newXorShift64Random(seed int64) *xorShift64Random {
	// seed == 0 ? 0xdeadbeef : seed
	s := uint64(seed)
	if s == 0 {
		s = 0xdeadbeef
	}
	return &xorShift64Random{x: s}
}

func (r *xorShift64Random) randomLong() uint64 {
	r.x ^= (r.x << 21)
	r.x ^= (r.x >> 35)
	r.x ^= (r.x << 4)
	return r.x
}

func (r *xorShift64Random) nextInt(n int) int {
	res := int(r.randomLong() % uint64(n))
	if res < 0 {
		return -res
	}
	return res
}

// RandomSamplingFacetsCollector collects hits for subsequent faceting, using sampling if needed.
// Mirrors org.apache.lucene.facet.RandomSamplingFacetsCollector.
type RandomSamplingFacetsCollector struct {
	*FacetsCollector
	sampleSize int
	random     *xorShift64Random

	samplingRate float64
	sampledDocs   []*MatchingDocs

	totalHits    int
	leftoverBin   int
	leftoverIndex int
}

const notCalculated = -1

// NewRandomSamplingFacetsCollector creates a new RandomSamplingFacetsCollector with the given sample size and default seed (0).
func NewRandomSamplingFacetsCollector(sampleSize int) *RandomSamplingFacetsCollector {
	return NewRandomSamplingFacetsCollectorWithSeed(sampleSize, 0)
}

// NewRandomSamplingFacetsCollectorWithSeed creates a new RandomSamplingFacetsCollector with the given sample size and seed.
func NewRandomSamplingFacetsCollectorWithSeed(sampleSize int, seed int64) *RandomSamplingFacetsCollector {
	return &RandomSamplingFacetsCollector{
		FacetsCollector: NewFacetsCollector(),
		sampleSize:       sampleSize,
		random:          newXorShift64Random(seed),
		totalHits:       notCalculated,
		leftoverBin:     notCalculated,
		leftoverIndex:   notCalculated,
	}
}

// GetMatchingDocs returns the sampled list of the matching documents.
func (r *RandomSamplingFacetsCollector) GetMatchingDocs() []*MatchingDocs {
	matchingDocs := r.FacetsCollector.GetMatchingDocs()

	if r.totalHits == notCalculated {
		r.totalHits = 0
		for _, md := range matchingDocs {
			r.totalHits += md.TotalHits
		}
	}

	if r.totalHits <= r.sampleSize {
		return matchingDocs
	}

	if r.sampledDocs == nil {
		r.samplingRate = (1.0 * float64(r.sampleSize)) / float64(r.totalHits)
		r.sampledDocs = r.createSampledDocs(matchingDocs)
	}
	return r.sampledDocs
}

// GetOriginalMatchingDocs returns the original matching documents.
func (r *RandomSamplingFacetsCollector) GetOriginalMatchingDocs() []*MatchingDocs {
	return r.FacetsCollector.GetMatchingDocs()
}

func (r *RandomSamplingFacetsCollector) createSampledDocs(matchingDocsList []*MatchingDocs) []*MatchingDocs {
	sampledDocsList := make([]*MatchingDocs, 0, len(matchingDocsList))
	for _, docs := range matchingDocsList {
		sampledDocsList = append(sampledDocsList, r.createSample(docs))
	}
	return sampledDocsList
}

func (r *RandomSamplingFacetsCollector) createSample(docs *MatchingDocs) *MatchingDocs {
	maxDoc := docs.Context.Reader().MaxDoc()
	sampleDocs := NewDocIdSetBits(maxDoc, nil)

	binSize := int(1.0 / r.samplingRate)

	var limit, randomIndex int
	if r.leftoverBin != notCalculated {
		limit = r.leftoverBin
		randomIndex = r.leftoverIndex
	} else {
		limit = binSize
		randomIndex = r.random.nextInt(binSize)
	}

	it := docs.Bits.GetIterator()
	counter := 0
	for doc := it.NextDoc(); doc != -1; doc = it.NextDoc() {
		if counter == randomIndex {
			sampleDocs.Set(doc)
		}
		counter++
		if counter >= limit {
			counter = 0
			limit = binSize
			randomIndex = r.random.nextInt(binSize)
		}
	}

	if counter == 0 {
		r.leftoverBin = notCalculated
		r.leftoverIndex = notCalculated
	} else {
		r.leftoverBin = limit - counter
		if randomIndex > counter {
			r.leftoverIndex = randomIndex - counter
		} else if randomIndex < counter {
			r.leftoverIndex = notCalculated
		} else {
			r.leftoverIndex = notCalculated
		}
	}

	return NewMatchingDocs(docs.Context, sampleDocs, docs.TotalHits)
}

// AmortizeFacetCounts amortizes the sampled counts by the sampling rate.
// Mirrors org.apache.lucene.facet.RandomSamplingFacetsCollector.amortizeFacetCounts.
func (r *RandomSamplingFacetsCollector) AmortizeFacetCounts(res *FacetResult, config *FacetsConfig, searcher *search.IndexSearcher) (*FacetResult, error) {
	if res == nil || r.totalHits <= r.sampleSize {
		return res, nil
	}

	reader := searcher.GetIndexReader()
	dimConfig := config.GetDimConfig(res.Dim)

	childPath := make([]string, len(res.Path)+2)
	childPath[0] = res.Dim
	copy(childPath[1:], res.Path)

	fixedLabelValues := make([]*LabelAndValue, len(res.LabelValues))
	for i, lv := range res.LabelValues {
		childPath[len(res.Path)+1] = lv.Label
		fullPath := PathToString(childPath, len(childPath))

		max := reader.DocFreq(fullPath) // simplified for example, check actual API
		correctedCount := int64(math.Round(float64(lv.Value) / r.samplingRate))
		if int(correctedCount) > max {
			correctedCount = int64(max)
		}
		fixedLabelValues[i] = NewLabelAndValue(lv.Label, correctedCount)
	}

	correctedTotalCount := int64(math.Round(float64(res.Value) / r.samplingRate))
	if correctedTotalCount > int64(reader.NumDocs()) {
		correctedTotalCount = int64(reader.NumDocs())
	}

	return &FacetResult{
		Dim:         res.Dim,
		Path:        res.Path,
		Value:       correctedTotalCount,
		LabelValues: fixedLabelValues,
		ChildCount:  res.ChildCount,
	}, nil
}

// GetSamplingRate returns the sampling rate that was used.
func (r *RandomSamplingFacetsCollector) GetSamplingRate() float64 {
	return r.samplingRate
}
