// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// VectorValueSource converts individual ValueSource instances to leverage the
// FunctionValues *Val functions that work with multiple values.
type VectorValueSource struct {
	function.BaseValueSource
	sources []function.ValueSource
}

// NewVectorValueSource creates a new VectorValueSource with the given sources.
func NewVectorValueSource(sources []function.ValueSource) *VectorValueSource {
	return &VectorValueSource{
		sources: sources,
	}
}

// Dimension returns the number of value sources.
func (v *VectorValueSource) Dimension() int {
	return len(v.sources)
}

// Name returns the name of this value source.
func (v *VectorValueSource) Name() string {
	return "vector"
}

// GetValues returns the aggregated FunctionValues for the given context and reader.
func (v *VectorValueSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	size := len(v.sources)

	// special-case x,y and lat,lon since it's so common
	if size == 2 {
		x, err := v.sources[0].GetValues(ctx, readerContext)
		if err != nil {
			return nil, err
		}
		y, err := v.sources[1].GetValues(ctx, readerContext)
		if err != nil {
			return nil, err
		}

		return &vectorValueSourceTwo{
			x: x,
			y: y,
			name: v.Name(),
		}, nil
	}

	valsArr := make([]function.FunctionValues, size)
	for i := 0; i < size; i++ {
		fv, err := v.sources[i].GetValues(ctx, readerContext)
		if err != nil {
			return nil, err
		}
		valsArr[i] = fv
	}

	return &vectorValueSourceMulti{
		valsArr: valsArr,
		name:    v.Name(),
	}, nil
}

// CreateWeight ensures all sources are weighted.
func (v *VectorValueSource) CreateWeight(ctx function.Context, searcher any) error {
	for _, source := range v.sources {
		if err := source.CreateWeight(ctx, searcher); err != nil {
			return err
		}
	}
	return nil
}

// Description renders a human-readable representation of the sources.
func (v *VectorValueSource) Description() string {
	var sb strings.Builder
	sb.WriteString(v.Name())
	sb.WriteByte('(')
	for i, source := range v.sources {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(source.Description())
	}
	sb.WriteByte(')')
	return sb.String()
}

// Equals reports value-equality.
func (v *VectorValueSource) Equals(other function.ValueSource) bool {
	that, ok := other.(*VectorValueSource)
	if !ok {
		return false
	}
	if len(v.sources) != len(that.sources) {
		return false
	}
	for i := range v.sources {
		if !v.sources[i].Equals(that.sources[i]) {
			return false
		}
	}
	return true
}

// HashCode returns a stable hash.
func (v *VectorValueSource) HashCode() int32 {
	var h int32 = 1
	for _, source := range v.sources {
		h = 31*h + source.HashCode()
	}
	return h
}

type vectorValueSourceTwo struct {
	function.BaseFunctionValues
	x, y function.FunctionValues
	name string
}

func (v *vectorValueSourceTwo) ByteValMulti(doc int, vals []int8) error {
	b1, err := v.x.ByteVal(doc)
	if err != nil {
		return err
	}
	b2, err := v.y.ByteVal(doc)
	if err != nil {
		return err
	}
	vals[0] = b1
	vals[1] = b2
	return nil
}

func (v *vectorValueSourceTwo) ShortValMulti(doc int, vals []int16) error {
	s1, err := v.x.ShortVal(doc)
	if err != nil {
		return err
	}
	s2, err := v.y.ShortVal(doc)
	if err != nil {
		return err
	}
	vals[0] = s1
	vals[1] = s2
	return nil
}

func (v *vectorValueSourceTwo) IntValMulti(doc int, vals []int32) error {
	i1, err := v.x.IntVal(doc)
	if err != nil {
		return err
	}
	i2, err := v.y.IntVal(doc)
	if err != nil {
		return err
	}
	vals[0] = i1
	vals[1] = i2
	return nil
}

func (v *vectorValueSourceTwo) LongValMulti(doc int, vals []int64) error {
	l1, err := v.x.LongVal(doc)
	if err != nil {
		return err
	}
	l2, err := v.y.LongVal(doc)
	if err != nil {
		return err
	}
	vals[0] = l1
	vals[1] = l2
	return nil
}

func (v *vectorValueSourceTwo) FloatValMulti(doc int, vals []float32) error {
	f1, err := v.x.FloatVal(doc)
	if err != nil {
		return err
	}
	f2, err := v.y.FloatVal(doc)
	if err != nil {
		return err
	}
	vals[0] = f1
	vals[1] = f2
	return nil
}

func (v *vectorValueSourceTwo) DoubleValMulti(doc int, vals []float64) error {
	d1, err := v.x.DoubleVal(doc)
	if err != nil {
		return err
	}
	d2, err := v.y.DoubleVal(doc)
	if err != nil {
		return err
	}
	vals[0] = d1
	vals[1] = d2
	return nil
}

func (v *vectorValueSourceTwo) StrValMulti(doc int, vals []string) error {
	s1, err := v.x.StrVal(doc)
	if err != nil {
		return err
	}
	s2, err := v.y.StrVal(doc)
	if err != nil {
		return err
	}
	vals[0] = s1
	vals[1] = s2
	return nil
}

func (v *vectorValueSourceTwo) ToString(doc int) (string, error) {
	s1, err := v.x.ToString(doc)
	if err != nil {
		return "", err
	}
	s2, err := v.y.ToString(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s(%s,%s)", v.name, s1, s2), nil
}

type vectorValueSourceMulti struct {
	function.BaseFunctionValues
	valsArr []function.FunctionValues
	name    string
}

func (v *vectorValueSourceMulti) ByteValMulti(doc int, vals []int8) error {
	for i, fv := range v.valsArr {
		bv, err := fv.ByteVal(doc)
		if err != nil {
			return err
		}
		vals[i] = bv
	}
	return nil
}

func (v *vectorValueSourceMulti) ShortValMulti(doc int, vals []int16) error {
	for i, fv := range v.valsArr {
		sv, err := fv.ShortVal(doc)
		if err != nil {
			return err
		}
		vals[i] = sv
	}
	return nil
}

func (v *vectorValueSourceMulti) FloatValMulti(doc int, vals []float32) error {
	for i, fv := range v.valsArr {
		fv_val, err := fv.FloatVal(doc)
		if err != nil {
			return err
		}
		vals[i] = fv_val
	}
	return nil
}

func (v *vectorValueSourceMulti) IntValMulti(doc int, vals []int32) error {
	for i, fv := range v.valsArr {
		iv, err := fv.IntVal(doc)
		if err != nil {
			return err
		}
		vals[i] = iv
	}
	return nil
}

func (v *vectorValueSourceMulti) LongValMulti(doc int, vals []int64) error {
	for i, fv := range v.valsArr {
		lv, err := fv.LongVal(doc)
		if err != nil {
			return err
		}
		vals[i] = lv
	}
	return nil
}

func (v *vectorValueSourceMulti) DoubleValMulti(doc int, vals []float64) error {
	for i, fv := range v.valsArr {
		dv, err := fv.DoubleVal(doc)
		if err != nil {
			return err
		}
		vals[i] = dv
	}
	return nil
}

func (v *vectorValueSourceMulti) StrValMulti(doc int, vals []string) error {
	for i, fv := range v.valsArr {
		sv, err := fv.StrVal(doc)
		if err != nil {
			return err
		}
		vals[i] = sv
	}
	return nil
}

func (v *vectorValueSourceMulti) ToString(doc int) (string, error) {
	var sb strings.Builder
	sb.WriteString(v.name)
	sb.WriteByte('(')
	for i, fv := range v.valsArr {
		if i > 0 {
			sb.WriteByte(',')
		}
		s, err := fv.ToString(doc)
		if err != nil {
			return "", err
		}
		sb.WriteString(s)
	}
	sb.WriteByte(')')
	return sb.String(), nil
}
