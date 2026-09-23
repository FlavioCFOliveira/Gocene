// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestXYPointDistanceSort.java
// (Apache Lucene 10.5.0): simple tests for XYDocValuesField.newDistanceSort
// (rendered by search.NewXYPointSortField). The @Nightly testRandomHuge lives
// in test_xy_point_distance_sort_monster_test.go behind the gocene_monsters
// build tag.

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// shapeTestUtilBlocker names org.apache.lucene.tests.geo.ShapeTestUtil.
const shapeTestUtilBlocker = "requires org.apache.lucene.tests.geo.ShapeTestUtil (not ported)"

// xypdsCartesianDistance renders the private cartesianDistance.
func xypdsCartesianDistance(x1, y1, x2, y2 float64) float64 {
	diffX := x1 - x2
	diffY := y1 - y2
	return math.Sqrt(diffX*diffX + diffY*diffY)
}

// xypdsAddXY adds new XYDocValuesField(name, x, y).
func xypdsAddXY(t *testing.T, doc *document.Document, name string, x, y float32) {
	t.Helper()
	f, err := document.NewXYDocValuesField(name, x, y)
	if err != nil {
		t.Fatalf("new XYDocValuesField: %v", err)
	}
	doc.Add(f.Field)
}

// xypdsNewDistanceSort renders XYDocValuesField.newDistanceSort(field, x, y).
func xypdsNewDistanceSort(t *testing.T, field string, x, y float32) *search.SortField {
	t.Helper()
	sf, err := search.NewXYPointSortField(field, x, y)
	if err != nil {
		t.Fatalf("newDistanceSort: %v", err)
	}
	return sf.SortField
}

func xypdsAssertDistance(t *testing.T, want float64, d *search.FieldDoc) {
	t.Helper()
	if got := d.Fields[0].(float64); got != want {
		t.Fatalf("distance = %v, want %v", got, want)
	}
}

// Add three points and sort by distance
func TestXYPointDistanceSortDistanceSort(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)

	// add some docs
	doc := document.NewDocument()
	xypdsAddXY(t, doc, "location", 40.759011, -73.9844722)
	mustAddDocument(t, iw, doc)
	d1 := xypdsCartesianDistance(float64(float32(40.759011)), float64(float32(-73.9844722)), float64(float32(40.7143528)), float64(float32(-74.0059731)))

	doc = document.NewDocument()
	xypdsAddXY(t, doc, "location", 40.718266, -74.007819)
	mustAddDocument(t, iw, doc)
	d2 := xypdsCartesianDistance(float64(float32(40.718266)), float64(float32(-74.007819)), float64(float32(40.7143528)), float64(float32(-74.0059731)))

	doc = document.NewDocument()
	xypdsAddXY(t, doc, "location", 40.7051157, -74.0088305)
	mustAddDocument(t, iw, doc)
	d3 := xypdsCartesianDistance(float64(float32(40.7051157)), float64(float32(-74.0088305)), float64(float32(40.7143528)), float64(float32(-74.0059731)))

	reader := mustGetReader(t, iw)
	searcher := newSearcher(t, reader)
	mustClose(t, iw)

	sort := search.NewSort(xypdsNewDistanceSort(t, "location", 40.7143528, -74.0059731))
	td, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 3, sort, false)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	xypdsAssertDistance(t, d2, td.FieldDocs[0])
	xypdsAssertDistance(t, d3, td.FieldDocs[1])
	xypdsAssertDistance(t, d1, td.FieldDocs[2])

	mustClose(t, reader, dir)
}

// Add two points (one doc missing) and sort by distance
func TestXYPointDistanceSortMissingLast(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)

	// missing
	doc := document.NewDocument()
	mustAddDocument(t, iw, doc)

	doc = document.NewDocument()
	xypdsAddXY(t, doc, "location", 40.718266, -74.007819)
	mustAddDocument(t, iw, doc)
	d2 := xypdsCartesianDistance(float64(float32(40.718266)), float64(float32(-74.007819)), float64(float32(40.7143528)), float64(float32(-74.0059731)))

	doc = document.NewDocument()
	xypdsAddXY(t, doc, "location", 40.7051157, -74.0088305)
	mustAddDocument(t, iw, doc)
	d3 := xypdsCartesianDistance(float64(float32(40.7051157)), float64(float32(-74.0088305)), float64(float32(40.7143528)), float64(float32(-74.0059731)))

	reader := mustGetReader(t, iw)
	searcher := newSearcher(t, reader)
	mustClose(t, iw)

	sort := search.NewSort(xypdsNewDistanceSort(t, "location", 40.7143528, -74.0059731))
	td, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), 3, sort, false)
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	xypdsAssertDistance(t, d2, td.FieldDocs[0])
	xypdsAssertDistance(t, d3, td.FieldDocs[1])
	xypdsAssertDistance(t, math.Inf(1), td.FieldDocs[2])

	mustClose(t, reader, dir)
}

// Run a few iterations with just 10 docs, hopefully easy to debug
func TestXYPointDistanceSortRandom(t *testing.T) {
	for iters := 0; iters < 100; iters++ {
		xypdsDoRandomTest(t, 10, 100)
	}
}

// xypdsResult renders the static class Result: holds an id+distance.
type xypdsResult struct {
	id       int
	distance float64
}

// compareTo renders Result.compareTo(Result).
func (r xypdsResult) compareTo(o xypdsResult) int {
	cmp := cmpFloat64Java(r.distance, o.distance)
	if cmp == 0 {
		switch {
		case r.id < o.id:
			return -1
		case r.id > o.id:
			return 1
		}
		return 0
	}
	return cmp
}

// equals renders Result.equals(Object).
func (r xypdsResult) equals(o xypdsResult) bool {
	return math.Float64bits(r.distance) == math.Float64bits(o.distance) && r.id == o.id
}

// xypdsDoRandomTest renders the private doRandomTest(int, int).
func xypdsDoRandomTest(t *testing.T, numDocs, numQueries int) {
	t.Helper()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	// else seeds may not to reproduce:
	iwc.SetMergeScheduler(index.NewSerialMergeScheduler())
	writer := newRandomIndexWriterWithConfig(t, dir, iwc)

	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		sf, err := document.NewStoredFieldFromInt("id", i)
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(sf)
		ndv, err := document.NewNumericDocValuesField("id", int64(i))
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(ndv)
		if random().Intn(10) > 7 {
			// float x = ShapeTestUtil.nextFloat(random());
			// float y = ShapeTestUtil.nextFloat(random());
			// doc.add(new XYDocValuesField("field", x, y));
			// doc.add(new StoredField("x", x));
			// doc.add(new StoredField("y", y));
			t.Fatal(shapeTestUtilBlocker)
		} // otherwise "missing"
		mustAddDocument(t, writer, doc)
	}
	reader := mustGetReader(t, writer)
	storedFields := mustStoredFields(t, reader)
	searcher := newSearcher(t, reader)

	for i := 0; i < numQueries; i++ {
		// float x = ShapeTestUtil.nextFloat(random());
		// float y = ShapeTestUtil.nextFloat(random());
		t.Fatal(shapeTestUtilBlocker)
		var x, y float32
		missingValue := math.Inf(1)

		expected := make([]xypdsResult, reader.MaxDoc())

		for doc := 0; doc < reader.MaxDoc(); doc++ {
			targetDoc := storedDocument(t, storedFields, doc)
			var distance float64
			if targetDoc.Get("x") == nil {
				distance = missingValue // missing
			} else {
				docX := float64(targetDoc.Get("x").NumericValue().(float32))
				docY := float64(targetDoc.Get("y").NumericValue().(float32))
				distance = xypdsCartesianDistance(float64(x), float64(y), docX, docY)
			}
			id := targetDoc.Get("id").NumericValue().(int)
			expected[doc] = xypdsResult{id: id, distance: distance}
		}

		sortResults(expected)

		// randomize the topN a bit
		topN := nextInt(1, reader.MaxDoc())
		// sort by distance, then ID
		distanceSort := xypdsNewDistanceSort(t, "field", x, y)
		sort := search.NewSort(distanceSort, search.NewSortField("id", spi.SortFieldTypeInt))

		topDocs, err := searcher.SearchWithSort(search.NewMatchAllDocsQuery(), topN, sort, false)
		if err != nil {
			t.Fatal(err)
		}
		for resultNumber := 0; resultNumber < topN; resultNumber++ {
			fieldDoc := topDocs.FieldDocs[resultNumber]
			actual := xypdsResult{id: fieldDoc.Fields[1].(int), distance: fieldDoc.Fields[0].(float64)}
			if !expected[resultNumber].equals(actual) {
				t.Fatalf("expected %v, got %v", expected[resultNumber], actual)
			}
		}

		// get page2 with searchAfter()
		if topN < reader.MaxDoc() {
			page2 := nextInt(1, reader.MaxDoc()-topN)
			topDocs2, err := searcher.SearchWithSortAfter(topDocs.FieldDocs[topN-1], search.NewMatchAllDocsQuery(), page2, sort, false)
			if err != nil {
				t.Fatal(err)
			}
			for resultNumber := 0; resultNumber < page2; resultNumber++ {
				fieldDoc := topDocs2.FieldDocs[resultNumber]
				actual := xypdsResult{id: fieldDoc.Fields[1].(int), distance: fieldDoc.Fields[0].(float64)}
				if !expected[topN+resultNumber].equals(actual) {
					t.Fatalf("expected %v, got %v", expected[topN+resultNumber], actual)
				}
			}
		}
	}
	mustClose(t, reader, writer, dir)
}

// sortResults renders Arrays.sort(Result[]) (a stable merge sort).
func sortResults(results []xypdsResult) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].compareTo(results[j-1]) < 0; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}
