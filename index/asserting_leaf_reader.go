//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// AssertingLeafReader is a LeafReader that can be used to apply additional checks for tests.
//
// This is the Go port of Lucene's org.apache.lucene.tests.index.AssertingLeafReader.
type AssertingLeafReader struct {
	*LeafReader
}

// NewAssertingLeafReader creates a new AssertingLeafReader wrapping the given reader.
func NewAssertingLeafReader(in LeafReader) *AssertingLeafReader {
	// basic reader sanity
	if in.MaxDoc() < 0 {
		panic("AssertingLeafReader: maxDoc < 0")
	}
	if in.NumDocs() > in.MaxDoc() {
		panic(fmt.Sprintf("AssertingLeafReader: numDocs (%d) > maxDoc (%d)", in.NumDocs(), in.MaxDoc()))
	}
	if in.NumDeletedDocs()+in.NumDocs() != in.MaxDoc() {
		panic(fmt.Sprintf("AssertingLeafReader: numDeletedDocs (%d) + numDocs (%d) != maxDoc (%d)", in.NumDeletedDocs(), in.NumDocs(), in.MaxDoc()))
	}
	if in.HasDeletions() && !(in.NumDeletedDocs() > 0 && in.NumDocs() < in.MaxDoc()) {
		panic("AssertingLeafReader: hasDeletions is true but inconsistent with numDeletedDocs/numDocs")
	}

	return &AssertingLeafReader{
		LeafReader: in,
	}
}

func (r *AssertingLeafReader) Terms(field string) (Terms, error) {
	terms, err := r.LeafReader.Terms(field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	return &AssertingTerms{in: terms}, nil
}

func (r *AssertingLeafReader) StoredFields() (StoredFields, error) {
	sf, err := r.LeafReader.StoredFields()
	if err != nil {
		return nil, err
	}
	if sf == nil {
		return nil, nil
	}
	return &AssertingStoredFields{in: sf}, nil
}

func (r *AssertingLeafReader) TermVectors() (TermVectors, error) {
	tv, err := r.LeafReader.TermVectors()
	if err != nil {
		return nil, err
	}
	if tv == nil {
		return nil, nil
	}
	return &AssertingTermVectors{in: tv}, nil
}

func (r *AssertingLeafReader) GetNumericDocValues(field string) (NumericDocValues, error) {
	dv, err := r.LeafReader.GetNumericDocValues(field)
	if err != nil {
		return nil, err
	}
	if dv == nil {
		return nil, nil
	}
	return &AssertingNumericDocValues{in: dv, maxDoc: r.MaxDoc()}, nil
}

func (r *AssertingLeafReader) GetBinaryDocValues(field string) (BinaryDocValues, error) {
	dv, err := r.LeafReader.GetBinaryDocValues(field)
	if err != nil {
		return nil, err
	}
	if dv == nil {
		return nil, nil
	}
	return &AssertingBinaryDocValues{in: dv, maxDoc: r.MaxDoc()}, nil
}

func (r *AssertingLeafReader) GetSortedDocValues(field string) (SortedDocValues, error) {
	dv, err := r.LeafReader.GetSortedDocValues(field)
	if err != nil {
		return nil, err
	}
	if dv == nil {
		return nil, nil
	}
	return &AssertingSortedDocValues{in: dv, maxDoc: r.MaxDoc()}, nil
}

func (r *AssertingLeafReader) GetSortedNumericDocValues(field string) (SortedNumericDocValues, error) {
	dv, err := r.LeafReader.GetSortedNumericDocValues(field)
	if err != nil {
		return nil, err
	}
	if dv == nil {
		return nil, nil
	}
	return &AssertingSortedNumericDocValues{in: dv, maxDoc: r.MaxDoc()}, nil
}

func (r *AssertingLeafReader) GetSortedSetDocValues(field string) (SortedSetDocValues, error) {
	dv, err := r.LeafReader.GetSortedSetDocValues(field)
	if err != nil {
		return nil, err
	}
	if dv == nil {
		return nil, nil
	}
	return &AssertingSortedSetDocValues{in: dv, maxDoc: r.MaxDoc()}, nil
}

func (r *AssertingLeafReader) GetPointValues(field string) (PointValues, error) {
	pv, err := r.LeafReader.GetPointValues(field)
	if err != nil {
		return nil, err
	}
	if pv == nil {
		return nil, nil
	}
	return &AssertingPointValues{in: pv, maxDoc: r.MaxDoc()}, nil
}

func (r *AssertingLeafReader) GetLiveDocs() util.Bits {
	liveDocs := r.LeafReader.GetLiveDocs()
	if liveDocs == nil {
		if r.MaxDoc() != r.NumDocs() {
			panic("AssertingLeafReader: liveDocs is nil but maxDoc != numDocs")
		}
		if r.HasDeletions() {
			panic("AssertingLeafReader: liveDocs is nil but hasDeletions is true")
		}
		return nil
	}
	if liveDocs.Length() != r.MaxDoc() {
		panic(fmt.Sprintf("AssertingLeafReader: liveDocs length (%d) != maxDoc (%d)", liveDocs.Length(), r.MaxDoc()))
	}
	return &AssertingBits{in: liveDocs}
}

// --- Wrappers ---

type AssertingTerms struct {
	in Terms
}

func (t *AssertingTerms) TermsEnum(field string) (TermsEnum, error) {
	te, err := t.in.TermsEnum(field)
	if err != nil {
		return nil, err
	}
	if te == nil {
		return nil, nil
	}
	return &AssertingTermsEnum{in: te}, nil
}

func (t *AssertingTerms) GetMin() (util.BytesRef, error) {
	v, err := t.in.GetMin()
	if err != nil {
		return nil, err
	}
	if v != nil && !v.IsValid() {
		panic("AssertingTerms: min term is invalid")
	}
	return v, nil
}

func (t *AssertingTerms) GetMax() (util.BytesRef, error) {
	v, err := t.in.GetMax()
	if err != nil {
		return nil, err
	}
	if v != nil && !v.IsValid() {
		panic("AssertingTerms: max term is invalid")
	}
	return v, nil
}

func (t *AssertingTerms) GetDocCount() (int, error) {
	count, err := t.in.GetDocCount()
	if err != nil {
		return 0, err
	}
	if count < 0 {
		panic("AssertingTerms: docCount < 0")
	}
	return count, nil
}

type AssertingTermsEnum struct {
	in TermsEnum
}

func (te *AssertingTermsEnum) NextDoc() (int, error) {
	doc, err := te.in.NextDoc()
	if err != nil {
		return 0, err
	}
	return doc, nil
}

func (te *AssertingTermsEnum) DocID() int {
	return te.in.DocID()
}

func (te *AssertingTermsEnum) Freq() (int, error) {
	freq, err := te.in.Freq()
	if err != nil {
		return 0, err
	}
	if freq <= 0 {
		panic(fmt.Sprintf("AssertingTermsEnum: freq %d <= 0", freq))
	}
	return freq, nil
}

type AssertingStoredFields struct {
	in StoredFields
}

func (sf *AssertingStoredFields) Document(docID int, visitor StoredFieldVisitor) error {
	return sf.in.Document(docID, visitor)
}

type AssertingTermVectors struct {
	in TermVectors
}

func (tv *AssertingTermVectors) Get(doc int) (Fields, error) {
	f, err := tv.in.Get(doc)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return nil, nil
	}
	return &AssertingFields{in: f}, nil
}

type AssertingFields struct {
	in Fields
}

func (f *AssertingFields) Iterator() []string {
	return f.in.Iterator()
}

func (f *AssertingFields) Terms(field string) (Terms, error) {
	t, err := f.in.Terms(field)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, nil
	}
	return &AssertingTerms{in: t}, nil
}

type AssertingNumericDocValues struct {
	in     NumericDocValues
	maxDoc int
}

func (dv *AssertingNumericDocValues) NextDoc() (int, error) {
	doc, err := dv.in.NextDoc()
	if err != nil {
		return 0, err
	}
	if doc != -1 && doc >= dv.maxDoc {
		panic(fmt.Sprintf("AssertingNumericDocValues: nextDoc %d >= maxDoc %d", doc, dv.maxDoc))
	}
	return doc, nil
}

type AssertingBinaryDocValues struct {
	in     BinaryDocValues
	maxDoc int
}

func (dv *AssertingBinaryDocValues) NextDoc() (int, error) {
	doc, err := dv.in.NextDoc()
	if err != nil {
		return 0, err
	}
	if doc != -1 && doc >= dv.maxDoc {
		panic(fmt.Sprintf("AssertingBinaryDocValues: nextDoc %d >= maxDoc %d", doc, dv.maxDoc))
	}
	return doc, nil
}

type AssertingSortedDocValues struct {
	in     SortedDocValues
	maxDoc int
}

func (dv *AssertingSortedDocValues) NextDoc() (int, error) {
	doc, err := dv.in.NextDoc()
	if err != nil {
		return 0, err
	}
	if doc != -1 && doc >= dv.maxDoc {
		panic(fmt.Sprintf("AssertingSortedDocValues: nextDoc %d >= maxDoc %d", doc, dv.maxDoc))
	}
	return doc, nil
}

type AssertingSortedNumericDocValues struct {
	in     SortedNumericDocValues
	maxDoc int
}

func (dv *AssertingSortedNumericDocValues) NextDoc() (int, error) {
	doc, err := dv.in.NextDoc()
	if err != nil {
		return 0, err
	}
	if doc != -1 && doc >= dv.maxDoc {
		panic(fmt.Sprintf("AssertingSortedNumericDocValues: nextDoc %d >= maxDoc %d", doc, dv.maxDoc))
	}
	return doc, nil
}

type AssertingSortedSetDocValues struct {
	in     SortedSetDocValues
	maxDoc int
}

func (dv *AssertingSortedSetDocValues) NextDoc() (int, error) {
	doc, err := dv.in.NextDoc()
	if err != nil {
		return 0, err
	}
	if doc != -1 && doc >= dv.maxDoc {
		panic(fmt.Sprintf("AssertingSortedSetDocValues: nextDoc %d >= maxDoc %d", doc, dv.maxDoc))
	}
	return doc, nil
}

type AssertingPointValues struct {
	in     PointValues
	maxDoc int
}

func (pv *AssertingPointValues) GetDocCount() int {
	count := pv.in.GetDocCount()
	if count < 0 || count > pv.maxDoc {
		panic(fmt.Sprintf("AssertingPointValues: docCount %d out of range [0, %d]", count, pv.maxDoc))
	}
	return count
}

type AssertingBits struct {
	in util.Bits
}

func (b *AssertingBits) Get(index int) bool {
	if index < 0 || index >= b.in.Length() {
		panic(fmt.Sprintf("AssertingBits: index %d out of range [0, %d)", index, b.in.Length()))
	}
	return b.in.Get(index)
}

func (b *AssertingBits) Length() int {
	return b.in.Length()
}
