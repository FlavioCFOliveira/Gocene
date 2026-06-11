// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestXYPointSortField_Constructor_RejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	nan32 := float32(math.NaN())
	posInf32 := float32(math.Inf(1))
	negInf32 := float32(math.Inf(-1))

	cases := []struct {
		name string
		f    string
		x, y float32
	}{
		{"empty_field", "", 0, 0},
		{"x_nan", "loc", nan32, 0},
		{"y_nan", "loc", 0, nan32},
		{"x_pos_inf", "loc", posInf32, 0},
		{"y_pos_inf", "loc", 0, posInf32},
		{"x_neg_inf", "loc", negInf32, 0},
		{"y_neg_inf", "loc", 0, negInf32},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewXYPointSortField(tc.f, tc.x, tc.y); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestXYPointSortField_Constructor_AcceptsExtremeFiniteFloats(t *testing.T) {
	t.Parallel()

	// Cartesian plane is unbounded: Float.MAX_VALUE must be accepted.
	cases := []struct {
		name string
		x, y float32
	}{
		{"max", math.MaxFloat32, math.MaxFloat32},
		{"neg_max", -math.MaxFloat32, -math.MaxFloat32},
		{"zero", 0, 0},
		{"sub_normal", math.SmallestNonzeroFloat32, math.SmallestNonzeroFloat32},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sf, err := NewXYPointSortField("loc", tc.x, tc.y)
			if err != nil {
				t.Fatalf("ctor: %v", err)
			}
			if sf.X() != tc.x || sf.Y() != tc.y {
				t.Errorf("origin = (%v,%v), want (%v,%v)", sf.X(), sf.Y(), tc.x, tc.y)
			}
		})
	}
}

func TestXYPointSortField_Constructor_DefaultsCustomAndAscending(t *testing.T) {
	t.Parallel()

	sf, err := NewXYPointSortField("loc", 3.5, -2.25)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if sf.SortField == nil {
		t.Fatalf("embedded SortField must not be nil")
	}
	if sf.SortField.Field != "loc" {
		t.Errorf("Field = %q, want %q", sf.SortField.Field, "loc")
	}
	if sf.SortField.Type != SortFieldTypeCustom {
		t.Errorf("Type = %v, want SortFieldTypeCustom", sf.SortField.Type)
	}
	if sf.SortField.Reverse {
		t.Errorf("Reverse must default to false (ascending: closest first)")
	}
	if sf.X() != 3.5 || sf.Y() != -2.25 {
		t.Errorf("origin = (%v,%v), want (3.5,-2.25)", sf.X(), sf.Y())
	}
	if got := sf.GetMissingValue(); !math.IsInf(got, 1) {
		t.Errorf("MissingValue = %v, want +Inf", got)
	}
}

func TestXYPointSortField_SetMissingValue_OnlyAcceptsPositiveInfinity(t *testing.T) {
	t.Parallel()

	sf, err := NewXYPointSortField("loc", 0, 0)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	if err := sf.SetMissingValue(math.Inf(1)); err != nil {
		t.Errorf("SetMissingValue(+Inf) unexpected error: %v", err)
	}

	rejections := []interface{}{
		float64(0),
		float64(1),
		math.Inf(-1),
		math.NaN(),
		"infinity",
		nil,
		int(42),
	}
	for _, v := range rejections {
		err := sf.SetMissingValue(v)
		if err == nil {
			t.Errorf("SetMissingValue(%v): expected error, got nil", v)
			continue
		}
		if !errors.Is(err, ErrXYPointSortFieldInvalidMissingValue) {
			t.Errorf("SetMissingValue(%v) = %v, want ErrXYPointSortFieldInvalidMissingValue", v, err)
		}
	}
	// MissingValue must remain unchanged at +Inf after every rejection.
	if got := sf.GetMissingValue(); !math.IsInf(got, 1) {
		t.Errorf("MissingValue mutated by rejection path: got %v", got)
	}
}

func TestXYPointSortField_Equals_HashCode(t *testing.T) {
	t.Parallel()

	a, err := NewXYPointSortField("loc", 1.0, 2.0)
	if err != nil {
		t.Fatalf("ctor a: %v", err)
	}
	b, err := NewXYPointSortField("loc", 1.0, 2.0)
	if err != nil {
		t.Fatalf("ctor b: %v", err)
	}
	differentField, err := NewXYPointSortField("other", 1.0, 2.0)
	if err != nil {
		t.Fatalf("ctor differentField: %v", err)
	}
	differentX, err := NewXYPointSortField("loc", 1.5, 2.0)
	if err != nil {
		t.Fatalf("ctor differentX: %v", err)
	}
	differentY, err := NewXYPointSortField("loc", 1.0, 2.5)
	if err != nil {
		t.Fatalf("ctor differentY: %v", err)
	}

	if !a.Equals(a) {
		t.Errorf("reflexive equality must hold")
	}
	if !a.Equals(b) {
		t.Errorf("a.Equals(b) must hold for matching fields/origins")
	}
	if a.HashCode() != b.HashCode() {
		t.Errorf("hash codes diverge for equal values: %d vs %d", a.HashCode(), b.HashCode())
	}
	if a.Equals(differentField) || a.Equals(differentX) || a.Equals(differentY) {
		t.Errorf("any divergent dimension must break equality")
	}
	if a.Equals(nil) {
		t.Errorf("nil comparison must be false")
	}
	var nilSF *XYPointSortField
	if nilSF.Equals(a) {
		t.Errorf("nil receiver comparison must be false")
	}
}

func TestXYPointSortField_String_OmitsDefaultMissingValue(t *testing.T) {
	t.Parallel()

	sf, err := NewXYPointSortField("loc", 3.5, -2.25)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	got := sf.String()
	want := `<distance:"loc" x=3.5 y=-2.25>`
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestXYPointSortField_String_IncludesNonDefaultMissingValue(t *testing.T) {
	t.Parallel()

	sf, err := NewXYPointSortField("loc", 3.5, -2.25)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	// Bypass the validator to install a non-default missing value, mimicking
	// Lucene's package-private setMissingValue reach when overridden.
	sf.SortField.MissingValue = 100.0
	got := sf.String()
	if !strings.Contains(got, "missingValue=100.0") {
		t.Errorf("String() = %q, want suffix missingValue=100.0", got)
	}
}

func TestXYPointSortField_GetComparator_SizesSlots(t *testing.T) {
	t.Parallel()

	sf, err := NewXYPointSortField("loc", 3.5, -2.25)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	cmp := sf.GetComparator(8, PruningNone)
	if cmp == nil {
		t.Fatalf("GetComparator returned nil")
	}
	if got := len(cmp.values); got != 8 {
		t.Errorf("values slot count = %d, want 8", got)
	}
	if cmp.field != "loc" {
		t.Errorf("field = %q, want loc", cmp.field)
	}
	if cmp.x != 3.5 || cmp.y != -2.25 {
		t.Errorf("comparator origin = (%v,%v), want (3.5,-2.25)", cmp.x, cmp.y)
	}
}

func TestXYPointSortField_SortWrapper_DoesNotNeedScores(t *testing.T) {
	t.Parallel()

	sf, err := NewXYPointSortField("loc", 0, 0)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	sort := NewSort(sf.SortField)
	if sort.NeedsScores() {
		t.Errorf("XYPointSortField wrapped in Sort must not require scores")
	}
}

func TestFormatJavaFloat_IntegralAndSpecial(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   float32
		want string
	}{
		{0, "0.0"},
		{1, "1.0"},
		{-90, "-90.0"},
		{1.5, "1.5"},
		{float32(math.Inf(1)), "Infinity"},
		{float32(math.Inf(-1)), "-Infinity"},
	}
	for _, tc := range cases {
		if got := formatJavaFloat(tc.in); got != tc.want {
			t.Errorf("formatJavaFloat(%v) = %q, want %q", tc.in, got, tc.want)
		}
	if got := formatJavaFloat(float32(math.NaN())); got != "NaN" {
		t.Errorf("formatJavaFloat(NaN) = %q, want NaN", got)
	}
}