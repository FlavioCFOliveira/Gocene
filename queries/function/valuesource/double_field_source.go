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

// DoubleFieldSource obtains double field values from NumericDocValues and
// makes those values available as other numeric types, casting as needed.
type DoubleFieldSource struct {
	FieldCacheSource
}

func NewDoubleFieldSource(field string) *DoubleFieldSource {
	return &DoubleFieldSource{
		FieldCacheSource: FieldCacheSource{Field: field},
	}
}

func (f *DoubleFieldSource) Description() string {
	return fmt.Sprintf("double(%s)", f.Field)
}

func (f *DoubleFieldSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	ndv, err := readerContext.Reader().GetNumericDocValues(f.Field)
	if err != nil {
		return nil, err
	}

	fv := &doubleDocValues{
		source: f,
		ndv:    ndv,
	}
	fv.SetSelf(fv)
	return fv, nil
}

type doubleDocValues struct {
	function.BaseFunctionValues
	source   *DoubleFieldSource
	ndv      index.NumericDocValues
	lastDocID int
}

func (f *doubleDocValues) DoubleVal(doc int) (float64, error) {
	exists, err := f.Exists(doc)
	if err != nil {
		return 0, err
	}
	if exists {
		val, err := f.ndv.LongValue()
		if err != nil {
			return 0, err
		}
		return math.Float64frombits(uint64(val)), nil
	}
	return 0, nil
}

func (f *doubleDocValues) Exists(doc int) (bool, error) {
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

func (f *doubleDocValues) ToString(doc int) (string, error) {
	return fmt.Sprintf("double(%s)=%g", f.source.Field, f.DoubleVal(doc)), nil
}
