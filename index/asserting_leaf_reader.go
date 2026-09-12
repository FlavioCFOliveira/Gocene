// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// AssertingLeafReader is a LeafReader that can be used to apply additional checks for tests.
//
// This is the Go port of Lucene's org.apache.lucene.tests.index.AssertingLeafReader.
type AssertingLeafReader struct {
	LeafReader
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

func (r *AssertingLeafReader) StoredFields() (spi.StoredFields, error) {
	sf, err := r.LeafReader.StoredFields()
	if err != nil {
		return nil, err
	}
	if sf == nil {
		return nil, nil
	}
	return &AssertingStoredFields{in: sf}, nil
}

func (r *AssertingLeafReader) TermVectors() (spi.TermVectors, error) {
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

func (r *AssertingLeafReader) GetPointValues(field string) (spi.PointValues, error) {
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

func (t *AssertingTerms) Field() string { return t.in.Field() }

func (t *AssertingTerms) GetIterator() (TermsEnum, error) {
	te, err := t.in.GetIterator()
	if err != nil {
		return nil, err
	}
	if te == nil {
		return nil, nil
	}
	return &AssertingTermsEnum{in: te}, nil
}

// Intersect delegates to the wrapped Terms and wraps the result. Mirrors
// org.apache.lucene.tests.index.AssertingLeafReader.AssertingTerms#intersect:
// the returned enumerator must never be null and the start term, when
// supplied, must carry valid bytes.
func (t *AssertingTerms) Intersect(compiled *automaton.CompiledAutomaton, startTerm *spi.Term) (TermsEnum, error) {
	te, err := t.in.Intersect(compiled, startTerm)
	if err != nil {
		return nil, err
	}
	if te == nil {
		panic("AssertingTerms: intersect returned a nil TermsEnum")
	}
	if startTerm != nil && (startTerm.Bytes == nil || !startTerm.Bytes.IsValid()) {
		panic("AssertingTerms: intersect start term is invalid")
	}
	return &AssertingTermsEnum{in: te}, nil
}

func (t *AssertingTerms) GetMin() (*spi.Term, error) {
	v, err := t.in.GetMin()
	if err != nil {
		return nil, err
	}
	if v != nil && (v.Bytes == nil || !v.Bytes.IsValid()) {
		panic("AssertingTerms: min term is invalid")
	}
	return v, nil
}

func (t *AssertingTerms) GetMax() (*spi.Term, error) {
	v, err := t.in.GetMax()
	if err != nil {
		return nil, err
	}
	if v != nil && (v.Bytes == nil || !v.Bytes.IsValid()) {
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

func (t *AssertingTerms) GetIteratorWithSeek(seekTerm *spi.Term) (TermsEnum, error) {
	te, err := t.in.GetIteratorWithSeek(seekTerm)
	if err != nil {
		return nil, err
	}
	if te == nil {
		return nil, nil
	}
	return &AssertingTermsEnum{in: te}, nil
}

func (t *AssertingTerms) GetPostingsReader(termText string, flags int) (spi.PostingsEnum, error) {
	return t.in.GetPostingsReader(termText, flags)
}

func (t *AssertingTerms) Size() int64 {
	return t.in.Size()
}

func (t *AssertingTerms) GetSumDocFreq() (int64, error) {
	return t.in.GetSumDocFreq()
}

func (t *AssertingTerms) GetSumTotalTermFreq() (int64, error) {
	return t.in.GetSumTotalTermFreq()
}

func (t *AssertingTerms) HasFreqs() bool {
	return t.in.HasFreqs()
}

func (t *AssertingTerms) HasOffsets() bool {
	return t.in.HasOffsets()
}

func (t *AssertingTerms) HasPositions() bool {
	return t.in.HasPositions()
}

func (t *AssertingTerms) HasPayloads() bool {
	return t.in.HasPayloads()
}

type AssertingTermsEnum struct {
	in TermsEnum
}

func (te *AssertingTermsEnum) Ord() int64 {
	return te.in.Ord()
}

func (te *AssertingTermsEnum) Impacts(flags int) (spi.ImpactsEnum, error) {
	return te.in.Impacts(flags)
}

func (te *AssertingTermsEnum) Next() (*Term, error) {
	term, err := te.in.Next()
	if err != nil {
		return nil, err
	}
	return term, nil
}

func (te *AssertingTermsEnum) DocFreq() (int, error) {
	return te.in.DocFreq()
}

func (te *AssertingTermsEnum) SeekCeil(term *spi.Term) (*spi.Term, error) {
	return te.in.SeekCeil(term)
}

func (te *AssertingTermsEnum) SeekExact(term *spi.Term) (bool, error) {
	return te.in.SeekExact(term)
}

func (te *AssertingTermsEnum) Term() *spi.Term {
	return te.in.Term()
}

func (te *AssertingTermsEnum) TotalTermFreq() (int64, error) {
	return te.in.TotalTermFreq()
}

func (te *AssertingTermsEnum) Postings(flags int) (spi.PostingsEnum, error) {
	return te.in.Postings(flags)
}

func (te *AssertingTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (spi.PostingsEnum, error) {
	return te.in.PostingsWithLiveDocs(liveDocs, flags)
}

type AssertingStoredFields struct {
	in spi.StoredFields
}

func (sf *AssertingStoredFields) Document(docID int, visitor StoredFieldVisitor) error {
	return sf.in.Document(docID, visitor)
}

func (sf *AssertingStoredFields) Prefetch(docIDs []int) error {
	return sf.in.Prefetch(docIDs)
}

type AssertingTermVectors struct {
	in spi.TermVectors
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

func (tv *AssertingTermVectors) Prefetch(docIDs []int) error {
	return tv.in.Prefetch(docIDs)
}

func (tv *AssertingTermVectors) GetField(docID int, field string) (spi.Terms, error) {
	t, err := tv.in.GetField(docID, field)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, nil
	}
	return &AssertingTerms{in: t}, nil
}

type AssertingFields struct {
	in Fields
}

func (f *AssertingFields) Size() int {
	return f.in.Size()
}

type AssertingFieldIterator struct {
	in spi.FieldIterator
}

func (fi *AssertingFieldIterator) Next() (string, error) {
	return fi.in.Next()
}

func (fi *AssertingFieldIterator) HasNext() bool {
	return fi.in.HasNext()
}

func (f *AssertingFields) Iterator() (spi.FieldIterator, error) {
	it, err := f.in.Iterator()
	if err != nil {
		return nil, err
	}
	return &AssertingFieldIterator{in: it}, nil
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

func (dv *AssertingNumericDocValues) DocID() int {
	return dv.in.DocID()
}

func (dv *AssertingNumericDocValues) Advance(target int) (int, error) {
	doc, err := dv.in.Advance(target)
	if err != nil {
		return 0, err
	}
	if doc != -1 && doc >= dv.maxDoc {
		panic(fmt.Sprintf("AssertingNumericDocValues: advance %d >= maxDoc %d", doc, dv.maxDoc))
	}
	return doc, nil
}

func (dv *AssertingNumericDocValues) AdvanceExact(target int) (bool, error) {
	return dv.in.AdvanceExact(target)
}

func (dv *AssertingNumericDocValues) LongValue() (int64, error) {
	return dv.in.LongValue()
}

func (dv *AssertingNumericDocValues) Cost() int64 {
	return dv.in.Cost()
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

func (dv *AssertingBinaryDocValues) DocID() int {
	return dv.in.DocID()
}

func (dv *AssertingBinaryDocValues) Advance(target int) (int, error) {
	doc, err := dv.in.Advance(target)
	if err != nil {
		return 0, err
	}
	if doc != -1 && doc >= dv.maxDoc {
		panic(fmt.Sprintf("AssertingBinaryDocValues: advance %d >= maxDoc %d", doc, dv.maxDoc))
	}
	return doc, nil
}

func (dv *AssertingBinaryDocValues) AdvanceExact(target int) (bool, error) {
	return dv.in.AdvanceExact(target)
}

func (dv *AssertingBinaryDocValues) BinaryValue() ([]byte, error) {
	return dv.in.BinaryValue()
}

func (dv *AssertingBinaryDocValues) Cost() int64 {
	return dv.in.Cost()
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

func (dv *AssertingSortedDocValues) DocID() int {
	return dv.in.DocID()
}

func (dv *AssertingSortedDocValues) Advance(target int) (int, error) {
	doc, err := dv.in.Advance(target)
	if err != nil {
		return 0, err
	}
	if doc != -1 && doc >= dv.maxDoc {
		panic(fmt.Sprintf("AssertingSortedDocValues: advance %d >= maxDoc %d", doc, dv.maxDoc))
	}
	return doc, nil
}

func (dv *AssertingSortedDocValues) AdvanceExact(target int) (bool, error) {
	return dv.in.AdvanceExact(target)
}

func (dv *AssertingSortedDocValues) LookupOrd(ord int) ([]byte, error) {
	return dv.in.LookupOrd(ord)
}

func (dv *AssertingSortedDocValues) OrdValue() (int, error) {
	return dv.in.OrdValue()
}

func (dv *AssertingSortedDocValues) LongValue() (int64, error) {
	return dv.in.LongValue()
}

func (dv *AssertingSortedDocValues) GetValueCount() int {
	return dv.in.GetValueCount()
}

func (dv *AssertingSortedDocValues) Cost() int64 {
	return dv.in.Cost()
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

func (dv *AssertingSortedNumericDocValues) DocID() int {
	return dv.in.DocID()
}

func (dv *AssertingSortedNumericDocValues) Advance(target int) (int, error) {
	doc, err := dv.in.Advance(target)
	if err != nil {
		return 0, err
	}
	if doc != -1 && doc >= dv.maxDoc {
		panic(fmt.Sprintf("AssertingSortedNumericDocValues: advance %d >= maxDoc %d", doc, dv.maxDoc))
	}
	return doc, nil
}

func (dv *AssertingSortedNumericDocValues) AdvanceExact(target int) (bool, error) {
	return dv.in.AdvanceExact(target)
}

func (dv *AssertingSortedNumericDocValues) LongValue() (int64, error) {
	return dv.in.LongValue()
}

func (dv *AssertingSortedNumericDocValues) NextValue() (int64, error) {
	return dv.in.NextValue()
}

func (dv *AssertingSortedNumericDocValues) DocValueCount() (int, error) {
	return dv.in.DocValueCount()
}

func (dv *AssertingSortedNumericDocValues) Cost() int64 {
	return dv.in.Cost()
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

func (dv *AssertingSortedSetDocValues) DocID() int {
	return dv.in.DocID()
}

func (dv *AssertingSortedSetDocValues) Advance(target int) (int, error) {
	doc, err := dv.in.Advance(target)
	if err != nil {
		return 0, err
	}
	if doc != -1 && doc >= dv.maxDoc {
		panic(fmt.Sprintf("AssertingSortedSetDocValues: advance %d >= maxDoc %d", doc, dv.maxDoc))
	}
	return doc, nil
}

func (dv *AssertingSortedSetDocValues) AdvanceExact(target int) (bool, error) {
	return dv.in.AdvanceExact(target)
}

func (dv *AssertingSortedSetDocValues) LookupOrd(ord int) ([]byte, error) {
	return dv.in.LookupOrd(ord)
}

func (dv *AssertingSortedSetDocValues) NextOrd() (int, error) {
	return dv.in.NextOrd()
}

func (dv *AssertingSortedSetDocValues) GetValueCount() int {
	return dv.in.GetValueCount()
}

func (dv *AssertingSortedSetDocValues) Cost() int64 {
	return dv.in.Cost()
}

type AssertingPointValues struct {
	in     spi.PointValues
	maxDoc int
}

func (pv *AssertingPointValues) GetDocCount() int {
	count := pv.in.GetDocCount()
	if count < 0 || count > pv.maxDoc {
		panic(fmt.Sprintf("AssertingPointValues: docCount %d out of range [0, %d]", count, pv.maxDoc))
	}
	return count
}

func (pv *AssertingPointValues) GetBytesPerDimension() int {
	return pv.in.GetBytesPerDimension()
}

func (pv *AssertingPointValues) GetDocCountWithValue() int64 {
	return pv.in.GetDocCountWithValue()
}

func (pv *AssertingPointValues) GetValueCount() int64 {
	return pv.in.GetValueCount()
}

func (pv *AssertingPointValues) GetMinPackedValue() ([]byte, error) {
	return pv.in.GetMinPackedValue()
}

func (pv *AssertingPointValues) GetMaxPackedValue() ([]byte, error) {
	return pv.in.GetMaxPackedValue()
}

func (pv *AssertingPointValues) GetNumDimensions() int {
	return pv.in.GetNumDimensions()
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

func (b *AssertingBits) Cardinality() int {
	return b.in.Cardinality()
}
