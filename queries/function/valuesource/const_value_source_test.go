// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/queries/function/valuesource"
)

func TestConstValueSource_DescriptionAndAccessors(t *testing.T) {
	t.Parallel()
	c := valuesource.NewConstValueSource(3.5)
	if got := c.Description(); got != "const(3.5)" {
		t.Fatalf("Description=%q", got)
	}
	if c.GetFloat() != 3.5 {
		t.Fatalf("GetFloat=%v", c.GetFloat())
	}
	if c.GetInt() != 3 {
		t.Fatalf("GetInt=%v", c.GetInt())
	}
	if c.GetLong() != 3 {
		t.Fatalf("GetLong=%v", c.GetLong())
	}
	if !c.GetBool() {
		t.Fatalf("GetBool false for non-zero constant")
	}
}

func TestConstValueSource_GetValuesYieldsConstant(t *testing.T) {
	t.Parallel()
	c := valuesource.NewConstValueSource(7)
	fv, err := c.GetValues(nil, nil)
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if v, _ := fv.FloatVal(0); v != 7 {
		t.Fatalf("FloatVal=%v", v)
	}
	if v, _ := fv.DoubleVal(123); v != 7 {
		t.Fatalf("DoubleVal=%v", v)
	}
	if s, _ := fv.ToString(0); s != "const(7)" {
		t.Fatalf("ToString=%q", s)
	}
}

func TestDoubleConstValueSource_GetValues(t *testing.T) {
	t.Parallel()
	c := valuesource.NewDoubleConstValueSource(0.25)
	fv, err := c.GetValues(nil, nil)
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if v, _ := fv.DoubleVal(0); v != 0.25 {
		t.Fatalf("DoubleVal=%v", v)
	}
	if v, _ := fv.LongVal(0); v != 0 {
		t.Fatalf("LongVal=%v", v)
	}
	if s, _ := fv.StrVal(0); s != "0.25" {
		t.Fatalf("StrVal=%q", s)
	}
}

func TestLiteralValueSource_GetValuesAndDescription(t *testing.T) {
	t.Parallel()
	l := valuesource.NewLiteralValueSource("foo")
	if got := l.Description(); got != "literal(foo)" {
		t.Fatalf("Description=%q", got)
	}
	fv, err := l.GetValues(nil, nil)
	if err != nil {
		t.Fatalf("GetValues: %v", err)
	}
	if s, _ := fv.StrVal(0); s != "foo" {
		t.Fatalf("StrVal=%q", s)
	}
	var bs []byte
	ok, err := fv.BytesVal(0, &bs)
	if err != nil || !ok {
		t.Fatalf("BytesVal: ok=%v err=%v", ok, err)
	}
	if string(bs) != "foo" {
		t.Fatalf("BytesVal payload=%q", bs)
	}
}

func TestConstValueSource_Equals(t *testing.T) {
	t.Parallel()
	a := valuesource.NewConstValueSource(1)
	b := valuesource.NewConstValueSource(1)
	c := valuesource.NewConstValueSource(2)
	if !a.Equals(b) {
		t.Fatalf("a != b")
	}
	if a.Equals(c) {
		t.Fatalf("a == c")
	}
}
