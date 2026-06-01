// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestXYPointQueries.java
//   (concrete instantiation of BaseXYPointTestCase)
//
// TestXYPointQueries adds no own test methods in Lucene — it wires XYPointField
// indexing and the box/distance/polygon query factories into the abstract
// BaseXYPointTestCase. This port covers the concrete (non-random) base methods
// the now-wired XY points path supports: testBoxBasics, testDistanceBasics,
// testBoxNull and testDistanceNull. Each indexes an XYPointField and runs a
// bounding-box (XYRectangle) or distance (XYCircle) XYPointInGeometryQuery
// through the live BKD points path.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// buildXYPointIndex indexes a single XYPointField document at (x, y).
func buildXYPointIndex(t *testing.T, field string, x, y float32) *index.DirectoryReader {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	doc := document.NewDocument()
	f, err := document.NewXYPointField(field, x, y)
	if err != nil {
		t.Fatalf("NewXYPointField: %v", err)
	}
	doc.Add(f)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	return reader
}

// newXYRectQuery mirrors TestXYPointQueries.newRectQuery: a box query over an
// XYRectangle(minX, maxX, minY, maxY).
func newXYRectQuery(t *testing.T, field string, minX, maxX, minY, maxY float32) search.Query {
	t.Helper()
	rect, err := geo.NewXYRectangle(minX, maxX, minY, maxY)
	if err != nil {
		t.Fatalf("NewXYRectangle: %v", err)
	}
	q, err := search.NewXYPointInGeometryQuery(field, rect)
	if err != nil {
		t.Fatalf("NewXYPointInGeometryQuery(rect): %v", err)
	}
	return q
}

// newXYDistanceQuery mirrors TestXYPointQueries.newDistanceQuery: a distance
// query over an XYCircle(centerX, centerY, radius).
func newXYDistanceQuery(t *testing.T, field string, centerX, centerY, radius float32) search.Query {
	t.Helper()
	circle, err := geo.NewXYCircle(centerX, centerY, radius)
	if err != nil {
		t.Fatalf("NewXYCircle: %v", err)
	}
	q, err := search.NewXYPointInGeometryQuery(field, circle)
	if err != nil {
		t.Fatalf("NewXYPointInGeometryQuery(circle): %v", err)
	}
	return q
}

func xyCount(t *testing.T, reader index.IndexReaderInterface, q search.Query) int {
	t.Helper()
	top, err := search.NewIndexSearcher(reader).Search(q, 100)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	return len(top.ScoreDocs)
}

// TestXYPointQueries is the concrete BaseXYPointTestCase instantiation; the
// sub-tests port its concrete (non-random) box/distance methods.
func TestXYPointQueries(t *testing.T) {
	t.Run("BoxBasics", func(t *testing.T) {
		// BaseXYPointTestCase.testBoxBasics: a point at (18.313694, -65.227444)
		// must be found by the box [18,19] x [-66,-65].
		reader := buildXYPointIndex(t, "field", 18.313694, -65.227444)
		if got := xyCount(t, reader, newXYRectQuery(t, "field", 18, 19, -66, -65)); got != 1 {
			t.Fatalf("box count: got %d want 1", got)
		}
	})

	t.Run("DistanceBasics", func(t *testing.T) {
		// BaseXYPointTestCase.testDistanceBasics: the same point must be found
		// within radius 20 of (18, -65).
		reader := buildXYPointIndex(t, "field", 18.313694, -65.227444)
		if got := xyCount(t, reader, newXYDistanceQuery(t, "field", 18, -65, 20)); got != 1 {
			t.Fatalf("distance count: got %d want 1", got)
		}
	})

	t.Run("BoxNull", func(t *testing.T) {
		// BaseXYPointTestCase.testBoxNull: a null/empty field name is rejected.
		rect, err := geo.NewXYRectangle(18, 19, -66, -65)
		if err != nil {
			t.Fatalf("NewXYRectangle: %v", err)
		}
		if _, err := search.NewXYPointInGeometryQuery("", rect); err == nil {
			t.Fatal("expected error for empty field name, got nil")
		}
	})

	t.Run("DistanceNull", func(t *testing.T) {
		// BaseXYPointTestCase.testDistanceNull: a null/empty field name is rejected.
		circle, err := geo.NewXYCircle(18, -65, 50000)
		if err != nil {
			t.Fatalf("NewXYCircle: %v", err)
		}
		if _, err := search.NewXYPointInGeometryQuery("", circle); err == nil {
			t.Fatal("expected error for empty field name, got nil")
		}
	})

	t.Run("BoxNoMatchOutside", func(t *testing.T) {
		// A box that does not contain the point yields zero hits (sanity for the
		// BKD pruning path, complementing testBoxBasics).
		reader := buildXYPointIndex(t, "field", 18.313694, -65.227444)
		if got := xyCount(t, reader, newXYRectQuery(t, "field", 0, 1, 0, 1)); got != 0 {
			t.Fatalf("disjoint box count: got %d want 0", got)
		}
	})
}
