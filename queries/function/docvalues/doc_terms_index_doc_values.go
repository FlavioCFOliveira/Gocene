// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package docvalues

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// ErrDocsOutOfOrder is returned when consecutive doc IDs requested from
// [DocTermsIndexDocValues] go backwards. Mirrors Lucene's
// IllegalArgumentException for out-of-order docs.
var ErrDocsOutOfOrder = errors.New("docvalues: docs were sent out-of-order")

// ErrLookupTermUnavailable is returned by [DocTermsIndexDocValues.GetRangeScorer].
// The Gocene SortedDocValues interface does not yet expose LookupTerm; this
// scorer is therefore deferred until that surface is extended. Tracked as a
// follow-up under the Sprint 30 backlog.
var ErrLookupTermUnavailable = errors.New("docvalues: GetRangeScorer needs SortedDocValues.LookupTerm (not yet ported)")

// DocTermsIndexDocValues is the Go port of
// org.apache.lucene.queries.function.docvalues.DocTermsIndexDocValues.
//
// It surfaces a [index.SortedDocValues] as a [function.FunctionValues]
// instance, exposing ord-based access, byte/string lookups, and an
// out-of-order guard that mirrors Lucene's IllegalArgumentException.
//
// Concrete embedders override [DocTermsIndexDocValues.ToTerm] when their
// external (user-facing) form differs from the indexed form.
type DocTermsIndexDocValues struct {
	function.BaseFunctionValues
	VS         function.ValueSource
	TermsIndex index.SortedDocValues
	// ToTermFunc translates a user-supplied bound into its indexed form.
	// Defaults to the identity function.
	ToTermFunc func(readableValue string) (string, error)
	lastDocID  int
}

// NewDocTermsIndexDocValues opens the SortedDocValues for field on the
// supplied leaf and returns a ready-to-use DocTermsIndexDocValues.
//
// Gocene deviation: Lucene's open() helper wraps DocValues.getSorted in a
// DocTermsIndexException. Here the error is propagated as-is; callers can
// inspect it through errors.Is.
func NewDocTermsIndexDocValues(vs function.ValueSource, ctx *index.LeafReaderContext, field string) (*DocTermsIndexDocValues, error) {
	leaf := ctx.LeafReader()
	if leaf == nil {
		return nil, fmt.Errorf("docvalues: leaf reader unavailable for field %q", field)
	}
	sd, ok := leaf.(sortedDocValuesReader)
	if !ok {
		return nil, fmt.Errorf("docvalues: leaf reader does not expose GetSortedDocValues (field %q)", field)
	}
	dv, err := sd.GetSortedDocValues(field)
	if err != nil {
		return nil, fmt.Errorf("docvalues: cannot open SortedDocValues for %q: %w", field, err)
	}
	return NewDocTermsIndexDocValuesFromDV(vs, dv), nil
}

// sortedDocValuesReader is the optional contract expected of LeafReaders
// that surface SortedDocValues. *index.LeafReader and *index.FilterLeafReader
// satisfy it today.
type sortedDocValuesReader interface {
	GetSortedDocValues(field string) (index.SortedDocValues, error)
}

// NewDocTermsIndexDocValuesFromDV wires an already-open SortedDocValues
// instance into a DocTermsIndexDocValues.
func NewDocTermsIndexDocValuesFromDV(vs function.ValueSource, dv index.SortedDocValues) *DocTermsIndexDocValues {
	v := &DocTermsIndexDocValues{
		VS:         vs,
		TermsIndex: dv,
		ToTermFunc: func(s string) (string, error) { return s, nil },
		lastDocID:  -1,
	}
	v.SetSelf(v)
	return v
}

// GetOrdForDoc returns the ord for doc, advancing the underlying
// SortedDocValues as needed and rejecting out-of-order requests.
func (d *DocTermsIndexDocValues) GetOrdForDoc(doc int) (int, error) {
	if doc < d.lastDocID {
		return 0, fmt.Errorf("%w: lastDocID=%d vs docID=%d", ErrDocsOutOfOrder, d.lastDocID, doc)
	}
	d.lastDocID = doc
	cur := d.TermsIndex.DocID()
	if doc > cur {
		next, err := d.TermsIndex.Advance(doc)
		if err != nil {
			return 0, err
		}
		cur = next
	}
	if doc == cur {
		return d.TermsIndex.GetOrd(doc)
	}
	return -1, nil
}

// Exists reports whether the doc has a value.
func (d *DocTermsIndexDocValues) Exists(doc int) (bool, error) {
	ord, err := d.GetOrdForDoc(doc)
	if err != nil {
		return false, err
	}
	return ord >= 0, nil
}

// OrdVal returns the per-doc ord.
func (d *DocTermsIndexDocValues) OrdVal(doc int) (int, error) { return d.GetOrdForDoc(doc) }

// NumOrd returns the total ord count.
func (d *DocTermsIndexDocValues) NumOrd() (int, error) { return d.TermsIndex.GetValueCount(), nil }

// BytesVal copies the term bytes for doc into target.
func (d *DocTermsIndexDocValues) BytesVal(doc int, target *[]byte) (bool, error) {
	if target == nil {
		ord, err := d.GetOrdForDoc(doc)
		return ord >= 0, err
	}
	*target = (*target)[:0]
	ord, err := d.GetOrdForDoc(doc)
	if err != nil || ord < 0 {
		return false, err
	}
	bs, err := d.TermsIndex.LookupOrd(ord)
	if err != nil {
		return false, err
	}
	*target = append(*target, bs...)
	return true, nil
}

// StrVal returns the term text for doc, or empty string when absent.
func (d *DocTermsIndexDocValues) StrVal(doc int) (string, error) {
	ord, err := d.GetOrdForDoc(doc)
	if err != nil || ord < 0 {
		return "", err
	}
	bs, err := d.TermsIndex.LookupOrd(ord)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// BoolVal mirrors Lucene's "boolVal == exists" convention for terms-index DV.
func (d *DocTermsIndexDocValues) BoolVal(doc int) (bool, error) { return d.Exists(doc) }

// ToString renders "<vs.description>=<strVal>".
func (d *DocTermsIndexDocValues) ToString(doc int) (string, error) {
	s, err := d.StrVal(doc)
	if err != nil {
		return "", err
	}
	return d.VS.Description() + "=" + s, nil
}

// GetRangeScorer is currently deferred: Gocene's [index.SortedDocValues]
// does not yet expose LookupTerm, which Lucene's implementation requires
// to translate textual bounds into ord ranges. Tracked via Sprint 30+
// backlog.
func (d *DocTermsIndexDocValues) GetRangeScorer(
	_ *index.LeafReaderContext,
	_, _ string,
	_, _ bool,
) (function.ValueSourceScorer, error) {
	return nil, ErrLookupTermUnavailable
}
