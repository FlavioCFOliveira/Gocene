// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"errors"
	"testing"
)

// fixedFloatValues is a minimal FunctionValues used to exercise the
// BaseFunctionValues defaults. Only FloatVal and ToString are overridden;
// every other typed accessor flows through the base ErrUnsupportedValue.
type fixedFloatValues struct {
	BaseFunctionValues
	score float32
}

func newFixedFloatValues(score float32) *fixedFloatValues {
	v := &fixedFloatValues{score: score}
	v.SetSelf(v)
	return v
}

func (f *fixedFloatValues) FloatVal(_ int) (float32, error) { return f.score, nil }
func (f *fixedFloatValues) ToString(_ int) (string, error)  { return "fixed", nil }
func (f *fixedFloatValues) IntVal(_ int) (int32, error)     { return int32(f.score), nil }

func TestBaseFunctionValues_DefaultsReturnUnsupported(t *testing.T) {
	t.Parallel()
	v := &fixedFloatValues{}
	v.SetSelf(v)
	if _, err := v.LongVal(0); !errors.Is(err, ErrUnsupportedValue) {
		t.Fatalf("LongVal default: want ErrUnsupportedValue, got %v", err)
	}
	if _, err := v.DoubleVal(0); !errors.Is(err, ErrUnsupportedValue) {
		t.Fatalf("DoubleVal default: want ErrUnsupportedValue, got %v", err)
	}
	if _, err := v.OrdVal(0); !errors.Is(err, ErrUnsupportedValue) {
		t.Fatalf("OrdVal default: want ErrUnsupportedValue, got %v", err)
	}
	if _, err := v.NumOrd(); !errors.Is(err, ErrUnsupportedValue) {
		t.Fatalf("NumOrd default: want ErrUnsupportedValue, got %v", err)
	}
	if err := v.IntValMulti(0, nil); !errors.Is(err, ErrUnsupportedValue) {
		t.Fatalf("IntValMulti default: want ErrUnsupportedValue, got %v", err)
	}
}

func TestBaseFunctionValues_DefaultBoolVal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		score float32
		want  bool
	}{
		{"zero", 0, false},
		{"positive", 1, true},
		{"negative", -3, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := newFixedFloatValues(tc.score)
			got, err := v.BoolVal(0)
			if err != nil {
				t.Fatalf("BoolVal: %v", err)
			}
			if got != tc.want {
				t.Fatalf("BoolVal(%v) = %v, want %v", tc.score, got, tc.want)
			}
		})
	}
}

func TestBaseFunctionValues_DefaultCostAndExists(t *testing.T) {
	t.Parallel()
	v := newFixedFloatValues(1)
	if got := v.Cost(); got != 100 {
		t.Fatalf("Cost default: got %v, want 100", got)
	}
	exists, err := v.Exists(0)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatalf("Exists default: got false, want true")
	}
}

func TestBaseFunctionValues_DefaultValueFillerUsesFloatVal(t *testing.T) {
	t.Parallel()
	v := newFixedFloatValues(2.5)
	filler := v.GetValueFiller()
	if err := filler.FillValue(0); err != nil {
		t.Fatalf("FillValue: %v", err)
	}
	mv := filler.GetValue()
	if !mv.Exists || mv.Value != 2.5 {
		t.Fatalf("MutableValueFloat = %+v, want {Value:2.5 Exists:true}", *mv)
	}
}

func TestBaseFunctionValues_BytesValCopiesStrVal(t *testing.T) {
	t.Parallel()
	v := &strFunctionValues{value: "hello"}
	v.SetSelf(v)
	var out []byte
	ok, err := v.BytesVal(0, &out)
	if err != nil {
		t.Fatalf("BytesVal: %v", err)
	}
	if !ok || string(out) != "hello" {
		t.Fatalf("BytesVal: got (%v, %q), want (true, %q)", ok, out, "hello")
	}
}

func TestBaseFunctionValues_ExplainStringifies(t *testing.T) {
	t.Parallel()
	v := newFixedFloatValues(1.25)
	got, err := v.Explain(0)
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	const want = "1.25 (fixed)"
	if got != want {
		t.Fatalf("Explain = %q, want %q", got, want)
	}
}

// strFunctionValues overrides StrVal to feed BytesVal.
type strFunctionValues struct {
	BaseFunctionValues
	value string
}

func (s *strFunctionValues) StrVal(_ int) (string, error)    { return s.value, nil }
func (s *strFunctionValues) ToString(_ int) (string, error)  { return "str", nil }
func (s *strFunctionValues) FloatVal(_ int) (float32, error) { return 0, nil }
