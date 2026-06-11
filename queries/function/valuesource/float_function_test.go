// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/valuesource"
)

func src2(a, b function.ValueSource) []function.ValueSource {
	return []function.ValueSource{a, b}
}

// ---------------------------------------------------------------------------
// DivFloatFunction
// ---------------------------------------------------------------------------

func TestDivFloatFunction_Description(t *testing.T) {
	t.Parallel()
	d := valuesource.NewDivFloatFunction(
		valuesource.NewConstValueSource(10),
		valuesource.NewConstValueSource(2))
	if got := d.Description(); got != "div(const(10),const(2))" {
		t.Fatalf("Description=%q", got)
	}
}

// ---------------------------------------------------------------------------
// SumFloatFunction
// ---------------------------------------------------------------------------

func TestSumFloatFunction_Description(t *testing.T) {
	t.Parallel()
	s := valuesource.NewSumFloatFunction(src2(
		valuesource.NewConstValueSource(1),
		valuesource.NewConstValueSource(2)))
	if got := s.Description(); got != "sum(const(1),const(2))" {
		t.Fatalf("Description=%q", got)
	}
}

// ---------------------------------------------------------------------------
// ProductFloatFunction
// ---------------------------------------------------------------------------

func TestProductFloatFunction_Description(t *testing.T) {
	t.Parallel()
	p := valuesource.NewProductFloatFunction(src2(
		valuesource.NewConstValueSource(2),
		valuesource.NewConstValueSource(3)))
	if got := p.Description(); got != "product(const(2),const(3))" {
		t.Fatalf("Description=%q", got)
	}
}

// ---------------------------------------------------------------------------
// MaxFloatFunction
// ---------------------------------------------------------------------------

func TestMaxFloatFunction_Description(t *testing.T) {
	t.Parallel()
	m := valuesource.NewMaxFloatFunction(src2(
		valuesource.NewConstValueSource(1),
		valuesource.NewConstValueSource(9)))
	if got := m.Description(); got != "max(const(1),const(9))" {
		t.Fatalf("Description=%q", got)
	}
}

// ---------------------------------------------------------------------------
// MinFloatFunction
// ---------------------------------------------------------------------------

func TestMinFloatFunction_Description(t *testing.T) {
	t.Parallel()
	m := valuesource.NewMinFloatFunction(src2(
		valuesource.NewConstValueSource(3),
		valuesource.NewConstValueSource(7)))
	if got := m.Description(); got != "min(const(3),const(7))" {
		t.Fatalf("Description=%q", got)
	}
}

// ---------------------------------------------------------------------------
// LinearFloatFunction
// ---------------------------------------------------------------------------

func TestLinearFloatFunction_Description(t *testing.T) {
	t.Parallel()
	l := valuesource.NewLinearFloatFunction(
		valuesource.NewConstValueSource(5), 2.0, 1.0)
	if got := l.Description(); got != "2*float(const(5))+1" {
		t.Fatalf("Description=%q, want '2*float(const(5))+1'", got)
	}
}

func TestLinearFloatFunction_GetValues(t *testing.T) {
	t.Parallel()
	l := valuesource.NewLinearFloatFunction(
		valuesource.NewConstValueSource(5), 2.0, 0.0)
	fv, err := l.GetValues(nil, nil)
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if v, _ := fv.FloatVal(0); v != 10.0 {
		t.Fatalf("FloatVal=%v, want 10.0", v)
	}
}

// ---------------------------------------------------------------------------
// PowFloatFunction
// ---------------------------------------------------------------------------

func TestPowFloatFunction_Description(t *testing.T) {
	t.Parallel()
	p := valuesource.NewPowFloatFunction(
		valuesource.NewConstValueSource(2),
		valuesource.NewConstValueSource(3))
	if got := p.Description(); got != "pow(const(2),const(3))" {
		t.Fatalf("Description=%q", got)
	}
}

// ---------------------------------------------------------------------------
// IfFunction
// ---------------------------------------------------------------------------

func TestIfFunction_Description(t *testing.T) {
	t.Parallel()
	s := valuesource.NewIfFunction(
		valuesource.NewConstValueSource(1),
		valuesource.NewConstValueSource(10),
		valuesource.NewConstValueSource(20))
	if got := s.Description(); got != "if(const(1),const(10),const(20))" {
		t.Fatalf("Description=%q", got)
	}
}

func TestIfFunction_GetValues_TrueCase(t *testing.T) {
	t.Parallel()
	s := valuesource.NewIfFunction(
		valuesource.NewConstValueSource(1),
		valuesource.NewConstValueSource(100),
		valuesource.NewConstValueSource(200))
	fv, err := s.GetValues(nil, nil)
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if v, _ := fv.FloatVal(0); v != 100.0 {
		t.Fatalf("FloatVal=%v, want 100.0 (true branch)", v)
	}
}

func TestIfFunction_GetValues_FalseCase(t *testing.T) {
	t.Parallel()
	s := valuesource.NewIfFunction(
		valuesource.NewConstValueSource(0),
		valuesource.NewConstValueSource(100),
		valuesource.NewConstValueSource(200))
	fv, err := s.GetValues(nil, nil)
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if v, _ := fv.FloatVal(0); v != 200.0 {
		t.Fatalf("FloatVal=%v, want 200.0 (false branch)", v)
	}
}

// ---------------------------------------------------------------------------
// LiteralValueSource
// ---------------------------------------------------------------------------

func TestLiteralValueSource_Description(t *testing.T) {
	t.Parallel()
	l := valuesource.NewLiteralValueSource("hello")
	if got := l.Description(); got != "literal(hello)" {
		t.Fatalf("Description=%q", got)
	}
	if got := l.GetValue(); got != "hello" {
		t.Fatalf("GetValue=%q", got)
	}
}

func TestLiteralValueSource_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	l1 := valuesource.NewLiteralValueSource("abc")
	l2 := valuesource.NewLiteralValueSource("abc")
	l3 := valuesource.NewLiteralValueSource("xyz")
	if !l1.Equals(l2) {
		t.Error("Equal literals should compare equal")
	}
	if l1.HashCode() != l2.HashCode() {
		t.Error("Equal literals should have equal hash codes")
	}
	if l1.Equals(l3) {
		t.Error("Different literals should not compare equal")
	}
}

func TestLiteralValueSource_NotEqualsNonLiteral(t *testing.T) {
	t.Parallel()
	l := valuesource.NewLiteralValueSource("abc")
	c := valuesource.NewConstValueSource(1)
	if l.Equals(c) {
		t.Error("Literal should not equal ConstValueSource")
	}
}

// ---------------------------------------------------------------------------
// ConstValueSource extended accessors
// ---------------------------------------------------------------------------

func TestConstValueSource_IntAndLongAccessors(t *testing.T) {
	t.Parallel()
	c := valuesource.NewConstValueSource(42.7)
	if got := c.GetInt(); got != 42 {
		t.Fatalf("GetInt=%v, want 42", got)
	}
	if got := c.GetLong(); got != 42 {
		t.Fatalf("GetLong=%v, want 42", got)
	}
}

func TestConstValueSource_Bool(t *testing.T) {
	t.Parallel()
	if c := valuesource.NewConstValueSource(0); c.GetBool() {
		t.Error("zero value should return false")
	}
	if c := valuesource.NewConstValueSource(1); !c.GetBool() {
		t.Error("non-zero value should return true")
	}
}

// ---------------------------------------------------------------------------
// DoubleConstValueSource
// ---------------------------------------------------------------------------

func TestDoubleConstValueSource_Accessors(t *testing.T) {
	t.Parallel()
	c := valuesource.NewDoubleConstValueSource(0.5)
	if got := c.GetDouble(); got != 0.5 {
		t.Fatalf("GetDouble=%v", got)
	}
	if got := c.GetInt(); got != 0 {
		t.Fatalf("GetInt=%v, want 0", got)
	}
	if got := c.GetLong(); got != 0 {
		t.Fatalf("GetLong=%v, want 0", got)
	}
}

func TestDoubleConstValueSource_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	d1 := valuesource.NewDoubleConstValueSource(3.14)
	d2 := valuesource.NewDoubleConstValueSource(3.14)
	d3 := valuesource.NewDoubleConstValueSource(2.71)
	if !d1.Equals(d2) {
		t.Error("Equal constants should compare equal")
	}
	if d1.HashCode() != d2.HashCode() {
		t.Error("Equal constants should have equal hash codes")
	}
	if d1.Equals(d3) {
		t.Error("Different constants should not compare equal")
	}
}
