// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestLatLonPointSortField_Constructor_RejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		field    string
		lat, lon float64
	}{
		{"empty_field", "", 0, 0},
		{"lat_too_high", "loc", 91, 0},
		{"lat_too_low", "loc", -91, 0},
		{"lat_nan", "loc", math.NaN(), 0},
		{"lon_too_high", "loc", 0, 181},
		{"lon_too_low", "loc", 0, -181},
		{"lon_nan", "loc", 0, math.NaN()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewLatLonPointSortField(tc.field, tc.lat, tc.lon); err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestLatLonPointSortField_Constructor_DefaultsCustomAndAscending(t *testing.T) {
	t.Parallel()

	sf, err := NewLatLonPointSortField("loc", 38.7223, -9.1393)
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
	if sf.Latitude() != 38.7223 || sf.Longitude() != -9.1393 {
		t.Errorf("origin = (%v,%v), want (38.7223,-9.1393)", sf.Latitude(), sf.Longitude())
	}
	if got := sf.GetMissingValue(); !math.IsInf(got, 1) {
		t.Errorf("MissingValue = %v, want +Inf", got)
	}
}

func TestLatLonPointSortField_SetMissingValue_OnlyAcceptsPositiveInfinity(t *testing.T) {
	t.Parallel()

	sf, err := NewLatLonPointSortField("loc", 0, 0)
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
		if !errors.Is(err, ErrLatLonPointSortFieldInvalidMissingValue) {
			t.Errorf("SetMissingValue(%v) = %v, want ErrLatLonPointSortFieldInvalidMissingValue", v, err)
		}
	}
	// MissingValue must remain unchanged at +Inf after every rejection.
	if got := sf.GetMissingValue(); !math.IsInf(got, 1) {
		t.Errorf("MissingValue mutated by rejection path: got %v", got)
	}
}

func TestLatLonPointSortField_Equals_HashCode(t *testing.T) {
	t.Parallel()

	a, err := NewLatLonPointSortField("loc", 1.0, 2.0)
	if err != nil {
		t.Fatalf("ctor a: %v", err)
	}
	b, err := NewLatLonPointSortField("loc", 1.0, 2.0)
	if err != nil {
		t.Fatalf("ctor b: %v", err)
	}
	differentField, err := NewLatLonPointSortField("other", 1.0, 2.0)
	if err != nil {
		t.Fatalf("ctor differentField: %v", err)
	}
	differentLat, err := NewLatLonPointSortField("loc", 1.5, 2.0)
	if err != nil {
		t.Fatalf("ctor differentLat: %v", err)
	}
	differentLon, err := NewLatLonPointSortField("loc", 1.0, 2.5)
	if err != nil {
		t.Fatalf("ctor differentLon: %v", err)
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
	if a.Equals(differentField) || a.Equals(differentLat) || a.Equals(differentLon) {
		t.Errorf("any divergent dimension must break equality")
	}
	if a.Equals(nil) {
		t.Errorf("nil comparison must be false")
	}
	var nilSF *LatLonPointSortField
	if nilSF.Equals(a) {
		t.Errorf("nil receiver comparison must be false")
	}
}

func TestLatLonPointSortField_String_OmitsDefaultMissingValue(t *testing.T) {
	t.Parallel()

	sf, err := NewLatLonPointSortField("loc", 38.7223, -9.1393)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	got := sf.String()
	want := `<distance:"loc" latitude=38.7223 longitude=-9.1393>`
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestLatLonPointSortField_String_IncludesNonDefaultMissingValue(t *testing.T) {
	t.Parallel()

	sf, err := NewLatLonPointSortField("loc", 38.7223, -9.1393)
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

func TestLatLonPointSortField_GetComparator_SizesSlots(t *testing.T) {
	t.Parallel()

	sf, err := NewLatLonPointSortField("loc", 38.7223, -9.1393)
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
	if cmp.latitude != 38.7223 || cmp.longitude != -9.1393 {
		t.Errorf("comparator origin = (%v,%v), want (38.7223,-9.1393)", cmp.latitude, cmp.longitude)
	}
}

func TestLatLonPointSortField_SortWrapper_DoesNotNeedScores(t *testing.T) {
	t.Parallel()

	sf, err := NewLatLonPointSortField("loc", 0, 0)
	if err != nil {
		t.Fatalf("ctor: %v", err)
	}
	sort := NewSort(sf.SortField)
	if sort.NeedsScores() {
		t.Errorf("LatLonPointSortField wrapped in Sort must not require scores")
	}
}

// distanceFakeSortedNumeric is a minimal SortedNumericDocValues stub used to
// drive the distance comparator without standing up a full segment reader.
// The name is intentionally namespaced so it does not collide with the
// generic fakeSortedNumeric helper already living in lat_lon_doc_values_query_test.go.
type distanceFakeSortedNumeric struct {
	docs   []int
	values [][]int64
	idx    int
}

func newDistanceFakeSortedNumeric(docs []int, values [][]int64) *distanceFakeSortedNumeric {
	if len(docs) != len(values) {
		panic("distanceFakeSortedNumeric: docs and values length mismatch")
	}
	return &distanceFakeSortedNumeric{docs: docs, values: values, idx: -1}
}

func (f *distanceFakeSortedNumeric) Get(docID int) ([]int64, error) {
	if f.idx < 0 || f.idx >= len(f.docs) {
		return nil, nil
	}
	if f.docs[f.idx] != docID {
		return nil, nil
	}
	return f.values[f.idx], nil
}

func (f *distanceFakeSortedNumeric) Advance(target int) (int, error) {
	if f.idx < 0 {
		f.idx = 0
	}
	for f.idx < len(f.docs) && f.docs[f.idx] < target {
		f.idx++
	}
	if f.idx >= len(f.docs) {
		return index.NO_MORE_DOCS, nil
	}
	return f.docs[f.idx], nil
}

func (f *distanceFakeSortedNumeric) NextDoc() (int, error) {
	f.idx++
	if f.idx >= len(f.docs) {
		return index.NO_MORE_DOCS, nil
	}
	return f.docs[f.idx], nil
}

func (f *distanceFakeSortedNumeric) DocID() int {
	if f.idx < 0 {
		return -1
	}
	if f.idx >= len(f.docs) {
		return index.NO_MORE_DOCS
	}
	return f.docs[f.idx]
}

func encodeDistancePoint(lat, lon float64) int64 {
	latBits := int64(geo.EncodeLatitude(lat)) & 0xFFFFFFFF
	lonBits := int64(geo.EncodeLongitude(lon)) & 0xFFFFFFFF
	return (latBits << 32) | lonBits
}

func TestLatLonPointDistanceComparator_CopyAndCompareOrderClosestFirst(t *testing.T) {
	t.Parallel()

	originLat, originLon := 38.7223, -9.1393 // Lisboa
	points := []struct {
		lat, lon float64
	}{
		{38.7223, -9.1393}, // self -> 0 m
		{40.4168, -3.7038}, // Madrid
		{48.8566, 2.3522},  // Paris
	}
	docs := []int{0, 1, 2}
	values := make([][]int64, len(points))
	for i, p := range points {
		values[i] = []int64{encodeDistancePoint(p.lat, p.lon)}
	}

	cmp := NewLatLonPointDistanceComparator("loc", originLat, originLon, len(points))
	cmp.currentDocs = newDistanceFakeSortedNumeric(docs, values)

	for slot, doc := range docs {
		if err := cmp.Copy(slot, doc); err != nil {
			t.Fatalf("Copy(slot=%d, doc=%d): %v", slot, doc, err)
		}
	}

	// Slot 0 (origin) must beat slot 1 (Madrid) which must beat slot 2 (Paris).
	if cmp.Compare(0, 1) >= 0 {
		t.Errorf("Compare(self, Madrid) = %d, want <0", cmp.Compare(0, 1))
	}
	if cmp.Compare(1, 2) >= 0 {
		t.Errorf("Compare(Madrid, Paris) = %d, want <0", cmp.Compare(1, 2))
	}
	if cmp.Compare(0, 0) != 0 {
		t.Errorf("Compare(self, self) = %d, want 0", cmp.Compare(0, 0))
	}

	// Value returns metres; the origin slot should round to ~0 metres.
	if got := cmp.Value(0); got > 1.0 {
		t.Errorf("Value(self) = %v, want ~0 m", got)
	}
	if got := cmp.Value(2); got < 1_400_000 || got > 1_500_000 {
		t.Errorf("Value(Paris) = %v, want ~1450 km", got)
	}
}

func TestLatLonPointDistanceComparator_MultiValued_PicksClosestPoint(t *testing.T) {
	t.Parallel()

	cmp := NewLatLonPointDistanceComparator("loc", 38.7223, -9.1393, 1)
	// Single doc with two points: Madrid + Lisboa itself.
	docs := []int{0}
	values := [][]int64{{
		encodeDistancePoint(40.4168, -3.7038), // Madrid
		encodeDistancePoint(38.7223, -9.1393), // Lisboa (closer; should win)
	}}
	cmp.currentDocs = newDistanceFakeSortedNumeric(docs, values)
	if err := cmp.Copy(0, 0); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	got := cmp.Value(0)
	if got > 1.0 {
		t.Errorf("multi-valued doc Value = %v m, want ~0 m (closest point wins)", got)
	}
}

func TestLatLonPointDistanceComparator_MissingDoc_ReturnsInfinity(t *testing.T) {
	t.Parallel()

	cmp := NewLatLonPointDistanceComparator("loc", 0, 0, 1)
	cmp.currentDocs = newDistanceFakeSortedNumeric([]int{2}, [][]int64{{encodeDistancePoint(0, 0)}})
	// Probe a doc id past the only entry.
	if err := cmp.Copy(0, 5); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if got := cmp.Value(0); !math.IsInf(got, 1) {
		t.Errorf("Value(missing) = %v, want +Inf", got)
	}
}

func TestLatLonPointDistanceComparator_NilDocs_ReturnsInfinity(t *testing.T) {
	t.Parallel()

	cmp := NewLatLonPointDistanceComparator("loc", 0, 0, 1)
	if err := cmp.Copy(0, 0); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if got := cmp.Value(0); !math.IsInf(got, 1) {
		t.Errorf("Value(nil docs) = %v, want +Inf", got)
	}
}

func TestLatLonPointDistanceComparator_SetBottom_BuildsBoundingBox(t *testing.T) {
	t.Parallel()

	cmp := NewLatLonPointDistanceComparator("loc", 0, 0, 1)
	// Seed slot 0 with a Haversine sort key corresponding to ~10 km.
	sortKey := util.HaversinSortKey(0, 0, 0.0898, 0) // ~9.99 km
	cmp.values[0] = sortKey
	if err := cmp.SetBottom(0); err != nil {
		t.Fatalf("SetBottom: %v", err)
	}
	if cmp.bottom != sortKey {
		t.Errorf("bottom = %v, want %v", cmp.bottom, sortKey)
	}
	// The bounding box must now be a non-default subset of the integer range.
	if cmp.minLat == math.MinInt32 || cmp.maxLat == math.MaxInt32 {
		t.Errorf("setBottom did not tighten latitude bounds: minLat=%d maxLat=%d", cmp.minLat, cmp.maxLat)
	}
	if cmp.minLon == math.MinInt32 && cmp.maxLon == math.MaxInt32 {
		t.Errorf("setBottom did not tighten longitude bounds")
	}
	if cmp.setBottomCounter != 1 {
		t.Errorf("setBottomCounter = %d, want 1", cmp.setBottomCounter)
	}
}

func TestLatLonPointDistanceComparator_CompareBottom_RejectsOutsideBox(t *testing.T) {
	t.Parallel()

	cmp := NewLatLonPointDistanceComparator("loc", 0, 0, 1)
	// Bottom = ~10 km radius around origin.
	sortKey := util.HaversinSortKey(0, 0, 0.0898, 0)
	cmp.values[0] = sortKey
	if err := cmp.SetBottom(0); err != nil {
		t.Fatalf("SetBottom: %v", err)
	}
	// Set up docs: doc 0 at origin (better), doc 1 at Paris (much worse, outside box).
	cmp.currentDocs = newDistanceFakeSortedNumeric(
		[]int{0, 1},
		[][]int64{
			{encodeDistancePoint(0, 0)},
			{encodeDistancePoint(48.8566, 2.3522)},
		},
	)
	// doc 0 must be competitive (positive sign).
	got, err := cmp.CompareBottom(0)
	if err != nil {
		t.Fatalf("CompareBottom(0): %v", err)
	}
	if got <= 0 {
		t.Errorf("CompareBottom(self) = %d, want positive", got)
	}
	// doc 1 (Paris) must be rejected (negative or zero) thanks to bbox.
	got, err = cmp.CompareBottom(1)
	if err != nil {
		t.Fatalf("CompareBottom(1): %v", err)
	}
	if got > 0 {
		t.Errorf("CompareBottom(Paris) = %d, want non-positive", got)
	}
}

func TestLatLonPointDistanceComparator_GetLeafComparator_NilContextErrors(t *testing.T) {
	t.Parallel()

	cmp := NewLatLonPointDistanceComparator("loc", 0, 0, 1)
	if _, err := cmp.GetLeafComparator(nil); err == nil {
		t.Fatalf("GetLeafComparator(nil) must error")
	}
}

func TestLatLonPointDistanceComparator_SetTopValue_CompareTop(t *testing.T) {
	t.Parallel()

	cmp := NewLatLonPointDistanceComparator("loc", 0, 0, 1)
	cmp.currentDocs = newDistanceFakeSortedNumeric([]int{0}, [][]int64{{encodeDistancePoint(0, 0)}})
	// Set the top reference well above the doc's actual distance (which is ~0).
	cmp.SetTopValue(10_000) // 10 km
	got, err := cmp.CompareTop(0)
	if err != nil {
		t.Fatalf("CompareTop: %v", err)
	}
	// topValue (10000 m) > doc distance (~0 m) -> positive
	if got <= 0 {
		t.Errorf("CompareTop = %d, want positive (top reference greater than doc distance)", got)
	}
}

func TestLatLonPointDistanceComparator_CompetitiveIterator_Nil(t *testing.T) {
	t.Parallel()

	cmp := NewLatLonPointDistanceComparator("loc", 0, 0, 1)
	it, err := cmp.CompetitiveIterator()
	if err != nil {
		t.Fatalf("CompetitiveIterator: %v", err)
	}
	if it != nil {
		t.Errorf("CompetitiveIterator() = %T, want nil", it)
	}
	// SetHitsThresholdReached must not panic.
	cmp.SetHitsThresholdReached()
	// SetScorer must not panic / error with a nil scorable.
	if err := cmp.SetScorer(nil); err != nil {
		t.Errorf("SetScorer(nil) = %v, want nil", err)
	}
}

func TestCompareFloat64_Semantics(t *testing.T) {
	t.Parallel()

	nan := math.NaN()
	cases := []struct {
		name string
		a, b float64
		want int
	}{
		{"less", 1, 2, -1},
		{"greater", 5, 1, 1},
		{"equal", 3.14, 3.14, 0},
		{"plus_inf_eq", math.Inf(1), math.Inf(1), 0},
		{"nan_vs_value", nan, 1, 1},
		{"value_vs_nan", 1, nan, -1},
		{"nan_vs_nan", nan, nan, 0},
		{"neg_zero_vs_pos_zero", math.Copysign(0, -1), 0, -1},
		{"pos_zero_vs_neg_zero", 0, math.Copysign(0, -1), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := compareFloat64(tc.a, tc.b); got != tc.want {
				t.Errorf("compareFloat64(%v,%v) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestFormatJavaDouble_IntegralAndSpecial(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   float64
		want string
	}{
		{0, "0.0"},
		{1, "1.0"},
		{-90, "-90.0"},
		{1.5, "1.5"},
		{math.Inf(1), "Infinity"},
		{math.Inf(-1), "-Infinity"},
	}
	for _, tc := range cases {
		if got := formatJavaDouble(tc.in); got != tc.want {
			t.Errorf("formatJavaDouble(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := formatJavaDouble(math.NaN()); got != "NaN" {
		t.Errorf("formatJavaDouble(NaN) = %q, want NaN", got)
	}
}

// Make sure the comparator is wired so a real LatLonDocValuesField-encoded
// value round-trips through Copy/Value into the right ballpark of metres,
// regardless of the encoding helper used at insert time.
func TestLatLonPointDistanceComparator_MatchesLatLonDocValuesFieldEncoding(t *testing.T) {
	t.Parallel()

	originLat, originLon := 38.7223, -9.1393
	otherLat, otherLon := 40.4168, -3.7038 // Madrid (~500 km from Lisboa)

	cmp := NewLatLonPointDistanceComparator("loc", originLat, originLon, 1)
	encoded := document.EncodeLatLonAsLong(otherLat, otherLon)
	cmp.currentDocs = newDistanceFakeSortedNumeric([]int{0}, [][]int64{{encoded}})
	if err := cmp.Copy(0, 0); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	got := cmp.Value(0)
	if got < 450_000 || got > 600_000 {
		t.Errorf("Madrid distance = %v, want in [450 km, 600 km]", got)
	}
}
