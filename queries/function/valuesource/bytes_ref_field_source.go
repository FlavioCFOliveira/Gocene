// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
	"github.com/FlavioCFOliveira/Gocene/schema"
)

// BytesRefFieldSource retrieves FunctionValues for string-based fields.
//
// Go port of org.apache.lucene.queries.function.valuesource.BytesRefFieldSource.
type BytesRefFieldSource struct {
	function.BaseValueSource
	field string
}

// NewBytesRefFieldSource creates a BytesRefFieldSource for the given field.
func NewBytesRefFieldSource(field string) *BytesRefFieldSource {
	return &BytesRefFieldSource{field: field}
}

// Description returns the field name.
func (s *BytesRefFieldSource) Description() string { return s.field }

// GetField returns the field name.
func (s *BytesRefFieldSource) GetField() string { return s.field }

// GetValues returns FunctionValues backed by BinaryDocValues or SortedDocValues.
func (s *BytesRefFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	leaf := readerContext.LeafReader()
	if leaf == nil {
		return &bytesRefMissingValues{description: s.Description()}, nil
	}

	// Try BINARY doc-values path.
	fi, ok := leaf.(fieldInfosReader)
	if ok {
		fieldInfo := fi.GetFieldInfos().GetByName(s.field)
		if fieldInfo != nil && fieldInfo.DocValuesType() == schema.DocValuesTypeBinary {
			return s.binaryValues(leaf)
		}
	}

	// Fall back to DocTermsIndexDocValues (sorted path).
	dtv, err := docvalues.NewDocTermsIndexDocValues(s, readerContext, s.field)
	if err != nil {
		return nil, err
	}
	v := &bytesRefTermValues{DocTermsIndexDocValues: *dtv, vs: s}
	v.SetSelf(v)
	return v, nil
}

func (s *BytesRefFieldSource) binaryValues(leaf index.IndexReaderInterface) (function.FunctionValues, error) {
	bdr, ok := leaf.(binaryDocValuesReader)
	if !ok {
		return &bytesRefMissingValues{description: s.Description()}, nil
	}
	arr, err := bdr.GetBinaryDocValues(s.field)
	if err != nil {
		return nil, err
	}
	if arr == nil {
		return &bytesRefMissingValues{description: s.Description()}, nil
	}
	v := &bytesRefBinaryValues{arr: arr, vs: s}
	v.SetSelf(v)
	return v, nil
}

// Equals reports value equality.
func (s *BytesRefFieldSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*BytesRefFieldSource)
	if !ok || o == nil {
		return false
	}
	return s.field == o.field
}

// HashCode returns a stable hash.
func (s *BytesRefFieldSource) HashCode() int32 {
	return hashString("bytes") + hashString(s.field)
}

// bytesRefBinaryValues wraps BinaryDocValues as FunctionValues.
type bytesRefBinaryValues struct {
	function.BaseFunctionValues
	arr index.BinaryDocValues
	vs  *BytesRefFieldSource
	lastDocID int
}

func (v *bytesRefBinaryValues) exists(doc int) bool {
	if doc < v.lastDocID {
		return false
	}
	v.lastDocID = doc
	curDocID := v.arr.DocID()
	if doc > curDocID {
		var err error
		curDocID, err = v.arr.Advance(doc)
		if err != nil {
			return false
		}
	}
	return doc == curDocID
}

func (v *bytesRefBinaryValues) StrVal(doc int) (string, error) {
	if !v.exists(doc) {
		return "", nil
	}
	bs, err := v.arr.BinaryValue()
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func (v *bytesRefBinaryValues) ObjectVal(doc int) (any, error) {
	s, err := v.StrVal(doc)
	if err != nil || s == "" {
		return nil, err
	}
	return s, nil
}

func (v *bytesRefBinaryValues) Exists(doc int) (bool, error) { return v.exists(doc), nil }

func (v *bytesRefBinaryValues) BytesVal(doc int, target *[]byte) (bool, error) {
	if !v.exists(doc) {
		if target != nil {
			*target = (*target)[:0]
		}
		return false, nil
	}
	bs, err := v.arr.BinaryValue()
	if err != nil || len(bs) == 0 {
		if target != nil {
			*target = (*target)[:0]
		}
		return false, err
	}
	if target != nil {
		*target = append((*target)[:0], bs...)
	}
	return true, nil
}

func (v *bytesRefBinaryValues) ToString(doc int) (string, error) {
	s, err := v.StrVal(doc)
	if err != nil {
		return "", err
	}
	return v.vs.Description() + "=" + s, nil
}

func (v *bytesRefBinaryValues) Cost() float32 { return 100 }

func (v *bytesRefBinaryValues) BoolVal(doc int) (bool, error) { return v.exists(doc), nil }

func (v *bytesRefBinaryValues) GetValueFiller() function.ValueFiller {
	return &bytesRefValueFiller{vals: v}
}

type bytesRefValueFiller struct {
	vals *bytesRefBinaryValues
	mval function.MutableValueFloat
}

func (f *bytesRefValueFiller) GetValue() *function.MutableValueFloat { return &f.mval }
func (f *bytesRefValueFiller) FillValue(doc int) error {
	ok, err := f.vals.Exists(doc)
	if err != nil || !ok {
		f.mval.Value = 0
		f.mval.Exists = false
		return err
	}
	f.mval.Value = 1.0
	f.mval.Exists = true
	return nil
}

func (v *bytesRefBinaryValues) GetScorer(readerContext *index.LeafReaderContext) function.ValueSourceScorer {
	return newAllValueSourceScorer(readerContext, v)
}

// bytesRefTermValues wraps DocTermsIndexDocValues with proper ObjectVal.
type bytesRefTermValues struct {
	docvalues.DocTermsIndexDocValues
	vs *BytesRefFieldSource
}

func (v *bytesRefTermValues) ObjectVal(doc int) (any, error) { return v.StrVal(doc) }

func (v *bytesRefTermValues) ToString(doc int) (string, error) {
	s, err := v.StrVal(doc)
	if err != nil {
		return "", err
	}
	return v.vs.Description() + "=" + s, nil
}

type bytesRefMissingValues struct {
	missingValuesBase
	description string
}

func (v *bytesRefMissingValues) ToString(doc int) (string, error) { return v.description + "=null", nil }
func (v *bytesRefMissingValues) GetScorer(readerContext *index.LeafReaderContext) function.ValueSourceScorer {
	return newAllValueSourceScorer(readerContext, v)
}

var _ function.ValueSource = (*BytesRefFieldSource)(nil)
