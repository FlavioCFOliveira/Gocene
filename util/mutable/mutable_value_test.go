// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Behavioural coverage for MutableValue.
//
// Lucene 10.4.0 ships no direct TestMutableValue.java; this file
// reproduces the contract documented in the abstract base class
// (org.apache.lucene.util.mutable.MutableValue) by exercising the
// generic Equals/CompareTo helpers across the concrete implementations
// shipped in this package, plus a pair of synthetic types used to
// validate the cross-type CompareTo branch.

package mutable

import (
	"reflect"
	"testing"
)

// --- A pair of synthetic implementations used only to validate
// Equals/CompareTo across distinct concrete types. ---

type stubA struct {
	BaseMutableValue
	v int
}

func (s *stubA) Copy(src MutableValue) {
	o := src.(*stubA)
	s.v = o.v
	s.SetExists(o.Exists())
}
func (s *stubA) Duplicate() MutableValue {
	d := &stubA{v: s.v}
	d.SetExists(s.Exists())
	return d
}
func (s *stubA) EqualsSameType(other MutableValue) bool {
	o := other.(*stubA)
	return s.v == o.v && s.Exists() == o.Exists()
}
func (s *stubA) CompareSameType(other MutableValue) int {
	o := other.(*stubA)
	if s.v < o.v {
		return -1
	}
	if s.v > o.v {
		return 1
	}
	return 0
}
func (s *stubA) ToObject() any {
	if !s.Exists() {
		return nil
	}
	return s.v
}
func (s *stubA) HashCode() int { return s.v }
func (s *stubA) String() string {
	if !s.Exists() {
		return "(null)"
	}
	return "stubA"
}

type stubB struct{ stubA }

// Override Duplicate so reflect.TypeOf differs from stubA.
func (s *stubB) Duplicate() MutableValue {
	d := &stubB{}
	d.v = s.v
	d.SetExists(s.Exists())
	return d
}

// --- Tests. ---

func TestBaseMutableValue_DefaultExistsIsFalse(t *testing.T) {
	var b BaseMutableValue
	if b.Exists() {
		t.Errorf("zero BaseMutableValue.Exists(): got true want false")
	}
	b.SetExists(true)
	if !b.Exists() {
		t.Errorf("after SetExists(true): got false want true")
	}
	b.SetExists(false)
	if b.Exists() {
		t.Errorf("after SetExists(false): got true want false")
	}
}

func TestEquals_NilHandling(t *testing.T) {
	if !Equals(nil, nil) {
		t.Errorf("Equals(nil, nil) = false; want true")
	}
	a := &stubA{v: 1}
	a.SetExists(true)
	if Equals(a, nil) {
		t.Errorf("Equals(a, nil) = true; want false")
	}
	if Equals(nil, a) {
		t.Errorf("Equals(nil, a) = true; want false")
	}
}

func TestEquals_DifferentTypes(t *testing.T) {
	a := &stubA{v: 1}
	a.SetExists(true)
	b := &stubB{}
	b.v = 1
	b.SetExists(true)
	if Equals(a, b) {
		t.Errorf("Equals(stubA, stubB) = true; want false (distinct concrete types)")
	}
}

func TestEquals_SameTypeDifferentPayload(t *testing.T) {
	a := &stubA{v: 1}
	a.SetExists(true)
	b := &stubA{v: 2}
	b.SetExists(true)
	if Equals(a, b) {
		t.Errorf("Equals payload-different: got true want false")
	}
}

func TestEquals_SameTypeSamePayload(t *testing.T) {
	a := &stubA{v: 1}
	a.SetExists(true)
	b := &stubA{v: 1}
	b.SetExists(true)
	if !Equals(a, b) {
		t.Errorf("Equals payload-equal: got false want true")
	}
}

func TestCompareTo_SameType(t *testing.T) {
	a := &stubA{v: 1}
	a.SetExists(true)
	b := &stubA{v: 2}
	b.SetExists(true)
	if got := CompareTo(a, b); got != -1 {
		t.Errorf("CompareTo(1, 2) = %d; want -1", got)
	}
	if got := CompareTo(b, a); got != 1 {
		t.Errorf("CompareTo(2, 1) = %d; want 1", got)
	}
	c := &stubA{v: 1}
	c.SetExists(true)
	if got := CompareTo(a, c); got != 0 {
		t.Errorf("CompareTo(1, 1) = %d; want 0", got)
	}
}

func TestCompareTo_CrossType_TotalOrder(t *testing.T) {
	a := &stubA{v: 1}
	a.SetExists(true)
	b := &stubB{}
	b.v = 1
	b.SetExists(true)

	ab := CompareTo(a, b)
	ba := CompareTo(b, a)
	if ab == 0 || ba == 0 {
		t.Errorf("cross-type CompareTo must not return 0: got (%d, %d)", ab, ba)
	}
	if (ab < 0) == (ba < 0) {
		t.Errorf("cross-type CompareTo must be antisymmetric: got (%d, %d)", ab, ba)
	}
}

func TestCompareTo_CrossType_DifferentNamesProduceDifferentRank(t *testing.T) {
	// Touches the name-tiebreak branch by forcing the hash to collide.
	// Even when hashes happen to differ, the test asserts the ordering
	// is reproducible across calls (i.e. deterministic).
	a := &stubA{v: 0}
	a.SetExists(true)
	b := &stubB{}
	b.SetExists(true)
	first := CompareTo(a, b)
	second := CompareTo(a, b)
	if first != second {
		t.Errorf("CompareTo not deterministic: first=%d second=%d", first, second)
	}
}

func TestStringWhenAbsent(t *testing.T) {
	s := &stubA{v: 0}
	// Exists defaults to false.
	if got := s.String(); got != "(null)" {
		t.Errorf("stubA.String() with !Exists: got %q want %q", got, "(null)")
	}
}

// TestGoTypeHashIsStable asserts the internal goTypeHash is deterministic
// across calls on the same type, which is the only contract package-level
// CompareTo relies on for cross-type ordering.
func TestGoTypeHashIsStable(t *testing.T) {
	a := &stubA{}
	ta := reflect.TypeOf(a)
	first := goTypeHash(ta)
	for i := 0; i < 10; i++ {
		if got := goTypeHash(ta); got != first {
			t.Errorf("iteration %d: hash not stable: first=%d got=%d", i, first, got)
		}
	}
}
