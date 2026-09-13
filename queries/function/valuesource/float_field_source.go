// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// FloatFieldSource obtains float field values from NumericDocValues and
// makes those values available as other numeric types, casting as needed.
type FloatFieldSource struct {
	FieldCacheSource
}

func NewFloatFieldSource(field string) *FloatFieldSource {
	return &FloatFieldSource{
		FieldCacheSource: FieldCacheSource{Field: field},
	}
}

func (f *FloatFieldSource) Description() string {
	return fmt.Sprintf("float(%s)", f.Field)
}

func (f *FloatFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	ndv, err := readerContext.LeafReader().GetNumericDocValues(f.Field)
	if err != nil {
		return nil, err
	}

	fv := &floatDocValues{
		source: f,
		ndv:    ndv,
	}
	fv.SetSelf(fv)
	return fv, nil
}

type floatDocValues struct {
	function.BaseFunctionValues
	source    *FloatFieldSource
	ndv       index.NumericDocValues
	lastDocID int
}

func (f *floatDocValues) FloatVal(doc int) (float32, error) {
	exists, err := f.Exists(doc)
	if err != nil {
		return 0, err
	}
	if exists {
		val, err := f.ndv.LongValue()
		if err != nil {
			return 0, err
		}
		return math.Float32frombits(uint32(val)), nil
	}
	return 0, nil
}

func (f *floatDocValues) Exists(doc int) (bool, error) {
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

func (f *floatDocValues) ToString(doc int) (string, error) {
	return fmt.Sprintf("float(%s)=%g", f.source.Field, f.FloatVal(doc)), nil
}
