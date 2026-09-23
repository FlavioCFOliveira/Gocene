// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterReader.java
// (Apache Lucene 10.5.0). The @Nightly testDuringAddIndexes lives in
// index_writer_reader_monster_test.go.

package index_test

import (
	"math/rand"
	"sort"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// indexWriterReaderNumThreads renders the instance field numThreads.
func indexWriterReaderNumThreads() int {
	if testNightly {
		return 5
	}
	return 2
}

// indexWriterReaderCount renders the public static count(Term, IndexReader).
func indexWriterReaderCount(t testing.TB, term *index.Term, r index.IndexReader) int {
	t.Helper()
	count := 0
	td, err := testUtilDocsForTerm(r, term.Field, term.Text(), 0)
	if err != nil {
		t.Fatalf("TestUtil.docs: %v", err)
	}

	if td != nil {
		liveDocs, err := index.MultiBitsGetLiveDocs(r)
		if err != nil {
			t.Fatalf("MultiBits.getLiveDocs: %v", err)
		}
		for {
			doc, err := td.NextDoc()
			if err != nil {
				t.Fatalf("nextDoc: %v", err)
			}
			if doc == spi.NO_MORE_DOCS {
				break
			}
			if liveDocs == nil || liveDocs.Get(td.DocID()) {
				count++
			}
		}
	}
	return count
}

// indexWriterReaderCreateIndexNoClose renders the public static
// createIndexNoClose(boolean, String, IndexWriter), whose documents come from
// DocHelper.createDocument(int, String, int).
func indexWriterReaderCreateIndexNoClose(t testing.TB) {
	t.Helper()
	t.Fatal(docHelperMissing + " (DocHelper.createDocument(int, String, int))")
}

// getAssertNoDeletesDirectory renders the private
// getAssertNoDeletesDirectory(Directory).
func getAssertNoDeletesDirectory(directory store.Directory) store.Directory {
	if mdw, ok := directory.(*store.MockDirectoryWrapper); ok {
		mdw.SetAssertNoDeleteOpenFile(true)
	}
	return directory
}

func assertIsCurrent(t testing.TB, expected bool, r *index.DirectoryReader, what string) {
	t.Helper()
	current, err := r.IsCurrent()
	if err != nil {
		t.Fatalf("isCurrent: %v", err)
	}
	if current != expected {
		t.Fatalf("%s: isCurrent() expected %v, got %v", what, expected, current)
	}
}

func TestIndexWriterReaderAddCloseOpen(t *testing.T) {
	// Can't use assertNoDeletes: this test pulls a non-NRT
	// reader in the end:
	dir1 := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())

	writer := mustNewIndexWriter(t, dir1, iwc)
	reader := openReaderFromWriter(t, writer)
	defer mustClose(t, reader, writer, dir1)
	indexWriterReaderCreateIndexNoClose(t)
}

func TestIndexWriterReaderUpdateDocument(t *testing.T) {
	dir1 := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	if iwc.GetMaxBufferedDocs() < 20 {
		iwc.SetMaxBufferedDocs(20)
	}
	// no merging
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	writer := mustNewIndexWriter(t, dir1, iwc)
	defer mustClose(t, writer, dir1)

	// create the index
	indexWriterReaderCreateIndexNoClose(t)
}

func TestIndexWriterReaderIsCurrent(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())

	writer := mustNewIndexWriter(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(newTextField(t, "field", "a b c", false))
	mustAddDocument(t, writer, doc)
	mustClose(t, writer)

	iwc = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer = mustNewIndexWriter(t, dir, iwc)
	doc = document.NewDocument()
	doc.Add(newTextField(t, "field", "a b c", false))
	nrtReader := openReaderFromWriter(t, writer)
	assertIsCurrent(t, true, nrtReader, "nrtReader")
	mustAddDocument(t, writer, doc)
	assertIsCurrent(t, false, nrtReader, "nrtReader should see the changes")
	if err := writer.ForceMerge(1); err != nil { // make sure we don't have a merge going on
		t.Fatalf("forceMerge: %v", err)
	}
	assertIsCurrent(t, false, nrtReader, "nrtReader")
	mustClose(t, nrtReader)

	dirReader := mustOpenDirectoryReader(t, dir)
	nrtReader = openReaderFromWriter(t, writer)

	assertIsCurrent(t, true, dirReader, "dirReader")
	assertIsCurrent(t, true, nrtReader, "nothing was committed yet so we are still current")
	if nrtReader.MaxDoc() != 2 { // sees the actual document added
		t.Fatalf("nrtReader.maxDoc: expected 2, got %d", nrtReader.MaxDoc())
	}
	if dirReader.MaxDoc() != 1 {
		t.Fatalf("dirReader.maxDoc: expected 1, got %d", dirReader.MaxDoc())
	}
	mustClose(t, writer) // close is actually a commit both should see the changes
	assertIsCurrent(t, false, nrtReader, "nrtReader")
	// this reader has been opened before the writer was closed / committed
	assertIsCurrent(t, false, dirReader, "dirReader")

	mustClose(t, dirReader, nrtReader, dir)
}

// Test using IW.addIndexes
func TestIndexWriterReaderAddIndexes(t *testing.T) {
	dir1 := getAssertNoDeletesDirectory(newDirectory())
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMaxFullFlushMergeWaitMillis(0)
	if iwc.GetMaxBufferedDocs() < 20 {
		iwc.SetMaxBufferedDocs(20)
	}
	// no merging
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	writer := mustNewIndexWriter(t, dir1, iwc)
	defer mustClose(t, writer, dir1)

	// create the index
	indexWriterReaderCreateIndexNoClose(t)
}

func nrtWriter(t testing.TB, dir store.Directory) *index.IndexWriter {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxFullFlushMergeWaitMillis(0)
	return mustNewIndexWriter(t, dir, conf)
}

func TestIndexWriterReaderAddIndexes2(t *testing.T) {
	dir1 := getAssertNoDeletesDirectory(newDirectory())
	writer := nrtWriter(t, dir1)

	// create a 2nd index
	dir2 := newDirectory()
	writer2 := nrtWriter(t, dir2)
	defer mustClose(t, writer2, writer, dir1, dir2)
	indexWriterReaderCreateIndexNoClose(t)
}

// Deletes using IW.deleteDocuments
func TestIndexWriterReaderDeleteFromIndexWriter(t *testing.T) {
	dir1 := getAssertNoDeletesDirectory(newDirectory())
	writer := nrtWriter(t, dir1)
	defer mustClose(t, writer, dir1)
	// create the index
	indexWriterReaderCreateIndexNoClose(t)
}

func TestIndexWriterReaderAddIndexesAndDoDeletesThreads(t *testing.T) {
	mainDir := getAssertNoDeletesDirectory(newDirectory())

	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(newLogMergePolicy())
	conf.SetMaxFullFlushMergeWaitMillis(0)
	mainWriter := mustNewIndexWriter(t, mainDir, conf)
	reduceOpenFiles(mainWriter)

	// new AddDirectoriesThreads(numIter, mainWriter) indexes its initial
	// documents with DocHelper.createDocument(i, "addindex", 4):
	addDir := newDirectory()
	conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxFullFlushMergeWaitMillis(0)
	conf.SetMaxBufferedDocs(2)
	writer := mustNewIndexWriter(t, addDir, conf)
	reduceOpenFiles(writer)
	defer mustClose(t, writer, addDir, mainWriter, mainDir)
	indexWriterReaderCreateIndexNoClose(t)
}

func TestIndexWriterReaderIndexWriterReopenSegmentFullMerge(t *testing.T) {
	indexWriterReaderDoTestIndexWriterReopenSegment(t)
}

func TestIndexWriterReaderIndexWriterReopenSegment(t *testing.T) {
	indexWriterReaderDoTestIndexWriterReopenSegment(t)
}

// indexWriterReaderDoTestIndexWriterReopenSegment renders
// doTestIndexWriterReopenSegment(boolean): tests creating a segment, then
// check to insure the segment can be seen via IW.getReader.
func indexWriterReaderDoTestIndexWriterReopenSegment(t *testing.T) {
	t.Helper()
	dir1 := getAssertNoDeletesDirectory(newDirectory())
	writer := nrtWriter(t, dir1)
	r1 := openReaderFromWriter(t, writer)
	if r1.MaxDoc() != 0 {
		t.Fatalf("maxDoc: expected 0, got %d", r1.MaxDoc())
	}
	defer mustClose(t, r1, writer, dir1)
	indexWriterReaderCreateIndexNoClose(t)
}

func TestIndexWriterReaderMergeWarmer(t *testing.T) {
	dir1 := getAssertNoDeletesDirectory(newDirectory())
	defer mustClose(t, dir1)
	// Enroll warmer; the index is created with createIndexNoClose
	indexWriterReaderCreateIndexNoClose(t)
}

func TestIndexWriterReaderAfterCommit(t *testing.T) {
	dir1 := getAssertNoDeletesDirectory(newDirectory())
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergeScheduler(index.NewConcurrentMergeScheduler())
	conf.SetMaxFullFlushMergeWaitMillis(0)
	writer := mustNewIndexWriter(t, dir1, conf)
	mustCommit(t, writer)
	defer mustClose(t, writer, dir1)

	// create the index
	indexWriterReaderCreateIndexNoClose(t)
}

// Make sure reader remains usable even if IndexWriter closes
func TestIndexWriterReaderAfterClose(t *testing.T) {
	dir1 := getAssertNoDeletesDirectory(newDirectory())
	writer := nrtWriter(t, dir1)
	defer mustClose(t, writer, dir1)

	// create the index
	indexWriterReaderCreateIndexNoClose(t)
}

// Stress test reopen during add/delete
func TestIndexWriterReaderDuringAddDelete(t *testing.T) {
	dir1 := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicyWithMergeFactor(2))
	if testNightly {
		// if we have a ton of iterations we need to make sure we don't do
		// unnecessary extra flushing otherwise we will time out on nightly
		iwc.SetRAMBufferSizeMB(index.DefaultRAMBufferSizeMB)
		iwc.SetMaxBufferedDocs(index.DisableAutoFlush)
	}
	writer := mustNewIndexWriter(t, dir1, iwc)
	defer mustClose(t, writer, dir1)

	// create the index
	indexWriterReaderCreateIndexNoClose(t)
}

func TestIndexWriterReaderForceMergeDeletes(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(newLogMergePolicy())
	w := mustNewIndexWriter(t, dir, conf)
	doc := document.NewDocument()
	doc.Add(newTextField(t, "field", "a b c", false))
	id := newStringField(t, "id", "", false)
	doc.Add(id)
	id.SetStringValue("0")
	mustAddDocument(t, w, doc)
	id.SetStringValue("1")
	mustAddDocument(t, w, doc)
	mustDeleteTerm(t, w, "id", "0")

	r := openReaderFromWriter(t, w)
	if err := w.ForceMergeDeletes(); err != nil {
		t.Fatalf("forceMergeDeletes: %v", err)
	}
	mustClose(t, w, r)
	r = mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 1, r)
	if r.HasDeletions() {
		t.Fatal("assertFalse(r.hasDeletions())")
	}
	mustClose(t, r, dir)
}

func TestIndexWriterReaderDeletesNumDocs(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	doc.Add(newTextField(t, "field", "a b c", false))
	id := newStringField(t, "id", "", false)
	doc.Add(id)
	id.SetStringValue("0")
	mustAddDocument(t, w, doc)
	id.SetStringValue("1")
	mustAddDocument(t, w, doc)
	r := openReaderFromWriter(t, w)
	assertReaderNumDocs(t, 2, r)
	mustClose(t, r)

	mustDeleteTerm(t, w, "id", "0")
	r = openReaderFromWriter(t, w)
	assertReaderNumDocs(t, 1, r)
	mustClose(t, r)

	mustDeleteTerm(t, w, "id", "1")
	r = openReaderFromWriter(t, w)
	assertReaderNumDocs(t, 0, r)
	mustClose(t, r, w, dir)
}

func TestIndexWriterReaderEmptyIndex(t *testing.T) {
	// Ensures that getReader works on an empty index, which hasn't been
	// committed yet.
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	r := openReaderFromWriter(t, w)
	assertReaderNumDocs(t, 0, r)
	mustClose(t, r, w, dir)
}

func TestIndexWriterReaderSegmentWarmer(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal("org.apache.lucene.tests.search.AssertingIndexSearcher (built by LuceneTestCase.newSearcher(IndexReader) " +
		"inside the merged-segment warmer) is not ported")
}

// simpleMergedSegmentWarmerInfoStream renders the anonymous InfoStream of
// testSimpleMergedSegmentWarmer: it records any "SMSW" message.
type simpleMergedSegmentWarmerInfoStream struct {
	didWarm *bool
}

func (s *simpleMergedSegmentWarmerInfoStream) Close() error { return nil }

func (s *simpleMergedSegmentWarmerInfoStream) Message(component, message string) {
	if component == "SMSW" {
		*s.didWarm = true
	}
}

func (s *simpleMergedSegmentWarmerInfoStream) IsEnabled(string) bool { return true }

func TestIndexWriterReaderSimpleMergedSegmentWarmer(t *testing.T) {
	dir := newDirectory()
	didWarm := false
	infoStream := &simpleMergedSegmentWarmerInfoStream{didWarm: &didWarm}
	mp := newLogMergePolicyWithMergeFactor(10)
	mp.SetTargetSearchConcurrency(1)
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	conf.SetReaderPooling(true)
	conf.SetInfoStream(infoStream)
	mustClose(t, dir)
	t.Fatal("org.apache.lucene.index.SimpleMergedSegmentWarmer does not implement IndexWriter.IndexReaderWarmer: " +
		"its Warm signature diverges, so IndexWriterConfig#setMergedSegmentWarmer(new SimpleMergedSegmentWarmer(infoStream)) is not expressible")
}

func TestIndexWriterReaderReopenAfterNoRealChange(t *testing.T) {
	d := getAssertNoDeletesDirectory(newDirectory())
	w := nrtWriter(t, d)

	r := openReaderFromWriter(t, w) // start pooling readers

	if r2 := openIfChanged(t, r); r2 != nil {
		t.Fatalf("assertNull(r2): %v", r2)
	}

	mustAddDocument(t, w, document.NewDocument())
	r3 := openIfChanged(t, r)
	if r3 == nil {
		t.Fatal("assertNotNull(r3)")
	}
	if r3.GetVersion() == r.GetVersion() {
		t.Fatalf("assertTrue(r3.getVersion() != r.getVersion()): %d", r3.GetVersion())
	}
	assertIsCurrent(t, true, r3, "r3")

	// Deletes nothing in reality...:
	mustDeleteTerm(t, w, "foo", "bar")

	// ... but IW marks this as not current:
	assertIsCurrent(t, false, r3, "r3")
	if r4 := openIfChanged(t, r3); r4 != nil {
		t.Fatalf("assertNull(r4): %v", r4)
	}

	// Deletes nothing in reality...:
	mustDeleteTerm(t, w, "foo", "bar")
	r5, err := index.OpenIfChangedFromWriter(r3, w)
	if err != nil {
		t.Fatalf("openIfChanged(r3, w): %v", err)
	}
	if r5 != nil {
		t.Fatalf("assertNull(r5): %v", r5)
	}

	mustClose(t, r3, w, d)
}

func TestIndexWriterReaderNRTOpenExceptions(t *testing.T) {
	// LUCENE-5262: test that several failed attempts to obtain an NRT reader
	// don't leak file handles.
	dir := getAssertNoDeletesDirectory(newDirectory())
	defer mustClose(t, dir)
	t.Fatal("MockDirectoryWrapper.Failure#callStackContainsAnyOf(String...) is not ported")
}

// Make sure if all we do is open NRT reader against writer, we don't see
// merge starvation.
func TestIndexWriterReaderTooManySegments(t *testing.T) {
	dir := getAssertNoDeletesDirectory(store.NewByteBuffersDirectory())
	// Don't use newIndexWriterConfig, because we need a
	// "sane" mergePolicy:
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMaxFullFlushMergeWaitMillis(0)
	w := mustNewIndexWriter(t, dir, iwc)
	// Create 500 segments:
	for i := 0; i < 500; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", strconv.Itoa(i), false))
		mustAddDocument(t, w, doc)
		r := openReaderFromWriter(t, w)
		// Make sure segment count never exceeds 100:
		leaves, err := r.Leaves()
		if err != nil {
			t.Fatalf("leaves: %v", err)
		}
		if !(len(leaves) < 100) {
			t.Fatalf("assertTrue(r.leaves().size() < 100): %d", len(leaves))
		}
		mustClose(t, r)
	}
	mustClose(t, w, dir)
}

// LUCENE-5912: make sure when you reopen an NRT reader using a commit point,
// the SegmentReaders are in fact shared:
func TestIndexWriterReaderReopenNRTReaderOnCommit(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w := mustNewIndexWriter(t, dir, iwc)
	mustAddDocument(t, w, document.NewDocument())

	// Pull NRT reader; it has 1 segment:
	r1 := openReaderFromWriter(t, w)
	assertLeafCount(t, 1, r1)
	mustAddDocument(t, w, document.NewDocument())
	mustCommit(t, w)

	commits := mustListCommits(t, dir)
	if len(commits) != 1 {
		t.Fatalf("commits.size(): expected 1, got %d", len(commits))
	}
	nr, err := index.OpenIfChangedWithCommit(r1, commits[0])
	if err != nil {
		t.Fatalf("openIfChanged(r1, commit): %v", err)
	}
	if nr == nil {
		t.Fatal("openIfChanged(r1, commit) returned null")
	}
	assertLeafCount(t, 2, nr)

	// Make sure we shared same instance of SegmentReader w/ first reader:
	l1, _ := r1.Leaves()
	l2, _ := nr.Leaves()
	if l1[0].LeafReader() != l2[0].LeafReader() {
		t.Fatal("assertTrue(r1.leaves().get(0).reader() == r2.leaves().get(0).reader())")
	}
	mustClose(t, r1, nr, w, dir)
}

func TestIndexWriterReaderIndexReaderWriterWithLeafSorter(t *testing.T) {
	const fieldName = "field1"
	ascSort := rand.Intn(2) == 0
	missingValue := int64(-1 << 63) // missing values at the end
	if ascSort {
		missingValue = 1<<63 - 1
	}

	// create a comparator that sort leaf readers according to
	// the min value (asc sort) or max value (desc sort) of its points
	sortValue := func(r index.LeafReader) int64 {
		points, err := r.GetPointValues(fieldName)
		if err == nil && points != nil {
			var packed []byte
			if ascSort {
				packed, err = points.GetMinPackedValue()
			} else {
				packed, err = points.GetMaxPackedValue()
			}
			if err == nil {
				return document.DecodeDimension(packed, 0)
			}
		}
		return missingValue
	}
	leafSorter := func(a, b index.LeafReader) int {
		va, vb := sortValue(a), sortValue(b)
		cmp := 0
		if va < vb {
			cmp = -1
		} else if va > vb {
			cmp = 1
		}
		if !ascSort {
			cmp = -cmp
		}
		return cmp
	}

	numDocs := atLeast(30)
	dir := newDirectory()
	iwc := index.NewIndexWriterConfig()
	iwc.SetLeafSorter(leafSorter)
	writer := mustNewIndexWriter(t, dir, iwc)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(document.NewLongPoint(fieldName, int64(nextInt(1, 99))))
		mustAddDocument(t, writer, doc)
		if i > 0 && i%10 == 0 {
			mustFlush(t, writer)
		}
	}

	// Test1: test that leafReaders are sorted according to leafSorter
	// provided in IndexWriterConfig
	reader := openReaderFromWriter(t, writer)
	assertLeavesSorted(t, reader, leafSorter)
	mustClose(t, reader, writer, dir)

	// Test2: test that leafReaders are sorted according to the provided
	// leafSorter when opened from directory
	t.Fatal("org.apache.lucene.index.DirectoryReader#open(Directory, Comparator<LeafReader>) is not ported")
}

// assertLeavesSorted renders the private assertLeavesSorted(DirectoryReader,
// Comparator<LeafReader>).
func assertLeavesSorted(t testing.TB, reader *index.DirectoryReader, leafSorter func(a, b index.LeafReader) int) {
	t.Helper()
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	lrs := make([]index.LeafReader, len(leaves))
	for i, l := range leaves {
		lrs[i] = l.LeafReader()
	}
	expected := append([]index.LeafReader(nil), lrs...)
	sort.SliceStable(expected, func(i, j int) bool { return leafSorter(expected[i], expected[j]) < 0 })
	for i := range lrs {
		if lrs[i] != expected[i] {
			t.Fatalf("leaves are not sorted by the leafSorter at %d", i)
		}
	}
}
