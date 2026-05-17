// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"math"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// constValueSource yields a fixed float for every document.
type constValueSource struct {
	BaseValueSource
	value float32
}

func newConstValueSource(v float32) *constValueSource { return &constValueSource{value: v} }

func (c *constValueSource) GetValues(_ Context, _ *index.LeafReaderContext) (FunctionValues, error) {
	fv := &constFunctionValues{value: c.value}
	fv.SetSelf(fv)
	return fv, nil
}
func (c *constValueSource) Equals(other ValueSource) bool {
	o, ok := other.(*constValueSource)
	return ok && o.value == c.value
}
func (c *constValueSource) HashCode() int32     { return hashFloat32(c.value) }
func (c *constValueSource) Description() string { return "const(" + fStr(c.value) + ")" }

func fStr(v float32) string {
	if math.Trunc(float64(v)) == float64(v) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(float64(v), 'g', -1, 32)
}

type constFunctionValues struct {
	BaseFunctionValues
	value float32
}

func (c *constFunctionValues) FloatVal(_ int) (float32, error)  { return c.value, nil }
func (c *constFunctionValues) DoubleVal(_ int) (float64, error) { return float64(c.value), nil }
func (c *constFunctionValues) IntVal(_ int) (int32, error)      { return int32(c.value), nil }
func (c *constFunctionValues) LongVal(_ int) (int64, error)     { return int64(c.value), nil }
func (c *constFunctionValues) ToString(_ int) (string, error)   { return "const", nil }

func TestFunctionQuery_EqualsAndHashCode(t *testing.T) {
	t.Parallel()
	q1 := NewFunctionQuery(newConstValueSource(1.5))
	q2 := NewFunctionQuery(newConstValueSource(1.5))
	q3 := NewFunctionQuery(newConstValueSource(2))
	if !q1.Equals(q2) {
		t.Fatalf("q1 != q2 for equal value sources")
	}
	if q1.HashCode() != q2.HashCode() {
		t.Fatalf("hash mismatch: %d vs %d", q1.HashCode(), q2.HashCode())
	}
	if q1.Equals(q3) {
		t.Fatalf("q1 == q3 for different value sources")
	}
}

func TestFunctionQuery_CloneIsIndependent(t *testing.T) {
	t.Parallel()
	q := NewFunctionQuery(newConstValueSource(3))
	c := q.Clone()
	if !q.Equals(c) {
		t.Fatalf("clone not equal to original")
	}
	if q == c {
		t.Fatalf("clone returned same pointer")
	}
}

func TestFunctionQuery_StringDescribesValueSource(t *testing.T) {
	t.Parallel()
	got := NewFunctionQuery(newConstValueSource(4)).String()
	if got != "const(4)" {
		t.Fatalf("String() = %q, want %q", got, "const(4)")
	}
}

func TestFunctionRangeQuery_StringInclusiveBounds(t *testing.T) {
	t.Parallel()
	q := NewFunctionRangeQuery(newConstValueSource(0), "1", "5", true, true)
	if got := q.String(); got != "frange(const(0)):[1 TO 5]" {
		t.Fatalf("String() = %q", got)
	}
}

func TestFunctionRangeQuery_StringExclusiveBounds(t *testing.T) {
	t.Parallel()
	q := NewFunctionRangeQuery(newConstValueSource(0), "1", "5", false, false)
	if got := q.String(); got != "frange(const(0)):{1 TO 5}" {
		t.Fatalf("String() = %q", got)
	}
}

func TestFunctionRangeQuery_StringUnboundedEnds(t *testing.T) {
	t.Parallel()
	q := NewFunctionRangeQueryUnbounded(newConstValueSource(0), "", false, "10", true, true, true)
	if got := q.String(); got != "frange(const(0)):[* TO 10]" {
		t.Fatalf("String() = %q", got)
	}
}

func TestFunctionRangeQuery_EqualsAcrossBoundFlags(t *testing.T) {
	t.Parallel()
	a := NewFunctionRangeQuery(newConstValueSource(0), "1", "5", true, true)
	b := NewFunctionRangeQuery(newConstValueSource(0), "1", "5", true, true)
	c := NewFunctionRangeQuery(newConstValueSource(0), "1", "5", true, false)
	if !a.Equals(b) {
		t.Fatalf("equal bounds: a != b")
	}
	if a.Equals(c) {
		t.Fatalf("different upper inclusivity: a == c")
	}
}

func TestNormaliseScore(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want float32
	}{
		{1, 1},
		{0, 0},
		{-1, 0},
		{float32(math.NaN()), 0},
	}
	for _, tc := range cases {
		got := normaliseScore(tc.in)
		// NaN comparisons fail; treat both as 0 above.
		if got != tc.want {
			t.Fatalf("normaliseScore(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
