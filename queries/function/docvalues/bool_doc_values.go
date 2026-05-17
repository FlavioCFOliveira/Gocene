// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package docvalues

import (
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// BoolDocValues is the Go port of
// org.apache.lucene.queries.function.docvalues.BoolDocValues. Concrete
// embedders supply BoolVal; every other typed accessor coerces through
// the boolean result.
type BoolDocValues struct {
	function.BaseFunctionValues
	VS       function.ValueSource
	BoolFunc func(doc int) (bool, error)
}

// NewBoolDocValues returns a ready-to-use BoolDocValues. boolFunc is
// invoked from every coerced accessor; it must not be nil.
func NewBoolDocValues(vs function.ValueSource, boolFunc func(doc int) (bool, error)) *BoolDocValues {
	v := &BoolDocValues{VS: vs, BoolFunc: boolFunc}
	v.SetSelf(v)
	return v
}

// BoolVal delegates to the embedded predicate.
func (b *BoolDocValues) BoolVal(doc int) (bool, error) { return b.BoolFunc(doc) }

// ByteVal returns 1 / 0 from BoolVal.
func (b *BoolDocValues) ByteVal(doc int) (int8, error) {
	v, err := b.BoolFunc(doc)
	if err != nil || !v {
		return 0, err
	}
	return 1, nil
}

// ShortVal returns 1 / 0 from BoolVal.
func (b *BoolDocValues) ShortVal(doc int) (int16, error) {
	v, err := b.BoolFunc(doc)
	if err != nil || !v {
		return 0, err
	}
	return 1, nil
}

// FloatVal returns 1 / 0 from BoolVal.
func (b *BoolDocValues) FloatVal(doc int) (float32, error) {
	v, err := b.BoolFunc(doc)
	if err != nil || !v {
		return 0, err
	}
	return 1, nil
}

// IntVal returns 1 / 0 from BoolVal.
func (b *BoolDocValues) IntVal(doc int) (int32, error) {
	v, err := b.BoolFunc(doc)
	if err != nil || !v {
		return 0, err
	}
	return 1, nil
}

// LongVal returns 1 / 0 from BoolVal.
func (b *BoolDocValues) LongVal(doc int) (int64, error) {
	v, err := b.BoolFunc(doc)
	if err != nil || !v {
		return 0, err
	}
	return 1, nil
}

// DoubleVal returns 1 / 0 from BoolVal.
func (b *BoolDocValues) DoubleVal(doc int) (float64, error) {
	v, err := b.BoolFunc(doc)
	if err != nil || !v {
		return 0, err
	}
	return 1, nil
}

// StrVal returns "true" / "false".
func (b *BoolDocValues) StrVal(doc int) (string, error) {
	v, err := b.BoolFunc(doc)
	if err != nil {
		return "", err
	}
	return strconv.FormatBool(v), nil
}

// ObjectVal returns the bool when the doc has a value, otherwise nil.
func (b *BoolDocValues) ObjectVal(doc int) (any, error) {
	ok, err := b.Exists(doc)
	if err != nil || !ok {
		return nil, err
	}
	return b.BoolFunc(doc)
}

// ToString renders "<vs.description>=<true|false>".
func (b *BoolDocValues) ToString(doc int) (string, error) {
	s, err := b.StrVal(doc)
	if err != nil {
		return "", err
	}
	return b.VS.Description() + "=" + s, nil
}
