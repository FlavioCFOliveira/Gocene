// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// IfFunction returns the value of trueSource if ifSource evaluates to true,
// otherwise returns the value of falseSource.
//
// Mirrors org.apache.lucene.queries.function.valuesource.IfFunction.
type IfFunction struct {
	BoolFunction
	ifSource    ValueSource
	trueSource  ValueSource
	falseSource ValueSource
}

// NewIfFunction constructs an IfFunction.
func NewIfFunction(ifSource, trueSource, falseSource ValueSource) *IfFunction {
	return &IfFunction{
		ifSource:    ifSource,
		trueSource:  trueSource,
		falseSource: falseSource,
	}
}

func (f *IfFunction) GetValues(ctx Context, readerContext *index.LeafReaderContext) (FunctionValues, error) {
	ifVals, err := f.ifSource.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	trueVals, err := f.trueSource.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	falseVals, err := f.falseSource.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}

	res := &ifFunctionValues{
		ifVals:    ifVals,
		trueVals:  trueVals,
		falseVals: falseVals,
	}
	res.SetSelf(res)
	return res, nil
}

func (f *IfFunction) Description() string {
	return fmt.Sprintf("if(%s,%s,%s)",
		f.ifSource.Description(),
		f.trueSource.Description(),
		f.falseSource.Description())
}

func (f *IfFunction) Equals(other ValueSource) bool {
	o, ok := other.(*IfFunction)
	if !ok {
		return false
	}
	return f.ifSource.Equals(o.ifSource) &&
		f.trueSource.Equals(o.trueSource) &&
		f.falseSource.Equals(o.falseSource)
}

func (f *IfFunction) HashCode() int32 {
	var h int32 = f.ifSource.HashCode()
	h = h*31 + f.trueSource.HashCode()
	h = h*31 + f.falseSource.HashCode()
	return h
}

func (f *IfFunction) CreateWeight(ctx Context, searcher any) error {
	if err := f.ifSource.CreateWeight(ctx, searcher); err != nil {
		return err
	}
	if err := f.trueSource.CreateWeight(ctx, searcher); err != nil {
		return err
	}
	return f.falseSource.CreateWeight(ctx, searcher)
}

type ifFunctionValues struct {
	BaseFunctionValues
	ifVals    FunctionValues
	trueVals  FunctionValues
	falseVals FunctionValues
}

func (v *ifFunctionValues) ByteVal(doc int) (int8, error) {
	if b, err := v.ifVals.BoolVal(doc); err == nil && b {
		return v.trueVals.ByteVal(doc)
	} else if err == nil {
		return v.falseVals.ByteVal(doc)
	} else {
		return 0, err
	}
}

func (v *ifFunctionValues) ShortVal(doc int) (int16, error) {
	if b, err := v.ifVals.BoolVal(doc); err == nil && b {
		return v.trueVals.ShortVal(doc)
	} else if err == nil {
		return v.falseVals.ShortVal(doc)
	} else {
		return 0, err
	}
}

func (v *ifFunctionValues) FloatVal(doc int) (float32, error) {
	if b, err := v.ifVals.BoolVal(doc); err == nil && b {
		return v.trueVals.FloatVal(doc)
	} else if err == nil {
		return v.falseVals.FloatVal(doc)
	} else {
		return 0, err
	}
}

func (v *ifFunctionValues) IntVal(doc int) (int32, error) {
	if b, err := v.ifVals.BoolVal(doc); err == nil && b {
		return v.trueVals.IntVal(doc)
	} else if err == nil {
		return v.falseVals.IntVal(doc)
	} else {
		return 0, err
	}
}

func (v *ifFunctionValues) LongVal(doc int) (int64, error) {
	if b, err := v.ifVals.BoolVal(doc); err == nil && b {
		return v.trueVals.LongVal(doc)
	} else if err == nil {
		return v.falseVals.LongVal(doc)
	} else {
		return 0, err
	}
}

func (v *ifFunctionValues) DoubleVal(doc int) (float64, error) {
	if b, err := v.ifVals.BoolVal(doc); err == nil && b {
		return v.trueVals.DoubleVal(doc)
	} else if err == nil {
		return v.falseVals.DoubleVal(doc)
	} else {
		return 0, err
	}
}

func (v *ifFunctionValues) StrVal(doc int) (string, error) {
	if b, err := v.ifVals.BoolVal(doc); err == nil && b {
		return v.trueVals.StrVal(doc)
	} else if err == nil {
		return v.falseVals.StrVal(doc)
	} else {
		return "", err
	}
}

func (v *ifFunctionValues) BoolVal(doc int) (bool, error) {
	if b, err := v.ifVals.BoolVal(doc); err == nil && b {
		return v.trueVals.BoolVal(doc)
	} else if err == nil {
		return v.falseVals.BoolVal(doc)
	} else {
		return false, err
	}
}

func (v *ifFunctionValues) BytesVal(doc int, target *[]byte) (bool, error) {
	if b, err := v.ifVals.BoolVal(doc); err == nil && b {
		return v.trueVals.BytesVal(doc, target)
	} else if err == nil {
		return v.falseVals.BytesVal(doc, target)
	} else {
		return false, err
	}
}

func (v *ifFunctionValues) ObjectVal(doc int) (any, error) {
	if b, err := v.ifVals.BoolVal(doc); err == nil && b {
		return v.trueVals.ObjectVal(doc)
	} else if err == nil {
		return v.falseVals.ObjectVal(doc)
	} else {
		return nil, err
	}
}

func (v *ifFunctionValues) Exists(doc int) (bool, error) {
	if b, err := v.ifVals.BoolVal(doc); err == nil && b {
		return v.trueVals.Exists(doc)
	} else if err == nil {
		return v.falseVals.Exists(doc)
	} else {
		return false, err
	}
}

func (v *ifFunctionValues) ToString(doc int) (string, error) {
	ifStr, err := v.ifVals.ToString(doc)
	if err != nil {
		return "", err
	}
	ifTrue, err := v.trueVals.ToString(doc)
	if err != nil {
		return "", err
	}
	ifFalse, err := v.falseVals.ToString(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("if(%s,%s,%s)", ifStr, ifTrue, ifFalse), nil
}
