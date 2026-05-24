// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.utils.BaseFacetBuilder.
package utils

import (
	"sort"

	"github.com/FlavioCFOliveira/Gocene/facets"
)

// OrdRecorder is the minimal interface over
// org.apache.lucene.sandbox.facet.recorders.CountFacetRecorder that
// BaseFacetBuilder requires.
type OrdRecorder interface {
	// RecordedOrds returns an iterator over ordinals that received at
	// least one record.
	RecordedOrds() OrdinalIterEx
	// GetCount returns the count for the given ordinal.
	GetCount(ord int) int
}

// OrdinalIterEx is the sentinel-based ordinal iterator that BaseFacetBuilder
// uses internally.
//
// Mirrors org.apache.lucene.sandbox.facet.iterators.OrdinalIterator (int
// nextOrd() / NO_MORE_ORDS).
type OrdinalIterEx interface {
	// NextOrd returns the next ordinal or NoMoreOrdsEx.
	NextOrd() int
}

// NoMoreOrdsEx is the sentinel returned by OrdinalIterEx.NextOrd when there
// are no more ordinals.
const NoMoreOrdsEx = -1

// OrdToLabelEx maps an ordinal to its human-readable label.
//
// Mirrors org.apache.lucene.sandbox.facet.labels.OrdToLabel.
type OrdToLabelEx interface {
	GetLabel(ord int) string
}

// ComparableSupplierEx provides a per-ordinal sort key.
//
// Mirrors org.apache.lucene.sandbox.facet.iterators.ComparableSupplier.
type ComparableSupplierEx interface {
	// Compare returns negative when ord a should sort before ord b.
	Compare(a, b int) int
}

// BaseFacetBuilderConfig holds the mutable configuration that BaseFacetBuilder
// sub-types manipulate via the WithTopN / WithSortByCount / WithSortByOrdinal
// methods.
//
// Callers embed this struct and call its methods to configure the builder.
type BaseFacetBuilderConfig struct {
	Dimension string
	Path      []string

	topN   int
	sortFn func(recorder OrdRecorder) ComparableSupplierEx
}

// NewBaseFacetBuilderConfig creates a config with the given dimension and path.
// The default sort order is by count (descending).
func NewBaseFacetBuilderConfig(dimension string, path ...string) *BaseFacetBuilderConfig {
	c := &BaseFacetBuilderConfig{
		Dimension: dimension,
		Path:      append([]string(nil), path...),
		topN:      -1,
	}
	c.sortFn = func(r OrdRecorder) ComparableSupplierEx {
		return &byCountComparable{recorder: r}
	}
	return c
}

// WithTopN limits the number of results returned.
//
// Mirrors BaseFacetBuilder.withTopN(int).
func (c *BaseFacetBuilderConfig) WithTopN(n int) *BaseFacetBuilderConfig {
	c.topN = n
	return c
}

// WithSortByCount restores the default count-descending sort order.
//
// Mirrors BaseFacetBuilder.withSortByCount().
func (c *BaseFacetBuilderConfig) WithSortByCount() *BaseFacetBuilderConfig {
	c.sortFn = func(r OrdRecorder) ComparableSupplierEx {
		return &byCountComparable{recorder: r}
	}
	return c
}

// WithSortByOrdinal switches to ascending ordinal sort.
//
// Mirrors BaseFacetBuilder.withSortByOrdinal().
func (c *BaseFacetBuilderConfig) WithSortByOrdinal() *BaseFacetBuilderConfig {
	c.sortFn = func(_ OrdRecorder) ComparableSupplierEx {
		return byOrdComparable{}
	}
	return c
}

// GetResult builds a FacetResult from the given recorder and label mapping.
//
// This is the shared implementation of BaseFacetBuilder.getResult() in Java.
// Callers (concrete sub-types) supply the recorder, the overall value, and
// the OrdToLabelEx for the dimension.
//
// Mirrors BaseFacetBuilder.getResult().
func (c *BaseFacetBuilderConfig) GetResult(
	recorder OrdRecorder,
	overallValue int64,
	ordToLabel OrdToLabelEx,
	matchingOrds OrdinalIterEx,
) *facets.FacetResult {
	// Collect all matching ordinals.
	var ords []int
	for {
		ord := matchingOrds.NextOrd()
		if ord == NoMoreOrdsEx {
			break
		}
		ords = append(ords, ord)
	}

	// Apply topN / sort.
	cmp := c.sortFn(recorder)
	if c.topN > 0 && len(ords) > c.topN {
		// Partial sort: keep only the top-N.
		sort.Slice(ords, func(i, j int) bool { return cmp.Compare(ords[i], ords[j]) < 0 })
		ords = ords[:c.topN]
	} else {
		sort.Slice(ords, func(i, j int) bool { return cmp.Compare(ords[i], ords[j]) < 0 })
	}

	fr := &facets.FacetResult{
		Dim:        c.Dimension,
		Path:       c.Path,
		Value:      overallValue,
		ChildCount: len(ords),
	}
	for _, ord := range ords {
		label := ordToLabel.GetLabel(ord)
		count := int64(recorder.GetCount(ord))
		fr.LabelValues = append(fr.LabelValues, facets.NewLabelAndValue(label, count))
	}
	return fr
}

// ---- comparables ----

// byCountComparable sorts ordinals by descending count.
type byCountComparable struct {
	recorder OrdRecorder
}

func (c *byCountComparable) Compare(a, b int) int {
	ca, cb := c.recorder.GetCount(a), c.recorder.GetCount(b)
	if ca > cb {
		return -1
	}
	if ca < cb {
		return 1
	}
	return 0
}

// byOrdComparable sorts ordinals by ascending ordinal value.
type byOrdComparable struct{}

func (byOrdComparable) Compare(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
