// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"math"
	"testing"
)

// This file is the Go port of
// lucene/core/src/test/org/apache/lucene/document/TestLatLonPointDistanceSort.java
// (Apache Lucene 10.4.0).
//
// The Java class exercises `LatLonDocValuesField.newDistanceSort` end
// to end:
//
//   - testDistanceSort indexes three documents holding a single
//     LatLonDocValuesField each, sorts them by haversin distance to a
//     fixed (lat, lon) anchor, and asserts the three expected
//     distances in metres (462.10..., 1054.98..., 5285.88...).
//   - testMissingLast indexes one missing-value document plus two
//     value-bearing ones and asserts the missing document sorts last
//     with `Double.POSITIVE_INFINITY`.
//   - testRandom runs 100 iterations of doRandomTest(10, 100), driving
//     a brute-force comparison between `SloppyMath.haversinMeters` and
//     the searcher output for both the first page and a randomised
//     `searchAfter` second page.
//   - testRandomHuge is the `@Nightly` variant of testRandom with
//     2000 docs.
//
// Gocene currently lacks the surfaces these tests depend on, in order:
//
//  1. `LatLonDocValuesField.NewDistanceSort(field, lat, lon)` — the
//     Sort/SortField factory that emits a NumericDocValuesField-backed
//     comparator returning haversin metres as a Double. Not yet ported
//     into `document/lat_lon_doc_values_field*.go`.
//  2. The `Sort` / `SortField` / `FieldDoc` collector wiring that
//     surfaces the comparator value as `fieldDoc.fields[0]`.
//  3. The `IndexSearcher.Search(query, n, sort)` overload that drives
//     the sort and the matching `SearchAfter` overload used by the
//     second-page assertion.
//  4. `RandomIndexWriter` plus `GeoTestUtil.nextLatitude/Longitude`,
//     which back the randomised iterations.
//  5. `SerialMergeScheduler` wiring inside the random harness so seeds
//     reproduce deterministically.
//
// Per Sprint 55 stub-degraded contract (option c):
//   - the test file exists and compiles;
//   - every `@Test` method in the Java reference has a 1:1 Go
//     counterpart;
//   - each Test* opens with t.Skip naming the missing piece
//     explicitly, so `go test -v` records the work without ever
//     touching the non-existent surfaces;
//   - the typed fixtures below are constructible but never invoke any
//     of the absent factories — the skip happens before any fixture is
//     exercised.

// ---------------------------------------------------------------------
// Constants captured verbatim from the Java reference (lines 53-78).
// ---------------------------------------------------------------------
//
// Java keeps these inline. They are surfaced here as package-private
// values so the eventual activation patch can drive them through the
// real `NewDistanceSort` factory without re-deriving the literals from
// the Lucene source tree.

// latLonPointDistanceSortAnchorLat / Lon mirror the fixed query anchor
// used in both testDistanceSort and testMissingLast.
const (
	latLonPointDistanceSortAnchorLat = 40.7143528
	latLonPointDistanceSortAnchorLon = -74.0059731
)

// latLonPointDistanceSortDoc holds one indexed point. A nil pointer
// represents the Java "missing" document (no LatLonDocValuesField
// added).
type latLonPointDistanceSortDoc struct {
	lat float64
	lon float64
}

// latLonPointDistanceSortFixedDocs mirrors the three documents added by
// testDistanceSort (Java lines 52-62), in the same insertion order.
var latLonPointDistanceSortFixedDocs = []*latLonPointDistanceSortDoc{
	{lat: 40.759011, lon: -73.9844722},
	{lat: 40.718266, lon: -74.007819},
	{lat: 40.7051157, lon: -74.0088305},
}

// latLonPointDistanceSortMissingDocs mirrors testMissingLast (Java
// lines 88-99): a nil sentinel for the missing-valued document
// followed by two valued documents.
var latLonPointDistanceSortMissingDocs = []*latLonPointDistanceSortDoc{
	nil,
	{lat: 40.718266, lon: -74.007819},
	{lat: 40.7051157, lon: -74.0088305},
}

// latLonPointDistanceSortFixedExpected is the ordered slice of
// expected distances in metres asserted by testDistanceSort
// (Java lines 72-78), reproduced bit-exact from the reference asserts.
var latLonPointDistanceSortFixedExpected = []float64{
	462.1028401330431,
	1054.9842850974826,
	5285.881528419706,
}

// latLonPointDistanceSortMissingExpected mirrors the testMissingLast
// asserts (Java lines 108-115): two real distances followed by a
// positive-infinity sentinel for the missing-valued document.
var latLonPointDistanceSortMissingExpected = []float64{
	462.1028401330431,
	1054.9842850974826,
	math.Inf(+1),
}

// ---------------------------------------------------------------------
// Result helper mirroring the Java inner class (lines 138-183).
// ---------------------------------------------------------------------
//
// Reproduced as a Go struct so the future activation patch for
// doRandomTest can populate and sort it identically to the Java
// reference. Sort order matches Java's `compareTo`: primary key is
// distance (ascending, NaN-aware via `Double.compare`), secondary key
// is id ascending.

type latLonPointDistanceSortResult struct {
	id       int
	distance float64
}

// less mirrors `Result.compareTo` (Java lines 148-154). It is unused
// while the stub is skipped, but kept compiled-and-exercised by
// `_ = ...` below so a future activation does not need to re-derive
// the tiebreak rule.
func (r latLonPointDistanceSortResult) less(o latLonPointDistanceSortResult) bool {
	if r.distance != o.distance {
		// Match Java's Double.compare: NaN sorts greater than any
		// non-NaN value and equal to itself.
		switch {
		case math.IsNaN(r.distance) && !math.IsNaN(o.distance):
			return false
		case !math.IsNaN(r.distance) && math.IsNaN(o.distance):
			return true
		}
		return r.distance < o.distance
	}
	return r.id < o.id
}

// Force `less` to be reachable from the test binary so the linker
// keeps it. The eventual activation patch drops this guard.
var _ = latLonPointDistanceSortResult{}.less

// ---------------------------------------------------------------------
// Test methods — 1:1 mapping with the Java reference.
// ---------------------------------------------------------------------

// TestLatLonPointDistanceSort_DistanceSort mirrors
// TestLatLonPointDistanceSort.testDistanceSort (Java lines 47-82).
func TestLatLonPointDistanceSort_DistanceSort(t *testing.T) {
	t.Skip("blocked: requires (1) document.LatLonDocValuesField.NewDistanceSort factory, (2) IndexSearcher.Search(query, n, sort) wiring, and (3) FieldDoc surfacing the haversin metres as fields[0].")
	_ = latLonPointDistanceSortAnchorLat
	_ = latLonPointDistanceSortAnchorLon
	_ = latLonPointDistanceSortFixedDocs
	_ = latLonPointDistanceSortFixedExpected
}

// TestLatLonPointDistanceSort_MissingLast mirrors
// TestLatLonPointDistanceSort.testMissingLast (Java lines 85-119).
func TestLatLonPointDistanceSort_MissingLast(t *testing.T) {
	t.Skip("blocked: requires the missing-last branch of NewDistanceSort (Double.POSITIVE_INFINITY sentinel) on top of the same Search(query, n, sort) wiring missing for the fixed-anchor case.")
	_ = latLonPointDistanceSortMissingDocs
	_ = latLonPointDistanceSortMissingExpected
}

// TestLatLonPointDistanceSort_Random mirrors
// TestLatLonPointDistanceSort.testRandom (Java lines 122-126), which
// invokes doRandomTest(10, 100) 100 times.
func TestLatLonPointDistanceSort_Random(t *testing.T) {
	t.Skip("blocked: requires RandomIndexWriter + GeoTestUtil random lat/lon generators + SerialMergeScheduler wiring + IndexSearcher.SearchAfter overload; none ported yet.")
}

// TestLatLonPointDistanceSort_RandomHuge mirrors
// TestLatLonPointDistanceSort.testRandomHuge (Java lines 129-134).
// The Java method is `@Nightly`; Gocene has no nightly bucket yet, so
// the stub mirrors the method name 1:1 and skips with the same
// rationale as the non-huge counterpart plus an explicit nightly
// marker so a future activation can gate it behind a build tag.
func TestLatLonPointDistanceSort_RandomHuge(t *testing.T) {
	t.Skip("blocked (nightly): same dependencies as TestLatLonPointDistanceSort_Random; additionally needs the Gocene equivalent of @Nightly so the 2000-doc/iter workload is opt-in.")
}
