// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// IntFieldSource obtains int field values from NumericDocValues and
// makes those values available as other numeric types, casting as needed.
type IntFieldSource struct {
	FieldCacheSource
}

func NewIntFieldSource(field string) *IntFieldSource {
	return &IntFieldSource{
		FieldCacheSource: FieldCacheSource{Field: field},
	}
}

func (f *IntFieldSource) Description() string {
	return fmt.Sprintf("int(%s)", f.Field)
}

func (f *IntFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	ndv, err := readerContext.LeafReader().GetNumericDocValues(f.Field)
	if err != nil {
		return nil, err
	}

	fv := &intDocValues{
		source: f,
		ndv:    ndv,
	}
	fv.SetSelf(fv)
	return fv, nil
}

type intDocValues struct {
	function.BaseFunctionValues
	source    *IntFieldSource
	ndv       index.NumericDocValues
	lastDocID int
}

func (f *intDocValues) IntVal(doc int) (int32, error) {
	exists, err := f.Exists(doc)
	if err != nil {
		return 0, err
	}
	if exists {
		val, err := f.ndv.LongValue()
		if err != nil {
			return 0, err
		}
		return int32(val), nil
	}
	return 0, nil
}

func (f *intDocValues) StrVal(doc int) (string, error) {
	val, err := f.IntVal(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", val), nil
}

func (f *intDocValues) Exists(doc int) (bool, error) {
	if doc < f.lastDocID {
		return false, fmt.Errorf("docs were sent out-of-order: lastDocID=%d vs docID=%d", f.lastDocID, doc)
	}
	f.lastDocID = doc
	curDocID := f.ndv.DocID()
	if doc > curDocID {
		next, err := f.ndv.Advance(doc)
		if err != nil {
			return false, err
		}
		curDocID = next
	}
	return doc == curDocID, nil
}

func (f *intDocValues) ToString(doc int) (string, error) {
	return fmt.Sprintf("int(%s)=%d", f.source.Field, f.IntVal(doc)), nil
}
