// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"
)

// TestNewLatLonPointBoxQuery_SpecialCases covers the three early-return
// special cases that mirror the Java reference's exact double handling:
// minLat==90 and the two minLon==180 branches.
func TestNewLatLonPointBoxQuery_SpecialCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		minLat    float64
		maxLat    float64
		minLon    float64
		maxLon    float64
		wantNoDoc bool
	}{
		{
			name:   "minLat=90 → MatchNoDocs",
			minLat: 90.0, maxLat: 90.0, minLon: -10.0, maxLon: 10.0,
			wantNoDoc: true,
		},
		{
			name:   "minLon=maxLon=180 → MatchNoDocs",
			minLat: 0.0, maxLat: 10.0, minLon: 180.0, maxLon: 180.0,
			wantNoDoc: true,
		},
		{
			name:   "normal non-dateline box",
			minLat: -10.0, maxLat: 10.0, minLon: -20.0, maxLon: 20.0,
			wantNoDoc: false,
		},
		{
			name:   "dateline-crossing box",
			minLat: -10.0, maxLat: 10.0, minLon: 170.0, maxLon: -170.0,
			wantNoDoc: false,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			q, err := NewLatLonPointBoxQuery("loc", c.minLat, c.maxLat, c.minLon, c.maxLon)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if q == nil {
				t.Fatal("NewLatLonPointBoxQuery returned nil query")
			}
			_, isNoDoc := q.(*MatchNoDocsQuery)
			if c.wantNoDoc && !isNoDoc {
				t.Fatalf("expected MatchNoDocsQuery, got %T", q)
			}
			if !c.wantNoDoc && isNoDoc {
				t.Fatalf("expected real query, got MatchNoDocsQuery")
			}
		})
	}
}

// TestNewLatLonPointBoxQuery_DatelineCrossing verifies that a
// dateline-crossing box produces a ConstantScoreQuery wrapping a
// two-clause SHOULD BooleanQuery, matching the Java reference's split
// strategy.
func TestNewLatLonPointBoxQuery_DatelineCrossing(t *testing.T) {
	t.Parallel()
	// Box from lon=170 to lon=-170 crosses the dateline.
	q, err := NewLatLonPointBoxQuery("loc", -10.0, 10.0, 170.0, -170.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	csq, ok := q.(*ConstantScoreQuery)
	if !ok {
		t.Fatalf("expected ConstantScoreQuery for dateline box, got %T", q)
	}
	bq, ok := csq.Query().(*BooleanQuery)
	if !ok {
		t.Fatalf("expected BooleanQuery inside ConstantScoreQuery, got %T", csq.Query())
	}
	if len(bq.Clauses()) != 2 {
		t.Fatalf("expected 2 clauses in BooleanQuery, got %d", len(bq.Clauses()))
	}
	for i, clause := range bq.Clauses() {
		if clause.Occur != SHOULD {
			t.Errorf("clause[%d] occur = %v; want SHOULD", i, clause.Occur)
		}
		if _, ok := clause.Query.(*PointRangeQuery); !ok {
			t.Errorf("clause[%d] query type = %T; want *PointRangeQuery", i, clause.Query)
		}
	}
}

// TestNewLatLonPointBoxQuery_NonDateline verifies that a normal
// (non-dateline) box produces a single PointRangeQuery.
func TestNewLatLonPointBoxQuery_NonDateline(t *testing.T) {
	t.Parallel()
	q, err := NewLatLonPointBoxQuery("loc", -10.0, 10.0, -20.0, 20.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := q.(*PointRangeQuery); !ok {
		t.Fatalf("expected *PointRangeQuery for normal box, got %T", q)
	}
}

// TestNewLatLonPointBoxQuery_MinLon180DatelineCeil verifies the edge
// case where minLon==180 and maxLon < minLon (dateline wrap). Java
// corrects minLon to -180 and continues; this must NOT produce a
// MatchNoDocsQuery.
func TestNewLatLonPointBoxQuery_MinLon180DatelineCeil(t *testing.T) {
	t.Parallel()
	// minLon=180 and maxLon=-170 means "wrap around from +180", which
	// Java corrects to minLon=-180 before encoding.
	q, err := NewLatLonPointBoxQuery("loc", -10.0, 10.0, 180.0, -170.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := q.(*MatchNoDocsQuery); ok {
		t.Fatal("expected real query, got MatchNoDocsQuery for minLon=180/dateline-wrap")
	}
}
