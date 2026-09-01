// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// RangeFacetCounts is a Facets implementation that computes counts for a range of values.
//
// This is the Go port of Lucene's org.apache.lucene.facet.RangeFacetCounts.
type RangeFacetCounts struct {
	field    string
	min      float64
	max      float64
	numBuckets int
	counts   []int
	totCount int
}

// NewRangeFacetCounts creates a new RangeFacetCounts.
func NewRangeFacetCounts(field string, min, max float64, numBuckets int, hits *FacetsCollector) error {
	f := &RangeFacetCounts{
		field:      field,
		min:        min,
		max:        max,
		numBuckets: numBuckets,
		counts:     make([]int, numBuckets),
	}

	if err := f.count(hits); err != nil {
		return err
	}
	return nil
}

func (f *RangeFacetCounts) count(hits *FacetsCollector) error {
	if hits == nil {
		return nil
	}

	matchingDocs := hits.GetMatchingDocs()
	for _, md := range matchingDocs {
		if md.TotalHits == 0 {
			continue
		}

		dv, err := index.GetNumericDocValues(md.Context, f.field)
		if err != nil {
			return err
		}
		if dv == nil {
			continue
		}

		for doc := dv.NextDoc(); doc != search.NoMoreDocs; doc = dv.NextDoc() {
			if md.Bits.Get(doc) {
				val, err := dv.LongValue() // Simplification: using LongValue
				if err != nil {
					return err
				}
				f.increment(float64(val))
			}
		}
	}
	return nil
}

func (f *RangeFacetCounts) increment(val float64) {
	if val < f.min || val > f.max {
		return
	}
	bucket := int((val - f.min) / (f.max - f.min) * float64(f.numBuckets))
	if bucket >= f.numBuckets {
		bucket = f.numBuckets - 1
	}
	if bucket < 0 {
		bucket = 0
	}
	f.counts[bucket]++
	f.totCount++
}

func (f *RangeFacetCounts) GetAllChildren(dim string, path ...string) (*FacetResult, error) {
	if dim != f.field {
		return nil, fmt.Errorf("invalid dim %q; should be %q", dim, f.field)
	}
	if len(path) != 0 {
		return nil, fmt.Errorf("path.length should be 0")
	}

	var labelValues []*LabelAndValue
	bucketSize := (f.max - f.min) / float64(f.numBuckets)
	for i, count := range f.counts {
		if count != 0 {
			low := f.min + float64(i)*bucketSize
			high := f.min + float64(i+1)*bucketSize
			label := fmt.Sprintf("[%f, %f)", low, high)
			labelValues = append(labelValues, NewLabelAndValue(label, i))
			labelValues[len(labelValues)-1].Count = count
		}
	}

	return NewFacetResult(f.field, []string{}, f.totCount, labelValues, len(labelValues)), nil
}

func (f *RangeFacetCounts) GetTopChildren(topN int, dim string, path ...string) (*FacetResult, error) {
	if err := ValidateTopN(topN); err != nil {
		return nil, err
	}
	if dim != f.field {
		return nil, fmt.Errorf("invalid dim %q; should be %q", dim, f.field)
	}
	if len(path) != 0 {
		return nil, fmt.Errorf("path.length should be 0")
	}

	res, err := f.GetAllChildren(dim, path...)
	if err != nil {
		return nil, err
	}

	// Sort results by count descending.
	sort.Slice(res.LabelValues, func(i, j int) bool {
		if res.LabelValues[i].Count != res.LabelValues[j].Count {
			return res.LabelValues[i].Count > res.LabelValues[j].Count
		}
		return res.LabelValues[i].Label < res.LabelValues[j].Label
	})

	limit := topN
	if len(res.LabelValues) < topN {
		limit = len(res.LabelValues)
	}

	return NewFacetResult(f.field, []string{}, f.totCount, res.LabelValues[:limit], res.NumChildren), nil
}

func (f *RangeFacetCounts) GetSpecificValue(dim string, path ...string) (interface{}, error) {
	if dim != f.field {
		return nil, fmt.Errorf("invalid dim %q; should be %q", dim, f.field)
	}
	if len(path) != 1 {
		return nil, fmt.Errorf("path must be length=1")
	}

	var val float64
	_, err := fmt.Sscanf(path[0], "%f", &val)
	if err != nil {
		return nil, err
	}

	bucket := int((val - f.min) / (f.max - f.min) * float64(f.numBuckets))
	if bucket < 0 || bucket >= f.numBuckets {
		return -1, nil
	}
	return f.counts[bucket], nil
}

func (f *RangeFacetCounts) GetAllDims(topN int) ([]*FacetResult, error) {
	res, err := f.GetTopChildren(topN, f.field)
	if err != nil {
		return nil, err
	}
	return []*FacetResult{res}, nil
}

func (f *RangeFacetCounts) GetTopDims(topNDims, topNChildren int) ([]*FacetResult, error) {
	res, err := f.GetAllDims(topNChildren)
	if err != nil {
		return nil, err
	}
	limit := topNDims
	if len(res) < topNDims {
		limit = len(res)
	}
	return res[:limit], nil
}
