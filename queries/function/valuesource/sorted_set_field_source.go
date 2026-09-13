// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// SortedSetSelectorType mirrors org.apache.lucene.search.SortedSetSelector.Type.
type SortedSetSelectorType int

const (
	SortedSetSelectorMin SortedSetSelectorType = iota
	SortedSetSelectorMax
)

func (t SortedSetSelectorType) String() string {
	switch t {
	case SortedSetSelectorMin:
		return "MIN"
	case SortedSetSelectorMax:
		return "MAX"
	default:
		return "UNKNOWN"
	}
}

// SortedSetFieldSource retrieves [function.FunctionValues] instances for
// multi-valued string based fields.
type SortedSetFieldSource struct {
	FieldCacheSource
	Selector SortedSetSelectorType
}

func NewSortedSetFieldSource(field string) *SortedSetFieldSource {
	return NewSortedSetFieldSourceWithSelector(field, SortedSetSelectorMin)
}

func NewSortedSetFieldSourceWithSelector(field string, selector SortedSetSelectorType) *SortedSetFieldSource {
	return &SortedSetFieldSource{
		FieldCacheSource: FieldCacheSource{Field: field},
		Selector:         selector,
	}
}

func (f *SortedSetFieldSource) Description() string {
	return fmt.Sprintf("sortedset(%s,selector=%s)", f.Field, f.Selector)
}

func (f *SortedSetFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	// In Lucene, SortedSetSelector.wrap(sortedSet, selector) is used.
	// In Gocene, we rely on DocTermsIndexDocValues to surface SortedDocValues.
	// However, SortedSetDocValues can contain multiple values per doc.
	// Lucene's SortedSetSelector.wrap returns a SortedDocValues view that
	// selects a single representative value.

	// Since Gocene's SortedSetDocValues doesn't have a built-in "Selector" wrapper yet,
	// and SortedSetFieldSource expects a single value, we must implement the selection.

	sdv, err := readerContext.LeafReader().GetSortedSetDocValues(f.Field)
	if err != nil {
		return nil, err
	}

	// Wrap the SortedSetDocValues in a view that selects MIN or MAX.
	view := wrapSortedSet(sdv, f.Selector)

	dv := docvalues.NewDocTermsIndexDocValuesFromDV(f, view)
	dv.SetSelf(dv)

	return &sortedSetFallback{
		DocTermsIndexDocValues: dv,
	}, nil
}

type sortedSetFallback struct {
	*docvalues.DocTermsIndexDocValues
}

func (f *sortedSetFallback) ObjectVal(doc int) (any, error) {
	return f.StrVal(doc)
}

// wrapSortedSet provides a SortedDocValues view of a SortedSetDocValues
// based on the selected representative value (MIN or MAX).
func wrapSortedSet(sdv index.SortedSetDocValues, selector SortedSetSelectorType) index.SortedDocValues {
	return &sortedSetView{
		sdv:      sdv,
		selector: selector,
	}
}

type sortedSetView struct {
	sdv      index.SortedSetDocValues
	selector SortedSetSelectorType
}

func (v *sortedSetView) DocID() int {
	return v.sdv.DocID()
}

func (v *sortedSetView) NextDoc() (int, error) {
	return v.sdv.NextDoc()
}

func (v *sortedSetView) Advance(target int) (int, error) {
	return v.sdv.Advance(target)
}

func (v *sortedSetView) AdvanceExact(target int) (bool, error) {
	return v.sdv.AdvanceExact(target)
}

func (v *sortedSetView) OrdValue() (int, error) {
	// Select the representative ordinal
	if v.selector == SortedSetSelectorMin {
		// The first ordinal for the document is the MIN.
		// SortedSetDocValues.NextOrd() returns the first one for the current doc
		// if we just advanced to it.
		return v.sdv.NextOrd()
	} else {
		// MAX: we must iterate through all ordinals for this doc.
		var lastOrd int = -1
		for {
			ord, err := v.sdv.NextOrd()
			if err != nil {
				return -1, err
			}
			if ord == -1 {
				break
			}
			lastOrd = ord
		}
		return lastOrd, nil
	}
}

func (v *sortedSetView) LookupOrd(ord int) ([]byte, error) {
	return v.sdv.LookupOrd(ord)
}

func (v *sortedSetView) GetValueCount() int {
	return v.sdv.GetValueCount()
}
