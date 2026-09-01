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

// LongValueFacetCounts is a Facets implementation that computes counts for all unique long values.
//
// This is the Go port of Lucene's org.apache.lucene.facet.LongValueFacetCounts.
type LongValueFacetCounts struct {
	counts      []int
	hashCounts  map[int64]int
	initialized bool
	field       string
	totCount    int
}

// NewLongValueFacetCounts creates a new LongValueFacetCounts.
func NewLongValueFacetCounts(field string, hits *FacetsCollector) error {
	f := &LongValueFacetCounts{
		field: field,
	}
	if err := f.count(nil, hits.GetMatchingDocs()); err != nil {
		return err
	}
	return nil
}

func (f *LongValueFacetCounts) initializeCounters() {
	if f.initialized {
		return
	}
	f.counts = make([]int, 1024)
	f.hashCounts = make(map[int64]int)
	f.initialized = true
}

func (f *LongValueFacetCounts) increment(value int64) {
	if value >= 0 && value < int64(len(f.counts)) {
		f.counts[value]++
	} else {
		f.hashCounts[value]++
	}
}

func (f *LongValueFacetCounts) count(valueSource search.LongValuesSource, matchingDocs []*MatchingDocs) error {
	for _, hits := range matchingDocs {
		if hits.TotalHits == 0 {
			continue
		}
		f.initializeCounters()

		var fv search.LongValues
		if valueSource != nil {
			// Gocene's LongValuesSource.GetValues returns []int64, not an iterator.
			// We must adapt this to the iterator-like logic of the original Lucene code.
			// However, the Lucene source for LongValueFacetCounts.count() uses LongValues.advanceExact.
			// Let's check if search.LongValuesSource provides a way to get an iterator.
			// Based on search/long_values_source.go, it doesn't.
			// But we can use the slice.
			values, err := valueSource.GetValues(hits.Context)
			if err != nil {
				return err
			}
			for doc, val := range values {
				if hits.Bits.Get(doc) {
					f.increment(val)
					f.totCount++
				}
			}
		} else {
			// Field-based counting
			dv, err := index.GetNumericDocValues(hits.Context, f.field)
			if err != nil {
				return err
			}
			if dv == nil {
				continue
			}

			// Use the iterator provided by NumericDocValues
			for doc := dv.NextDoc(); doc != search.NoMoreDocs; doc = dv.NextDoc() {
				if hits.Bits.Get(doc) {
					val, err := dv.LongValue()
					if err != nil {
						return err
					}
					f.increment(val)
					f.totCount++
				}
			}
		}
	}
	return nil
}

func (f *LongValueFacetCounts) GetAllChildren(dim string, path ...string) (*FacetResult, error) {
	if dim != f.field {
		return nil, fmt.Errorf("invalid dim %q; should be %q", dim, f.field)
	}
	if len(path) != 0 {
		return nil, fmt.Errorf("path.length should be 0")
	}

	if !f.initialized {
		return NewFacetResult(f.field, []string{}, f.totCount, []*LabelAndValue{}, 0), nil
	}

	var labelValues []*LabelAndValue
	for i, count := range f.counts {
		if count != 0 {
			labelValues = append(labelValues, NewLabelAndValue(fmt.Sprintf("%d", i), i))
			labelValues[len(labelValues)-1].Count = count
		}
	}
	for val, count := range f.hashCounts {
		if count != 0 {
			labelValues = append(labelValues, NewLabelAndValue(fmt.Sprintf("%d", val), val))
			labelValues[len(labelValues)-1].Count = count
		}
	}

	return NewFacetResult(f.field, []string{}, f.totCount, labelValues, len(labelValues)), nil
}

func (f *LongValueFacetCounts) GetTopChildren(topN int, dim string, path ...string) (*FacetResult, error) {
	if err := ValidateTopN(topN); err != nil {
		return nil, err
	}
	if dim != f.field {
		return nil, fmt.Errorf("invalid dim %q; should be %q", dim, f.field)
	}
	if len(path) != 0 {
		return nil, fmt.Errorf("path.length should be 0")
	}

	if !f.initialized {
		return NewFacetResult(f.field, []string{}, f.totCount, []*LabelAndValue{}, 0), nil
	}

	type entry struct {
		value int64
		count int
	}
	var entries []entry
	for i, count := range f.counts {
		if count != 0 {
			entries = append(entries, entry{int64(i), count})
		}
	}
	for val, count := range f.hashCounts {
		if count != 0 {
			entries = append(entries, entry{val, count})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].value < entries[j].value
	})

	limit := topN
	if len(entries) < topN {
		limit = len(entries)
	}

	var results []*LabelAndValue
	for i := 0; i < limit; i++ {
		e := entries[i]
		results = append(results, NewLabelAndValue(fmt.Sprintf("%d", e.value), e.value))
		results[len(results)-1].Count = e.count
	}

	return NewFacetResult(f.field, []string{}, f.totCount, results, len(entries)), nil
}

func (f *LongValueFacetCounts) GetSpecificValue(dim string, path ...string) (interface{}, error) {
	panic("unsupported")
}

func (f *LongValueFacetCounts) GetAllDims(topN int) ([]*FacetResult, error) {
	res, err := f.GetTopChildren(topN, f.field)
	if err != nil {
		return nil, err
	}
	return []*FacetResult{res}, nil
}

func (f *LongValueFacetCounts) GetTopDims(topNDims, topNChildren int) ([]*FacetResult, error) {
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
