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

// DoubleValueFacetCounts is a Facets implementation that computes counts for all unique double values.
//
// This is the Go port of Lucene's org.apache.lucene.facet.DoubleValueFacetCounts.
type DoubleValueFacetCounts struct {
	hashCounts  map[float64]int
	initialized bool
	field       string
	totCount    int
}

// NewDoubleValueFacetCounts creates a new DoubleValueFacetCounts.
func NewDoubleValueFacetCounts(field string, hits *FacetsCollector) error {
	f := &DoubleValueFacetCounts{
		field: field,
	}
	if err := f.count(nil, hits.GetMatchingDocs()); err != nil {
		return err
	}
	return nil
}

func (f *DoubleValueFacetCounts) initializeCounters() {
	if f.initialized {
		return
	}
	f.hashCounts = make(map[float64]int)
	f.initialized = true
}

func (f *DoubleValueFacetCounts) increment(value float64) {
	f.hashCounts[value]++
}

func (f *DoubleValueFacetCounts) count(valueSource search.DoubleValueSource, matchingDocs []*MatchingDocs) error {
	for _, hits := range matchingDocs {
		if hits.TotalHits == 0 {
			continue
		}
		f.initializeCounters()

		if valueSource != nil {
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
			dv, err := index.GetNumericDocValues(hits.Context, f.field)
			if err != nil {
				return err
			}
			if dv == nil {
				continue
			}

			for doc := dv.NextDoc(); doc != search.NoMoreDocs; doc = dv.NextDoc() {
				if hits.Bits.Get(doc) {
					val, err := dv.DoubleValue()
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

func (f *DoubleValueFacetCounts) GetAllChildren(dim string, path ...string) (*FacetResult, error) {
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
	for val, count := range f.hashCounts {
		if count != 0 {
			labelValues = append(labelValues, NewLabelAndValue(fmt.Sprintf("%f", val), val))
			labelValues[len(labelValues)-1].Count = count
		}
	}

	return NewFacetResult(f.field, []string{}, f.totCount, labelValues, len(labelValues)), nil
}

func (f *DoubleValueFacetCounts) GetTopChildren(topN int, dim string, path ...string) (*FacetResult, error) {
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
		value float64
		count int
	}
	var entries []entry
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
		results = append(results, NewLabelAndValue(fmt.Sprintf("%f", e.value), e.value))
		results[len(results)-1].Count = e.count
	}

	return NewFacetResult(f.field, []string{}, f.totCount, results, len(entries)), nil
}

func (f *DoubleValueFacetCounts) GetSpecificValue(dim string, path ...string) (interface{}, error) {
	panic("unsupported")
}

func (f *DoubleValueFacetCounts) GetAllDims(topN int) ([]*FacetResult, error) {
	res, err := f.GetTopChildren(topN, f.field)
	if err != nil {
		return nil, err
	}
	return []*FacetResult{res}, nil
}

func (f *DoubleValueFacetCounts) GetTopDims(topNDims, topNChildren int) ([]*FacetResult, error) {
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
