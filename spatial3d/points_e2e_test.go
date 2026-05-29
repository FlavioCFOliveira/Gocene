// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial3d_test

import (
	"testing"

	// Blank-import the production codec and its BKD points implementation so
	// the default Lucene104 codec is registered and Lucene90PointsFormat's
	// FieldsWriter/FieldsReader hooks are installed (rmp #4769).
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spatial3d"
	"github.com/FlavioCFOliveira/Gocene/spatial3d/geom"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestPointsEndToEnd_IntLongRangeAndGeo3D is the rmp #4769 acceptance test: it
// writes IntPoint, LongPoint and Geo3DPoint values through the production
// IndexWriter, commits, reopens via OpenDirectoryReader, and verifies that
//
//  1. LeafReader.GetPointValues returns a non-nil BKD-backed PointValues with
//     the correct docCount / numDimensions / bytesPerDim,
//  2. search.PointRangeQuery returns the correct doc set end-to-end through
//     IndexSearcher for both the int and the long field,
//  3. spatial3d.PointInGeo3DShapeQuery returns the correct doc set end-to-end.
//
// It is the on-disk counterpart of the in-memory-stub tests in
// search/point_range_explain_test.go and spatial3d/geo3d_query_test.go.
func TestPointsEndToEnd_IntLongRangeAndGeo3D(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	const (
		numDocs   = 40
		intField  = "ip"
		longField = "lp"
		geoField  = "gp"
	)

	// A near (0,0) lat/lon cluster and a far (45,45) cluster, so a tight
	// circle around (0,0) selects exactly the near docs.
	type geoDoc struct {
		lat, lon float64
		near     bool
	}
	geoDocs := make([]geoDoc, numDocs)

	iwc := index.NewIndexWriterConfig(nil)
	w, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()

		ip, err := document.NewIntPointLucene(intField, int32(i))
		if err != nil {
			t.Fatalf("NewIntPointLucene[%d]: %v", i, err)
		}
		doc.Add(ip)

		lp, err := document.NewLongPointLucene(longField, int64(i)*1000)
		if err != nil {
			t.Fatalf("NewLongPointLucene[%d]: %v", i, err)
		}
		doc.Add(lp)

		near := i%2 == 0
		var lat, lon float64
		if near {
			lat, lon = 0.0+float64(i)*0.001, 0.0+float64(i)*0.001
		} else {
			lat, lon = 45.0, 45.0
		}
		geoDocs[i] = geoDoc{lat: lat, lon: lon, near: near}
		g3 := spatial3d.NewGeo3DPointLatLon(geoField, lat, lon)
		fields, err := g3.ToIndexableFields()
		if err != nil {
			t.Fatalf("Geo3DPoint.ToIndexableFields[%d]: %v", i, err)
		}
		for _, f := range fields {
			doc.Add(f)
		}

		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument[%d]: %v", i, err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if got := reader.NumDocs(); got != numDocs {
		t.Fatalf("NumDocs = %d, want %d", got, numDocs)
	}

	// (1) LeafReader.GetPointValues returns the BKD PointValues with correct
	// metadata for every point field.
	segReaders := reader.GetSegmentReaders()
	if len(segReaders) == 0 {
		t.Fatal("no segment readers")
	}
	var seenInt, seenLong, seenGeo int
	for _, sr := range segReaders {
		for _, c := range []struct {
			field    string
			dims     int
			bytesDim int
		}{
			{intField, 1, 4},
			{longField, 1, 8},
			{geoField, 3, 4},
		} {
			pv, err := sr.GetPointValues(c.field)
			if err != nil {
				t.Fatalf("GetPointValues(%q): %v", c.field, err)
			}
			if pv == nil {
				t.Fatalf("GetPointValues(%q) returned nil", c.field)
			}
			if got := pv.GetNumDimensions(); got != c.dims {
				t.Errorf("field %q GetNumDimensions = %d, want %d", c.field, got, c.dims)
			}
			if got := pv.GetBytesPerDimension(); got != c.bytesDim {
				t.Errorf("field %q GetBytesPerDimension = %d, want %d", c.field, got, c.bytesDim)
			}
			switch c.field {
			case intField:
				seenInt += pv.GetDocCount()
			case longField:
				seenLong += pv.GetDocCount()
			case geoField:
				seenGeo += pv.GetDocCount()
			}
		}
	}
	if seenInt != numDocs {
		t.Errorf("int field total docCount = %d, want %d", seenInt, numDocs)
	}
	if seenLong != numDocs {
		t.Errorf("long field total docCount = %d, want %d", seenLong, numDocs)
	}
	if seenGeo != numDocs {
		t.Errorf("geo field total docCount = %d, want %d", seenGeo, numDocs)
	}

	searcher := search.NewIndexSearcher(reader)

	// (2a) PointRangeQuery on the int field: [10, 19] -> docs with int in
	// [10, 19] = 10 docs.
	{
		lo := make([]byte, 4)
		hi := make([]byte, 4)
		document.EncodeDimensionIntLucene(10, lo, 0)
		document.EncodeDimensionIntLucene(19, hi, 0)
		q, err := search.NewPointRangeQuery(intField, lo, hi)
		if err != nil {
			t.Fatalf("NewPointRangeQuery(int): %v", err)
		}
		td, err := searcher.Search(q, numDocs)
		if err != nil {
			t.Fatalf("Search(int range): %v", err)
		}
		if got := len(td.ScoreDocs); got != 10 {
			t.Errorf("int range [10,19] hits = %d, want 10", got)
		}
	}

	// (2b) PointRangeQuery on the long field: [10000, 14000] -> int i in
	// [10, 14] = 5 docs.
	{
		lo := make([]byte, 8)
		hi := make([]byte, 8)
		document.EncodeDimensionLongLucene(10000, lo, 0)
		document.EncodeDimensionLongLucene(14000, hi, 0)
		q, err := search.NewPointRangeQuery(longField, lo, hi)
		if err != nil {
			t.Fatalf("NewPointRangeQuery(long): %v", err)
		}
		td, err := searcher.Search(q, numDocs)
		if err != nil {
			t.Fatalf("Search(long range): %v", err)
		}
		if got := len(td.ScoreDocs); got != 5 {
			t.Errorf("long range [10000,14000] hits = %d, want 5", got)
		}
	}

	// (3) PointInGeo3DShapeQuery: a 5-degree circle around (0,0) selects
	// exactly the near docs (the even ids, which cluster near the origin).
	{
		wantNear := 0
		for _, gd := range geoDocs {
			if gd.near {
				wantNear++
			}
		}
		circle, err := geom.MakeGeoCircle(geom.SPHERE, 0.0*spatial3d.RadiansPerDegree, 0.0*spatial3d.RadiansPerDegree, 5.0*spatial3d.RadiansPerDegree)
		if err != nil {
			t.Fatalf("MakeGeoCircle: %v", err)
		}
		q := spatial3d.NewPointInGeo3DShapeQuery(geoField, circle)
		td, err := searcher.Search(q, numDocs)
		if err != nil {
			t.Fatalf("Search(geo3d): %v", err)
		}
		if got := len(td.ScoreDocs); got != wantNear {
			t.Errorf("geo3d circle hits = %d, want %d (near docs)", got, wantNear)
		}
	}
}
