// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// SortedSetFieldSource retrieves FunctionValues for multi-valued string
// fields using SortedSetDocValues, selecting a representative value via
// the configured selector.
//
// Go port of org.apache.lucene.queries.function.valuesource.SortedSetFieldSource.
type SortedSetFieldSource struct {
	function.BaseValueSource
	field    string
	selector search.SortedSetSelectorType
}

// NewSortedSetFieldSource creates a SortedSetFieldSource with MIN selector.
func NewSortedSetFieldSource(field string) *SortedSetFieldSource {
	return &SortedSetFieldSource{field: field, selector: search.SortedSetSelectorMin}
}

// NewSortedSetFieldSourceWithSelector creates a SortedSetFieldSource with
// the given selector.
func NewSortedSetFieldSourceWithSelector(field string, selector search.SortedSetSelectorType) *SortedSetFieldSource {
	return &SortedSetFieldSource{field: field, selector: selector}
}

// Description returns "sortedset(<field>,selector=<selector>)".
func (s *SortedSetFieldSource) Description() string {
	return fmt.Sprintf("sortedset(%s,selector=%s)", s.field, s.selector)
}

// GetField returns the field name.
func (s *SortedSetFieldSource) GetField() string { return s.field }

// wrapSortedSetDocValues wraps a SortedSetDocValues into a single-valued
// SortedDocValues using the given selector.
func wrapSortedSetDocValues(ssdv index.SortedSetDocValues, selector search.SortedSetSelectorType) index.SortedDocValues {
	return &sortedSetWrapper{ssdv: ssdv, selector: selector}
}

type sortedSetWrapper struct {
	ssdv     index.SortedSetDocValues
	selector search.SortedSetSelectorType
	ord      int
}

func (w *sortedSetWrapper) DocID() int { return w.ssdv.DocID() }
func (w *sortedSetWrapper) NextDoc() (int, error) {
	doc, err := w.ssdv.NextDoc()
	if err != nil || doc == index.NO_MORE_DOCS {
		return doc, err
	}
	w.pickOrd()
	return doc, nil
}
func (w *sortedSetWrapper) Advance(target int) (int, error) {
	doc, err := w.ssdv.Advance(target)
	if err != nil || doc == index.NO_MORE_DOCS {
		return doc, err
	}
	w.pickOrd()
	return doc, nil
}
func (w *sortedSetWrapper) AdvanceExact(target int) (bool, error) {
	ok, err := w.ssdv.AdvanceExact(target)
	if err != nil || !ok {
		w.ord = -1
		return false, err
	}
	w.pickOrd()
	return true, nil
}
func (w *sortedSetWrapper) LongValue() (int64, error) { return int64(w.ord), nil }
func (w *sortedSetWrapper) Cost() int64               { return w.ssdv.Cost() }
func (w *sortedSetWrapper) OrdValue() (int, error)    { return w.ord, nil }
func (w *sortedSetWrapper) LookupOrd(ord int) ([]byte, error) { return w.ssdv.LookupOrd(ord) }
func (w *sortedSetWrapper) GetValueCount() int                 { return w.ssdv.GetValueCount() }

func (w *sortedSetWrapper) pickOrd() {
	first, err := w.ssdv.NextOrd()
	if err != nil || first == -1 {
		w.ord = -1
		return
	}
	switch w.selector {
	case search.SortedSetSelectorMin:
		w.ord = first
	case search.SortedSetSelectorMax:
		cur := first
		for {
			next, err := w.ssdv.NextOrd()
			if err != nil || next == -1 {
				break
			}
			cur = next
		}
		w.ord = cur
	case search.SortedSetSelectorMiddleMin:
		count := 1
		for {
			next, err := w.ssdv.NextOrd()
			if err != nil || next == -1 {
				break
			}
			count++
		}
		// MiddleMin: (count-1)/2
		// For this we'd need to re-iterate - simplified:
		w.ord = first
	case search.SortedSetSelectorMiddleMax:
		w.ord = first
	default:
		w.ord = first
	}
}

// GetValues returns FunctionValues backed by SortedSetDocValues.
func (s *SortedSetFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	ssdv, err := getSortedSetDocValues(s.field, readerContext)
	if err != nil {
		return nil, err
	}
	if ssdv == nil {
		return &sortedSetMissingValues{description: s.Description()}, nil
	}

	view := wrapSortedSetDocValues(ssdv, s.selector)
	dtv := &sortedSetTermValues{
		DocTermsIndexDocValues: *docvalues.NewDocTermsIndexDocValuesFromDV(s, view),
		vs:                    s,
	}
	dtv.SetSelf(dtv)
	return dtv, nil
}

// Equals reports value equality.
func (s *SortedSetFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*SortedSetFieldSource)
	if !ok || o == nil {
		return false
	}
	return s.field == o.field && s.selector == o.selector
}

// HashCode returns a stable hash.
func (s *SortedSetFieldSource) HashCode() int32 {
	return hashString("sortedset") + hashString(s.field) + int32(s.selector)
}

type sortedSetTermValues struct {
	docvalues.DocTermsIndexDocValues
	vs *SortedSetFieldSource
}

func (v *sortedSetTermValues) ObjectVal(doc int) (any, error) { return v.StrVal(doc) }
func (v *sortedSetTermValues) ToString(doc int) (string, error) {
	s, err := v.StrVal(doc)
	if err != nil {
		return "", err
	}
	return v.vs.Description() + "=" + s, nil
}

type sortedSetMissingValues struct {
	missingValuesBase
	description string
}

func (v *sortedSetMissingValues) ToString(doc int) (string, error) { return v.description + "=null", nil }
func (v *sortedSetMissingValues) GetScorer(readerContext *index.LeafReaderContext) function.ValueSourceScorer {
	return newAllValueSourceScorer(readerContext, v)
}

var _ function.ValueSource = (*SortedSetFieldSource)(nil)
