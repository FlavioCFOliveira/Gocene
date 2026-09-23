// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestAddIndexes.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"errors"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	testutil "github.com/FlavioCFOliveira/Gocene/tests/util"
)

// Missing members the Java tests reach.
const (
	indexWriterMaxDocIntMissing               = "org.apache.lucene.index.IndexWriter#maxDoc(int) is not ported"
	indexWriterMaybeMergeMissing              = "org.apache.lucene.index.IndexWriter#maybeMerge() is not ported"
	cmsMergeThreadCountMissing                = "org.apache.lucene.index.ConcurrentMergeScheduler#mergeThreadCount() is not ported"
	mergePolicyFindMergesReadersNotDispatched = "org.apache.lucene.index.MergePolicy#findMerges(CodecReader...) is not part of " +
		"Gocene's MergePolicy interface (BaseMergePolicy.FindMergesForReaders is not dispatched)"
)

// addIndexesNewWriter renders the private newWriter(Directory,
// IndexWriterConfig): the config's merge policy is replaced by a
// LogDocMergePolicy.
func addIndexesNewWriter(t testing.TB, dir store.Directory, conf *index.IndexWriterConfig) *index.IndexWriter {
	t.Helper()
	conf.SetMergePolicy(index.NewLogDocMergePolicy())
	return mustNewIndexWriter(t, dir, conf)
}

func addIndexesConfig(mode index.OpenMode) *index.IndexWriterConfig {
	c := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	c.SetOpenMode(mode)
	return c
}

// addIndexesAddDocsContent renders the private addDocs/addDocs2: numDocs
// documents whose "content" is the given text.
func addIndexesAddDocsContent(t testing.TB, writer *index.IndexWriter, numDocs int, content string) {
	t.Helper()
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(newTextField(t, "content", content, false))
		doc.Add(document.NewIntPoint("doc", int32(i)))
		doc.Add(document.NewIntPoint("doc2d", int32(i), int32(i)))
		doc.Add(numericDVField(t, "dv", int64(i)))
		mustAddDocument(t, writer, doc)
	}
}

// addIndexesAddDocs renders the private addDocs(IndexWriter, int).
func addIndexesAddDocs(t testing.TB, writer *index.IndexWriter, numDocs int) {
	t.Helper()
	addIndexesAddDocsContent(t, writer, numDocs, "aaa")
}

// addIndexesAddDocs2 renders the private addDocs2(IndexWriter, int).
func addIndexesAddDocs2(t testing.TB, writer *index.IndexWriter, numDocs int) {
	t.Helper()
	addIndexesAddDocsContent(t, writer, numDocs, "bbb")
}

// addIndexesAddDocsWithID renders the private addDocsWithID(IndexWriter,
// int, int): just like addDocs but with ID, starting from docStart.
func addIndexesAddDocsWithID(t testing.TB, writer *index.IndexWriter, numDocs, docStart int) {
	t.Helper()
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(newTextField(t, "content", "aaa", false))
		doc.Add(newTextField(t, "id", strconv.Itoa(docStart+i), true))
		doc.Add(document.NewIntPoint("doc", int32(i)))
		doc.Add(document.NewIntPoint("doc2d", int32(i), int32(i)))
		doc.Add(numericDVField(t, "dv", int64(i)))
		mustAddDocument(t, writer, doc)
	}
}

// addIndexesVerifyNumDocs renders the private verifyNumDocs(Directory, int).
func addIndexesVerifyNumDocs(t testing.TB, dir store.Directory, numDocs int) {
	t.Helper()
	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader)
	if reader.MaxDoc() != numDocs || reader.NumDocs() != numDocs {
		t.Fatalf("maxDoc/numDocs: expected %d/%d, got %d/%d", numDocs, numDocs, reader.MaxDoc(), reader.NumDocs())
	}
}

// addIndexesVerifyTermDocs renders the private verifyTermDocs(Directory,
// Term, int).
func addIndexesVerifyTermDocs(t testing.TB, dir store.Directory, field, text string, numDocs int) {
	t.Helper()
	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader)
	postingsEnum, err := testUtilDocsForTerm(reader, field, text, index.PostingsFlagNone)
	if err != nil {
		t.Fatalf("TestUtil.docs(%s:%s): %v", field, text, err)
	}
	count := 0
	for postingsEnum != nil {
		doc, err := postingsEnum.NextDoc()
		if err != nil {
			t.Fatalf("nextDoc: %v", err)
		}
		if doc == spi.NO_MORE_DOCS {
			break
		}
		count++
	}
	if count != numDocs {
		t.Fatalf("%s:%s: expected %d docs, got %d", field, text, numDocs, count)
	}
}

// addIndexesSetUpDirs renders the private setUpDirs(Directory, Directory,
// boolean).
func addIndexesSetUpDirs(t testing.TB, dir, aux store.Directory, withID bool) {
	t.Helper()
	conf := addIndexesConfig(index.Create)
	conf.SetMaxBufferedDocs(1000)
	writer := addIndexesNewWriter(t, dir, conf)
	// add 1000 documents in 1 segment
	if withID {
		addIndexesAddDocsWithID(t, writer, 1000, 0)
	} else {
		addIndexesAddDocs(t, writer, 1000)
	}
	assertWriterMaxDoc(t, writer, 1000)
	assertSegmentCount(t, 1, writer)
	mustClose(t, writer)

	auxConf := func(mode index.OpenMode) *index.IndexWriterConfig {
		c := addIndexesConfig(mode)
		c.SetMaxBufferedDocs(1000)
		c.SetMergePolicy(newLogMergePolicyWithCFS(false, 10))
		return c
	}
	writer = addIndexesNewWriter(t, aux, auxConf(index.Create))
	// add 30 documents in 3 segments
	for i := 0; i < 3; i++ {
		if withID {
			addIndexesAddDocsWithID(t, writer, 10, 10*i)
		} else {
			addIndexesAddDocs(t, writer, 10)
		}
		mustClose(t, writer)
		writer = addIndexesNewWriter(t, aux, auxConf(index.Append))
	}
	assertWriterMaxDoc(t, writer, 30)
	assertSegmentCount(t, 3, writer)
	mustClose(t, writer)
}

func assertWriterMaxDoc(t testing.TB, w *index.IndexWriter, maxDoc int) {
	t.Helper()
	if got := iwDocStats(t, w).MaxDoc; got != maxDoc {
		t.Fatalf("writer.getDocStats().maxDoc: expected %d, got %d", maxDoc, got)
	}
}

func mustAddIndexes(t testing.TB, w *index.IndexWriter, dirs ...store.Directory) {
	t.Helper()
	if _, err := w.AddIndexes(dirs...); err != nil {
		t.Fatalf("addIndexes: %v", err)
	}
}

func TestAddIndexesSimpleCase(t *testing.T) {
	// main directory
	dir := newDirectory()
	// two auxiliary directories
	aux := newDirectory()
	aux2 := newDirectory()
	defer mustClose(t, dir, aux, aux2)

	writer := addIndexesNewWriter(t, dir, addIndexesConfig(index.Create))
	// add 100 documents
	addIndexesAddDocs(t, writer, 100)
	assertWriterMaxDoc(t, writer, 100)
	mustClose(t, writer)
	t.Fatal(testUtilCheckIndexMissing)
}

// withPendingDeletesUpdates renders the loop of the testWithPendingDeletes
// tests: adds 10 docs, then replaces them with another 10 docs, so 10
// pending deletes.
func withPendingDeletesUpdates(t testing.TB, writer *index.IndexWriter) {
	t.Helper()
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", strconv.Itoa(i%10), false))
		doc.Add(newTextField(t, "content", "bbb "+strconv.Itoa(i), false))
		doc.Add(document.NewIntPoint("doc", int32(i)))
		doc.Add(document.NewIntPoint("doc2d", int32(i), int32(i)))
		doc.Add(numericDVField(t, "dv", int64(i)))
		mustUpdateDocument(t, writer, index.NewTerm("id", strconv.Itoa(i%10)), doc)
	}
}

// deletePhraseBbb14 renders writer.deleteDocuments(new PhraseQuery("content",
// "bbb", "14")), which deletes one of the 10 added docs, leaving 9.
func deletePhraseBbb14(t testing.TB, writer *index.IndexWriter) {
	t.Helper()
	q := search.NewPhraseQuery(0, "content", "bbb", "14")
	if _, err := writer.DeleteDocumentsQuery([]index.Query{q}); err != nil {
		t.Fatalf("deleteDocuments(query): %v", err)
	}
}

func withPendingDeletesVerify(t testing.TB, writer *index.IndexWriter, dir store.Directory) {
	t.Helper()
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustCommit(t, writer)

	addIndexesVerifyNumDocs(t, dir, 1039)
	addIndexesVerifyTermDocs(t, dir, "content", "aaa", 1030)
	addIndexesVerifyTermDocs(t, dir, "content", "bbb", 9)
}

func TestAddIndexesWithPendingDeletes(t *testing.T) {
	// main directory
	dir := newDirectory()
	// auxiliary directory
	aux := newDirectory()

	addIndexesSetUpDirs(t, dir, aux, false)
	writer := addIndexesNewWriter(t, dir, addIndexesConfig(index.Append))
	mustAddIndexes(t, writer, aux)

	withPendingDeletesUpdates(t, writer)
	deletePhraseBbb14(t, writer)

	withPendingDeletesVerify(t, writer, dir)

	mustClose(t, writer, dir, aux)
}

func TestAddIndexesWithPendingDeletes2(t *testing.T) {
	// main directory
	dir := newDirectory()
	// auxiliary directory
	aux := newDirectory()

	addIndexesSetUpDirs(t, dir, aux, false)
	writer := addIndexesNewWriter(t, dir, addIndexesConfig(index.Append))

	withPendingDeletesUpdates(t, writer)

	mustAddIndexes(t, writer, aux)

	deletePhraseBbb14(t, writer)

	withPendingDeletesVerify(t, writer, dir)

	mustClose(t, writer, dir, aux)
}

func TestAddIndexesWithPendingDeletes3(t *testing.T) {
	// main directory
	dir := newDirectory()
	// auxiliary directory
	aux := newDirectory()

	addIndexesSetUpDirs(t, dir, aux, false)
	writer := addIndexesNewWriter(t, dir, addIndexesConfig(index.Append))

	withPendingDeletesUpdates(t, writer)

	deletePhraseBbb14(t, writer)

	mustAddIndexes(t, writer, aux)

	withPendingDeletesVerify(t, writer, dir)

	mustClose(t, writer, dir, aux)
}

// case 0: add self or exceed maxMergeDocs, expect exception
func TestAddIndexesAddSelf(t *testing.T) {
	// main directory
	dir := newDirectory()
	// auxiliary directory
	aux := newDirectory()

	writer := addIndexesNewWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	// add 100 documents
	addIndexesAddDocs(t, writer, 100)
	assertWriterMaxDoc(t, writer, 100)
	mustClose(t, writer)

	auxConf := func() *index.IndexWriterConfig {
		c := addIndexesConfig(index.Create)
		c.SetMaxBufferedDocs(1000)
		c.SetMergePolicy(newLogMergePolicyUseCFS(false))
		return c
	}
	writer = addIndexesNewWriter(t, aux, auxConf())
	// add 140 documents in separate files
	addIndexesAddDocs(t, writer, 40)
	mustClose(t, writer)
	writer = addIndexesNewWriter(t, aux, auxConf())
	addIndexesAddDocs(t, writer, 100)
	mustClose(t, writer)

	// cannot add self
	writer2 := addIndexesNewWriter(t, dir, addIndexesConfig(index.Append))
	if _, err := writer2.AddIndexes(aux, dir); err == nil {
		t.Fatal("expected IllegalArgumentException from addIndexes(aux, dir)")
	}
	assertWriterMaxDoc(t, writer2, 100)
	mustClose(t, writer2)

	// make sure the index is correct
	addIndexesVerifyNumDocs(t, dir, 100)
	mustClose(t, dir, aux)
}

// tailSegmentsConfig renders the APPEND configs of the tail-segment tests.
func tailSegmentsConfig(maxBufferedDocs int) *index.IndexWriterConfig {
	c := addIndexesConfig(index.Append)
	c.SetMaxBufferedDocs(maxBufferedDocs)
	c.SetMergePolicy(newLogMergePolicyWithMergeFactor(4))
	return c
}

// in all the remaining tests, make the doc count of the oldest segment
// in dir large so that it is never merged in addIndexes()
// case 1: no tail segments
func TestAddIndexesNoTailSegments(t *testing.T) {
	// main directory
	dir := newDirectory()
	// auxiliary directory
	aux := newDirectory()

	addIndexesSetUpDirs(t, dir, aux, false)

	writer := addIndexesNewWriter(t, dir, tailSegmentsConfig(10))
	addIndexesAddDocs(t, writer, 10)

	mustAddIndexes(t, writer, aux)
	assertWriterMaxDoc(t, writer, 1040)
	defer mustClose(t, writer, dir, aux)
	// assertEquals(1000, writer.maxDoc(0))
	t.Fatal(indexWriterMaxDocIntMissing)
}

// case 2: tail segments, invariants hold, no copy
func TestAddIndexesNoCopySegments(t *testing.T) {
	// main directory
	dir := newDirectory()
	// auxiliary directory
	aux := newDirectory()

	addIndexesSetUpDirs(t, dir, aux, false)

	writer := addIndexesNewWriter(t, dir, tailSegmentsConfig(9))
	addIndexesAddDocs(t, writer, 2)

	mustAddIndexes(t, writer, aux)
	assertWriterMaxDoc(t, writer, 1032)
	defer mustClose(t, writer, dir, aux)
	// assertEquals(1000, writer.maxDoc(0))
	t.Fatal(indexWriterMaxDocIntMissing)
}

func mustRAMCopyWrapper(t testing.TB, dir store.Directory) *store.MockDirectoryWrapper {
	t.Helper()
	cp, err := testutil.RamCopyOf(dir)
	if err != nil {
		t.Fatalf("TestUtil.ramCopyOf: %v", err)
	}
	return store.NewMockDirectoryWrapper(cp)
}

// case 3: tail segments, invariants hold, copy, invariants hold
func TestAddIndexesNoMergeAfterCopy(t *testing.T) {
	// main directory
	dir := newDirectory()
	// auxiliary directory
	aux := newDirectory()

	addIndexesSetUpDirs(t, dir, aux, false)

	writer := addIndexesNewWriter(t, dir, tailSegmentsConfig(10))

	mustAddIndexes(t, writer, aux, mustRAMCopyWrapper(t, aux))
	assertWriterMaxDoc(t, writer, 1060)
	defer mustClose(t, writer, dir, aux)
	// assertEquals(1000, writer.maxDoc(0))
	t.Fatal(indexWriterMaxDocIntMissing)
}

// case 4: tail segments, invariants hold, copy, invariants not hold
func TestAddIndexesMergeAfterCopy(t *testing.T) {
	// main directory
	dir := newDirectory()
	// auxiliary directory
	aux := newDirectory()

	addIndexesSetUpDirs(t, dir, aux, true)

	writer := noMergeWriter(t, aux)
	for i := 0; i < 20; i++ {
		mustDeleteTerm(t, writer, "id", strconv.Itoa(i))
	}
	mustClose(t, writer)
	reader := mustOpenDirectoryReader(t, aux)
	assertReaderNumDocs(t, 10, reader)
	mustClose(t, reader)

	writer = addIndexesNewWriter(t, dir, tailSegmentsConfig(4))

	mustAddIndexes(t, writer, aux, mustRAMCopyWrapper(t, aux))
	assertWriterMaxDoc(t, writer, 1020)
	defer mustClose(t, writer, dir, aux)
	// assertEquals(1000, writer.maxDoc(0))
	t.Fatal(indexWriterMaxDocIntMissing)
}

// case 5: tail segments, invariants not hold
func TestAddIndexesMoreMerges(t *testing.T) {
	// main directory
	dir := newDirectory()
	// auxiliary directory
	aux := newDirectory()
	aux2 := newDirectory()

	addIndexesSetUpDirs(t, dir, aux, true)

	conf := addIndexesConfig(index.Create)
	conf.SetMaxBufferedDocs(100)
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(10))
	writer := addIndexesNewWriter(t, aux2, conf)
	mustAddIndexes(t, writer, aux)
	assertWriterMaxDoc(t, writer, 30)
	assertSegmentCount(t, 3, writer)
	mustClose(t, writer)

	writer = noMergeWriter(t, aux)
	for i := 0; i < 27; i++ {
		mustDeleteTerm(t, writer, "id", strconv.Itoa(i))
	}
	mustClose(t, writer)
	reader := mustOpenDirectoryReader(t, aux)
	assertReaderNumDocs(t, 3, reader)
	mustClose(t, reader)

	writer = noMergeWriter(t, aux2)
	for i := 0; i < 8; i++ {
		mustDeleteTerm(t, writer, "id", strconv.Itoa(i))
	}
	mustClose(t, writer)
	reader = mustOpenDirectoryReader(t, aux2)
	assertReaderNumDocs(t, 22, reader)
	mustClose(t, reader)

	writer = addIndexesNewWriter(t, dir, tailSegmentsConfig(6))

	mustAddIndexes(t, writer, aux, aux2)
	assertWriterMaxDoc(t, writer, 1040)
	defer mustClose(t, writer, dir, aux, aux2)
	// assertEquals(1000, writer.maxDoc(0))
	t.Fatal(indexWriterMaxDocIntMissing)
}

// LUCENE-1270
func TestAddIndexesHangOnClose(t *testing.T) {
	dir := newDirectory()
	lmp := index.NewLogByteSizeMergePolicy()
	lmp.SetNoCFSRatio(0.0)
	lmp.SetMergeFactor(100)
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(5)
	conf.SetMergePolicy(lmp)
	writer := mustNewIndexWriter(t, dir, conf)

	doc := document.NewDocument()
	doc.Add(newField(t, "content", "aaa bbb ccc ddd eee fff ggg hhh iii", textStoredWithVectors(true, true)))
	for i := 0; i < 60; i++ {
		mustAddDocument(t, writer, doc)
	}

	doc2 := document.NewDocument()
	customType2 := document.NewFieldType()
	customType2.SetStored(true)
	for i := 0; i < 4; i++ {
		doc2.Add(newField(t, "content", "aaa bbb ccc ddd eee fff ggg hhh iii", customType2))
	}
	for i := 0; i < 10; i++ {
		mustAddDocument(t, writer, doc2)
	}
	mustClose(t, writer)

	dir2 := newDirectory()
	lmp = index.NewLogByteSizeMergePolicy()
	lmp.SetMinMergeMB(0.0001)
	lmp.SetNoCFSRatio(0.0)
	lmp.SetMergeFactor(4)
	conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergeScheduler(index.NewSerialMergeScheduler())
	conf.SetMergePolicy(lmp)
	writer = mustNewIndexWriter(t, dir2, conf)
	mustAddIndexes(t, writer, dir)
	mustClose(t, writer, dir, dir2)
}

// concurrentAddIndexesMergePolicy renders the private
// ConcurrentAddIndexesMergePolicy: a TieredMergePolicy whose
// findMerges(CodecReader...) creates one OneMerge per reader so addIndexes
// processes them concurrently.
type concurrentAddIndexesMergePolicy struct {
	*index.TieredMergePolicy
}

func newConcurrentAddIndexesMergePolicy() *concurrentAddIndexesMergePolicy {
	return &concurrentAddIndexesMergePolicy{TieredMergePolicy: index.NewTieredMergePolicy()}
}

// FindMergesForReaders renders the findMerges(CodecReader...) override.
func (p *concurrentAddIndexesMergePolicy) FindMergesForReaders(readers []index.CodecReader) (*index.MergeSpecification, error) {
	// create a oneMerge for each reader to let them get concurrently processed by addIndexes()
	mergeSpec := index.NewMergeSpecification()
	for _, reader := range readers {
		mergeSpec.Add(index.NewOneMergeFromReaders([]index.CodecReader{reader}))
	}
	return mergeSpec, nil
}

// addIndexesWithReadersSetup renders the private AddIndexesWithReadersSetup.
type addIndexesWithReadersSetup struct {
	dir, destDir store.Directory
	destWriter   *index.IndexWriter
	readers      []*index.DirectoryReader
}

const (
	addIndexesAddedDocsPerReader = 15
	addIndexesInitDocs           = 25
	addIndexesNumReaders         = 15
)

func newAddIndexesWithReadersSetup(t testing.TB, ms index.MergeScheduler, mp index.MergePolicy) *addIndexesWithReadersSetup {
	t.Helper()
	c := &addIndexesWithReadersSetup{}
	c.dir = store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	writer := mustNewIndexWriter(t, c.dir, conf)
	for i := 0; i < addIndexesAddedDocsPerReader; i++ {
		testIndexWriterAddDoc(t, writer)
	}
	mustClose(t, writer)

	c.destDir = newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(mp)
	iwc.SetMergeScheduler(ms)
	c.destWriter = mustNewIndexWriter(t, c.destDir, iwc)
	for i := 0; i < addIndexesInitDocs; i++ {
		testIndexWriterAddDoc(t, c.destWriter)
	}
	mustCommit(t, c.destWriter)

	c.readers = make([]*index.DirectoryReader, addIndexesNumReaders)
	for i := range c.readers {
		c.readers[i] = mustOpenDirectoryReader(t, c.dir)
	}
	return c
}

func (c *addIndexesWithReadersSetup) closeAll(t testing.TB) {
	t.Helper()
	mustClose(t, c.destWriter)
	for _, r := range c.readers {
		mustClose(t, r)
	}
	mustClose(t, c.destDir, c.dir)
}

func TestAddIndexesAddIndexesWithConcurrentMerges(t *testing.T) {
	mp := newConcurrentAddIndexesMergePolicy()
	c := newAddIndexesWithReadersSetup(t, index.NewConcurrentMergeScheduler(), mp)
	defer c.closeAll(t)
	t.Fatal(testUtilAddIndexesSlowlyMissing)
}

// partialMergeScheduler renders the private PartialMergeScheduler: it runs
// the first mergesToDo merges and marks every later one as failed.
type partialMergeScheduler struct {
	*index.BaseMergeScheduler
	mergesToDo      int
	mergesTriggered int
}

func newPartialMergeScheduler(mergesToDo int) *partialMergeScheduler {
	return &partialMergeScheduler{BaseMergeScheduler: index.NewBaseMergeScheduler(), mergesToDo: mergesToDo}
}

func (s *partialMergeScheduler) Merge(mergeSource index.MergeSource, _ index.MergeTrigger) error {
	for {
		merge := mergeSource.GetNextMerge()
		if merge == nil {
			break
		}
		if s.mergesTriggered >= s.mergesToDo {
			if err := merge.Close(false, false, func(*index.MergeReader) error { return nil }); err != nil {
				return err
			}
			mergeSource.OnMergeFinished(merge)
		} else {
			if err := mergeSource.Merge(merge); err != nil {
				return err
			}
			s.mergesTriggered++
		}
	}
	return nil
}

func (s *partialMergeScheduler) Close() error { return nil }

func TestAddIndexesAddIndexesWithPartialMergeFailures(t *testing.T) {
	// The merge policy subclasses ConcurrentAddIndexesMergePolicy to collect
	// the merges its findMerges(CodecReader...) returns.
	mp := newConcurrentAddIndexesMergePolicy()
	c := newAddIndexesWithReadersSetup(t, newPartialMergeScheduler(2), mp)
	defer c.closeAll(t)
	t.Fatal(testUtilAddIndexesSlowlyMissing)
}

// nullMergeSpecPolicy renders the anonymous TieredMergePolicy of
// testAddIndexesWithNullMergeSpec.
type nullMergeSpecPolicy struct {
	*index.TieredMergePolicy
}

func (nullMergeSpecPolicy) FindMergesForReaders([]index.CodecReader) (*index.MergeSpecification, error) {
	return nil, nil
}

func TestAddIndexesAddIndexesWithNullMergeSpec(t *testing.T) {
	mp := nullMergeSpecPolicy{TieredMergePolicy: index.NewTieredMergePolicy()}
	c := newAddIndexesWithReadersSetup(t, index.NewConcurrentMergeScheduler(), mp)
	defer c.closeAll(t)
	t.Fatal(testUtilAddIndexesSlowlyMissing)
}

// emptyMergeSpecPolicy renders the anonymous TieredMergePolicy of
// testAddIndexesWithEmptyMergeSpec.
type emptyMergeSpecPolicy struct {
	*index.TieredMergePolicy
}

func (emptyMergeSpecPolicy) FindMergesForReaders([]index.CodecReader) (*index.MergeSpecification, error) {
	return index.NewMergeSpecification(), nil
}

func TestAddIndexesAddIndexesWithEmptyMergeSpec(t *testing.T) {
	mp := emptyMergeSpecPolicy{TieredMergePolicy: index.NewTieredMergePolicy()}
	c := newAddIndexesWithReadersSetup(t, index.NewConcurrentMergeScheduler(), mp)
	defer c.closeAll(t)
	t.Fatal(testUtilAddIndexesSlowlyMissing)
}

// countingSerialMergeScheduler renders the private
// CountingSerialMergeScheduler.
type countingSerialMergeScheduler struct {
	*index.BaseMergeScheduler
	explicitMerges   int
	addIndexesMerges int
}

func newCountingSerialMergeScheduler() *countingSerialMergeScheduler {
	return &countingSerialMergeScheduler{BaseMergeScheduler: index.NewBaseMergeScheduler()}
}

func (s *countingSerialMergeScheduler) Merge(mergeSource index.MergeSource, trigger index.MergeTrigger) error {
	for {
		merge := mergeSource.GetNextMerge()
		if merge == nil {
			break
		}
		if err := mergeSource.Merge(merge); err != nil {
			return err
		}
		if trigger == index.MergeTriggerExplicit {
			s.explicitMerges++
		}
		if trigger == index.MergeTriggerAddIndexes {
			s.addIndexesMerges++
		}
	}
	return nil
}

func (s *countingSerialMergeScheduler) Close() error { return nil }

func TestAddIndexesAddIndexesWithEmptyReaders(t *testing.T) {
	destDir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newConcurrentAddIndexesMergePolicy())
	ms := newCountingSerialMergeScheduler()
	iwc.SetMergeScheduler(ms)
	destWriter := mustNewIndexWriter(t, destDir, iwc)
	const initialDocs = 15
	for i := 0; i < initialDocs; i++ {
		testIndexWriterAddDoc(t, destWriter)
	}
	mustCommit(t, destWriter)

	// create empty readers
	dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	writer := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustClose(t, writer)
	const numReaders = 20
	readers := make([]*index.DirectoryReader, numReaders)
	for i := range readers {
		readers[i] = mustOpenDirectoryReader(t, dir)
	}
	defer func() {
		mustClose(t, destWriter)
		for _, r := range readers {
			mustClose(t, r)
		}
		mustClose(t, destDir, dir)
	}()

	t.Fatal(testUtilAddIndexesSlowlyMissing)
}

func TestAddIndexesCascadingMergesTriggered(t *testing.T) {
	mp := newConcurrentAddIndexesMergePolicy()
	ms := newCountingSerialMergeScheduler()
	c := newAddIndexesWithReadersSetup(t, ms, mp)
	defer c.closeAll(t)
	t.Fatal(testUtilAddIndexesSlowlyMissing)
}

func TestAddIndexesAddIndexesHittingMaxDocsLimit(t *testing.T) {
	const writerMaxDocs = 15
	setIndexWriterMaxDocs(t, writerMaxDocs)
	defer restoreIndexWriterMaxDocs(t)

	// create destination writer
	destDir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newConcurrentAddIndexesMergePolicy())
	ms := newCountingSerialMergeScheduler()
	iwc.SetMergeScheduler(ms)
	destWriter := mustNewIndexWriter(t, destDir, iwc)
	for i := 0; i < writerMaxDocs; i++ {
		testIndexWriterAddDoc(t, destWriter)
	}
	mustCommit(t, destWriter)

	// create readers to add
	dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	writer := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	for i := 0; i < 10; i++ {
		testIndexWriterAddDoc(t, writer)
	}
	mustClose(t, writer)
	const numReaders = 20
	readers := make([]*index.DirectoryReader, numReaders)
	for i := range readers {
		readers[i] = mustOpenDirectoryReader(t, dir)
	}
	defer func() {
		mustClose(t, destWriter)
		for _, r := range readers {
			mustClose(t, r)
		}
		mustClose(t, destDir, dir)
	}()

	t.Fatal(testUtilAddIndexesSlowlyMissing)
}

// runAddIndexesThreads renders the private abstract RunAddIndexesThreads;
// doBody and handle are its abstract methods.
type runAddIndexesThreads struct {
	t        *testing.T
	dir      store.Directory
	dir2     store.Directory
	writer2  *index.IndexWriter
	failures []error
	failMu   sync.Mutex
	didClose bool
	readers  []*index.DirectoryReader
	numCopy  int
	threads  int
	wg       sync.WaitGroup
	doBody   func(j int, dirs []store.Directory) error
	handle   func(err error)
}

const runAddIndexesNumInitDocs = 17

func newRunAddIndexesThreads(t *testing.T, numCopy int) *runAddIndexesThreads {
	t.Helper()
	c := &runAddIndexesThreads{t: t, numCopy: numCopy}
	c.dir = store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	writer := mustNewIndexWriter(t, c.dir, conf)
	for i := 0; i < runAddIndexesNumInitDocs; i++ {
		testIndexWriterAddDoc(t, writer)
	}
	mustClose(t, writer)

	c.dir2 = newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newConcurrentAddIndexesMergePolicy())
	c.writer2 = mustNewIndexWriter(t, c.dir2, iwc)
	mustCommit(t, c.writer2)

	c.readers = make([]*index.DirectoryReader, numCopy)
	for i := range c.readers {
		c.readers[i] = mustOpenDirectoryReader(t, c.dir)
	}
	c.threads = 2
	if testNightly {
		c.threads = 5
	}
	return c
}

// errAddIndexesSlowlyMissing reports, from an indexing thread, that
// TestUtil.addIndexesSlowly is not ported; the goroutine stops there.
var errAddIndexesSlowlyMissing = errors.New(testUtilAddIndexesSlowlyMissing)

func (c *runAddIndexesThreads) launchThreads(numIter int) {
	for i := 0; i < c.threads; i++ {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			dirs := make([]store.Directory, c.numCopy)
			for k := range dirs {
				cp, err := testutil.RamCopyOf(c.dir)
				if err != nil {
					c.handle(err)
					return
				}
				dirs[k] = store.NewMockDirectoryWrapper(cp)
			}

			j := 0
			for {
				if numIter > 0 && j == numIter {
					break
				}
				if err := c.doBody(j, dirs); err != nil {
					if errors.Is(err, errAddIndexesSlowlyMissing) {
						c.t.Errorf("%v", err)
						return
					}
					c.handle(err)
					return
				}
				j++
			}
		}()
	}
}

func (c *runAddIndexesThreads) joinThreads() { c.wg.Wait() }

func (c *runAddIndexesThreads) close(doWait bool) error {
	c.didClose = true
	if !doWait {
		return c.writer2.Rollback()
	}
	return c.writer2.Close()
}

func (c *runAddIndexesThreads) closeDir(t testing.TB) {
	t.Helper()
	for _, r := range c.readers {
		mustClose(t, r)
	}
	mustClose(t, c.dir2)
}

func (c *runAddIndexesThreads) addFailure(err error) {
	c.failMu.Lock()
	c.failures = append(c.failures, err)
	c.failMu.Unlock()
}

// commitAndAddIndexesDoBody renders CommitAndAddIndexes.doBody.
func (c *runAddIndexesThreads) commitAndAddIndexesDoBody(j int, dirs []store.Directory) error {
	switch j % 5 {
	case 0:
		if _, err := c.writer2.AddIndexes(dirs...); err != nil {
			return err
		}
		if err := c.writer2.ForceMerge(1); err != nil {
			if !errors.Is(errors.Unwrap(err), index.ErrMergeAborted) {
				return err
			}
			// OK
		}
	case 1:
		if _, err := c.writer2.AddIndexes(dirs...); err != nil {
			return err
		}
	case 2:
		// TestUtil.addIndexesSlowly(writer2, readers)
		return errAddIndexesSlowlyMissing
	case 3:
		if _, err := c.writer2.AddIndexes(dirs...); err != nil {
			return err
		}
		return errors.New(indexWriterMaybeMergeMissing)
	case 4:
		if _, err := c.writer2.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// newCommitAndAddIndexes renders the private CommitAndAddIndexes.
func newCommitAndAddIndexes(t *testing.T, numCopy int) *runAddIndexesThreads {
	c := newRunAddIndexesThreads(t, numCopy)
	c.doBody = c.commitAndAddIndexesDoBody
	c.handle = c.addFailure
	return c
}

// LUCENE-1335: test simultaneous addIndexes & commits
// from multiple threads
func TestAddIndexesAddIndexesWithThreads(t *testing.T) {
	numIter := 5
	if testNightly {
		numIter = 15
	}
	const numCopy = 3
	c := newCommitAndAddIndexes(t, numCopy)
	c.launchThreads(numIter)

	for i := 0; i < 100; i++ {
		testIndexWriterAddDoc(t, c.writer2)
	}

	c.joinThreads()
	if t.Failed() {
		mustClose(t, c.writer2)
		c.closeDir(t)
		t.FailNow()
	}

	expectedNumDocs := 100 + numCopy*(4*numIter/5)*c.threads*runAddIndexesNumInitDocs
	if got := iwDocStats(t, c.writer2).NumDocs; got != expectedNumDocs {
		t.Fatalf("expected num docs don't match - failures: %v: expected %d, got %d", c.failures, expectedNumDocs, got)
	}

	if err := c.close(true); err != nil {
		t.Fatalf("close: %v", err)
	}

	if len(c.failures) != 0 {
		t.Fatalf("found unexpected failures: %v", c.failures)
	}

	reader := mustOpenDirectoryReader(t, c.dir2)
	assertReaderNumDocs(t, expectedNumDocs, reader)
	mustClose(t, reader)

	c.closeDir(t)
}

// newCommitAndAddIndexes2 renders the private CommitAndAddIndexes2: its
// handle ignores AlreadyClosedException and NullPointerException.
func newCommitAndAddIndexes2(t *testing.T, numCopy int) *runAddIndexesThreads {
	c := newCommitAndAddIndexes(t, numCopy)
	c.handle = func(err error) {
		var ace *store.AlreadyClosedException
		if !errors.As(err, &ace) {
			c.addFailure(err)
		}
	}
	return c
}

// LUCENE-1335: test simultaneous addIndexes & close
func TestAddIndexesAddIndexesWithClose(t *testing.T) {
	const numCopy = 3
	c := newCommitAndAddIndexes2(t, numCopy)
	c.launchThreads(-1)

	// Close w/o first stopping/joining the threads
	if err := c.close(true); err != nil {
		t.Fatalf("close: %v", err)
	}

	c.joinThreads()

	c.closeDir(t)

	if len(c.failures) != 0 {
		t.Fatalf("assertTrue(c.failures.size() == 0): %v", c.failures)
	}
}

// commitAndAddIndexes3DoBody renders CommitAndAddIndexes3.doBody.
func (c *runAddIndexesThreads) commitAndAddIndexes3DoBody(j int, dirs []store.Directory) error {
	switch j % 5 {
	case 0:
		if _, err := c.writer2.AddIndexes(dirs...); err != nil {
			return err
		}
		if err := c.writer2.ForceMerge(1); err != nil {
			return err
		}
	case 1:
		if _, err := c.writer2.AddIndexes(dirs...); err != nil {
			return err
		}
	case 2:
		// TestUtil.addIndexesSlowly(writer2, readers)
		return errAddIndexesSlowlyMissing
	case 3:
		if err := c.writer2.ForceMerge(1); err != nil {
			return err
		}
	case 4:
		if _, err := c.writer2.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// newCommitAndAddIndexes3 renders the private CommitAndAddIndexes3.
func newCommitAndAddIndexes3(t *testing.T, numCopy int) *runAddIndexesThreads {
	c := newRunAddIndexesThreads(t, numCopy)
	c.doBody = c.commitAndAddIndexes3DoBody
	c.handle = func(err error) {
		report := true
		var ace *store.AlreadyClosedException
		switch {
		case errors.As(err, &ace), err == index.ErrMergeAborted:
			report = !c.didClose
		case isFileNotFoundOrNoSuchFile(err):
			report = !c.didClose
		case errors.Is(errors.Unwrap(err), index.ErrMergeAborted):
			report = !c.didClose
		}
		if report {
			c.addFailure(err)
		}
	}
	return c
}

// LUCENE-1335: test simultaneous addIndexes & close
func TestAddIndexesAddIndexesWithCloseNoWait(t *testing.T) {
	const numCopy = 50
	c := newCommitAndAddIndexes3(t, numCopy)
	c.launchThreads(-1)

	time.Sleep(time.Duration(nextInt(10, 500)) * time.Millisecond)

	// Close w/o first stopping/joining the threads
	if err := c.close(false); err != nil {
		t.Fatalf("close(false): %v", err)
	}

	c.joinThreads()

	c.closeDir(t)
	if len(c.failures) != 0 {
		t.Fatalf("assertTrue(c.failures.size() == 0): %v", c.failures)
	}
}

// LUCENE-1335: test simultaneous addIndexes & close
func TestAddIndexesAddIndexesWithRollback(t *testing.T) {
	numCopy := 5
	if testNightly {
		numCopy = 50
	}
	c := newCommitAndAddIndexes3(t, numCopy)
	c.launchThreads(-1)

	time.Sleep(time.Duration(nextInt(10, 500)) * time.Millisecond)

	// Close w/o first stopping/joining the threads
	c.didClose = true
	ms := c.writer2.GetConfig().GetMergeScheduler()

	if err := c.writer2.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	c.joinThreads()

	if _, ok := ms.(*index.ConcurrentMergeScheduler); ok {
		c.closeDir(t)
		t.Fatal(cmsMergeThreadCountMissing)
	}

	c.closeDir(t)
	if len(c.failures) != 0 {
		t.Fatalf("assertTrue(c.failures.size() == 0): %v", c.failures)
	}
}

// LUCENE-2996: tests that addIndexes(IndexReader) applies existing deletes correctly.
func TestAddIndexesExistingDeletes(t *testing.T) {
	dirs := make([]store.Directory, 2)
	for i := range dirs {
		dirs[i] = newDirectory()
		conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		writer := mustNewIndexWriter(t, dirs[i], conf)
		mustAddDocument(t, writer, stringFieldDoc(t, "id", "myid"))
		mustClose(t, writer)
	}

	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dirs[0], conf)

	// Now delete the document
	mustDeleteTerm(t, writer, "id", "myid")
	r := mustOpenDirectoryReader(t, dirs[1])
	defer mustClose(t, r, writer, dirs[0], dirs[1])
	t.Fatal(testUtilAddIndexesSlowlyMissing)
}

func TestAddIndexesSimpleCaseCustomCodec(t *testing.T) {
	// main directory
	dir := newDirectory()
	// two auxiliary directories
	aux := newDirectory()
	aux2 := newDirectory()
	defer mustClose(t, dir, aux, aux2)
	// Codec codec = new CustomPerFieldCodec(), an AssertingCodec subclass.
	t.Fatal(assertingCodecMissing)
}

// LUCENE-2790: tests that the non CFS files were deleted by addIndexes
func TestAddIndexesNonCFSLeftovers(t *testing.T) {
	dirs := make([]store.Directory, 2)
	for i := range dirs {
		dirs[i] = store.NewByteBuffersDirectory()
		w := mustNewIndexWriter(t, dirs[i], index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
		d := document.NewDocument()
		customType := document.NewFieldTypeFrom(document.TextFieldTypeStored)
		customType.SetStoreTermVectors(true)
		d.Add(mustNewFieldNoRandom(t, "c", "v", customType))
		mustAddDocument(t, w, d)
		mustClose(t, w)
	}

	readers := []*index.DirectoryReader{mustOpenDirectoryReader(t, dirs[0]), mustOpenDirectoryReader(t, dirs[1])}

	dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(newLogMergePolicyUseCFS(true))
	lmp := conf.GetMergePolicy().(logMergePolicy)
	// Force creation of CFS:
	lmp.SetNoCFSRatio(1.0)
	lmp.SetMaxCFSSegmentSizeMB(math.Inf(1))
	w3 := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, w3, readers[0], readers[1], dir, dirs[0], dirs[1])
	t.Fatal(testUtilAddIndexesSlowlyMissing)
}

// simple test that ensures we getting expected exceptions
func TestAddIndexesAddIndexMissingCodec(t *testing.T) {
	toAdd := newDirectory()
	defer mustClose(t, toAdd)
	// Disable checkIndex, else we get an exception because
	// of the unregistered codec:
	t.Fatal(setCheckIndexOnCloseMissing)
}

// LUCENE-3575
func TestAddIndexesFieldNamesChanged(t *testing.T) {
	d1 := newDirectory()
	w := newRandomIndexWriter(t, d1)
	doc := document.NewDocument()
	doc.Add(newStringField(t, "f1", "doc1 field1", true))
	doc.Add(newStringField(t, "id", "1", true))
	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	r1, err := w.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	mustClose(t, w)

	d2 := newDirectory()
	w = newRandomIndexWriter(t, d2)
	doc = document.NewDocument()
	doc.Add(newStringField(t, "f2", "doc2 field2", true))
	doc.Add(newStringField(t, "id", "2", true))
	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	r2, err := w.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	mustClose(t, w)

	d3 := newDirectory()
	w = newRandomIndexWriter(t, d3)
	defer mustClose(t, r1, d1, r2, d2, w, d3)
	t.Fatal(testUtilAddIndexesSlowlyMissing)
}

func TestAddIndexesAddEmpty(t *testing.T) {
	d1 := newDirectory()
	w := newRandomIndexWriter(t, d1)
	defer mustClose(t, w, d1)
	// w.addIndexes(new CodecReader[0])
	t.Fatal(addIndexesCodecReadersMissing)
}

// Currently it's impossible to end up with a segment with all documents
// deleted, as such segments are dropped. Still, to validate that addIndexes
// works with such segments, or readers that end up in such state, we fake an
// all deleted segment.
func TestAddIndexesFakeAllDeleted(t *testing.T) {
	src := newDirectory()
	dest := newDirectory()
	w := newRandomIndexWriter(t, src)
	if _, err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	r, err := w.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	allDeletedReader := index.NewAllDeletedFilterReader(mustLeaves(t, r)[0].LeafReader())
	mustClose(t, w)

	w = newRandomIndexWriter(t, dest)
	defer mustClose(t, w, allDeletedReader, src, dest)
	// w.addIndexes(SlowCodecReaderWrapper.wrap(allDeletedReader))
	t.Fatal(addIndexesCodecReadersMissing)
}

// Make sure an open IndexWriter on an incoming Directory causes a LockObtainFailedException
func TestAddIndexesLocksBlock(t *testing.T) {
	src := newDirectory()
	w1 := newRandomIndexWriter(t, src)
	if _, err := w1.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if _, err := w1.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	dest := newDirectory()

	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w2 := newRandomIndexWriterWithConfig(t, dest, iwc)

	_, err := w2.AddIndexes(src)
	var lofe *store.LockObtainFailedException
	if !errors.As(err, &lofe) {
		t.Fatalf("expected LockObtainFailedException from addIndexes(src), got %v", err)
	}

	mustClose(t, w1, w2, src, dest)
}

// intSortedIndex renders the w1 prologue of the testIllegalIndexSortChange
// tests: an index sorted on the int field "foo", force-merged so the index
// sort is in fact burned into the index.
func intSortedIndex(t testing.TB) store.Directory {
	t.Helper()
	dir1 := newDirectory()
	iwc1 := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc1.SetIndexSort(index.NewSort(index.NewSortField("foo", index.SortTypeInt)))
	w1 := newRandomIndexWriterWithConfig(t, dir1, iwc1)
	for i := 0; i < 2; i++ {
		if _, err := w1.AddDocument(document.NewDocument()); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
		if _, err := w1.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
	// so the index sort is in fact burned into the index:
	if err := w1.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, w1)
	return dir1
}

func stringSortedWriter(t testing.TB, dir2 store.Directory) interface {
	AddIndexes(...store.Directory) (int64, error)
	Close() error
} {
	t.Helper()
	iwc2 := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc2.SetIndexSort(index.NewSort(index.NewSortField("foo", index.SortTypeString)))
	return newRandomIndexWriterWithConfig(t, dir2, iwc2)
}

func TestAddIndexesIllegalIndexSortChange1(t *testing.T) {
	dir1 := intSortedIndex(t)

	dir2 := newDirectory()
	w2 := stringSortedWriter(t, dir2)
	_, err := w2.AddIndexes(dir1)
	expectIAEMessage(t, err, `cannot change index sort from <int: "foo"> to <string: "foo">`)
	mustClose(t, dir1, w2, dir2)
}

func TestAddIndexesIllegalIndexSortChange2(t *testing.T) {
	dir1 := intSortedIndex(t)

	dir2 := newDirectory()
	w2 := stringSortedWriter(t, dir2)
	r1 := mustOpenDirectoryReader(t, dir1)
	defer mustClose(t, r1, dir1, w2, dir2)
	// w2.addIndexes((SegmentReader) getOnlyLeafReader(r1))
	t.Fatal(addIndexesCodecReadersMissing)
}

// softDeleteVersionIndex renders the w1 prologue of the
// testAddIndexesDVUpdate tests.
func softDeleteVersionIndex(t testing.TB) store.Directory {
	t.Helper()
	dir1 := newDirectory()
	iwc1 := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w1 := mustNewIndexWriter(t, dir1, iwc1)
	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "1", true))
	doc.Add(newStringFieldNoRandom(t, "version", "1", true))
	doc.Add(numericDVField(t, "soft_delete", 1))
	mustAddDocument(t, w1, doc)
	mustFlush(t, w1)

	mustUpdateDocValues(t, w1, index.NewTerm("id", "1"), numericDVField(t, "soft_delete", 1).Field)
	mustCommit(t, w1)
	mustClose(t, w1)
	return dir1
}

// reopenTwice renders the w3 epilogue of the testAddIndexesDVUpdate tests.
func reopenTwice(t testing.TB, dir2 store.Directory) {
	t.Helper()
	for i := 0; i < 2; i++ {
		iwc3 := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		mustClose(t, mustNewIndexWriter(t, dir2, iwc3))
	}
}

func TestAddIndexesAddIndexesDVUpdateSameSegmentName(t *testing.T) {
	dir1 := softDeleteVersionIndex(t)

	iwc2 := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	dir2 := newDirectory()
	w2 := mustNewIndexWriter(t, dir2, iwc2)
	mustAddIndexes(t, w2, dir1)
	mustCommit(t, w2)
	mustClose(t, w2)

	reopenTwice(t, dir2)
	mustClose(t, dir1, dir2)
}

func TestAddIndexesAddIndexesDVUpdateNewSegmentName(t *testing.T) {
	dir1 := softDeleteVersionIndex(t)

	iwc2 := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	dir2 := newDirectory()
	w2 := mustNewIndexWriter(t, dir2, iwc2)
	mustAddDocument(t, w2, document.NewDocument())
	mustCommit(t, w2)

	mustAddIndexes(t, w2, dir1)
	mustCommit(t, w2)
	mustClose(t, w2)

	reopenTwice(t, dir2)
	mustClose(t, dir1, dir2)
}

func TestAddIndexesAddIndicesWithSoftDeletes(t *testing.T) {
	dir1 := newDirectory()
	iwc1 := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc1.SetSoftDeletesField("soft_delete")
	writer := mustNewIndexWriter(t, dir1, iwc1)
	for i := 0; i < 30; i++ {
		docID := rand.Intn(5)
		doc := stringFieldStoredDoc(t, "id", strconv.Itoa(docID))
		if _, err := writer.SoftUpdateDocument(index.NewTerm("id", strconv.Itoa(docID)), doc,
			[]*document.Field{numericDVField(t, "soft_delete", 1).Field}); err != nil {
			t.Fatalf("softUpdateDocument: %v", err)
		}
		if rand.Intn(2) == 0 {
			mustFlush(t, writer)
		}
	}
	mustCommit(t, writer)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir1)
	// wrappedReader filters out soft deleted docs
	wrappedReader, err := index.NewSoftDeletesDirectoryReaderWrapper(reader, "soft_delete")
	if err != nil {
		t.Fatalf("new SoftDeletesDirectoryReaderWrapper: %v", err)
	}
	dir2 := newDirectory()
	numDocs := reader.NumDocs()
	maxDoc := reader.MaxDoc()
	if numDocs != maxDoc {
		t.Fatalf("assertEquals(numDocs, maxDoc): %d != %d", numDocs, maxDoc)
	}
	iwc1 = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc1.SetSoftDeletesField("soft_delete")
	writer = mustNewIndexWriter(t, dir2, iwc1)
	defer mustClose(t, reader, wrappedReader, writer, dir2, dir1)
	// writer.addIndexes(CodecReader[] readers)
	t.Fatal(addIndexesCodecReadersMissing)
}

func addValueBlocks(t testing.TB, dir store.Directory, hasBlocks bool) {
	t.Helper()
	writer := newRandomIndexWriter(t, dir)
	numBlocks := 1 + rand.Intn(9)
	for i := 0; i < numBlocks; i++ {
		numDocs := 1
		if hasBlocks {
			numDocs = 2 + rand.Intn(8)
		}
		docs := make([]*document.Document, 0, numDocs)
		for j := 0; j < numDocs; j++ {
			docs = append(docs, stringFieldStoredDoc(t, "value", strconv.Itoa(rand.Intn(5))))
		}
		if _, err := writer.AddDocuments(docs); err != nil {
			t.Fatalf("addDocuments: %v", err)
		}
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	mustClose(t, writer)
}

func TestAddIndexesAddIndicesWithBlocks(t *testing.T) {
	addHasBlocksPerm := []bool{true, true, false, false}
	baseHasBlocksPerm := []bool{true, false, true, false}
	for perm := range addHasBlocksPerm {
		addHasBlocks := addHasBlocksPerm[perm]
		baseHasBlocks := baseHasBlocksPerm[perm]
		dir := newDirectory()
		addValueBlocks(t, dir, baseHasBlocks)

		addDir := newDirectory()
		addValueBlocks(t, addDir, addHasBlocks)

		writer := mustNewIndexWriter(t, dir, newIndexWriterConfig())
		if rand.Intn(2) == 0 {
			mustAddIndexes(t, writer, addDir)
		} else {
			mustClose(t, writer, addDir, dir)
			// writer.addIndexes(CodecReader[] readers)
			t.Fatal(addIndexesCodecReadersMissing)
		}
		if _, err := writer.ForceMergeWithObserver(1, true); err != nil {
			t.Fatalf("forceMerge(1, true): %v", err)
		}
		mustClose(t, writer)

		reader := mustOpenDirectoryReader(t, dir)
		leaves := mustLeaves(t, reader)
		codecReader := leafSegmentReader(t, leaves[0])
		if len(leaves) != 1 {
			t.Fatalf("reader.leaves().size(): expected 1, got %d", len(leaves))
		}
		hasBlocks := codecReader.GetSegmentCommitInfo().SegmentInfo().GetHasBlocks()
		if (addHasBlocks || baseHasBlocks) != hasBlocks {
			t.Fatalf("addHasBlocks: %v baseHasBlocks: %v: getHasBlocks() = %v", addHasBlocks, baseHasBlocks, hasBlocks)
		}
		mustClose(t, reader, addDir, dir)
	}
}

func TestAddIndexesSetDiagnostics(t *testing.T) {
	// The merge policy's findMerges(CodecReader...) wraps each merge in an
	// anonymous OneMerge subclass overriding setMergeInfo(SegmentCommitInfo).
	t.Fatal(oneMergeOverrideMissing)
}

// parentDocBlocks renders the w1 prologue of testIllegalParentDocChange.
func parentDocBlocks(t testing.TB, parentField string) store.Directory {
	t.Helper()
	dir1 := newDirectory()
	iwc1 := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc1.SetParentField(parentField)
	w1 := newRandomIndexWriterWithConfig(t, dir1, iwc1)
	parent := document.NewDocument()
	for i := 0; i < 2; i++ {
		if _, err := w1.AddDocuments([]*document.Document{document.NewDocument(), document.NewDocument(), parent}); err != nil {
			t.Fatalf("addDocuments: %v", err)
		}
		if _, err := w1.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
	// so the index sort is in fact burned into the index:
	if err := w1.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, w1)
	return dir1
}

func TestAddIndexesIllegalParentDocChange(t *testing.T) {
	dir1 := parentDocBlocks(t, "foobar")

	dir2 := newDirectory()
	iwc2 := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc2.SetParentField("foo")
	w2 := newRandomIndexWriterWithConfig(t, dir2, iwc2)

	r1 := mustOpenDirectoryReader(t, dir1)
	defer mustClose(t, r1, dir1, w2, dir2)
	// w2.addIndexes((SegmentReader) getOnlyLeafReader(r1))
	t.Fatal(addIndexesCodecReadersMissing)
}

func TestAddIndexesIllegalNonParentField(t *testing.T) {
	dir1 := newDirectory()
	iwc1 := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w1 := newRandomIndexWriterWithConfig(t, dir1, iwc1)
	parent := document.NewDocument()
	parent.Add(newStringFieldNoRandom(t, "foo", "XXX", false))
	if _, err := w1.AddDocument(parent); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	mustClose(t, w1)

	dir2 := newDirectory()
	iwc2 := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc2.SetParentField("foo")
	w2 := newRandomIndexWriterWithConfig(t, dir2, iwc2)

	r1 := mustOpenDirectoryReader(t, dir1)
	defer mustClose(t, r1, dir1, w2, dir2)
	// w2.addIndexes((SegmentReader) getOnlyLeafReader(r1))
	t.Fatal(addIndexesCodecReadersMissing)
}
