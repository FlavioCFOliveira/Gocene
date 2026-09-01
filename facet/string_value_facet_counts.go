// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// StringValueFacetCounts computes facet counts from a previously indexed SortedSetDocValues or SortedDocValues field.
//
// This is the Go port of Lucene's org.apache.lucene.facet.StringValueFacetCounts.
type StringValueFacetCounts struct {
	reader      *index.IndexReader
	field       string
	ordinalMap  *index.OrdinalMap
	docValues   index.SortedSetDocValues
	denseCounts []int
	sparseCounts map[int]int
	initialized  bool
	cardinality  int
	totalDocCount int
}

// NewStringValueFacetCountsCoutns returns all facet counts for the field.
func NewStringValueFacetCounts(state *StringDocValuesReaderState) (*StringValueFacetCounts, error) {
	return NewStringValueFacetCountsWithCollector(state, nil)
}

// NewStringValueFacetCountsWithCollector counts facets across the provided hits.
func NewStringValueFacetCountsWithCollector(state *StringDocValuesReaderState, facetsCollector *FacetsCollector) (*StringValueFacetCounts, error) {
	dv, err := getDocValues(state)
	if err != nil {
		return nil, err
	}

	valueCount := dv.GetValueCount()
	if valueCount > 2147483647 { // Integer.MAX_VALUE
		return nil, fmt.Errorf("can only handle valueCount < 2147483647; got %d", valueCount)
	}
	cardinality := int(valueCount)

	f := &StringValueFacetCounts{
		reader:     state.Reader,
		field:      state.Field,
		ordinalMap: state.OrdinalMap,
		docValues:   dv,
		cardinality: cardinality,
	}

	if facetsCollector != nil {
		if cardinality < 1024 {
			f.initialized = false
			if err := f.count(facetsCollector); err != nil {
				return nil, err
			}
		} else {
			totalHits := 0
			totalDocs := 0
			for _, matchingDocs := range facetsCollector.GetMatchingDocs() {
				totalHits += matchingDocs.TotalHits
				totalDocs += matchingDocs.Context.Reader().MaxDoc()
			}

			if totalHits == 0 {
				f.initialized = true
			} else {
				if totalHits < totalDocs/10 {
					f.sparseCounts = make(map[int]int)
					f.initialized = true
				} else {
					f.denseCounts = make([]int, cardinality)
					f.initialized = true
				}
				if err := f.count(facetsCollector); err != nil {
					return nil, err
				}
			}
		}
	} else {
		f.denseCounts = make([]int, cardinality)
		f.initialized = true
		if err := f.countAll(); err != nil {
			return nil, err
		}
	}

	return f, nil
}

func getDocValues(state *StringDocValuesReaderState) (index.SortedSetDocValues, error) {
	leaves := state.Reader.Leaves()
	if len(leaves) == 0 {
		return index.EmptySortedSet(), nil
	}
	if len(leaves) == 1 {
		return index.GetSortedSet(leaves[0].Reader(), state.Field)
	}

	docValues := make([]index.SortedSetDocValues, len(leaves))
	starts := make([]int, len(leaves)+1)
	var cost int64
	for i := 0; i < len(leaves); i++ {
		context := leaves[i]
		dv, err := index.GetSortedSet(context.Reader(), state.Field)
		if err != nil {
			return nil, err
		}
		docValues[i] = dv
		starts[i] = context.DocBase
		cost += dv.Cost()
	}
	starts[len(leaves)] = state.Reader.MaxDoc()

	return index.NewMultiSortedSetDocValues(docValues, starts, state.OrdinalMap, cost), nil
}

func (f *StringValueFacetCounts) count(facetsCollector *FacetsCollector) error {
	matchingDocs := facetsCollector.GetMatchingDocs()
	if len(matchingDocs) == 0 {
		return nil
	}

	if len(matchingDocs) == 1 {
		hits := matchingDocs[0]
		if hits.TotalHits == 0 {
			return nil
		}
		return f.countOneSegment(f.docValues, hits.Context.Ord, hits, nil)
	}

	for _, hits := range matchingDocs {
		if hits.TotalHits == 0 {
			continue
		}
		multiValues := f.docValues.(*index.MultiSortedSetDocValues)
		if err := f.countOneSegment(multiValues.Values[hits.Context.Ord], hits.Context.Ord, hits, nil); err != nil {
			return err
		}
	}
	return nil
}

func (f *StringValueFacetCounts) countAll() error {
	leaves := f.reader.Leaves()
	numLeaves := len(leaves)
	if numLeaves == 0 {
		return nil
	}

	if numLeaves == 1 {
		context := leaves[0]
		liveDocs := context.Reader().GetLiveDocs()
		if liveDocs == nil {
			return f.countOneSegmentNHLD(f.docValues, context.Ord)
		}
		return f.countOneSegment(f.docValues, context.Ord, nil, liveDocs)
	}

	multiValues := f.docValues.(*index.MultiSortedSetDocValues)
	for i := 0; i < numLeaves; i++ {
		context := leaves[i]
		liveDocs := context.Reader().GetLiveDocs()
		if liveDocs == nil {
			if err := f.countOneSegmentNHLD(multiValues.Values[i], context.Ord); err != nil {
				return err
			}
		} else {
			if err := f.countOneSegment(multiValues.Values[i], context.Ord, nil, liveDocs); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *StringValueFacetCounts) countOneSegment(multiValues index.SortedSetDocValues, segmentOrd int, hits *MatchingDocs, liveDocs util.Bits) error {
	singleValues := index.UnwrapSingleton(multiValues)
	var valuesIt search.DocIdSetIterator
	if singleValues != nil {
		valuesIt = singleValues
	} else {
		valuesIt = multiValues
	}

	var it search.DocIdSetIterator
	if hits == nil {
		if liveDocs == nil {
			return fmt.Errorf("liveDocs must be provided when hits is nil")
		}
		it = facet.LiveDocsDISI(valuesIt, liveDocs)
	} else {
		it = search.IntersectIterators([]search.DocIdSetIterator{hits.Bits.Iterator(), valuesIt})
	}

	if f.ordinalMap == nil {
		if singleValues != nil {
			for doc := it.NextDoc(); doc != search.NoMoreDocs; doc = it.NextDoc() {
				f.increment(singleValues.OrdValue())
				f.totalDocCount++
			}
		} else {
			for doc := it.NextDoc(); doc != search.NoMoreDocs; doc = it.NextDoc() {
				booleanCounted := false
				for i := 0; i < multiValues.DocValueCount(); i++ {
					term := multiValues.NextOrd()
					f.increment(term)
					if !booleanCounted {
						f.totalDocCount++
						booleanCounted = true
					}
				}
			}
		}
	} else {
		ordMap := f.ordinalMap.GetGlobalOrds(segmentOrd)
		if hits != nil && hits.TotalHits < int(multiValues.GetValueCount())/10 {
			if singleValues != nil {
				for doc := it.NextDoc(); doc != search.NoMoreDocs; doc = it.NextDoc() {
					f.increment(int(ordMap.Get(singleValues.OrdValue())))
					f.totalDocCount++
				}
			} else {
				for doc := it.NextDoc(); doc != search.NoMoreDocs; doc = it.NextDoc() {
					booleanCounted := false
					for i := 0; i < multiValues.DocValueCount(); i++ {
						term := multiValues.NextOrd()
						f.increment(int(ordMap.Get(term)))
						if !booleanCounted {
							f.totalDocCount++
							booleanCounted = true
						}
					}
				}
			}
		} else {
			segmentCardinality := int(multiValues.GetValueCount())
			segCounts := make([]int, segmentCardinality)
			if singleValues != nil {
				for doc := it.NextDoc(); doc != search.NoMoreDocs; doc = it.NextDoc() {
					segCounts[singleValues.OrdValue()]++
					f.totalDocCount++
				}
			} else {
				for doc := it.NextDoc(); doc != search.NoMoreDocs; doc = it.NextDoc() {
					booleanCounted := false
					for i := 0; i < multiValues.DocValueCount(); i++ {
						term := multiValues.NextOrd()
						segCounts[term]++
						if !booleanCounted {
							f.totalDocCount++
							booleanCounted = true
						}
					}
				}
			}
			for ord, count := range segCounts {
				if count != 0 {
					f.increment(int(ordMap.Get(int64(ord))), count)
				}
			}
		}
	}
	return nil
}

func (f *StringValueFacetCounts) countOneSegmentNHLD(multiValues index.SortedSetDocValues, segmentOrd int) error {
	singleValues := index.UnwrapSingleton(multiValues)
	if f.ordinalMap == nil {
		if singleValues != nil {
			for doc := singleValues.NextDoc(); doc != search.NoMoreDocs; doc = singleValues.NextDoc() {
				f.increment(singleValues.OrdValue())
				f.totalDocCount++
			}
		} else {
			for doc := multiValues.NextDoc(); doc != search.NoMoreDocs; doc = multiValues.NextDoc() {
				booleanCounted := false
				for i := 0; i < multiValues.DocValueCount(); i++ {
					term := multiValues.NextOrd()
					f.increment(term)
					if !booleanCounted {
						f.totalDocCount++
						booleanCounted = true
					}
				}
			}
		}
	} else {
		ordMap := f.ordinalMap.GetGlobalOrds(segmentOrd)
		segmentCardinality := int(multiValues.GetValueCount())
		segCounts := make([]int, segmentCardinality)
		if singleValues != nil {
			for doc := singleValues.NextDoc(); doc != search.NoMoreDocs; doc = singleValues.NextDoc() {
				segCounts[singleValues.OrdValue()]++
				f.totalDocCount++
			}
		} else {
			for doc := multiValues.NextDoc(); doc != search.NoMoreDocs; doc = multiValues.NextDoc() {
				booleanCounted := false
				for i := 0; i < multiValues.DocValueCount(); i++ {
					term := multiValues.NextOrd()
					segCounts[term]++
					if !booleanCounted {
						f.totalDocCount++
						booleanCounted = true
					}
				}
			}
		}
		for ord, count := range segCounts {
			if count != 0 {
				f.increment(int(ordMap.Get(int64(ord))), count)
			}
		}
	}
	return nil
}

func (f *StringValueFacetCounts) increment(ordinal int) {
	f.incrementWithAmount(ordinal, 1)
}

func (f *StringValueFacetCounts) incrementWithAmount(ordinal int, amount int) {
	if f.sparseCounts != nil {
		f.sparseCounts[ordinal] += amount
	} else {
		f.denseCounts[ordinal] += amount
	}
}

func (f *StringValueFacetCounts) GetAllChildren(dim string, path ...string) (*FacetResult, error) {
	if dim != f.field {
		return nil, fmt.Errorf("invalid dim %q; should be %q", dim, f.field)
	}
	if len(path) != 0 {
		return nil, fmt.Errorf("path.length should be 0")
	}

	var labelValues []*LabelAndValue
	if f.sparseCounts != nil {
		for ord, count := range f.sparseCounts {
			term := f.docValues.LookupOrd(ord)
			labelValues = append(labelValues, NewLabelAndValue(term.UTF8ToString(), count))
			labelValues[len(labelValues)-1].Count = count
		}
	} else if f.denseCounts != nil {
		for i, count := range f.denseCounts {
			if count != 0 {
				term := f.docValues.LookupOrd(i)
				labelValues = append(labelValues, NewLabelAndValue(term.UTF8ToString(), count))
				labelValues[len(labelValues)-1].Count = count
			}
		}
	}

	return NewFacetResult(f.field, []string{}, f.totalDocCount, labelValues, len(labelValues)), nil
}

func (f *StringValueFacetCounts) GetTopChildren(topN int, dim string, path ...string) (*FacetResult, error) {
	if err := ValidateTopN(topN); err != nil {
		return nil, err
	}
	if dim != f.field {
		return nil, fmt.Errorf("invalid dim %q; should be %q", dim, f.field)
	}
	if len(path) != 0 {
		return nil, fmt.Errorf("path.length should be 0")
	}

	topN = min(topN, f.cardinality)
	type entry struct {
		ord   int
		count int
	}
	var q []entry
	childCount := 0

	if f.sparseCounts != nil {
		for ord, count := range f.sparseCounts {
			childCount++
			q = append(q, entry{ord, count})
		}
	} else if f.denseCounts != nil {
		for i, count := range f.denseCounts {
			if count != 0 {
				childCount++
				q = append(q, entry{i, count})
			}
		}
	}

	sort.Slice(q, func(i, j int) bool {
		if q[i].count != q[j].count {
			return q[i].count > q[j].count
		}
		return q[i].ord < q[j].ord
	})

	limit := topN
	if len(q) < topN {
		limit = len(q)
	}

	var results []*LabelAndValue
	for i := 0; i < limit; i++ {
		e := q[i]
		term := f.docValues.LookupOrd(e.ord)
		results = append(results, NewLabelAndValue(term.UTF8ToString(), e.ord))
		results[len(results)-1].Count = e.count
	}

	return NewFacetResult(f.field, []string{}, f.totalDocCount, results, childCount), nil
}

func (f *StringValueFacetCounts) GetSpecificValue(dim string, path ...string) (interface{}, error) {
	if dim != f.field {
		return nil, fmt.Errorf("invalid dim %q; should be %q", dim, f.field)
	}
	if len(path) != 1 {
		return nil, fmt.Errorf("path must be length=1")
	}
	ord := f.docValues.LookupTerm(index.NewBytesRef(path[0]))
	if ord < 0 {
		return -1, nil
	}

	if f.sparseCounts != nil {
		return f.sparseCounts[ord], nil
	}
	if f.denseCounts != nil {
		return f.denseCounts[ord], nil
	}
	return 0, nil
}

func (f *StringValueFacetCounts) GetAllDims(topN int) ([]*FacetResult, error) {
	res, err := f.GetTopChildren(topN, f.field)
	if err != nil {
		return nil, err
	}
	return []*FacetResult{res}, nil
}

func (f *StringValueFacetCounts) GetTopDims(topNDims, topNChildren int) ([]*FacetResult, error) {
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
