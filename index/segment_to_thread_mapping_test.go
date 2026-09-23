// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestSegmentToThreadMapping.java
// (Apache Lucene 10.5.0). The Java class uses search.IndexSearcher, so the port
// lives in the external index_test package.

package index_test

import (
	"math/rand/v2"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// leafSliceGetMaxDocsMissing names the production member the Java assertions
// read: IndexSearcher.LeafSlice#getMaxDocs() (with the maxDocs field computed
// by the LeafSlice constructor from LeafReaderContextPartition#maxDocs, after
// sorting the partitions with LeafSlice.COMPARATOR).
const leafSliceGetMaxDocsMissing = "org.apache.lucene.search.IndexSearcher.LeafSlice#getMaxDocs() is not ported"

// segmentToThreadMappingDummyReader renders the anonymous LeafReader subclass
// built by dummyIndexReader(int). The abstract methods return what the Java
// overrides return; *spi.IndexReader supplies the IndexReader base members.
type segmentToThreadMappingDummyReader struct {
	*spi.IndexReader
	maxDoc        int
	readerContext *spi.LeafReaderContext
}

func dummyIndexReader(maxDoc int) *segmentToThreadMappingDummyReader {
	r := &segmentToThreadMappingDummyReader{IndexReader: spi.NewIndexReader(), maxDoc: maxDoc}
	// LeafReader's constructor: readerContext = new LeafReaderContext(this).
	r.readerContext = spi.NewLeafReaderContextForReader(r)
	return r
}

func (r *segmentToThreadMappingDummyReader) MaxDoc() int  { return r.maxDoc }
func (r *segmentToThreadMappingDummyReader) NumDocs() int { return r.maxDoc }
func (r *segmentToThreadMappingDummyReader) GetFieldInfos() *spi.FieldInfos {
	return spi.EmptyFieldInfos
}
func (r *segmentToThreadMappingDummyReader) GetLiveDocs() util.Bits { return nil }
func (r *segmentToThreadMappingDummyReader) Terms(string) (spi.Terms, error) {
	return nil, nil
}
func (r *segmentToThreadMappingDummyReader) TermVectors() (spi.TermVectors, error) {
	return index.NewEmptyTermVectors(), nil
}
func (r *segmentToThreadMappingDummyReader) GetNumericDocValues(string) (spi.NumericDocValues, error) {
	return nil, nil
}
func (r *segmentToThreadMappingDummyReader) GetBinaryDocValues(string) (spi.BinaryDocValues, error) {
	return nil, nil
}
func (r *segmentToThreadMappingDummyReader) GetSortedDocValues(string) (spi.SortedDocValues, error) {
	return nil, nil
}
func (r *segmentToThreadMappingDummyReader) GetSortedNumericDocValues(string) (spi.SortedNumericDocValues, error) {
	return nil, nil
}
func (r *segmentToThreadMappingDummyReader) GetSortedSetDocValues(string) (spi.SortedSetDocValues, error) {
	return nil, nil
}
func (r *segmentToThreadMappingDummyReader) GetNormValues(string) (spi.NumericDocValues, error) {
	return nil, nil
}
func (r *segmentToThreadMappingDummyReader) GetDocValuesSkipper(string) (spi.DocValuesSkipper, error) {
	return nil, nil
}
func (r *segmentToThreadMappingDummyReader) GetPointValues(string) (spi.PointValues, error) {
	return nil, nil
}
func (r *segmentToThreadMappingDummyReader) GetFloatVectorValues(string) (spi.FloatVectorValues, error) {
	return nil, nil
}
func (r *segmentToThreadMappingDummyReader) GetByteVectorValues(string) (spi.ByteVectorValues, error) {
	return nil, nil
}
func (r *segmentToThreadMappingDummyReader) SearchNearestVectorsCollector(string, []float32, spi.KnnCollector, util.Bits) error {
	return nil
}
func (r *segmentToThreadMappingDummyReader) SearchNearestVectorsByteCollector(string, []byte, spi.KnnCollector, util.Bits) error {
	return nil
}
func (r *segmentToThreadMappingDummyReader) StoredFields() (spi.StoredFields, error) {
	return segmentToThreadMappingStoredFields{}, nil
}
func (r *segmentToThreadMappingDummyReader) CheckIntegrity() error { return nil }
func (r *segmentToThreadMappingDummyReader) GetCoreCacheHelper() spi.CacheHelper {
	return nil
}
func (r *segmentToThreadMappingDummyReader) GetReaderCacheHelper() spi.CacheHelper {
	return nil
}

// DocFreq, TotalTermFreq, Postings and SearchNearestVectors carry the final
// bodies of LeafReader: Terms.getTerms(this, field) yields Terms.EMPTY because
// terms(field) returns null, so the term is never found.

func (r *segmentToThreadMappingDummyReader) DocFreq(spi.Term) (int, error)         { return 0, nil }
func (r *segmentToThreadMappingDummyReader) TotalTermFreq(spi.Term) (int64, error) { return 0, nil }
func (r *segmentToThreadMappingDummyReader) Postings(spi.Term, int) (spi.PostingsEnum, error) {
	return nil, nil
}

// SearchNearestVectors returns TopDocsCollector.EMPTY_TOPDOCS: FieldInfos.EMPTY
// holds no field, so fieldInfo(field) is null.
func (r *segmentToThreadMappingDummyReader) SearchNearestVectors(field string, _ []float32, _ int, _ util.Bits, _ int) (spi.TopDocs, error) {
	if fi := r.GetFieldInfos().FieldInfo(field); fi == nil || fi.VectorDimension() == 0 {
		return spi.TopDocs{TotalHits: spi.NewTotalHits(0, spi.EQUAL_TO), ScoreDocs: []*spi.ScoreDoc{}}, nil
	}
	panic("unreachable: FieldInfos.EMPTY holds no field")
}

// DocID is the Go-only LeafReader member; the dummy reader is a top-level leaf.
func (r *segmentToThreadMappingDummyReader) DocID() int { return 0 }

// GetMetaData renders getMetaData(): new LeafMetaData(Version.LATEST.major,
// Version.LATEST, null, false).
func (r *segmentToThreadMappingDummyReader) GetMetaData() *spi.IndexReaderMetaData {
	return &spi.IndexReaderMetaData{NumDocs: r.maxDoc, MaxDoc: r.maxDoc}
}

// GetContext is LeafReader#getContext(): the context built by the constructor.
func (r *segmentToThreadMappingDummyReader) GetContext() (spi.IndexReaderContext, error) {
	return r.readerContext, nil
}

// segmentToThreadMappingStoredFields is the anonymous StoredFields returned by
// the dummy reader's storedFields(): document(int, StoredFieldVisitor) is a
// no-op and prefetch keeps StoredFields' default no-op body.
type segmentToThreadMappingStoredFields struct{}

func (segmentToThreadMappingStoredFields) Prefetch([]int) error                       { return nil }
func (segmentToThreadMappingStoredFields) Document(int, spi.StoredFieldVisitor) error { return nil }

func createLeafReaderContexts(maxDocs ...int) []*index.LeafReaderContext {
	leafReaderContexts := make([]*index.LeafReaderContext, 0, len(maxDocs))
	for _, maxDoc := range maxDocs {
		leafReaderContexts = append(leafReaderContexts, spi.NewLeafReaderContextForReader(dummyIndexReader(maxDoc)))
	}
	rand.Shuffle(len(leafReaderContexts), func(i, j int) {
		leafReaderContexts[i], leafReaderContexts[j] = leafReaderContexts[j], leafReaderContexts[i]
	})
	return leafReaderContexts
}

func assertSliceCount(t *testing.T, expected int, slices []search.LeafSlice) {
	t.Helper()
	if len(slices) != expected {
		t.Fatalf("expected %d slices, got %d", expected, len(slices))
	}
}

func assertPartitionCount(t *testing.T, expected int, slice search.LeafSlice) {
	t.Helper()
	if len(slice.Partitions) != expected {
		t.Fatalf("expected %d partitions, got %d", expected, len(slice.Partitions))
	}
}

func TestSegmentToThreadMappingSingleSlice(t *testing.T) {
	leafReaderContexts := createLeafReaderContexts(50_000, 30_000, 30_000, 30_000)
	resultSlices := search.Slices(leafReaderContexts, 250_000, nextInt(4, 10), false)
	assertSliceCount(t, 1, resultSlices)
	assertPartitionCount(t, 4, resultSlices[0])
}

func TestSegmentToThreadMappingSingleSliceWithPartitions(t *testing.T) {
	leafReaderContexts := createLeafReaderContexts(50_000, 30_000, 30_000, 30_000)
	resultSlices := search.Slices(leafReaderContexts, 250_000, nextInt(4, 10), true)
	assertSliceCount(t, 1, resultSlices)
	assertPartitionCount(t, 4, resultSlices[0])
}

// The next tests run every assertion up to the first getMaxDocs() read, then
// fail naming the missing production member.

func TestSegmentToThreadMappingMaxSegmentsPerSlice(t *testing.T) {
	leafReaderContexts := createLeafReaderContexts(50_000, 30_000, 30_000, 30_000)
	resultSlices := search.Slices(leafReaderContexts, 250_000, 3, false)
	assertSliceCount(t, 2, resultSlices)
	assertPartitionCount(t, 3, resultSlices[0])
	t.Fatal(leafSliceGetMaxDocsMissing)
}

func TestSegmentToThreadMappingMaxSegmentsPerSliceWithPartitions(t *testing.T) {
	leafReaderContexts := createLeafReaderContexts(50_000, 30_000, 30_000, 30_000)
	resultSlices := search.Slices(leafReaderContexts, 250_000, 3, true)
	assertSliceCount(t, 2, resultSlices)
	assertPartitionCount(t, 3, resultSlices[0])
	t.Fatal(leafSliceGetMaxDocsMissing)
}

func TestSegmentToThreadMappingSmallSegments(t *testing.T) {
	leafReaderContexts := createLeafReaderContexts(10_000, 10_000, 10_000, 10_000, 10_000, 10_000, 130_000, 130_000)
	resultSlices := search.Slices(leafReaderContexts, 250_000, 5, false)
	assertSliceCount(t, 3, resultSlices)
	assertPartitionCount(t, 2, resultSlices[0])
	t.Fatal(leafSliceGetMaxDocsMissing)
}

func TestSegmentToThreadMappingSmallSegmentsWithPartitions(t *testing.T) {
	leafReaderContexts := createLeafReaderContexts(10_000, 10_000, 10_000, 10_000, 10_000, 10_000, 130_000, 130_000)
	resultSlices := search.Slices(leafReaderContexts, 250_000, 5, true)
	assertSliceCount(t, 3, resultSlices)
	assertPartitionCount(t, 2, resultSlices[0])
	t.Fatal(leafSliceGetMaxDocsMissing)
}

func TestSegmentToThreadMappingLargeSlices(t *testing.T) {
	leafReaderContexts := createLeafReaderContexts(290_900, 170_000, 170_000, 170_000)
	resultSlices := search.Slices(leafReaderContexts, 250_000, 5, false)

	assertSliceCount(t, 3, resultSlices)
	assertPartitionCount(t, 1, resultSlices[0])
	assertPartitionCount(t, 2, resultSlices[1])
	assertPartitionCount(t, 1, resultSlices[2])
}

func TestSegmentToThreadMappingLargeSlicesWithPartitions(t *testing.T) {
	leafReaderContexts := createLeafReaderContexts(290_900, 170_000, 170_000, 170_000)
	resultSlices := search.Slices(leafReaderContexts, 250_000, nextInt(5, 10), true)

	assertSliceCount(t, 4, resultSlices)
	assertPartitionCount(t, 1, resultSlices[0])
	t.Fatal(leafSliceGetMaxDocsMissing)
}

func TestSegmentToThreadMappingSingleSegmentPartitions(t *testing.T) {
	leafReaderContexts := createLeafReaderContexts(750_001)
	resultSlices := search.Slices(leafReaderContexts, 250_000, nextInt(1, 10), true)

	assertSliceCount(t, 4, resultSlices)
	assertPartitionCount(t, 1, resultSlices[0])
	t.Fatal(leafSliceGetMaxDocsMissing)
}

func TestSegmentToThreadMappingExtremeSegmentsPartitioning(t *testing.T) {
	leafReaderContexts := createLeafReaderContexts(2, 5, 10)
	resultSlices := search.Slices(leafReaderContexts, 1, 1, true)

	assertSliceCount(t, 12, resultSlices)
	t.Fatal(leafSliceGetMaxDocsMissing)
}

// indexTwoCommitsOfTwoDocs renders the shared prologue of the two
// intra-slice doc-id order tests.
func indexTwoCommitsOfTwoDocs(t *testing.T) (*store.MockDirectoryWrapper, index.IndexReaderInterface) {
	t.Helper()
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			if _, err := w.AddDocument(document.NewDocument()); err != nil {
				t.Fatalf("addDocument: %v", err)
			}
		}
		if _, err := w.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
	r, err := w.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	mustClose(t, w)
	return dir, r
}

func assertIntraSliceDocIDOrder(t *testing.T, slices []search.LeafSlice) {
	t.Helper()
	if slices == nil {
		t.Fatal("assertNotNull(slices)")
	}
	for _, leafSlice := range slices {
		previousDocBase := leafSlice.Partitions[0].Ctx.DocBase
		for _, leafReaderContextPartition := range leafSlice.Partitions {
			if !(previousDocBase <= leafReaderContextPartition.Ctx.DocBase) {
				t.Fatalf("docBase %d after %d", leafReaderContextPartition.Ctx.DocBase, previousDocBase)
			}
			previousDocBase = leafReaderContextPartition.Ctx.DocBase
		}
	}
}

type segmentToThreadMappingNoopExecutor struct{}

func (segmentToThreadMappingNoopExecutor) Execute(func()) {}

func TestSegmentToThreadMappingIntraSliceDocIDOrder(t *testing.T) {
	dir, r := indexTwoCommitsOfTwoDocs(t)

	s := search.NewIndexSearcherWithExecutor(r, segmentToThreadMappingNoopExecutor{})
	assertIntraSliceDocIDOrder(t, s.GetSlices())
	mustClose(t, r, dir)
}

// TestSegmentToThreadMappingIntraSliceDocIDOrderWithPartitions ports
// testIntraSliceDocIDOrderWithPartitions, whose anonymous IndexSearcher
// subclass overrides the protected slices(List<LeafReaderContext>) so that
// getSlices() partitions every segment one document per partition.
func TestSegmentToThreadMappingIntraSliceDocIDOrderWithPartitions(t *testing.T) {
	dir, r := indexTwoCommitsOfTwoDocs(t)
	defer mustClose(t, r, dir)
	t.Fatal("overriding org.apache.lucene.search.IndexSearcher#slices(List<LeafReaderContext>) " +
		"is not ported: getSlices() cannot dispatch to a subclass override")
}

func TestSegmentToThreadMappingRandom(t *testing.T) {
	maxDocs, minDocs := 500_000, 10_000
	numSegments := 1 + rand.IntN(50)

	leafReaderContexts := make([]*index.LeafReaderContext, 0, numSegments)
	for i := 0; i < numSegments; i++ {
		leafReaderContexts = append(leafReaderContexts,
			spi.NewLeafReaderContextForReader(dummyIndexReader(rand.IntN((maxDocs-minDocs)+1)+minDocs)))
	}
	resultSlices := search.Slices(leafReaderContexts, 250_000, 5, rand.IntN(2) == 0)
	if !(len(resultSlices) > 0) {
		t.Fatal("assertTrue(resultSlices.length > 0)")
	}
}
