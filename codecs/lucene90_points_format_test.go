// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

// Port of
// lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90PointsFormat.java
// (Apache Lucene 10.5.0).
//
// The class extends org.apache.lucene.tests.index.BasePointsFormatTestCase
// (not ported): its inherited test methods, and testMergeStability, which
// delegates to super.testMergeStability(), fail naming it. The Java
// constructor runs once per test method and is rendered by
// newLucene90PointsFormatTest. BaseIndexFileFormatTestCase.setUp installs
// getCodec() as the default codec; Gocene has no settable default codec, so
// the ported tests set the codec on their IndexWriterConfig explicitly.

import (
	"bytes"
	"math"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// lucene90PointsFormatTest holds the TestLucene90PointsFormat instance fields.
type lucene90PointsFormatTest struct {
	codec               codecs.Codec
	isFilterCodec       bool
	maxPointsInLeafNode int
}

// sneakyPointsCodec is the anonymous FilterCodec of the constructor ("sneaky
// impersonation!"): it overrides pointsFormat().
type sneakyPointsCodec struct {
	*codecs.FilterCodec
	pointsFormat codecs.PointsFormat
}

func (c *sneakyPointsCodec) PointsFormat() codecs.PointsFormat { return c.pointsFormat }

// sneakyPointsFormat is the anonymous PointsFormat of the constructor.
type sneakyPointsFormat struct {
	maxPointsInLeafNode int
	maxMBSortInHeap     float64
}

func (f *sneakyPointsFormat) Name() string { return codecs.NewLucene90PointsFormat().Name() }

func (f *sneakyPointsFormat) FieldsWriter(writeState *codecs.SegmentWriteState) (codecs.PointsWriter, error) {
	return lucene90.NewLucene90PointsWriterWithSortParams(writeState, f.maxPointsInLeafNode, f.maxMBSortInHeap)
}

func (f *sneakyPointsFormat) FieldsReader(readState *codecs.SegmentReadState) (codecs.PointsReader, error) {
	return lucene90.NewLucene90PointsReader(readState)
}

// newLucene90PointsFormatTest renders the TestLucene90PointsFormat constructor.
func newLucene90PointsFormatTest(t *testing.T, r *rand.Rand) *lucene90PointsFormatTest {
	t.Helper()
	// standard issue
	defaultCodec, err := codecs.GetDefault()
	if err != nil {
		t.Fatalf("default codec: %v", err)
	}
	tc := &lucene90PointsFormatTest{}
	if r.Intn(2) == 0 {
		// randomize parameters
		tc.maxPointsInLeafNode = luceneNextInt(r, 50, 500)
		maxMBSortInHeap := 3.0 + (3 * r.Float64())
		// sneaky impersonation!
		tc.codec = &sneakyPointsCodec{
			FilterCodec: codecs.NewFilterCodec(defaultCodec.Name(), defaultCodec),
			pointsFormat: &sneakyPointsFormat{
				maxPointsInLeafNode: tc.maxPointsInLeafNode,
				maxMBSortInHeap:     maxMBSortInHeap,
			},
		}
		tc.isFilterCodec = true
	} else {
		// standard issue
		tc.codec = defaultCodec
		tc.maxPointsInLeafNode = bkd.DefaultMaxPointsInLeafNode
	}
	return tc
}

func TestLucene90PointsFormat_BasePointsFormatTestCase(t *testing.T) {
	t.Fatal("requires org.apache.lucene.tests.index.BasePointsFormatTestCase (not ported)")
}

func TestLucene90PointsFormat_testMergeStability(t *testing.T) {
	// assumeFalse("TODO: mess with the parameters and test gets angry!", codec instanceof FilterCodec);
	// super.testMergeStability();
	t.Fatal("requires org.apache.lucene.tests.index.BasePointsFormatTestCase.testMergeStability (not ported)")
}

// allPointsVisitor, noPointsVisitor and onePointMatchVisitor render the
// anonymous IntersectVisitor classes of the estimate tests.
type estimateVisitor struct {
	compare func(minPackedValue, maxPackedValue []byte) spi.Relation
}

func (v *estimateVisitor) Visit(docID int) error                                  { return nil }
func (v *estimateVisitor) VisitByPackedValue(docID int, packedValue []byte) error { return nil }
func (v *estimateVisitor) VisitByDocIDSetIterator(iterator spi.DocIdSetIterator) error {
	return spi.DefaultVisitByDocIDSetIterator(v, iterator)
}
func (v *estimateVisitor) VisitByIntsRef(ref *util.IntsRef) error {
	for i := ref.Offset; i < ref.Offset+ref.Length; i++ {
		if err := v.Visit(int(ref.Ints[i])); err != nil {
			return err
		}
	}
	return nil
}
func (v *estimateVisitor) VisitByDocIDSetIteratorAndPackedValue(iterator spi.DocIdSetIterator, packedValue []byte) error {
	for {
		docID, err := iterator.NextDoc()
		if err != nil {
			return err
		}
		if docID == spi.NO_MORE_DOCS {
			return nil
		}
		if err := v.VisitByPackedValue(docID, packedValue); err != nil {
			return err
		}
	}
}
func (v *estimateVisitor) Compare(minPackedValue, maxPackedValue []byte) spi.Relation {
	return v.compare(minPackedValue, maxPackedValue)
}
func (v *estimateVisitor) Grow(count int) {}

func constantRelationVisitor(rel spi.Relation) *estimateVisitor {
	return &estimateVisitor{compare: func(_, _ []byte) spi.Relation { return rel }}
}

// estimateIndex indexes the documents of testEstimatePointCount* and returns
// the point values of field "f" of the only leaf.
func estimateOpenPoints(t *testing.T, tc *lucene90PointsFormatTest, addDocs func(w *index.IndexWriter)) (spi.PointValues, func()) {
	t.Helper()
	dir := luceneNewDirectory()
	iwc := luceneNewIndexWriterConfig()
	// Avoid mockRandomMP since it may cause non-optimal merges that make the
	// number of points per leaf hard to predict: Gocene's newIndexWriterConfig
	// never picks MockRandomMergePolicy.
	iwc.SetCodec(tc.codec)
	w, err := index.NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	addDocs(w)
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	leaves, err := r.Leaves()
	if err != nil {
		t.Fatal(err)
	}
	if len(leaves) != 1 {
		t.Fatalf("reader has %d segments instead of exactly one", len(leaves))
	}
	points, err := leaves[0].LeafReader().GetPointValues("f")
	if err != nil {
		t.Fatalf("getPointValues: %v", err)
	}
	return points, func() {
		if err := r.Close(); err != nil {
			t.Error(err)
		}
		if err := dir.Close(); err != nil {
			t.Error(err)
		}
	}
}

func mustEstimate(t *testing.T, estimate func(spi.IntersectVisitor) (int64, error), v spi.IntersectVisitor) int64 {
	t.Helper()
	n, err := estimate(v)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	return n
}

// checkEstimateDocCount renders the final docCount assertions of both
// estimate tests, including Java's long divisions.
func checkEstimateDocCount(t *testing.T, points spi.PointValues, multiValues bool, numDocs int, pointCount, docCount int64) {
	t.Helper()
	if multiValues {
		size := points.Size()
		want := int64(float64(docCount) * (1.0 - math.Pow(
			float64((int64(numDocs)-pointCount)/size), float64(size/docCount))))
		if docCount != want {
			t.Fatalf("docCount %d != %d", docCount, want)
		}
	} else {
		if want := min(pointCount, int64(numDocs)); want != docCount {
			t.Fatalf("docCount = %d, want %d", docCount, want)
		}
	}
}

func TestLucene90PointsFormat_testEstimatePointCount(t *testing.T) {
	r := luceneRandom(t)
	tc := newLucene90PointsFormatTest(t, r)
	pointValue := make([]byte, 3)
	uniquePointValue := make([]byte, 3)
	r.Read(uniquePointValue)
	numDocs := luceneAtLeast(r, 500) // at night, make sure we have several leaves
	multiValues := r.Intn(2) == 0
	totalValues := 0
	points, closeAll := estimateOpenPoints(t, tc, func(w *index.IndexWriter) {
		for i := 0; i < numDocs; i++ {
			doc := document.NewDocument()
			if i == numDocs/2 {
				totalValues++
				doc.Add(document.NewBinaryPoint("f", bytes.Clone(uniquePointValue)))
			} else {
				numValues := 1
				if multiValues {
					numValues = luceneNextInt(r, 2, 100)
				}
				for j := 0; j < numValues; j++ {
					for {
						r.Read(pointValue)
						if !bytes.Equal(pointValue, uniquePointValue) {
							break
						}
					}
					doc.Add(document.NewBinaryPoint("f", bytes.Clone(pointValue)))
					totalValues++
				}
			}
			if _, err := w.AddDocument(doc); err != nil {
				t.Fatalf("addDocument: %v", err)
			}
		}
	})
	defer closeAll()

	allPointsVisitor := constantRelationVisitor(spi.CellInsideQuery)
	if got := mustEstimate(t, points.EstimatePointCount, allPointsVisitor); got != int64(totalValues) {
		t.Fatalf("estimatePointCount(all) = %d, want %d", got, totalValues)
	}
	if got := mustEstimate(t, points.EstimateDocCount, allPointsVisitor); got != int64(numDocs) {
		t.Fatalf("estimateDocCount(all) = %d, want %d", got, numDocs)
	}

	noPointsVisitor := constantRelationVisitor(spi.CellOutsideQuery)
	// Return 0 if no points match
	if got := mustEstimate(t, points.EstimatePointCount, noPointsVisitor); got != 0 {
		t.Fatalf("estimatePointCount(none) = %d, want 0", got)
	}
	if got := mustEstimate(t, points.EstimateDocCount, noPointsVisitor); got != 0 {
		t.Fatalf("estimateDocCount(none) = %d, want 0", got)
	}

	onePointMatchVisitor := &estimateVisitor{compare: func(minPackedValue, maxPackedValue []byte) spi.Relation {
		if bytes.Compare(uniquePointValue[0:3], maxPackedValue[0:3]) > 0 ||
			bytes.Compare(uniquePointValue[0:3], minPackedValue[0:3]) < 0 {
			return spi.CellOutsideQuery
		}
		return spi.CellCrossesQuery
	}}

	// If only one point matches, then the point count is (maxPointsInLeafNode + 1) / 2
	// in general, or maybe 2x that if the point is a split value
	pointCount := mustEstimate(t, points.EstimatePointCount, onePointMatchVisitor)
	maxPoints := int64(tc.maxPointsInLeafNode)
	lastNodePointCount := int64(totalValues) % maxPoints
	if !(pointCount == (maxPoints+1)/2 || // common case
		pointCount == (lastNodePointCount+1)/2 || // not fully populated leaf
		pointCount == 2*((maxPoints+1)/2) || // if the point is a split value
		pointCount == ((maxPoints+1)/2)+((lastNodePointCount+1)/2)) { // if the point is a split value and one leaf is not fully populated
		t.Fatalf("%d", pointCount)
	}

	docCount := mustEstimate(t, points.EstimateDocCount, onePointMatchVisitor)
	checkEstimateDocCount(t, points, multiValues, numDocs, pointCount, docCount)
}

// The tree is always balanced in the N dims case, and leaves are
// not all full so things are a bit different
func TestLucene90PointsFormat_testEstimatePointCount2Dims(t *testing.T) {
	r := luceneRandom(t)
	tc := newLucene90PointsFormatTest(t, r)
	pointValue := [][]byte{make([]byte, 3), make([]byte, 3)}
	uniquePointValue := [][]byte{make([]byte, 3), make([]byte, 3)}
	r.Read(uniquePointValue[0])
	r.Read(uniquePointValue[1])
	numDocs := luceneAtLeast(r, 1000) // in nightly, make sure we have several leaves
	multiValues := r.Intn(2) == 0
	totalValues := 0
	points, closeAll := estimateOpenPoints(t, tc, func(w *index.IndexWriter) {
		for i := 0; i < numDocs; i++ {
			doc := document.NewDocument()
			if i == numDocs/2 {
				doc.Add(document.NewBinaryPoint("f", bytes.Clone(uniquePointValue[0]), bytes.Clone(uniquePointValue[1])))
				totalValues++
			} else {
				numValues := 1
				if multiValues {
					numValues = luceneNextInt(r, 2, 100)
				}
				for j := 0; j < numValues; j++ {
					for {
						r.Read(pointValue[0])
						r.Read(pointValue[1])
						if !(bytes.Equal(pointValue[0], uniquePointValue[0]) ||
							bytes.Equal(pointValue[1], uniquePointValue[1])) {
							break
						}
					}
					doc.Add(document.NewBinaryPoint("f", bytes.Clone(pointValue[0]), bytes.Clone(pointValue[1])))
					totalValues++
				}
			}
			if _, err := w.AddDocument(doc); err != nil {
				t.Fatalf("addDocument: %v", err)
			}
		}
	})
	defer closeAll()

	allPointsVisitor := constantRelationVisitor(spi.CellInsideQuery)
	if got := mustEstimate(t, points.EstimatePointCount, allPointsVisitor); got != int64(totalValues) {
		t.Fatalf("estimatePointCount(all) = %d, want %d", got, totalValues)
	}
	if got := mustEstimate(t, points.EstimateDocCount, allPointsVisitor); got != int64(numDocs) {
		t.Fatalf("estimateDocCount(all) = %d, want %d", got, numDocs)
	}

	noPointsVisitor := constantRelationVisitor(spi.CellOutsideQuery)
	// Return 0 if no points match
	if got := mustEstimate(t, points.EstimatePointCount, noPointsVisitor); got != 0 {
		t.Fatalf("estimatePointCount(none) = %d, want 0", got)
	}
	if got := mustEstimate(t, points.EstimateDocCount, noPointsVisitor); got != 0 {
		t.Fatalf("estimateDocCount(none) = %d, want 0", got)
	}

	onePointMatchVisitor := &estimateVisitor{compare: func(minPackedValue, maxPackedValue []byte) spi.Relation {
		for dim := 0; dim < 2; dim++ {
			if bytes.Compare(uniquePointValue[dim][0:3], maxPackedValue[dim*3:dim*3+3]) > 0 ||
				bytes.Compare(uniquePointValue[dim][0:3], minPackedValue[dim*3:dim*3+3]) < 0 {
				return spi.CellOutsideQuery
			}
		}
		return spi.CellCrossesQuery
	}}

	pointCount := mustEstimate(t, points.EstimatePointCount, onePointMatchVisitor)
	maxPoints := int64(tc.maxPointsInLeafNode)
	lastNodePointCount := int64(totalValues) % maxPoints
	if !(pointCount == (maxPoints+1)/2 || // common case
		pointCount == (lastNodePointCount+1)/2 || // not fully populated leaf
		pointCount == 2*((maxPoints+1)/2) || // if the point is a split value
		pointCount == ((maxPoints+1)/2)+((lastNodePointCount+1)/2) ||
		// in extreme cases, a point can be shared by 4 leaves
		pointCount == 4*((maxPoints+1)/2) ||
		pointCount == 3*((maxPoints+1)/2)+((lastNodePointCount+1)/2)) {
		t.Fatalf("%d", pointCount)
	}

	docCount := mustEstimate(t, points.EstimateDocCount, onePointMatchVisitor)
	checkEstimateDocCount(t, points, multiValues, numDocs, pointCount, docCount)
}
