// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package docvalues

import (
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/util/mutable"
)

// valueFiller renders the anonymous FunctionValues.ValueFiller subclasses
// that the getValueFiller() overrides of this package return: mval is the
// reused MutableValue (the anonymous class's private final field) and fill
// is the body of its fillValue(int).
type valueFiller struct {
	mval mutable.MutableValue
	fill func(doc int) error
}

// GetValue renders getValue().
func (f *valueFiller) GetValue() mutable.MutableValue { return f.mval }

// FillValue renders fillValue(int).
func (f *valueFiller) FillValue(doc int) error { return f.fill(doc) }

var _ function.ValueFiller = (*valueFiller)(nil)

// GetValueFiller renders BoolDocValues.getValueFiller(): a MutableValueBool
// filled from boolVal and exists.
func (b *BoolDocValues) GetValueFiller() function.ValueFiller {
	mval := mutable.NewMutableValueBool()
	return &valueFiller{mval: mval, fill: func(doc int) error {
		v, err := b.Self.BoolVal(doc)
		if err != nil {
			return err
		}
		mval.Value = v
		exists, err := b.Self.Exists(doc)
		if err != nil {
			return err
		}
		mval.SetExists(exists)
		return nil
	}}
}

// GetValueFiller renders DoubleDocValues.getValueFiller(): a
// MutableValueDouble filled from doubleVal and exists.
func (d *DoubleDocValues) GetValueFiller() function.ValueFiller {
	mval := mutable.NewMutableValueDouble()
	return &valueFiller{mval: mval, fill: func(doc int) error {
		v, err := d.Self.DoubleVal(doc)
		if err != nil {
			return err
		}
		mval.Value = v
		exists, err := d.Self.Exists(doc)
		if err != nil {
			return err
		}
		mval.SetExists(exists)
		return nil
	}}
}

// GetValueFiller renders FloatDocValues.getValueFiller(): a MutableValueFloat
// filled from floatVal and exists.
func (f *FloatDocValues) GetValueFiller() function.ValueFiller {
	mval := mutable.NewMutableValueFloat()
	return &valueFiller{mval: mval, fill: func(doc int) error {
		v, err := f.Self.FloatVal(doc)
		if err != nil {
			return err
		}
		mval.Value = v
		exists, err := f.Self.Exists(doc)
		if err != nil {
			return err
		}
		mval.SetExists(exists)
		return nil
	}}
}

// GetValueFiller renders IntDocValues.getValueFiller(): a MutableValueInt
// filled from intVal and exists.
func (i *IntDocValues) GetValueFiller() function.ValueFiller {
	mval := mutable.NewMutableValueInt()
	return &valueFiller{mval: mval, fill: func(doc int) error {
		v, err := i.Self.IntVal(doc)
		if err != nil {
			return err
		}
		mval.Value = v
		exists, err := i.Self.Exists(doc)
		if err != nil {
			return err
		}
		mval.SetExists(exists)
		return nil
	}}
}

// GetValueFiller renders LongDocValues.getValueFiller(): a MutableValueLong
// filled from longVal and exists.
func (l *LongDocValues) GetValueFiller() function.ValueFiller {
	mval := mutable.NewMutableValueLong()
	return &valueFiller{mval: mval, fill: func(doc int) error {
		v, err := l.Self.LongVal(doc)
		if err != nil {
			return err
		}
		mval.Value = v
		exists, err := l.Self.Exists(doc)
		if err != nil {
			return err
		}
		mval.SetExists(exists)
		return nil
	}}
}

// GetValueFiller renders StrDocValues.getValueFiller(): a MutableValueStr
// filled by bytesVal, whose result is the exists flag.
func (s *StrDocValues) GetValueFiller() function.ValueFiller {
	mval := mutable.NewMutableValueStr()
	return &valueFiller{mval: mval, fill: func(doc int) error {
		var target []byte
		exists, err := s.Self.BytesVal(doc, &target)
		if err != nil {
			return err
		}
		mval.Value = string(target)
		mval.SetExists(exists)
		return nil
	}}
}

// GetValueFiller renders DocTermsIndexDocValues.getValueFiller(): a
// MutableValueStr holding the term of the doc's ord, cleared and marked
// absent when the doc has no value.
func (d *DocTermsIndexDocValues) GetValueFiller() function.ValueFiller {
	mval := mutable.NewMutableValueStr()
	return &valueFiller{mval: mval, fill: func(doc int) error {
		ord, err := d.GetOrdForDoc(doc)
		if err != nil {
			return err
		}
		mval.Value = ""
		mval.SetExists(ord >= 0)
		if mval.Exists() {
			term, err := d.TermsIndex.LookupOrd(ord)
			if err != nil {
				return err
			}
			mval.Value = string(term)
		}
		return nil
	}}
}
