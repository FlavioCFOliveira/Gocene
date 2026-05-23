// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package taxonomy

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/facets"
)

const (
	// InvalidOrdinal is the sentinel value returned by taxonomy traversal
	// when there is no parent, child, or sibling. Mirrors
	// TaxonomyReader.INVALID_ORDINAL.
	InvalidOrdinal = -1

	// RootOrdinal is the ordinal of the taxonomy root category. Mirrors
	// TaxonomyReader.ROOT_ORDINAL.
	RootOrdinal = 0
)

// TaxonomyReaderI is the minimal interface consumed by TaxonomyFacets. It
// exposes only what the facet aggregation logic needs; concrete reader types
// (DirectoryTaxonomyReader, etc.) implement this by satisfying the method set.
type TaxonomyReaderI interface {
	// GetSize returns the number of categories in the taxonomy (exclusive
	// upper bound for all ordinals).
	GetSize() int

	// GetParallelTaxonomyArrays returns the parallel arrays used for tree
	// traversal.
	GetParallelTaxonomyArrays() ParallelTaxonomyArrays

	// GetOrdinal returns the ordinal for the given path components, or
	// InvalidOrdinal when not found. path[0] is the dimension, path[1..] are
	// optional sub-path components.
	GetOrdinal(path ...string) int

	// GetPath returns the path components for the given ordinal.
	GetPath(ord int) []string
}

// dimConfigI is the minimal set of DimConfig fields consumed by TaxonomyFacets.
type dimConfigI struct {
	Hierarchical    bool
	MultiValued     bool
	RequireDimCount bool
	IndexFieldName  string
}

// TaxonomyFacets is the base struct for taxonomy-based faceting. Concrete
// types embed this and supply aggregation-specific logic via the function
// fields. Mirrors org.apache.lucene.facet.taxonomy.TaxonomyFacets.
type TaxonomyFacets struct {
	// IndexFieldName is the index field this instance is counting against.
	IndexFieldName string

	// TaxoReader provides taxonomy metadata.
	TaxoReader TaxonomyReaderI

	// Config holds the per-dimension facet configuration.
	Config *facets.FacetsConfig

	// parents is the cached parents array from the taxonomy.
	parents []int

	// children is lazily loaded from the parallel arrays.
	children []int

	// siblings is lazily loaded from the parallel arrays.
	siblings []int

	// counts is the dense ordinal counter (used when index is large enough
	// or there is no FacetsCollector).
	counts []int

	// initialized tracks whether counting has been seeded.
	initialized bool

	// --- hooks for concrete subtypes ---

	// GetAggregationValue returns the aggregated value for an ordinal.
	// Defaults to getCount(ord) for count-based impls.
	GetAggregationValueFn func(ord int) float64

	// AggregateFn combines an existing and new value.
	AggregateFn func(existing, newVal float64) float64

	// MakeTopQueueFn creates the priority queue used for top-N children.
	MakeTopQueueFn func(topN int) *facets.TopOrdAndIntQueue

	// SetIncomingValueFn populates the incoming queue entry for ord.
	SetIncomingValueFn func(q *facets.TopOrdAndIntQueue, ord int)
}

// NewTaxonomyFacets allocates the base struct. Call initCounters before use.
func NewTaxonomyFacets(
	indexFieldName string,
	taxoReader TaxonomyReaderI,
	config *facets.FacetsConfig,
) *TaxonomyFacets {
	tf := &TaxonomyFacets{
		IndexFieldName: indexFieldName,
		TaxoReader:     taxoReader,
		Config:         config,
		parents:        taxoReader.GetParallelTaxonomyArrays().Parents(),
	}
	// Wire default int-based hooks.
	tf.GetAggregationValueFn = func(ord int) float64 { return float64(tf.getCount(ord)) }
	tf.AggregateFn = func(a, b float64) float64 { return a + b }
	tf.MakeTopQueueFn = func(topN int) *facets.TopOrdAndIntQueue {
		cap := topN
		if sz := taxoReader.GetSize(); sz < cap {
			cap = sz
		}
		return facets.NewTopOrdAndIntQueue(cap)
	}
	tf.SetIncomingValueFn = func(q *facets.TopOrdAndIntQueue, ord int) {
		q.InsertInt(ord, int32(tf.getCount(ord)))
	}
	return tf
}

// initCounters allocates the dense count buffer. Must be called before any
// counting begins.
func (tf *TaxonomyFacets) initCounters() {
	if tf.initialized {
		return
	}
	tf.initialized = true
	tf.counts = make([]int, tf.TaxoReader.GetSize())
}

// IncrementCount adds delta to countBuffer[ord].
func (tf *TaxonomyFacets) IncrementCount(ord, delta int) {
	if !tf.initialized {
		tf.initCounters()
	}
	tf.counts[ord] += delta
}

// SetCount sets countBuffer[ord] to newValue.
func (tf *TaxonomyFacets) SetCount(ord, newValue int) {
	if !tf.initialized {
		tf.initCounters()
	}
	tf.counts[ord] = newValue
}

func (tf *TaxonomyFacets) getCount(ord int) int {
	if !tf.initialized || ord < 0 || ord >= len(tf.counts) {
		return 0
	}
	return tf.counts[ord]
}

// HasValues reports whether any counts have been initialized.
func (tf *TaxonomyFacets) HasValues() bool { return tf.initialized }

// getChildren returns the first-child array (lazy init).
func (tf *TaxonomyFacets) getChildren() []int {
	if tf.children == nil {
		tf.children = tf.TaxoReader.GetParallelTaxonomyArrays().Children()
	}
	return tf.children
}

// getSiblings returns the sibling array (lazy init).
func (tf *TaxonomyFacets) getSiblings() []int {
	if tf.siblings == nil {
		tf.siblings = tf.TaxoReader.GetParallelTaxonomyArrays().Siblings()
	}
	return tf.siblings
}

func (tf *TaxonomyFacets) dimConfig(dim string) dimConfigI {
	dc := tf.Config.GetDimConfig(dim)
	if dc == nil {
		return dimConfigI{IndexFieldName: tf.IndexFieldName}
	}
	return dimConfigI{
		Hierarchical:    dc.Hierarchical,
		MultiValued:     dc.MultiValued,
		RequireDimCount: dc.RequireDimCount,
		IndexFieldName:  dc.IndexFieldName,
	}
}

// Rollup rolls up counts for single-valued hierarchical dimensions.
// Mirrors TaxonomyFacets.rollup().
func (tf *TaxonomyFacets) Rollup() error {
	if !tf.initialized {
		return nil
	}
	dims := tf.Config.GetDims()
	ch := tf.getChildren()
	for _, dim := range dims {
		dc := tf.dimConfig(dim)
		if dc.Hierarchical && !dc.MultiValued {
			dimRootOrd := tf.TaxoReader.GetOrdinal(dim)
			if dimRootOrd > 0 {
				tf.updateFromRollup(dimRootOrd, ch[dimRootOrd])
			}
		}
	}
	return nil
}

func (tf *TaxonomyFacets) updateFromRollup(ord, childOrd int) {
	tf.counts[ord] += tf.rollup(childOrd)
}

func (tf *TaxonomyFacets) rollup(ord int) int {
	ch := tf.getChildren()
	sib := tf.getSiblings()
	sum := 0
	for ord != InvalidOrdinal {
		cur := tf.getCount(ord)
		newVal := cur + tf.rollup(ch[ord])
		tf.counts[ord] = newVal
		sum += tf.counts[ord]
		ord = sib[ord]
	}
	return sum
}

// --- FacetResult accessors ---

// topChildrenForPath holds intermediate top-N state.
type topChildrenForPath struct {
	pathValue  float64
	childCount int
	q          *facets.TopOrdAndIntQueue
}

func (tf *TaxonomyFacets) getTopChildrenForPath(
	dc dimConfigI, pathOrd, topN int,
) *topChildrenForPath {
	q := tf.MakeTopQueueFn(topN)
	childCount := 0
	var aggregated float64

	ch := tf.getChildren()
	sib := tf.getSiblings()
	ord := ch[pathOrd]
	for ord != InvalidOrdinal {
		count := tf.getCount(ord)
		if count > 0 {
			aggregated = tf.AggregateFn(aggregated, tf.GetAggregationValueFn(ord))
			childCount++
			q.InsertInt(ord, int32(count))
		}
		ord = sib[ord]
	}

	pathValue := aggregated
	if dc.MultiValued {
		if dc.RequireDimCount {
			pathValue = tf.GetAggregationValueFn(pathOrd)
		} else {
			pathValue = -1
		}
	}
	return &topChildrenForPath{pathValue: pathValue, childCount: childCount, q: q}
}

func (tf *TaxonomyFacets) createFacetResult(
	top *topChildrenForPath, dim string, path []string,
) (*facets.FacetResult, error) {
	if top == nil || top.childCount == 0 {
		return nil, nil
	}
	q := top.q
	size := q.Size()
	type entry struct {
		ord   int
		count int32
	}
	entries := make([]entry, size)
	for i := 0; i < size; i++ {
		ord, v, _ := q.PopInt()
		entries[i] = entry{ord, v}
	}
	// Reverse (min-heap pops smallest first).
	for l, r := 0, len(entries)-1; l < r; l, r = l+1, r-1 {
		entries[l], entries[r] = entries[r], entries[l]
	}

	result := facets.NewFacetResultWithPath(dim, path)
	result.Value = int64(top.pathValue)
	result.ChildCount = top.childCount
	for _, e := range entries {
		parts := tf.TaxoReader.GetPath(e.ord)
		childIdx := len(path) + 1 // +1 for the dim component
		label := ""
		if childIdx < len(parts) {
			label = parts[childIdx]
		} else if len(parts) > 0 {
			label = parts[len(parts)-1]
		}
		result.AddLabelValue(facets.NewLabelAndValue(label, int64(e.count)))
	}
	return result, nil
}

// GetTopChildren returns the top N children for dim+path.
// Mirrors TaxonomyFacets.getTopChildren.
func (tf *TaxonomyFacets) GetTopChildren(topN int, dim string, path ...string) (*facets.FacetResult, error) {
	if topN <= 0 {
		return nil, fmt.Errorf("topN must be > 0 (got %d)", topN)
	}
	if !tf.initialized {
		return nil, nil
	}
	components := append([]string{dim}, path...)
	dimOrd := tf.TaxoReader.GetOrdinal(components...)
	if dimOrd == InvalidOrdinal {
		return nil, nil
	}
	dc := tf.dimConfig(dim)
	top := tf.getTopChildrenForPath(dc, dimOrd, topN)
	return tf.createFacetResult(top, dim, path)
}

// GetAllChildren returns all children with positive count for dim+path.
// Mirrors TaxonomyFacets.getAllChildren.
func (tf *TaxonomyFacets) GetAllChildren(dim string, path ...string) (*facets.FacetResult, error) {
	if !tf.initialized {
		return nil, nil
	}
	components := append([]string{dim}, path...)
	dimOrd := tf.TaxoReader.GetOrdinal(components...)
	if dimOrd == InvalidOrdinal {
		return nil, nil
	}
	dc := tf.dimConfig(dim)

	ch := tf.getChildren()
	sib := tf.getSiblings()

	var lv []*facets.LabelAndValue
	var aggregated float64
	aggregatedCount := 0

	cpLen := len(path) + 1 // dim + path components
	ord := ch[dimOrd]
	for ord != InvalidOrdinal {
		count := tf.getCount(ord)
		if count > 0 {
			aggregatedCount += count
			aggregated = tf.AggregateFn(aggregated, tf.GetAggregationValueFn(ord))
			parts := tf.TaxoReader.GetPath(ord)
			label := ""
			if cpLen < len(parts) {
				label = parts[cpLen]
			} else if len(parts) > 0 {
				label = parts[len(parts)-1]
			}
			lv = append(lv, facets.NewLabelAndValue(label, int64(count)))
		}
		ord = sib[ord]
	}

	if aggregatedCount == 0 {
		return nil, nil
	}

	aggValue := aggregated
	if dc.MultiValued {
		if dc.RequireDimCount {
			aggValue = tf.GetAggregationValueFn(dimOrd)
		} else {
			aggValue = -1
		}
	}

	result := facets.NewFacetResultWithPath(dim, path)
	result.Value = int64(aggValue)
	result.ChildCount = len(lv)
	for _, l := range lv {
		result.AddLabelValue(l)
	}
	return result, nil
}

// GetSpecificValue returns the aggregation value for the exact path dim+path.
// Mirrors TaxonomyFacets.getSpecificValue.
func (tf *TaxonomyFacets) GetSpecificValue(dim string, path ...string) (float64, error) {
	components := append([]string{dim}, path...)
	ord := tf.TaxoReader.GetOrdinal(components...)
	if ord < 0 {
		return -1, nil
	}
	if !tf.initialized {
		return 0, nil
	}
	return tf.GetAggregationValueFn(ord), nil
}

// GetAllDims returns one FacetResult per dimension, sorted by value desc.
// Mirrors TaxonomyFacets.getAllDims.
func (tf *TaxonomyFacets) GetAllDims(topN int) ([]*facets.FacetResult, error) {
	if topN <= 0 {
		return nil, fmt.Errorf("topN must be > 0 (got %d)", topN)
	}
	if !tf.HasValues() {
		return nil, nil
	}

	ch := tf.getChildren()
	sib := tf.getSiblings()

	var results []*facets.FacetResult
	ord := ch[RootOrdinal]
	for ord != InvalidOrdinal {
		parts := tf.TaxoReader.GetPath(ord)
		if len(parts) == 0 {
			ord = sib[ord]
			continue
		}
		dim := parts[0]
		dc := tf.dimConfig(dim)
		if dc.IndexFieldName == "" || dc.IndexFieldName == tf.IndexFieldName {
			fr, err := tf.GetTopChildren(topN, dim)
			if err != nil {
				return nil, err
			}
			if fr != nil {
				results = append(results, fr)
			}
		}
		ord = sib[ord]
	}

	sort.Slice(results, func(i, j int) bool {
		vi, vj := results[i].Value, results[j].Value
		if vi != vj {
			return vi > vj
		}
		return results[i].Dim < results[j].Dim
	})
	return results, nil
}
