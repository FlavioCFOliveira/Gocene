// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// IfFunction returns the value of trueSource if the ifSource evaluates to
// true, otherwise the value of falseSource.
//
// Go port of org.apache.lucene.queries.function.valuesource.IfFunction.
type IfFunction struct {
	*BoolFunction
	IfSource    function.ValueSource
	TrueSource  function.ValueSource
	FalseSource function.ValueSource
}

// NewIfFunction creates an IfFunction.
func NewIfFunction(ifSource, trueSource, falseSource function.ValueSource) *IfFunction {
	return &IfFunction{
		BoolFunction: &BoolFunction{},
		IfSource:     ifSource,
		TrueSource:   trueSource,
		FalseSource:  falseSource,
	}
}

// Description returns "if(ifSource,trueSource,falseSource)".
func (s *IfFunction) Description() string {
	return "if(" + s.IfSource.Description() + "," + s.TrueSource.Description() + "," + s.FalseSource.Description() + ")"
}

// CreateWeight delegates to all three sources.
func (s *IfFunction) CreateWeight(ctx function.Context, searcher any) error {
	if err := s.IfSource.CreateWeight(ctx, searcher); err != nil {
		return err
	}
	if err := s.TrueSource.CreateWeight(ctx, searcher); err != nil {
		return err
	}
	return s.FalseSource.CreateWeight(ctx, searcher)
}

// GetValues returns FunctionValues that branch on the boolean ifSource.
func (s *IfFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	ifVals, err := s.IfSource.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	trueVals, err := s.TrueSource.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	falseVals, err := s.FalseSource.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}

	return &ifFunctionValues{
		ifVals:    ifVals,
		trueVals:  trueVals,
		falseVals: falseVals,
	}, nil
}

// Equals reports value equality.
func (s *IfFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(*IfFunction)
	if !ok || o == nil {
		return false
	}
	return s.IfSource.Equals(o.IfSource) && s.TrueSource.Equals(o.TrueSource) && s.FalseSource.Equals(o.FalseSource)
}

// HashCode returns a stable hash.
func (s *IfFunction) HashCode() int32 {
	h := s.IfSource.HashCode()
	h = h*31 + s.TrueSource.HashCode()
	h = h*31 + s.FalseSource.HashCode()
	return h
}

type ifFunctionValues struct {
	function.BaseFunctionValues
	ifVals, trueVals, falseVals function.FunctionValues
}

func (v *ifFunctionValues) branch(doc int) bool {
	b, _ := v.ifVals.BoolVal(doc)
	return b
}

func (v *ifFunctionValues) ByteVal(doc int) (int8, error) {
	if v.branch(doc) {
		return v.trueVals.ByteVal(doc)
	}
	return v.falseVals.ByteVal(doc)
}

func (v *ifFunctionValues) ShortVal(doc int) (int16, error) {
	if v.branch(doc) {
		return v.trueVals.ShortVal(doc)
	}
	return v.falseVals.ShortVal(doc)
}

func (v *ifFunctionValues) FloatVal(doc int) (float32, error) {
	if v.branch(doc) {
		return v.trueVals.FloatVal(doc)
	}
	return v.falseVals.FloatVal(doc)
}

func (v *ifFunctionValues) IntVal(doc int) (int32, error) {
	if v.branch(doc) {
		return v.trueVals.IntVal(doc)
	}
	return v.falseVals.IntVal(doc)
}

func (v *ifFunctionValues) LongVal(doc int) (int64, error) {
	if v.branch(doc) {
		return v.trueVals.LongVal(doc)
	}
	return v.falseVals.LongVal(doc)
}

func (v *ifFunctionValues) DoubleVal(doc int) (float64, error) {
	if v.branch(doc) {
		return v.trueVals.DoubleVal(doc)
	}
	return v.falseVals.DoubleVal(doc)
}

func (v *ifFunctionValues) StrVal(doc int) (string, error) {
	if v.branch(doc) {
		return v.trueVals.StrVal(doc)
	}
	return v.falseVals.StrVal(doc)
}

func (v *ifFunctionValues) BoolVal(doc int) (bool, error) {
	if v.branch(doc) {
		return v.trueVals.BoolVal(doc)
	}
	return v.falseVals.BoolVal(doc)
}

func (v *ifFunctionValues) BytesVal(doc int, target *[]byte) (bool, error) {
	if v.branch(doc) {
		return v.trueVals.BytesVal(doc, target)
	}
	return v.falseVals.BytesVal(doc, target)
}

func (v *ifFunctionValues) ObjectVal(doc int) (any, error) {
	if v.branch(doc) {
		return v.trueVals.ObjectVal(doc)
	}
	return v.falseVals.ObjectVal(doc)
}

func (v *ifFunctionValues) Exists(doc int) (bool, error) {
	if v.branch(doc) {
		return v.trueVals.Exists(doc)
	}
	return v.falseVals.Exists(doc)
}

func (v *ifFunctionValues) Cost() float32 {
	return v.ifVals.Cost() + v.trueVals.Cost() + v.falseVals.Cost()
}

func (v *ifFunctionValues) ToString(doc int) (string, error) {
	is, err := v.ifVals.ToString(doc)
	if err != nil {
		return "", err
	}
	ts, err := v.trueVals.ToString(doc)
	if err != nil {
		return "", err
	}
	fs, err := v.falseVals.ToString(doc)
	if err != nil {
		return "", err
	}
	return "if(" + is + "," + ts + "," + fs + ")", nil
}

func (v *ifFunctionValues) GetValueFiller() function.ValueFiller {
	return v.BaseFunctionValues.GetValueFiller()
}

var _ function.ValueSource = (*IfFunction)(nil)
