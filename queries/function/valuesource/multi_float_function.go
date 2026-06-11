// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// MultiFloatFunction is an abstract ValueSource that wraps multiple
// sources and applies an extendible float function to their values.
//
// Go port of org.apache.lucene.queries.function.valuesource.MultiFloatFunction.
type MultiFloatFunction struct {
	function.BaseValueSource
	Sources []function.ValueSource
	name    string
}

// NewMultiFloatFunction returns a MultiFloatFunction wrapping sources.
func NewMultiFloatFunction(sources []function.ValueSource, name string) *MultiFloatFunction {
	return &MultiFloatFunction{Sources: sources, name: name}
}

// Name returns the function name.
func (m *MultiFloatFunction) Name() string { return m.name }

// Description returns "name(src1,src2,...)".
func (m *MultiFloatFunction) Description() string {
	return MultiFunctionDescription(m.name, m.Sources)
}

// CreateWeight delegates to all sources.
func (m *MultiFloatFunction) CreateWeight(ctx function.Context, searcher any) error {
	for _, src := range m.Sources {
		if err := src.CreateWeight(ctx, searcher); err != nil {
			return err
		}
	}
	return nil
}

// GetValues returns FunctionValues that apply Func to the source values.
func (m *MultiFloatFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	valsArr, err := valsArrFromSources(m.Sources, ctx, readerContext)
	if err != nil {
		return nil, err
	}
	v := &multiFloatFunctionValues{
		FloatDocValues: *docvalues.NewFloatDocValues(m, func(doc int) (float32, error) {
			return m.Func(doc, valsArr)
		}),
		valsArr: valsArr,
		name:    m.name,
	}
	v.SetSelf(v)
	return v, nil
}

// Func applies the float function. Concrete types implement this.
func (m *MultiFloatFunction) Func(doc int, valsArr []function.FunctionValues) (float32, error) {
	return 0, nil
}

// ExistsFunc reports whether values exist. Default: true if all exist.
func (m *MultiFloatFunction) ExistsFunc(doc int, valsArr []function.FunctionValues) (bool, error) {
	return AllExists(doc, valsArr)
}

// Equals reports value equality.
func (m *MultiFloatFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(*MultiFloatFunction)
	if !ok || o == nil {
		return false
	}
	if m.name != o.name || len(m.Sources) != len(o.Sources) {
		return false
	}
	for i, src := range m.Sources {
		if !src.Equals(o.Sources[i]) {
			return false
		}
	}
	return true
}

// HashCode returns a stable hash.
func (m *MultiFloatFunction) HashCode() int32 {
	h := hashString(m.name)
	for _, src := range m.Sources {
		h += src.HashCode()
	}
	return h
}

type multiFloatFunctionValues struct {
	docvalues.FloatDocValues
	valsArr []function.FunctionValues
	name    string
}

func (v *multiFloatFunctionValues) ToString(doc int) (string, error) {
	return MultiFunctionToString(v.name, v.valsArr, doc)
}

var _ function.ValueSource = (*MultiFloatFunction)(nil)
