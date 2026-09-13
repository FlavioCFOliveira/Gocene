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
	// self is the most-derived ValueSource. Java's getValues builds its
	// FunctionValues with `this`, so a subclass's description() is the one
	// reported; Go embedding loses that, so the subclass installs itself here.
	self function.ValueSource
	// numericDocValues renders the protected, overridable
	// DoubleFieldSource.getNumericDocValues(Map, LeafReaderContext). A nil value selects
	// the base behaviour, DocValues.getNumeric(reader, field).
	numericDocValues func(ctx function.Context, readerContext *index.LeafReaderContext) (index.NumericDocValues, error)
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
	ndv, err := f.getNumericDocValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}

	fv := &doubleDocValues{
		source: f.valueSource(),
		ndv:    ndv,
	}
	fv.SetSelf(fv)
	return fv, nil
}

// getNumericDocValues dispatches to the installed override, or falls back to
// the base behaviour, DocValues.getNumeric(readerContext.reader(), field).
//
// Mirrors the protected DoubleFieldSource.getNumericDocValues(Map, LeafReaderContext).
func (f *DoubleFieldSource) getNumericDocValues(ctx function.Context, readerContext *index.LeafReaderContext) (index.NumericDocValues, error) {
	if f.numericDocValues != nil {
		return f.numericDocValues(ctx, readerContext)
	}
	return index.GetNumeric(readerContext.LeafReader(), f.Field)
}

// valueSource returns the most-derived ValueSource, standing in for Java's
// `this` inside getValues.
func (f *DoubleFieldSource) valueSource() function.ValueSource {
	if f.self != nil {
		return f.self
	}
	return f
}

type doubleDocValues struct {
	function.BaseFunctionValues
	source    function.ValueSource
	ndv       index.NumericDocValues
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
	val, err := f.DoubleVal(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s=%g", f.source.Description(), val), nil
}
