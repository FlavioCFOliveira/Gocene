// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// DefFunction is a ValueSource implementation which only returns the values
// from the provided ValueSources which are available for a particular docId.
// Consequently, when combined with a ConstValueSource, this function serves as
// a way to return a default value when the values for a field are unavailable.
//
// Mirrors org.apache.lucene.queries.function.valuesource.DefFunction.
type DefFunction struct {
	MultiFunction
}

var _ function.ValueSource = (*DefFunction)(nil)

// NewDefFunction mirrors DefFunction(List<ValueSource>).
func NewDefFunction(sources []function.ValueSource) *DefFunction {
	return &DefFunction{MultiFunction: *NewMultiFunction(sources, "def")}
}

// GetValues mirrors getValues(Map, LeafReaderContext).
func (d *DefFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	valsArr, err := valsArrFromSources(d.Sources, ctx, readerContext)
	if err != nil {
		return nil, err
	}
	v := &defFunctionValues{valsArr: valsArr, upto: len(valsArr) - 1, name: d.Name()}
	v.SetSelf(v)
	return v, nil
}

// defFunctionValues renders the anonymous MultiFunction.Values subclass that
// DefFunction.getValues returns.
type defFunctionValues struct {
	function.BaseFunctionValues
	valsArr []function.FunctionValues
	upto    int
	name    string
}

// get mirrors the private get(int doc) helper.
func (v *defFunctionValues) get(doc int) (function.FunctionValues, error) {
	for i := 0; i < v.upto; i++ {
		vals := v.valsArr[i]
		ok, err := vals.Exists(doc)
		if err != nil {
			return nil, err
		}
		if ok {
			return vals, nil
		}
	}
	return v.valsArr[v.upto], nil
}

func (v *defFunctionValues) ByteVal(doc int) (int8, error) {
	f, err := v.get(doc)
	if err != nil {
		return 0, err
	}
	return f.ByteVal(doc)
}

func (v *defFunctionValues) ShortVal(doc int) (int16, error) {
	f, err := v.get(doc)
	if err != nil {
		return 0, err
	}
	return f.ShortVal(doc)
}

func (v *defFunctionValues) FloatVal(doc int) (float32, error) {
	f, err := v.get(doc)
	if err != nil {
		return 0, err
	}
	return f.FloatVal(doc)
}

func (v *defFunctionValues) IntVal(doc int) (int32, error) {
	f, err := v.get(doc)
	if err != nil {
		return 0, err
	}
	return f.IntVal(doc)
}

func (v *defFunctionValues) LongVal(doc int) (int64, error) {
	f, err := v.get(doc)
	if err != nil {
		return 0, err
	}
	return f.LongVal(doc)
}

func (v *defFunctionValues) DoubleVal(doc int) (float64, error) {
	f, err := v.get(doc)
	if err != nil {
		return 0, err
	}
	return f.DoubleVal(doc)
}

func (v *defFunctionValues) StrVal(doc int) (string, error) {
	f, err := v.get(doc)
	if err != nil {
		return "", err
	}
	return f.StrVal(doc)
}

func (v *defFunctionValues) BoolVal(doc int) (bool, error) {
	f, err := v.get(doc)
	if err != nil {
		return false, err
	}
	return f.BoolVal(doc)
}

func (v *defFunctionValues) BytesVal(doc int, target *[]byte) (bool, error) {
	f, err := v.get(doc)
	if err != nil {
		return false, err
	}
	return f.BytesVal(doc, target)
}

func (v *defFunctionValues) ObjectVal(doc int) (any, error) {
	f, err := v.get(doc)
	if err != nil {
		return nil, err
	}
	return f.ObjectVal(doc)
}

// Exists mirrors the override, which returns true if any source exists.
func (v *defFunctionValues) Exists(doc int) (bool, error) {
	for _, vals := range v.valsArr {
		ok, err := vals.Exists(doc)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// ToString mirrors MultiFunction.Values.toString(int).
func (v *defFunctionValues) ToString(doc int) (string, error) {
	return MultiFunctionToString(v.name, v.valsArr, doc)
}
