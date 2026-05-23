// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package charfilter provides base utilities for implementing CharFilters.
package charfilter

import (
	"io"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

const initialOffsetMapCapacity = 64

// BaseCharFilter is the base utility class for implementing an
// analysis.CharFilter. Subclasses record offset-correction mappings by calling
// AddOffCorrectMap and retrieve the cumulative correction via Correct.
//
// The underlying offset-correction table stores (outputOffset, cumulativeDiff)
// pairs in ascending output-offset order. Correct performs a binary search to
// find the applicable correction for any query offset.
//
// This is the Go port of
// org.apache.lucene.analysis.charfilter.BaseCharFilter from
// Apache Lucene 10.4.0.
//
// Deviation: Lucene's BaseCharFilter extends CharFilter (which extends
// java.io.FilterReader) and overrides correctOffset. Gocene's CharFilter is a
// struct rather than an interface hierarchy; BaseCharFilter therefore embeds
// *analysis.CharFilter and overrides CorrectOffset and CorrectOffsetChained.
type BaseCharFilter struct {
	*analysis.CharFilter

	offsets []int // output-stream offsets at which corrections apply
	diffs   []int // cumulative diff at each recorded offset
	size    int   // number of entries recorded
}

// NewBaseCharFilter creates a new BaseCharFilter wrapping the given reader.
func NewBaseCharFilter(input io.Reader) *BaseCharFilter {
	return &BaseCharFilter{
		CharFilter: analysis.NewCharFilter(input),
	}
}

// Correct returns the corrected input offset for the given output offset.
// If no corrections have been recorded, the offset is returned unchanged.
func (f *BaseCharFilter) Correct(currentOff int) int {
	if f.offsets == nil {
		return currentOff
	}

	// Binary search for the largest recorded offset <= currentOff.
	idx := sort.SearchInts(f.offsets[:f.size], currentOff)
	// sort.SearchInts returns the first index where offsets[i] >= currentOff.
	// We want the last index where offsets[i] <= currentOff.
	if idx < f.size && f.offsets[idx] == currentOff {
		return currentOff + f.diffs[idx]
	}
	// idx is the insertion point; the entry before it (if any) applies.
	if idx == 0 {
		return currentOff
	}
	return currentOff + f.diffs[idx-1]
}

// CorrectOffset overrides analysis.CharFilter.CorrectOffset to use the
// binary-search correction table. If the wrapped input is itself a
// BaseCharFilter (or exposes CorrectOffset), the correction is chained.
func (f *BaseCharFilter) CorrectOffset(currentOff int) int {
	corrected := f.Correct(currentOff)
	// Chain through input if it also supports CorrectOffset.
	type offsetCorrector interface {
		CorrectOffset(int) int
	}
	if inner, ok := f.GetInput().(offsetCorrector); ok {
		return inner.CorrectOffset(corrected)
	}
	return corrected
}

// GetLastCumulativeDiff returns the last cumulative diff recorded, or 0 if
// no mappings have been added.
func (f *BaseCharFilter) GetLastCumulativeDiff() int {
	if f.offsets == nil {
		return 0
	}
	return f.diffs[f.size-1]
}

// AddOffCorrectMap records an offset-correction mapping.
//
// off is the output-stream offset at which the correction takes effect.
// cumulativeDiff is the difference to add to an output offset to obtain the
// corresponding input offset (inputOff = outputOff + cumulativeDiff).
//
// Successive calls must pass non-decreasing values of off. If off equals the
// last recorded offset, the entry is overwritten; this matches the Java
// behaviour of always recording the most recent diff for the same position.
func (f *BaseCharFilter) AddOffCorrectMap(off, cumulativeDiff int) {
	if f.offsets == nil {
		f.offsets = make([]int, initialOffsetMapCapacity)
		f.diffs = make([]int, initialOffsetMapCapacity)
	} else if f.size == len(f.offsets) {
		newCap := f.size * 2
		newOffsets := make([]int, newCap)
		newDiffs := make([]int, newCap)
		copy(newOffsets, f.offsets)
		copy(newDiffs, f.diffs)
		f.offsets = newOffsets
		f.diffs = newDiffs
	}

	if f.size == 0 || off != f.offsets[f.size-1] {
		f.offsets[f.size] = off
		f.diffs[f.size] = cumulativeDiff
		f.size++
	} else {
		// Overwrite the diff at the last recorded offset.
		f.diffs[f.size-1] = cumulativeDiff
	}
}

// GetInput returns the underlying io.Reader passed at construction.
func (f *BaseCharFilter) GetInput() io.Reader {
	return f.CharFilter.GetInput()
}
