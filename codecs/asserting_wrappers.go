package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

type AssertingTerms struct {
	in spi.Terms
}

func (t *AssertingTerms) TermsEnum(field string) (spi.TermsEnum, error) {
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
	in spi.TermsEnum
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
	in index.StoredFields
}

func (sf *AssertingStoredFields) Document(docID int, visitor index.StoredFieldVisitor) error {
	return sf.in.Document(docID, visitor)
}

type AssertingTermVectors struct {
	in index.TermVectors
}

func (tv *AssertingTermVectors) Get(doc int) (index.Fields, error) {
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
	in index.Fields
}

func (f *AssertingFields) Iterator() []string {
	return f.in.Iterator()
}

func (f *AssertingFields) Terms(field string) (spi.Terms, error) {
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
	in     index.NumericDocValues
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
	in     index.BinaryDocValues
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
	in     index.SortedDocValues
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
	in     index.SortedNumericDocValues
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
	in     index.SortedSetDocValues
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
	in     index.PointValues
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

func NewAssertingTerms(terms spi.Terms) *AssertingTerms {
	return &AssertingTerms{in: terms}
}
