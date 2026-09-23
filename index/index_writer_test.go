// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriter.java
// (Apache Lucene 10.5.0). The @Ignore'd testMassiveField is rendered by the
// unexported testIndexWriterMassiveField, which no test entry point runs.

package index_test

import (
	"errors"
	"io"
	"maps"
	"math"
	"math/rand"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Missing members the Java tests reach.
const (
	indexWriterFlushBoolBoolMissing         = "org.apache.lucene.index.IndexWriter#flush(boolean, boolean) is not ported"
	testUtilSyncConcurrentMergesMissing     = "TestUtil.syncConcurrentMerges(IndexWriter) is not ported"
	indexWriterFlushHooksOverrideMissing    = "overriding org.apache.lucene.index.IndexWriter#doBeforeFlush()/doAfterFlush() in an IndexWriter subclass is not ported"
	testUtilAddIndexesSlowlyMissing         = "TestUtil.addIndexesSlowly(IndexWriter, DirectoryReader...) is not ported"
	suppressingCMSMissing                   = "org.apache.lucene.tests.index.SuppressingConcurrentMergeScheduler is not ported"
	windowsFSMissing                        = "org.apache.lucene.tests.mockfile.WindowsFS is not ported"
	indexWriterHasUncommittedChangesMissing = "org.apache.lucene.index.IndexWriter#hasUncommittedChanges() is not ported"
	indexWriterAddDocumentIterableMissing   = "org.apache.lucene.index.IndexWriter#addDocument(Iterable<? extends IndexableField>) is not ported " +
		"(AddDocument takes a *document.Document)"
	indexWriterAddDocumentsIterableMissing = "org.apache.lucene.index.IndexWriter#addDocuments(Iterable<? extends Iterable<? extends IndexableField>>) " +
		"over a lazily failing Iterator is not ported (AddDocuments takes a []*document.Document)"
	indexWriterCloneSegmentInfosMissing    = "org.apache.lucene.index.IndexWriter#cloneSegmentInfos() is not ported"
	segmentInfosGetIDMissing               = "org.apache.lucene.index.SegmentInfos#getId() is not ported"
	indexWriterGetFieldNamesMissing        = "org.apache.lucene.index.IndexWriter#getFieldNames() is not ported"
	indexWriterSegmentInfosCounterMissing  = "org.apache.lucene.index.IndexWriter#advanceSegmentInfosCounter(long) and #getSegmentInfosCounter() are not ported"
	indexWriterNumRAMDocsMissing           = "org.apache.lucene.index.IndexWriter#numRamDocs() is not ported"
	indexWriterGetFlushDeletesCountMissing = "org.apache.lucene.index.IndexWriter#getFlushDeletesCount() is not ported"
	indexWriterEventQueueMissing           = "org.apache.lucene.index.IndexWriter.EventQueue is not ported: Gocene's eventQueue is unexported, " +
		"its add returns false instead of throwing AlreadyClosedException and its processEvents takes the writer"
	segmentReaderGetOriginalSegmentInfoMissing = "org.apache.lucene.index.SegmentReader#getOriginalSegmentInfo() is not ported"
	oneMergeOverrideMissing                    = "overriding org.apache.lucene.index.MergePolicy.OneMerge methods in an anonymous subclass is not ported " +
		"(OneMerge is a struct; its wrapForMerge/onMergeComplete/mergeFinished are not dispatched)"
	documentsWriterAccessMissing = "org.apache.lucene.index.IndexWriter#docWriter (DocumentsWriter.flushControl/perThreadPool/deleteQueue) " +
		"is not exposed to the index tests"
	indexWriterMaxStoredStringLengthMissing = "org.apache.lucene.index.IndexWriter#MAX_STORED_STRING_LENGTH is not ported"
	testUtilRandomSimpleStringMissing       = "TestUtil.randomSimpleString(Random) is not ported"
	testUtilRandomAnalysisStringMissing     = "TestUtil.randomAnalysisString(Random, int, boolean) is not ported"
	testUtilRandomBinaryTermMissing         = "TestUtil.randomBinaryTerm(Random) is not ported"
	testUtilGetDefaultCodecMissing          = "TestUtil.getDefaultCodec() is not ported"
	fsDirectoryPendingDeletionsMissing      = "org.apache.lucene.store.FSDirectory#getPendingDeletions() and #deletePendingFiles() are not ported"
	checkIndexPrintStreamMissing            = "org.apache.lucene.index.CheckIndex#setInfoStream(PrintStream, boolean) is not ported"
	extrasFSMissing                         = "org.apache.lucene.tests.mockfile.ExtrasFS is not ported"
)

// testIndexWriterAddDocWithField renders the private static
// addDocWithField(IndexWriter, String).
func testIndexWriterAddDocWithField(t testing.TB, writer *index.IndexWriter, field string) {
	t.Helper()
	doc := document.NewDocument()
	doc.Add(newField(t, field, "value", testIndexWriterStoredTextType))
	mustAddDocument(t, writer, doc)
}

// testIndexWriterGetSegmentCount renders the private static
// getSegmentCount(Directory): how many unique segment names are in the
// directory.
func testIndexWriterGetSegmentCount(t testing.TB, dir store.Directory) int {
	t.Helper()
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	segments := make(map[string]struct{})
	for _, file := range files {
		segments[index.ParseSegmentName(file)] = struct{}{}
	}
	return len(segments)
}

func mustNewIndexWriterConfigOpenMode(mode index.OpenMode) *index.IndexWriterConfig {
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetOpenMode(mode)
	return conf
}

func textStoredWithVectors(positions, offsets bool) *document.FieldType {
	ft := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorPositions(positions)
	ft.SetStoreTermVectorOffsets(offsets)
	return ft
}

func TestIndexWriterDocCount(t *testing.T) {
	dir := newDirectory()

	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))

	// add 100 documents
	for i := 0; i < 100; i++ {
		testIndexWriterAddDocWithIndex(t, writer, i)
		if rand.Intn(2) == 0 {
			mustCommit(t, writer)
		}
	}
	assertWriterDocStats(t, writer, 100, 100)
	mustClose(t, writer)

	// delete 40 documents
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(newKeepFullyDeletedSegmentsPolicy())
	writer = mustNewIndexWriter(t, dir, conf)

	for i := 0; i < 40; i++ {
		mustDeleteTerm(t, writer, "id", strconv.Itoa(i))
		if rand.Intn(2) == 0 {
			mustCommit(t, writer)
		}
	}
	mustFlush(t, writer)
	assertWriterDocStats(t, writer, 100, 60)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 60, reader)
	mustClose(t, reader)

	// merge the index down and check that the new doc count is correct
	writer = mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	if n := iwDocStats(t, writer).NumDocs; n != 60 {
		t.Fatalf("writer.getDocStats().numDocs: expected 60, got %d", n)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	assertWriterDocStats(t, writer, 60, 60)
	mustClose(t, writer)

	// check that the index reader gives the same numbers.
	reader = mustOpenDirectoryReader(t, dir)
	if reader.MaxDoc() != 60 || reader.NumDocs() != 60 {
		t.Fatalf("maxDoc/numDocs: expected 60/60, got %d/%d", reader.MaxDoc(), reader.NumDocs())
	}
	mustClose(t, reader)

	// make sure opening a new index for create over this existing one works correctly:
	writer = mustNewIndexWriter(t, dir, mustNewIndexWriterConfigOpenMode(index.Create))
	assertWriterDocStats(t, writer, 0, 0)
	mustClose(t, writer, dir)
}

// Make sure we can open an index for create even when a
// reader holds it open (this fails pre lock-less commits on Windows):
func TestIndexWriterCreateWithReader(t *testing.T) {
	dir := newDirectory()

	// add one document & close writer
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	testIndexWriterAddDoc(t, writer)
	mustClose(t, writer)

	// now open reader:
	reader := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 1, reader)

	// now open index for create:
	writer = mustNewIndexWriter(t, dir, mustNewIndexWriterConfigOpenMode(index.Create))
	if n := iwDocStats(t, writer).MaxDoc; n != 0 {
		t.Fatalf("should be zero documents: maxDoc=%d", n)
	}
	testIndexWriterAddDoc(t, writer)
	mustClose(t, writer)

	assertReaderNumDocs(t, 1, reader)
	reader2 := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 1, reader2)
	mustClose(t, reader, reader2, dir)
}

func TestIndexWriterChangesAfterClose(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))

	testIndexWriterAddDoc(t, writer)

	// close
	mustClose(t, writer)
	doc := document.NewDocument()
	doc.Add(newTextField(t, "content", "aaa", false))
	_, err := writer.AddDocument(doc)
	dwdqExpectAlreadyClosed(t, err)

	mustClose(t, dir)
}

func TestIndexWriterIndexNoDocuments(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustCommit(t, writer)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	if reader.MaxDoc() != 0 || reader.NumDocs() != 0 {
		t.Fatalf("maxDoc/numDocs: expected 0/0, got %d/%d", reader.MaxDoc(), reader.NumDocs())
	}
	mustClose(t, reader)

	writer = mustNewIndexWriter(t, dir, mustNewIndexWriterConfigOpenMode(index.Append))
	mustCommit(t, writer)
	mustClose(t, writer)

	reader = mustOpenDirectoryReader(t, dir)
	if reader.MaxDoc() != 0 || reader.NumDocs() != 0 {
		t.Fatalf("maxDoc/numDocs: expected 0/0, got %d/%d", reader.MaxDoc(), reader.NumDocs())
	}
	mustClose(t, reader, dir)
}

func TestIndexWriterSmallRAMBuffer(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetRAMBufferSizeMB(0.000001)
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(10))
	writer := mustNewIndexWriter(t, dir, conf)
	lastNumSegments := testIndexWriterGetSegmentCount(t, dir)
	for j := 0; j < 9; j++ {
		doc := document.NewDocument()
		doc.Add(newField(t, "field", "aaa"+strconv.Itoa(j), testIndexWriterStoredTextType))
		mustAddDocument(t, writer, doc)
		// Verify that with a tiny RAM buffer we see new segment after every doc
		numSegments := testIndexWriterGetSegmentCount(t, dir)
		if !(numSegments > lastNumSegments) {
			t.Fatalf("assertTrue(numSegments > lastNumSegments): %d <= %d at j=%d", numSegments, lastNumSegments, j)
		}
		lastNumSegments = numSegments
	}
	mustClose(t, writer, dir)
}

// Make sure it's OK to change RAM buffer size and maxBufferedDocs in a write session
func TestIndexWriterChangingRAMBuffer(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	defer mustClose(t, writer, dir)
	writer.GetConfig().SetMaxBufferedDocs(10)
	writer.GetConfig().SetRAMBufferSizeMB(index.DisableAutoFlush)

	doc := document.NewDocument()
	doc.Add(mustNewFieldNoRandom(t, "field", "aaa1", testIndexWriterStoredTextType))
	mustAddDocument(t, writer, doc)
	t.Fatal(testUtilSyncConcurrentMergesMissing)
}

func TestIndexWriterEnablingNorms(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(10)
	writer := mustNewIndexWriter(t, dir, conf)
	// Enable norms for only 1 doc, pre flush
	customType := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	customType.SetOmitNorms(true)
	for j := 0; j < 10; j++ {
		doc := document.NewDocument()
		var f *document.Field
		if j != 8 {
			f = newField(t, "field", "aaa", customType)
		} else {
			f = newField(t, "field", "aaa", testIndexWriterStoredTextType)
		}
		doc.Add(f)
		mustAddDocument(t, writer, doc)
	}
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader, dir)
	newSearcher(t, reader)
}

func TestIndexWriterHighFreqTerm(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetRAMBufferSizeMB(0.01)
	writer := mustNewIndexWriter(t, dir, conf)
	// Massive doc that has 128 K a's
	var b strings.Builder
	b.Grow(1024 * 1024)
	for i := 0; i < 4096; i++ {
		b.WriteString(" a a a a a a a a")
		b.WriteString(" a a a a a a a a")
		b.WriteString(" a a a a a a a a")
		b.WriteString(" a a a a a a a a")
	}
	doc := document.NewDocument()
	doc.Add(newField(t, "field", b.String(), textStoredWithVectors(true, true)))
	mustAddDocument(t, writer, doc)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader, dir)
	if reader.MaxDoc() != 1 || reader.NumDocs() != 1 {
		t.Fatalf("maxDoc/numDocs: expected 1/1, got %d/%d", reader.MaxDoc(), reader.NumDocs())
	}
	// assertEquals(1, reader.docFreq(new Term("field", "a")))
	t.Fatal(indexReaderDocFreqMissing)
}

func TestIndexWriterFlushWithNoMerging(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(10))
	writer := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, writer, dir)
	doc := document.NewDocument()
	doc.Add(newField(t, "field", "aaa", textStoredWithVectors(true, true)))
	for i := 0; i < 19; i++ {
		mustAddDocument(t, writer, doc)
	}
	t.Fatal(indexWriterFlushBoolBoolMissing)
}

// Make sure we can flush segment w/ norms, then add empty doc (no norms) and flush
func TestIndexWriterEmptyDocAfterFlushingRealDoc(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	doc.Add(newField(t, "field", "aaa", textStoredWithVectors(true, true)))
	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)
	mustAddDocument(t, writer, document.NewDocument())
	mustClose(t, writer)
	reader := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 2, reader)
	mustClose(t, reader, dir)
}

// Test that no NullPointerException will be raised, when adding one document
// with a single, empty field and term vectors enabled.
func TestIndexWriterBadSegment(t *testing.T) {
	dir := newDirectory()
	iw := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))

	doc := document.NewDocument()
	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetStoreTermVectors(true)
	doc.Add(newField(t, "tvtest", "", customType))
	mustAddDocument(t, iw, doc)
	mustClose(t, iw, dir)
}

// LUCENE-1036. Java raises the indexing thread to Thread.MAX_PRIORITY; Go
// has no goroutine priority, so the rendering indexes at the only priority
// there is.
func TestIndexWriterMaxThreadPriority(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	conf.SetMergePolicy(newLogMergePolicy())
	conf.GetMergePolicy().(logMergePolicy).SetMergeFactor(2)
	iw := mustNewIndexWriter(t, dir, conf)
	doc := document.NewDocument()
	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetStoreTermVectors(true)
	doc.Add(newField(t, "tvtest", "a b c", customType))
	for i := 0; i < 4; i++ {
		mustAddDocument(t, iw, doc)
	}
	mustClose(t, iw, dir)
}

func TestIndexWriterVariableSchema(t *testing.T) {
	dir := newDirectory()
	for i := 0; i < 20; i++ {
		conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetMaxBufferedDocs(2)
		conf.SetMergePolicy(newLogMergePolicy())
		writer := mustNewIndexWriter(t, dir, conf)
		doc := document.NewDocument()
		contents := "aa bb cc dd ee ff gg hh ii jj kk"

		customType := document.NewFieldTypeFrom(document.TextFieldTypeStored)
		if i == 7 {
			// Add empty docs here
			doc.Add(newTextField(t, "content3", "", false))
		} else {
			var typ *document.FieldType
			if i%2 == 0 {
				doc.Add(newField(t, "content4", contents, customType))
				typ = customType
			} else {
				typ = document.TextFieldTypeNotStored
			}
			doc.Add(newTextField(t, "content1", contents, false))
			doc.Add(newField(t, "content3", "", customType))
			doc.Add(newField(t, "content5", "", typ))
		}

		for j := 0; j < 4; j++ {
			mustAddDocument(t, writer, doc)
		}

		mustClose(t, writer)

		if i%4 == 0 {
			writer = mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
			if err := writer.ForceMerge(1); err != nil {
				t.Fatalf("forceMerge: %v", err)
			}
			mustClose(t, writer)
		}
	}
	mustClose(t, dir)
}

// LUCENE-1084: test unlimited field length
func TestIndexWriterUnlimitedMaxFieldLength(t *testing.T) {
	dir := newDirectory()

	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))

	doc := document.NewDocument()
	doc.Add(newTextField(t, "field", strings.Repeat(" a", 10_000)+" x", false))
	mustAddDocument(t, writer, doc)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader, dir)
	// assertEquals(1, reader.docFreq(new Term("field", "x")))
	t.Fatal(indexReaderDocFreqMissing)
}

// LUCENE-1179
func TestIndexWriterEmptyFieldName(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	doc.Add(newTextField(t, "", "a b c", false))
	mustAddDocument(t, writer, doc)
	mustClose(t, writer, dir)
}

// assertNextTermBytes renders assertEquals(newBytesRef(expected), te.next()).
func assertNextTermBytes(t testing.TB, te index.TermsEnum, expected string) {
	t.Helper()
	term, err := te.Next()
	if err != nil {
		t.Fatalf("te.next(): %v", err)
	}
	if term == nil {
		t.Fatalf("te.next(): expected %q, got null", expected)
	}
	if got := term.Text(); got != expected {
		t.Fatalf("te.next(): expected %q, got %q", expected, got)
	}
}

func assertNoNextTerm(t testing.TB, te index.TermsEnum) {
	t.Helper()
	term, err := te.Next()
	if err != nil {
		t.Fatalf("te.next(): %v", err)
	}
	if term != nil {
		t.Fatalf("assertNull(te.next()): got %q", term.Text())
	}
}

// onlyLeafTermsEnum renders getOnlyLeafReader(reader).terms(field).iterator().
func onlyLeafTermsEnum(t testing.TB, reader *index.DirectoryReader, field string) index.TermsEnum {
	t.Helper()
	terms, err := getOnlyLeafReader(t, reader).Terms(field)
	if err != nil {
		t.Fatalf("terms(%q): %v", field, err)
	}
	if terms == nil {
		t.Fatalf("terms(%q) is null", field)
	}
	te, err := terms.Iterator()
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	return te
}

func TestIndexWriterEmptyFieldNameTerms(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	doc.Add(newTextField(t, "", "a b c", false))
	mustAddDocument(t, writer, doc)
	mustClose(t, writer)
	reader := mustOpenDirectoryReader(t, dir)
	te := onlyLeafTermsEnum(t, reader, "")
	assertNextTermBytes(t, te, "a")
	assertNextTermBytes(t, te, "b")
	assertNextTermBytes(t, te, "c")
	assertNoNextTerm(t, te)
	mustClose(t, reader, dir)
}

func TestIndexWriterEmptyFieldNameWithEmptyTerm(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	doc.Add(newStringField(t, "", "", false))
	doc.Add(newStringField(t, "", "a", false))
	doc.Add(newStringField(t, "", "b", false))
	doc.Add(newStringField(t, "", "c", false))
	mustAddDocument(t, writer, doc)
	mustClose(t, writer)
	reader := mustOpenDirectoryReader(t, dir)
	te := onlyLeafTermsEnum(t, reader, "")
	assertNextTermBytes(t, te, "")
	assertNextTermBytes(t, te, "a")
	assertNextTermBytes(t, te, "b")
	assertNextTermBytes(t, te, "c")
	assertNoNextTerm(t, te)
	mustClose(t, reader, dir)
}

// LUCENE-1222
func TestIndexWriterDoBeforeAfterFlush(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	// MockIndexWriter overrides doAfterFlush() and doBeforeFlush().
	t.Fatal(indexWriterFlushHooksOverrideMissing)
}

// negativePositionsTokenStream renders the anonymous TokenStream of
// testNegativePositions.
type negativePositionsTokenStream struct {
	*analysis.BaseTokenStream
	termAtt    analysis.CharTermAttribute
	posIncrAtt tokenattributes.PositionIncrementAttribute
	terms      []string
	first      bool
}

func newNegativePositionsTokenStream() *negativePositionsTokenStream {
	s := &negativePositionsTokenStream{BaseTokenStream: analysis.NewBaseTokenStream(), terms: []string{"a", "b", "c"}, first: true}
	s.termAtt = s.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	s.posIncrAtt = s.AddAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	return s
}

func (s *negativePositionsTokenStream) IncrementToken() (bool, error) {
	if len(s.terms) == 0 {
		return false, nil
	}
	s.ClearAttributes()
	s.termAtt.AppendString(s.terms[0])
	s.terms = s.terms[1:]
	if s.first {
		s.posIncrAtt.SetPositionIncrement(0)
	} else {
		s.posIncrAtt.SetPositionIncrement(1)
	}
	s.first = false
	return true, nil
}

// LUCENE-1255
func TestIndexWriterNegativePositions(t *testing.T) {
	tokens := newNegativePositionsTokenStream()

	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	f, err := document.NewField("field", tokens, document.TextFieldTypeNotStored)
	if err != nil {
		t.Fatalf("new TextField: %v", err)
	}
	doc.Add(f)
	if _, err := w.AddDocument(doc); err == nil {
		t.Fatal("expected IllegalArgumentException from addDocument")
	}

	mustClose(t, w, dir)
}

// LUCENE-2529
func TestIndexWriterPositionIncrementGapEmptyField(t *testing.T) {
	dir := newDirectory()
	analyzer := testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true)
	analyzer.SetPositionIncrementGap(100)
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(analyzer))
	doc := document.NewDocument()
	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetStoreTermVectors(true)
	customType.SetStoreTermVectorPositions(true)
	f := newField(t, "field", "", customType)
	f2 := newField(t, "field", "crunch man", customType)
	doc.Add(f)
	doc.Add(f2)
	mustAddDocument(t, w, doc)
	mustClose(t, w)

	r := mustOpenDirectoryReader(t, dir)
	termsEnum := fieldVectorTermsEnum(t, r)
	mustNextTerm(t, termsEnum)
	dpEnum := mustPostingsAll(t, termsEnum)
	if dpEnum == nil {
		t.Fatal("assertNotNull(dpEnum)")
	}
	assertHasDoc(t, dpEnum)
	assertFreqAndNextPosition(t, dpEnum, 1, 100)

	mustNextTerm(t, termsEnum)
	dpEnum = mustPostingsAll(t, termsEnum)
	if dpEnum == nil {
		t.Fatal("assertNotNull(dpEnum)")
	}
	assertHasDoc(t, dpEnum)
	assertFreqAndNextPosition(t, dpEnum, 1, 101)
	assertNoNextTerm(t, termsEnum)

	mustClose(t, r, dir)
}

func assertFreqAndNextPosition(t testing.TB, dpEnum index.PostingsEnum, freq, position int) {
	t.Helper()
	if got, err := dpEnum.Freq(); err != nil || got != freq {
		t.Fatalf("dpEnum.freq(): expected %d, got %d (%v)", freq, got, err)
	}
	if got, err := dpEnum.NextPosition(); err != nil || got != position {
		t.Fatalf("dpEnum.nextPosition(): expected %d, got %d (%v)", position, got, err)
	}
}

func TestIndexWriterDeadlock(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	writer := mustNewIndexWriter(t, dir, conf)
	doc := document.NewDocument()

	doc.Add(newField(t, "content", "aaa bbb ccc ddd eee fff ggg hhh iii", textStoredWithVectors(true, true)))
	mustAddDocument(t, writer, doc)
	mustAddDocument(t, writer, doc)
	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)
	// index has 2 segments

	dir2 := newDirectory()
	writer2 := mustNewIndexWriter(t, dir2, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustAddDocument(t, writer2, doc)
	mustClose(t, writer2)

	r1 := mustOpenDirectoryReader(t, dir2)
	defer mustClose(t, writer, r1, dir2, dir)
	t.Fatal(testUtilAddIndexesSlowlyMissing)
}

func TestIndexWriterThreadInterruptDeadlock(t *testing.T) {
	// new IndexerThreadInterrupt(1): newIndexWriterConfig's merge scheduler
	// is a ConcurrentMergeScheduler, which the constructor replaces with a
	// SuppressingConcurrentMergeScheduler subclass before indexing into its
	// addIndexes source directory.
	adder := newDirectory()
	defer mustClose(t, adder)
	t.Fatal(suppressingCMSMissing)
}

const (
	indexWriterWriteLockNameMissing = "org.apache.lucene.index.IndexWriter#WRITE_LOCK_NAME is not ported"
	fsDirectoryLockFactoryMissing   = "LuceneTestCase.newFSDirectory(Path, LockFactory) is not ported (no FSDirectory constructor takes a LockFactory)"
)

// indexStoreCombosField renders the anonymous Field subclasses of
// testIndexStoreCombos whose tokenStream(Analyzer, TokenStream) returns a
// fixed MockTokenizer.
type indexStoreCombosField struct {
	*document.Field
	stream analysis.TokenStream
}

func (f *indexStoreCombosField) TokenStream(analysis.Analyzer, analysis.TokenStream) analysis.TokenStream {
	return f.stream
}

func assertTermHasDoc(t testing.TB, r index.IndexReader, field, text string) {
	t.Helper()
	pe, err := testUtilDocsForTerm(r, field, text, index.PostingsFlagNone)
	if err != nil {
		t.Fatalf("TestUtil.docs(%s:%s): %v", field, text, err)
	}
	if pe == nil {
		t.Fatalf("TestUtil.docs(%s:%s) is null", field, text)
	}
	if doc, err := pe.NextDoc(); err != nil || doc == spi.NO_MORE_DOCS {
		t.Fatalf("TestUtil.docs(%s:%s).nextDoc() == NO_MORE_DOCS (%v)", field, text, err)
	}
}

func TestIndexWriterIndexStoreCombos(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	b := make([]byte, 50)
	for i := 0; i < 50; i++ {
		b[i] = byte(i + 77)
	}

	doc := document.NewDocument()

	customType := document.NewFieldTypeFrom(document.StoredFieldType)
	customType.SetTokenized(true)

	field1 := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, false, testanalysis.DefaultMaxTokenLength)
	// new Field("binary", b, 10, 17, customType): the (array, offset, length)
	// triple is the Go slice b[10:27].
	fBase, err := document.NewField("binary", b[10:27], customType)
	if err != nil {
		t.Fatalf("new Field(binary): %v", err)
	}
	f := &indexStoreCombosField{Field: fBase, stream: field1}
	// TODO: this is evil, changing the type after creating the field:
	customType.SetIndexOptions(index.IndexOptionsDocs)
	field1.SetReader(strings.NewReader("doc1field1"))

	customType2 := document.NewFieldTypeFrom(document.TextFieldTypeStored)

	field2 := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, false, testanalysis.DefaultMaxTokenLength)
	f2Base, err := document.NewField("string", "value", customType2)
	if err != nil {
		t.Fatalf("new Field(string): %v", err)
	}
	f2 := &indexStoreCombosField{Field: f2Base, stream: field2}

	field2.SetReader(strings.NewReader("doc1field2"))
	doc.Add(f)
	doc.Add(f2)
	mustAddDocument(t, w, doc)

	// add 2 docs to test in-memory merging
	field1.SetReader(strings.NewReader("doc2field1"))
	field2.SetReader(strings.NewReader("doc2field2"))
	mustAddDocument(t, w, doc)

	// force segment flush so we can force a segment merge with doc3 later.
	mustCommit(t, w)

	field1.SetReader(strings.NewReader("doc3field1"))
	field2.SetReader(strings.NewReader("doc3field2"))

	mustAddDocument(t, w, doc)
	mustCommit(t, w)
	if err := w.ForceMerge(1); err != nil { // force segment merge.
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, w)

	ir := mustOpenDirectoryReader(t, dir)
	storedFields, err := ir.StoredFields()
	if err != nil {
		t.Fatalf("storedFields: %v", err)
	}
	doc2 := storedDocument(t, storedFields, 0)
	f3 := doc2.Get("binary")
	if f3 == nil {
		t.Fatal("doc2.getField(\"binary\") is null")
	}
	b = f3.BinaryValue()
	if b == nil {
		t.Fatal("assertNotNull(b)")
	}
	if len(b) != 17 {
		t.Fatalf("b.length: expected 17, got %d", len(b))
	}
	if b[0] != 87 {
		t.Fatalf("b[0]: expected 87, got %d", b[0])
	}

	for i := 0; i < 3; i++ {
		bf := storedDocument(t, storedFields, i).Get("binary")
		if bf == nil || bf.BinaryValue() == nil {
			t.Fatalf("assertNotNull(storedFields.document(%d).getField(\"binary\").binaryValue())", i)
		}
	}

	for i := 0; i < 3; i++ {
		if v := docGet(storedDocument(t, storedFields, i), "string"); v == nil || *v != "value" {
			t.Fatalf("storedFields.document(%d).get(\"string\"): expected \"value\", got %v", i, v)
		}
	}

	// test that the terms were indexed.
	assertTermHasDoc(t, ir, "binary", "doc1field1")
	assertTermHasDoc(t, ir, "binary", "doc2field1")
	assertTermHasDoc(t, ir, "binary", "doc3field1")
	assertTermHasDoc(t, ir, "string", "doc1field2")
	assertTermHasDoc(t, ir, "string", "doc2field2")
	assertTermHasDoc(t, ir, "string", "doc3field2")

	mustClose(t, ir, dir)
}

func TestIndexWriterNoDocsIndex(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustAddDocument(t, writer, document.NewDocument())
	mustClose(t, writer, dir)
}

func TestIndexWriterDeleteUnusedFiles(t *testing.T) {
	// relies on Windows semantics
	t.Fatal(windowsFSMissing)
}

func TestIndexWriterDeleteUnusedFiles2(t *testing.T) {
	// Validates that iw.deleteUnusedFiles() also deletes unused index commits
	// in case a deletion policy which holds onto commits is used.
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetIndexDeletionPolicy(index.NewSnapshotDeletionPolicy(index.NewKeepOnlyLastCommitDeletionPolicy()))
	writer := mustNewIndexWriter(t, dir, conf)
	sdp := writer.GetConfig().GetIndexDeletionPolicy().(*index.SnapshotDeletionPolicy)

	// First commit
	doc := document.NewDocument()
	customType := textStoredWithVectors(true, true)
	doc.Add(newField(t, "c", "val", customType))
	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)
	if n := len(mustListCommits(t, dir)); n != 1 {
		t.Fatalf("listCommits(dir).size(): expected 1, got %d", n)
	}

	// Keep that commit
	id := mustSnapshot(t, sdp)

	// Second commit - now KeepOnlyLastCommit cannot delete the prev commit.
	doc = document.NewDocument()
	doc.Add(newField(t, "c", "val", customType))
	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)
	if n := len(mustListCommits(t, dir)); n != 2 {
		t.Fatalf("listCommits(dir).size(): expected 2, got %d", n)
	}

	// Should delete the unreferenced commit
	mustRelease(t, sdp, id)
	defer mustClose(t, writer, dir)
	t.Fatal(indexWriterDeleteUnusedFilesMissing)
}

func TestIndexWriterEmptyFSDirWithNoLock(t *testing.T) {
	// Tests that if FSDir is opened w/ a NoLockFactory (or SingleInstanceLF),
	// then IndexWriter ctor succeeds. Previously (LUCENE-2386) it failed
	// when listAll() was called in IndexFileDeleter.
	t.Fatal(fsDirectoryLockFactoryMissing)
}

// codecFilePatternMatches renders
// IndexFileNames.CODEC_FILE_PATTERN.matcher(file).matches(): the whole name
// must match.
func codecFilePatternMatches(file string) bool {
	loc := index.CodecFilePattern.FindStringIndex(file)
	return loc != nil && loc[0] == 0 && loc[1] == len(file)
}

func TestIndexWriterEmptyDirRollback(t *testing.T) {
	// Tests that if IW is created over an empty Directory, some documents are
	// indexed, flushed (but not committed) and then IW rolls back, then no
	// files are left in the Directory.
	dir := newDirectory()

	origFiles, err := dir.ListAll()
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	conf.SetMergePolicy(newLogMergePolicy())
	conf.SetUseCompoundFile(false)
	writer := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, writer, dir)
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}

	// Creating over empty dir should not create any files, or, at most the write.lock file
	extraFileCount := len(files) - len(origFiles)
	if extraFileCount == 1 {
		t.Fatal(indexWriterWriteLockNameMissing)
	}
	sort.Strings(origFiles)
	sort.Strings(files)
	if !slices.Equal(origFiles, files) {
		t.Fatalf("assertArrayEquals(origFiles, files): %v != %v", origFiles, files)
	}

	doc := document.NewDocument()
	// create as many files as possible
	doc.Add(newField(t, "c", "val", textStoredWithVectors(true, true)))
	mustAddDocument(t, writer, doc)
	// Adding just one document does not call flush yet; counting the files
	// compares each name with IndexWriter.WRITE_LOCK_NAME.
	t.Fatal(indexWriterWriteLockNameMissing)
}

func TestIndexWriterNoUnwantedTVFiles(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetRAMBufferSizeMB(0.01)
	conf.SetMergePolicy(newLogMergePolicy())
	indexWriter := mustNewIndexWriter(t, dir, conf)
	indexWriter.GetConfig().GetMergePolicy().(logMergePolicy).SetNoCFSRatio(0.0)

	big := "alskjhlaksjghlaksjfhalksvjepgjioefgjnsdfjgefgjhelkgjhqewlrkhgwlekgrhwelkgjhwelkgrhwlkejg"
	big = big + big + big + big

	customType := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	customType.SetOmitNorms(true)
	customType2 := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	customType2.SetTokenized(false)
	customType3 := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	customType3.SetTokenized(false)
	customType3.SetOmitNorms(true)

	for i := 0; i < 2; i++ {
		doc := document.NewDocument()
		doc.Add(mustNewFieldNoRandom(t, "id", strconv.Itoa(i)+big, customType3))
		doc.Add(mustNewFieldNoRandom(t, "str", strconv.Itoa(i)+big, customType2))
		doc.Add(mustNewFieldNoRandom(t, "str2", strconv.Itoa(i)+big, testIndexWriterStoredTextType))
		doc.Add(mustNewFieldNoRandom(t, "str3", strconv.Itoa(i)+big, customType))
		mustAddDocument(t, indexWriter, doc)
	}

	mustClose(t, indexWriter)

	defer mustClose(t, dir)
	t.Fatal(testUtilCheckIndexMissing)
}

// stringSplitTokenizer renders the private static StringSplitTokenizer: it
// splits the whole input on single spaces.
type stringSplitTokenizer struct {
	*analysis.BaseTokenizer
	tokens  []string
	upto    int
	termAtt analysis.CharTermAttribute
}

func newStringSplitTokenizer() *stringSplitTokenizer {
	tk := &stringSplitTokenizer{BaseTokenizer: analysis.NewBaseTokenizer()}
	tk.termAtt = tk.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	return tk
}

func (tk *stringSplitTokenizer) IncrementToken() (bool, error) {
	tk.ClearAttributes()
	if tk.upto < len(tk.tokens) {
		tk.termAtt.SetEmpty()
		tk.termAtt.AppendString(tk.tokens[tk.upto])
		tk.upto++
		return true, nil
	}
	return false, nil
}

func (tk *stringSplitTokenizer) Reset() error {
	if err := tk.BaseTokenizer.Reset(); err != nil {
		return err
	}
	tk.upto = 0
	b, err := io.ReadAll(tk.GetReader())
	if err != nil {
		return err
	}
	tk.tokens = javaStringSplit(string(b), " ")
	return nil
}

// javaStringSplit renders String.split(regex) for a literal separator:
// trailing empty strings are removed.
func javaStringSplit(s, sep string) []string {
	parts := strings.Split(s, sep)
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// newStringSplitAnalyzer renders the static StringSplitAnalyzer.
func newStringSplitAnalyzer() analysis.Analyzer {
	a := analysis.NewAnalyzer(nil)
	a.CreateComponents = func(string) *analysis.TokenStreamComponents {
		tk := newStringSplitTokenizer()
		return &analysis.TokenStreamComponents{
			Source: func(r io.Reader) error {
				tk.SetReader(r)
				return nil
			},
			Sink: tk,
		}
	}
	return a
}

// Make sure we skip wicked long terms.
func TestIndexWriterWickedLongTerm(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriterWithAnalyzer(t, dir, newStringSplitAnalyzer())

	bigTerm := strings.Repeat("x", index.MAX_TERM_LENGTH)
	hugeDoc := document.NewDocument()

	// This contents produces a too-long term:
	contents := "abc xyz x" + bigTerm + " another term"
	hugeDoc.Add(mustNewTextFieldNoRandom(t, "content", contents, false))
	if _, err := w.AddDocument(hugeDoc); err == nil {
		t.Fatal("expected IllegalArgumentException from addDocument(hugeDoc)")
	}

	// Make sure we can add another normal document
	doc := document.NewDocument()
	doc.Add(mustNewTextFieldNoRandom(t, "content", "abc bbb ccc", false))
	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}

	// So we remove the deleted doc:
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	reader, err := w.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	mustClose(t, w)
	defer mustClose(t, reader, dir)

	// Make sure all terms < max size were indexed
	t.Fatal(indexReaderDocFreqMissing)
}

func TestIndexWriterDeleteAllNRTLeftoverFiles(t *testing.T) {
	d := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	w := mustNewIndexWriter(t, d, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	for i := 0; i < 20; i++ {
		for j := 0; j < 100; j++ {
			mustAddDocument(t, w, doc)
		}
		mustCommit(t, w)
		mustClose(t, openReaderFromWriter(t, w))

		mustDeleteAll(t, w)
		mustCommit(t, w)
		// Make sure we accumulate no files except for empty segments_N and segments.gen:
		if n := listAllCount(t, d); !(n <= 2) {
			t.Fatalf("assertTrue(d.listAll().length <= 2): %d", n)
		}
	}

	mustClose(t, w, d)
}

func TestIndexWriterNRTReaderVersion(t *testing.T) {
	d := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	w := mustNewIndexWriter(t, d, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", "0", true))
	mustAddDocument(t, w, doc)
	r := openReaderFromWriter(t, w)
	version := r.GetVersion()
	mustClose(t, r)

	mustAddDocument(t, w, doc)
	r = openReaderFromWriter(t, w)
	version2 := r.GetVersion()
	mustClose(t, r)
	if !(version2 > version) {
		t.Fatalf("assert (version2 > version): %d <= %d", version2, version)
	}

	mustDeleteTerm(t, w, "id", "0")
	r = openReaderFromWriter(t, w)
	mustClose(t, w)
	version3 := r.GetVersion()
	mustClose(t, r)
	if !(version3 > version2) {
		t.Fatalf("assert (version3 > version2): %d <= %d", version3, version2)
	}
	mustClose(t, d)
}

func TestIndexWriterWhetherDeleteAllDeletesWriteLock(t *testing.T) {
	// Must use SimpleFSLockFactory...
	// NativeFSLockFactory somehow "knows" a lock is held against write.lock
	// even if you remove that file:
	t.Fatal(fsDirectoryLockFactoryMissing)
}

const (
	leafReaderMetaDataHasBlocksMissing = "org.apache.lucene.index.LeafReader#getMetaData() returning LeafMetaData is not ported " +
		"(GetMetaData returns spi.IndexReaderMetaData, which has no hasBlocks())"
	mockTokenFilterAutomatonMissing = "org.apache.lucene.tests.analysis.MockTokenFilter(TokenStream, CharacterRunAutomaton) is not ported " +
		"(Gocene's NewMockTokenFilter takes a stop-word map)"
	addDocumentsNullMissing = "org.apache.lucene.index.IndexWriter#addDocuments(Iterable) accepting a null value is not ported " +
		"(a nil []*document.Document is the empty list)"
	checkIndexStatusMissing = "org.apache.lucene.index.CheckIndex#setInfoStream(PrintStream, boolean) is not ported"
)

func stringFieldDoc(t testing.TB, name, value string) *document.Document {
	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, name, value, false))
	return doc
}

func leafSegmentReader(t testing.TB, ctx *index.LeafReaderContext) *index.SegmentReader {
	t.Helper()
	sr, ok := ctx.LeafReader().(*index.SegmentReader)
	if !ok {
		t.Fatalf("leaf reader %T is not a SegmentReader", ctx.LeafReader())
	}
	return sr
}

func mustLeaves(t testing.TB, r *index.DirectoryReader) []*index.LeafReaderContext {
	t.Helper()
	leaves, err := r.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	return leaves
}

func TestIndexWriterHasBlocksMergeFullyDelSegments(t *testing.T) {
	documentSupplier := func() *document.Document {
		return stringFieldDoc(t, "foo", "bar")
	}
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	docs := []*document.Document{documentSupplier(), documentSupplier()}
	if _, err := writer.UpdateDocuments(index.NewTerm("foo", "bar"), docs); err != nil {
		t.Fatalf("updateDocuments: %v", err)
	}
	mustCommit(t, writer)
	if rand.Intn(2) == 0 {
		if _, err := writer.UpdateDocuments(index.NewTerm("foo", "bar"), docs); err != nil {
			t.Fatalf("updateDocuments: %v", err)
		}
		mustCommit(t, writer) // second segment
	}
	mustUpdateDocument(t, writer, index.NewTerm("foo", "bar"), documentSupplier())
	if rand.Intn(2) == 0 {
		if _, err := writer.ForceMergeDeletesWithObserver(true); err != nil {
			t.Fatalf("forceMergeDeletes(true): %v", err)
		}
	} else {
		if _, err := writer.ForceMergeWithObserver(1, true); err != nil {
			t.Fatalf("forceMerge(1, true): %v", err)
		}
	}
	mustCommit(t, writer)
	reader := mustOpenDirectoryReader(t, dir)
	leaves := mustLeaves(t, reader)
	if len(leaves) != 1 {
		t.Fatalf("reader.leaves().size(): expected 1, got %d", len(leaves))
	}
	defer mustClose(t, reader, writer, dir)
	// assertFalse("hasBlocks should be cleared", reader.leaves().get(0).reader().getMetaData().hasBlocks())
	t.Fatal(leafReaderMetaDataHasBlocksMissing)
}

func TestIndexWriterSingleDocsDoNotTriggerHasBlocks(t *testing.T) {
	dir := newDirectory()
	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(math.MaxInt32)
	conf.SetRAMBufferSizeMB(100)
	w := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, w, dir)

	docs := 1 + rand.Intn(99)
	for i := 0; i < docs; i++ {
		if _, err := w.AddDocuments([]*document.Document{stringFieldDoc(t, "id", strconv.Itoa(i))}); err != nil {
			t.Fatalf("addDocuments: %v", err)
		}
	}
	mustCommit(t, w)
	t.Fatal(indexWriterCloneSegmentInfosMissing)
}

func TestIndexWriterCarryOverHasBlocks(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))

	docs := []*document.Document{document.NewDocument()}
	if _, err := w.UpdateDocuments(index.NewTerm("foo", "bar"), docs); err != nil {
		t.Fatalf("updateDocuments: %v", err)
	}
	mustCommit(t, w)
	reader := mustOpenDirectoryReader(t, dir)
	segmentInfo := leafSegmentReader(t, mustLeaves(t, reader)[0]).GetSegmentCommitInfo()
	if segmentInfo.SegmentInfo().GetHasBlocks() {
		t.Fatal("assertFalse(segmentInfo.info.getHasBlocks())")
	}
	mustClose(t, reader)

	docs = append(docs, document.NewDocument()) // now we have 2 docs
	if _, err := w.UpdateDocuments(index.NewTerm("foo", "bar"), docs); err != nil {
		t.Fatalf("updateDocuments: %v", err)
	}
	mustCommit(t, w)
	reader = mustOpenDirectoryReader(t, dir)
	leaves := mustLeaves(t, reader)
	if len(leaves) != 2 {
		t.Fatalf("reader.leaves().size(): expected 2, got %d", len(leaves))
	}
	segmentInfo = leafSegmentReader(t, leaves[0]).GetSegmentCommitInfo()
	if segmentInfo.SegmentInfo().GetHasBlocks() {
		t.Fatalf("codec: %v: leaf 0 must not have blocks", segmentInfo.SegmentInfo().GetCodec())
	}
	segmentInfo = leafSegmentReader(t, leaves[1]).GetSegmentCommitInfo()
	if !segmentInfo.SegmentInfo().GetHasBlocks() {
		t.Fatalf("codec: %v: leaf 1 must have blocks", segmentInfo.SegmentInfo().GetCodec())
	}
	mustClose(t, reader)
	if _, err := w.ForceMergeWithObserver(1, true); err != nil {
		t.Fatalf("forceMerge(1, true): %v", err)
	}
	mustCommit(t, w)
	reader = mustOpenDirectoryReader(t, dir)
	leaves = mustLeaves(t, reader)
	if len(leaves) != 1 {
		t.Fatalf("reader.leaves().size(): expected 1, got %d", len(leaves))
	}
	segmentInfo = leafSegmentReader(t, leaves[0]).GetSegmentCommitInfo()
	if !segmentInfo.SegmentInfo().GetHasBlocks() {
		t.Fatalf("codec: %v: merged leaf must have blocks", segmentInfo.SegmentInfo().GetCodec())
	}
	mustClose(t, reader)
	mustCommit(t, w)
	mustClose(t, w, dir)
}

// LUCENE-3872
func TestIndexWriterPrepareCommitThenClose(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))

	if _, err := w.PrepareCommit(); err != nil {
		t.Fatalf("prepareCommit: %v", err)
	}
	if err := w.Close(); err == nil {
		t.Fatal("expected IllegalStateException from close() after prepareCommit()")
	}
	mustCommit(t, w)
	mustClose(t, w)
	r := mustOpenDirectoryReader(t, dir)
	if r.MaxDoc() != 0 {
		t.Fatalf("r.maxDoc(): expected 0, got %d", r.MaxDoc())
	}
	mustClose(t, r, dir)
}

func mustIndexExists(t testing.TB, dir store.Directory) bool {
	t.Helper()
	exists, err := index.IndexExists(dir)
	if err != nil {
		t.Fatalf("indexExists: %v", err)
	}
	return exists
}

// LUCENE-3872
func TestIndexWriterPrepareCommitThenRollback(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))

	if _, err := w.PrepareCommit(); err != nil {
		t.Fatalf("prepareCommit: %v", err)
	}
	if err := w.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if mustIndexExists(t, dir) {
		t.Fatal("assertFalse(DirectoryReader.indexExists(dir))")
	}
	mustClose(t, dir)
}

// LUCENE-3872
func TestIndexWriterPrepareCommitThenRollback2(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))

	mustCommit(t, w)
	mustAddDocument(t, w, document.NewDocument())
	if _, err := w.PrepareCommit(); err != nil {
		t.Fatalf("prepareCommit: %v", err)
	}
	if err := w.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if !mustIndexExists(t, dir) {
		t.Fatal("assertTrue(DirectoryReader.indexExists(dir))")
	}
	r := mustOpenDirectoryReader(t, dir)
	if r.MaxDoc() != 0 {
		t.Fatalf("r.maxDoc(): expected 0, got %d", r.MaxDoc())
	}
	mustClose(t, r, dir)
}

// dontInvokeMeAnalyzer renders the anonymous Analyzer of
// testDontInvokeAnalyzerForUnAnalyzedFields: every hook throws
// IllegalStateException, rendered as a panic because the Go hooks have no
// error result.
type dontInvokeMeAnalyzer struct {
	*analysis.BaseAnalyzer
}

func (dontInvokeMeAnalyzer) GetPositionIncrementGap(string) int { panic("don't invoke me!") }
func (dontInvokeMeAnalyzer) GetOffsetGap(string) int            { panic("don't invoke me!") }

func TestIndexWriterDontInvokeAnalyzerForUnAnalyzedFields(t *testing.T) {
	base := analysis.NewAnalyzer(nil)
	base.CreateComponents = func(string) *analysis.TokenStreamComponents {
		panic("don't invoke me!")
	}
	analyzer := dontInvokeMeAnalyzer{BaseAnalyzer: base}
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(analyzer))
	doc := document.NewDocument()
	customType := document.NewFieldTypeFrom(document.StringFieldTypeNotStored)
	customType.SetStoreTermVectors(true)
	customType.SetStoreTermVectorPositions(true)
	customType.SetStoreTermVectorOffsets(true)
	f := newField(t, "field", "abcd", customType)
	doc.Add(f)
	doc.Add(f)
	f2 := newField(t, "field", "", customType)
	doc.Add(f2)
	doc.Add(f)
	mustAddDocument(t, w, doc)
	mustClose(t, w, dir)
}

// LUCENE-1468 -- make sure opening an IndexWriter with
// create=true does not remove non-index files
func TestIndexWriterOtherFiles(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	iw := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustAddDocument(t, iw, document.NewDocument())
	mustClose(t, iw)
	// Create my own random file:
	out, err := dir.CreateOutput("myrandomfile", newIOContext())
	if err != nil {
		t.Fatalf("createOutput: %v", err)
	}
	if err := out.WriteByte(42); err != nil {
		t.Fatalf("writeByte: %v", err)
	}
	mustClose(t, out)

	mustClose(t, mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer())))

	if !slowFileExists(t, dir, "myrandomfile") {
		t.Fatal("assertTrue(slowFileExists(dir, \"myrandomfile\"))")
	}
}

// LUCENE-3849
func TestIndexWriterStopwordsPosIncHole(t *testing.T) {
	dir := newDirectory()
	a := analysis.NewAnalyzer(nil)
	a.CreateComponents = func(string) *analysis.TokenStreamComponents {
		tokenizer := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, true, testanalysis.DefaultMaxTokenLength)
		stream := testanalysis.NewMockTokenFilter(tokenizer, testanalysis.ENGLISH_STOPSET)
		return &analysis.TokenStreamComponents{
			Source: func(r io.Reader) error {
				tokenizer.SetReader(r)
				return nil
			},
			Sink: stream,
		}
	}
	iw := newRandomIndexWriterWithAnalyzer(t, dir, a)
	doc := document.NewDocument()
	doc.Add(mustNewTextFieldNoRandom(t, "body", "just a", false))
	doc.Add(mustNewTextFieldNoRandom(t, "body", "test of gaps", false))
	if _, err := iw.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	ir, err := iw.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	mustClose(t, iw)
	defer mustClose(t, ir, dir)
	newSearcher(t, ir)
}

// LUCENE-3849
func TestIndexWriterStopwordsPosIncHole2(t *testing.T) {
	// use two stopfilters for testing here
	dir := newDirectory()
	defer mustClose(t, dir)
	// The analyzer chains new MockTokenFilter(stream, new
	// CharacterRunAutomaton(Automata.makeString("foobar"))).
	t.Fatal(mockTokenFilterAutomatonMissing)
}

// LUCENE-4575
func TestIndexWriterCommitWithUserDataOnly(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(nil))
	mustCommit(t, writer) // first commit to complete IW create transaction.

	// this should store the commit data, even though no other changes were made
	writer.SetLiveCommitData(maps.All(map[string]string{"key": "value"}))
	mustCommit(t, writer)

	assertCommitUserData(t, dir, "value", "")

	// now check setCommitData and prepareCommit/commit sequence
	writer.SetLiveCommitData(maps.All(map[string]string{"key": "value1"}))
	if _, err := writer.PrepareCommit(); err != nil {
		t.Fatalf("prepareCommit: %v", err)
	}
	writer.SetLiveCommitData(maps.All(map[string]string{"key": "value2"}))
	mustCommit(t, writer) // should commit the first commitData only, per protocol

	assertCommitUserData(t, dir, "value1", "")

	// now should commit the second commitData - there was a bug where
	// IndexWriter.finishCommit overrode the second commitData
	mustCommit(t, writer)
	assertCommitUserData(t, dir, "value2", "IndexWriter.finishCommit may have overridden the second commitData")

	mustClose(t, writer, dir)
}

// assertCommitUserData renders
// assertEquals(expected, DirectoryReader.open(dir).getIndexCommit().getUserData().get("key")).
func assertCommitUserData(t testing.TB, dir store.Directory, expected, msg string) {
	t.Helper()
	r := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, r)
	if got := r.GetIndexCommit().GetUserData()["key"]; got != expected {
		t.Fatalf("%s: getUserData().get(\"key\"): expected %q, got %q", msg, expected, got)
	}
}

// testIndexWriterGetLiveCommitData renders the private
// getLiveCommitData(IndexWriter).
func testIndexWriterGetLiveCommitData(writer *index.IndexWriter) map[string]string {
	data := make(map[string]string)
	if seq := writer.GetLiveCommitData(); seq != nil {
		for k, v := range seq {
			data[k] = v
		}
	}
	return data
}

func assertLiveCommitDataKey(t testing.TB, writer *index.IndexWriter, expected string) {
	t.Helper()
	if got := testIndexWriterGetLiveCommitData(writer)["key"]; got != expected {
		t.Fatalf("getLiveCommitData(writer).get(\"key\"): expected %q, got %q", expected, got)
	}
}

func TestIndexWriterGetCommitData(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(nil))
	writer.SetLiveCommitData(maps.All(map[string]string{"key": "value"}))
	assertLiveCommitDataKey(t, writer, "value")
	mustClose(t, writer)

	// validate that it's also visible when opening a new IndexWriter
	conf := newIndexWriterConfigWithAnalyzer(nil)
	conf.SetOpenMode(index.Append)
	writer = mustNewIndexWriter(t, dir, conf)
	assertLiveCommitDataKey(t, writer, "value")
	mustClose(t, writer, dir)
}

// newSnapshotIndexWriterConfig renders
// LuceneTestCase.newSnapshotIndexWriterConfig(Analyzer).
func newSnapshotIndexWriterConfig(a analysis.Analyzer) *index.IndexWriterConfig {
	c := newIndexWriterConfigWithAnalyzer(a)
	c.SetIndexDeletionPolicy(index.NewSnapshotDeletionPolicy(index.NoDeletionPolicyInstance))
	return c
}

func TestIndexWriterGetCommitDataFromOldSnapshot(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newSnapshotIndexWriterConfig(nil))
	writer.SetLiveCommitData(maps.All(map[string]string{"key": "value"}))
	assertLiveCommitDataKey(t, writer, "value")
	mustCommit(t, writer)
	// Snapshot this commit to open later
	indexCommit := mustSnapshot(t, writer.GetConfig().GetIndexDeletionPolicy().(*index.SnapshotDeletionPolicy))
	mustClose(t, writer)

	// Modify the commit data and commit on close so the most recent commit data is different
	writer = mustNewIndexWriter(t, dir, newSnapshotIndexWriterConfig(nil))
	writer.SetLiveCommitData(maps.All(map[string]string{"key": "value2"}))
	assertLiveCommitDataKey(t, writer, "value2")
	mustClose(t, writer)

	// validate that when opening writer from older snapshotted index commit,
	// the old commit data is visible
	conf := newSnapshotIndexWriterConfig(nil)
	conf.SetOpenMode(index.Append)
	conf.SetIndexCommit(indexCommit)
	writer = mustNewIndexWriter(t, dir, conf)
	assertLiveCommitDataKey(t, writer, "value")
	mustClose(t, writer, dir)
}

func addThreeIDDocs(t testing.TB, iw interface {
	AddDocument(*document.Document) (int64, error)
}) {
	t.Helper()
	for i := 0; i < 3; i++ {
		if _, err := iw.AddDocument(stringFieldDoc(t, "id", strconv.Itoa(i))); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
}

func TestIndexWriterNullAnalyzer(t *testing.T) {
	dir := newDirectory()
	iwConf := newIndexWriterConfigWithAnalyzer(nil)
	iw := newRandomIndexWriterWithConfig(t, dir, iwConf)

	// add 3 good docs
	addThreeIDDocs(t, iw)

	defer mustClose(t, iw, dir)
	// add broken doc: new Field("test", "broken", DocHelper.TEXT_TYPE_STORED_WITH_TVS)
	t.Fatal(docHelperMissing)
}

// assertRIWReaderNumDocs renders IndexReader ir = iw.getReader();
// assertEquals(expected, ir.numDocs()); ir.close().
func assertRIWReaderNumDocs(t testing.TB, iw interface {
	GetReader() (*index.DirectoryReader, error)
}, expected int) {
	t.Helper()
	ir, err := iw.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	assertReaderNumDocs(t, expected, ir)
	mustClose(t, ir)
}

func TestIndexWriterNullDocument(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)

	// add 3 good docs
	addThreeIDDocs(t, iw)

	// add broken doc
	expectThrowsErrorOrPanic(t, "iw.addDocument(null)", func() error {
		_, err := iw.AddDocument(nil)
		return err
	})

	// ensure good docs are still ok
	assertRIWReaderNumDocs(t, iw, 3)

	mustClose(t, iw, dir)
}

func TestIndexWriterNullDocuments(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)

	// add 3 good docs
	addThreeIDDocs(t, iw)

	defer mustClose(t, iw, dir)
	// add broken doc block
	t.Fatal(addDocumentsNullMissing)
}

// iterableThrowsExceptionPrologue renders testIterableFieldThrowsException
// and testIterableThrowsException up to their first
// w.addDocument(new RandomFailingIterable<>(fields, random())), where fields
// is a List<IndexableField> of an "id" and a "foo" StringField.
func iterableThrowsExceptionPrologue(t *testing.T) {
	t.Helper()
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	defer mustClose(t, w, dir)
	t.Fatal(indexWriterAddDocumentIterableMissing)
}

func TestIndexWriterIterableFieldThrowsException(t *testing.T) {
	iterableThrowsExceptionPrologue(t)
}

func TestIndexWriterIterableThrowsException(t *testing.T) {
	iterableThrowsExceptionPrologue(t)
}

func TestIndexWriterIterableThrowsException2(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	defer mustClose(t, w, dir)
	t.Fatal(indexWriterAddDocumentsIterableMissing)
}

// LUCENE-2727/LUCENE-2812/LUCENE-4738:
func TestIndexWriterCorruptFirstCommit(t *testing.T) {
	for i := 0; i < 6; i++ {
		dir := newDirectory()

		// Create a corrupt first commit:
		out, err := dir.CreateOutput(index.FileNameFromGeneration(index.PendingSegmentsPrefix, "", 0), store.IOContextDefault)
		if err != nil {
			t.Fatalf("createOutput: %v", err)
		}
		mustClose(t, out)

		iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		mode := i / 2
		switch mode {
		case 0:
			iwc.SetOpenMode(index.Create)
		case 1:
			iwc.SetOpenMode(index.Append)
		case 2:
			iwc.SetOpenMode(index.CreateOrAppend)
		}

		w, err := index.NewIndexWriter(dir, iwc)
		if err == nil {
			if i&1 == 0 {
				err = w.Close()
			} else {
				err = w.Rollback()
			}
		}
		if err != nil {
			// OpenMode.APPEND should throw an exception since no index exists:
			if mode == 0 {
				// Unexpected
				t.Fatalf("mode=%d i=%d: %v", mode, i, err)
			}
		}

		if mode != 0 {
			t.Fatal(setCheckIndexOnCloseMissing)
		}

		mustClose(t, dir)
	}
}

func TestIndexWriterHasUncommittedChanges(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	// Disable merging to simplify this test, otherwise a commit might trigger
	// uncommitted merges.
	conf.SetMergePolicy(index.NewNoMergePolicy())
	writer := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, writer, dir)
	t.Fatal(indexWriterHasUncommittedChangesMissing)
}

func TestIndexWriterMergeAllDeleted(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	// The evil writer is RandomIndexWriter.mockIndexWriter(random(), dir, iwc,
	// testPoint) over a FilterMergePolicy.
	t.Fatal(mockIndexWriterMissing)
}

// LUCENE-5239
func TestIndexWriterDeleteSameTermAcrossFields(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w := mustNewIndexWriter(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(mustNewTextFieldNoRandom(t, "a", "foo", false))
	mustAddDocument(t, w, doc)

	// Should not delete the document;
	// with LUCENE-5239 the "foo" from the 2nd delete term would incorrectly match field a's "foo":
	mustDeleteTerm(t, w, "a", "xxx")
	mustDeleteTerm(t, w, "b", "foo")
	r := openReaderFromWriter(t, w)
	mustClose(t, w)

	// Make sure document was not (incorrectly) deleted:
	assertReaderNumDocs(t, 1, r)
	mustClose(t, r, dir)
}

func TestIndexWriterHasUncommittedChangesAfterException(t *testing.T) {
	analyzer := newMockAnalyzer()

	directory := newDirectory()
	// we don't use RandomIndexWriter because it might add more docvalues than we expect !!!!
	iwc := newIndexWriterConfigWithAnalyzer(analyzer)
	iwc.SetMergePolicy(newLogMergePolicy())
	iwriter := mustNewIndexWriter(t, directory, iwc)
	doc := document.NewDocument()
	doc.Add(sortedDVField(t, "dv", newBytesRef("foo!")))
	doc.Add(sortedDVField(t, "dv", newBytesRef("bar!")))
	if _, err := iwriter.AddDocument(doc); err == nil {
		t.Fatal("expected IllegalArgumentException from addDocument")
	}

	mustCommit(t, iwriter)
	defer mustClose(t, iwriter, directory)
	t.Fatal(indexWriterHasUncommittedChangesMissing)
}

func sortedDVDoc(t testing.TB) *document.Document {
	doc := document.NewDocument()
	doc.Add(sortedDVField(t, "dv", newBytesRef("foo!")))
	return doc
}

func TestIndexWriterDoubleClose(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustAddDocument(t, w, sortedDVDoc(t))
	mustClose(t, w)
	// Close again should have no effect
	mustClose(t, w, dir)
}

func TestIndexWriterRollbackThenClose(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustAddDocument(t, w, sortedDVDoc(t))
	if err := w.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	// Close after rollback should have no effect
	mustClose(t, w, dir)
}

func TestIndexWriterCloseThenRollback(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustAddDocument(t, w, sortedDVDoc(t))
	mustClose(t, w)
	// Rollback after close should have no effect
	if err := w.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	mustClose(t, dir)
}

func TestIndexWriterCloseWhileMergeIsRunning(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	// The merge scheduler is an anonymous ConcurrentMergeScheduler subclass
	// overriding doMerge(MergeSource, OneMerge) and close().
	t.Fatal("overriding org.apache.lucene.index.ConcurrentMergeScheduler#doMerge(MergeSource, OneMerge) in a subclass is not ported")
}

// Make sure that close waits for any still-running commits.
func TestIndexWriterCloseDuringCommit(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal(mockIndexWriterMissing)
}

// LUCENE-5895: Make sure we see ids per segment and per commit.
func TestIndexWriterIds(t *testing.T) {
	d := newDirectory()
	w := mustNewIndexWriter(t, d, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustAddDocument(t, w, document.NewDocument())
	mustClose(t, w)

	mustReadLatestCommit(t, d)
	defer mustClose(t, d)
	t.Fatal(segmentInfosGetIDMissing)
}

func TestIndexWriterEmptyNorm(t *testing.T) {
	d := newDirectory()
	w := mustNewIndexWriter(t, d, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	f, err := document.NewField("foo", testanalysis.NewCannedTokenStream(), document.TextFieldTypeNotStored)
	if err != nil {
		t.Fatalf("new TextField: %v", err)
	}
	doc.Add(f)
	mustAddDocument(t, w, doc)
	mustCommit(t, w)
	mustClose(t, w)
	r := mustOpenDirectoryReader(t, d)
	norms, err := getOnlyLeafReader(t, r).GetNormValues("foo")
	if err != nil {
		t.Fatalf("getNormValues: %v", err)
	}
	if norms == nil {
		t.Fatal("getNormValues(\"foo\") is null")
	}
	assertNextNumeric(t, norms, 0, 0)
	mustClose(t, r, d)
}

func TestIndexWriterManySeparateThreads(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMaxBufferedDocs(1000)
	w := mustNewIndexWriter(t, dir, iwc)
	// Index 100 docs, each from a new thread, but always only 1 thread is in IW at once:
	for i := 0; i < 100; i++ {
		done := make(chan struct{})
		go func() {
			defer close(done)
			doc := document.NewDocument()
			f, err := document.NewStringField("foo", "bar", false)
			if err != nil {
				t.Errorf("new StringField: %v", err)
				return
			}
			doc.Add(f)
			if _, err := w.AddDocument(doc); err != nil {
				t.Errorf("addDocument: %v", err)
			}
		}()
		<-done
	}
	mustClose(t, w)

	r := mustOpenDirectoryReader(t, dir)
	assertLeafCount(t, 1, r)
	mustClose(t, r, dir)
}

const (
	tokenOutOfBoundsPayloadMissing = "org.apache.lucene.analysis.Token#setPayload(BytesRef) with a BytesRef whose offset + length " +
		"exceeds its array is not representable (Gocene payloads are []byte)"
	indexWriterUpdateDocumentsQueryMissing = "org.apache.lucene.index.IndexWriter#updateDocuments(Query, Iterable) is not ported"
)

func assertCommitGenerationAndName(t testing.TB, r *index.DirectoryReader, gen int64) {
	t.Helper()
	commit := r.GetIndexCommit()
	if commit.GetGeneration() != gen {
		t.Fatalf("getIndexCommit().getGeneration(): expected %d, got %d", gen, commit.GetGeneration())
	}
	want := "segments_" + strconv.FormatInt(gen, 36)
	if commit.GetSegmentsFileName() != want {
		t.Fatalf("getIndexCommit().getSegmentsFileName(): expected %q, got %q", want, commit.GetSegmentsFileName())
	}
}

// LUCENE-6505
func TestIndexWriterNRTSegmentsFile(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w := mustNewIndexWriter(t, dir, iwc)
	// creates segments_1
	mustCommit(t, w)

	// newly opened NRT reader should see gen=1 segments file
	r := openReaderFromWriter(t, w)
	assertCommitGenerationAndName(t, r, 1)

	// newly opened non-NRT reader should see gen=1 segments file
	r2 := mustOpenDirectoryReader(t, dir)
	assertCommitGenerationAndName(t, r2, 1)
	mustClose(t, r2)

	// make a change and another commit
	mustAddDocument(t, w, document.NewDocument())
	mustCommit(t, w)
	r3 := openIfChanged(t, r)
	mustClose(t, r)
	if r3 == nil {
		t.Fatal("assertNotNull(r3)")
	}

	// reopened NRT reader should see gen=2 segments file
	assertCommitGenerationAndName(t, r3, 2)
	mustClose(t, r3)

	// newly opened non-NRT reader should see gen=2 segments file
	r4 := mustOpenDirectoryReader(t, dir)
	assertCommitGenerationAndName(t, r4, 2)
	mustClose(t, r4, w, dir)
}

// LUCENE-6505
func TestIndexWriterNRTAfterCommit(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w := mustNewIndexWriter(t, dir, iwc)
	mustCommit(t, w)

	mustAddDocument(t, w, document.NewDocument())
	r := openReaderFromWriter(t, w)
	mustCommit(t, w)

	// commit even with no other changes counts as a "change" that NRT reader reopen will see:
	r2 := mustOpenDirectoryReader(t, dir)
	assertCommitGenerationAndName(t, r2, 2)

	mustClose(t, r, r2, w, dir)
}

// LUCENE-6505
func TestIndexWriterNRTAfterSetUserDataWithoutCommit(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w := mustNewIndexWriter(t, dir, iwc)
	mustCommit(t, w)

	r := openReaderFromWriter(t, w)
	w.SetLiveCommitData(maps.All(map[string]string{"foo": "bar"}))

	// setLiveCommitData with no other changes should count as an NRT change:
	r2 := openIfChanged(t, r)
	if r2 == nil {
		t.Fatal("assertNotNull(r2)")
	}

	mustClose(t, r2, r, w, dir)
}

// LUCENE-6505
func TestIndexWriterNRTAfterSetUserDataWithCommit(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w := mustNewIndexWriter(t, dir, iwc)
	mustCommit(t, w)

	r := openReaderFromWriter(t, w)
	w.SetLiveCommitData(maps.All(map[string]string{"foo": "bar"}))
	mustCommit(t, w)
	// setLiveCommitData and also commit, with no other changes, should count as an NRT change:
	r2 := openIfChanged(t, r)
	if r2 == nil {
		t.Fatal("assertNotNull(r2)")
	}
	mustClose(t, r, r2, w, dir)
}

// LUCENE-6523
func TestIndexWriterCommitImmediatelyAfterNRTReopen(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w := mustNewIndexWriter(t, dir, iwc)
	mustCommit(t, w)

	mustAddDocument(t, w, document.NewDocument())

	r := openReaderFromWriter(t, w)
	mustCommit(t, w)

	assertIsCurrent(t, false, r, "r.isCurrent()")

	r2 := openIfChanged(t, r)
	if r2 == nil {
		t.Fatal("assertNotNull(r2)")
	}
	// segments_N should have changed:
	if r2.GetIndexCommit().GetSegmentsFileName() == r.GetIndexCommit().GetSegmentsFileName() {
		t.Fatalf("assertNotEquals: both readers see %s", r.GetIndexCommit().GetSegmentsFileName())
	}
	mustClose(t, r, r2, w, dir)
}

func TestIndexWriterPendingDeleteDVGeneration(t *testing.T) {
	// Use WindowsFS to prevent open files from being deleted:
	t.Fatal(windowsFSMissing)
}

func TestIndexWriterPendingDeletionsRollbackWithReader(t *testing.T) {
	// Use WindowsFS to prevent open files from being deleted:
	t.Fatal(windowsFSMissing)
}

func TestIndexWriterWithPendingDeletions(t *testing.T) {
	// Use WindowsFS to prevent open files from being deleted:
	t.Fatal(windowsFSMissing)
}

func TestIndexWriterPendingDeletesAlreadyWrittenFiles(t *testing.T) {
	// Use WindowsFS to prevent open files from being deleted:
	t.Fatal(windowsFSMissing)
}

func TestIndexWriterLeftoverTempFiles(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w := mustNewIndexWriter(t, dir, iwc)
	mustClose(t, w)

	out, err := dir.CreateTempOutput("_0", "bkd", store.IOContextDefault)
	if err != nil {
		t.Fatalf("createTempOutput: %v", err)
	}
	tempName := out.GetName()
	mustClose(t, out)
	iwc = index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w = mustNewIndexWriter(t, dir, iwc)

	// Make sure IW deleted the unref'd file:
	in, err := dir.OpenInput(tempName, store.IOContextDefault)
	if err == nil {
		mustClose(t, in)
		t.Fatal("did not hit exception")
	}
	if !isFileNotFoundOrNoSuchFile(err) {
		t.Fatalf("openInput(%s): %v", tempName, err)
	}
	mustClose(t, w, dir)
}

// testIndexWriterMassiveField renders the @Ignore'd testMassiveField
// ("requires running tests with biggish heap"): JUnit never runs it, so it
// has no Test entry point. It builds a StoredField longer than
// IndexWriter.MAX_STORED_STRING_LENGTH.
func testIndexWriterMassiveField(t *testing.T) {
	t.Fatal(indexWriterMaxStoredStringLengthMissing)
}

func TestIndexWriterRecordsIndexCreatedVersion(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	mustCommit(t, w)
	mustClose(t, w)
	if got := mustReadLatestCommit(t, dir).GetIndexCreatedVersionMajor(); int(got) != util.Latest.Major {
		t.Fatalf("getIndexCreatedVersionMajor(): expected %d, got %d", util.Latest.Major, got)
	}
	mustClose(t, dir)
}

// testIndexWriterIndexDocsForMultipleDWPTs renders the private
// indexDocsForMultipleDWPTs(IndexWriter): three threads meet at a latch and
// each index numDocsPerThread documents, so the writer holds more than one
// DWPT.
func testIndexWriterIndexDocsForMultipleDWPTs(t *testing.T, w *index.IndexWriter) int {
	t.Helper()
	const numThreads = 3
	var latch sync.WaitGroup
	latch.Add(numThreads)
	numDocsPerThread := 10 + rand.Intn(30)
	var wg sync.WaitGroup
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			latch.Done()
			latch.Wait()
			for j := 0; j < numDocsPerThread; j++ {
				doc := document.NewDocument()
				f, err := document.NewStringField("id", "foo", true)
				if err != nil {
					t.Errorf("new StringField: %v", err)
					return
				}
				doc.Add(f)
				if _, err := w.AddDocument(doc); err != nil {
					t.Errorf("addDocument: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}
	return numDocsPerThread * numThreads
}

func TestIndexWriterFlushLargestWriter(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfig())
	defer mustClose(t, w, dir)
	testIndexWriterIndexDocsForMultipleDWPTs(t, w)
	// w.docWriter.flushControl.findLargestNonPendingWriter()
	t.Fatal(documentsWriterAccessMissing)
}

func TestIndexWriterNeverCheckOutOnFullFlush(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfig())
	defer mustClose(t, w, dir)
	testIndexWriterIndexDocsForMultipleDWPTs(t, w)
	// w.docWriter.flushControl.findLargestNonPendingWriter()
	t.Fatal(documentsWriterAccessMissing)
}

func TestIndexWriterApplyDeletesWithoutFlushes(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	// The config's FlushPolicy is an anonymous subclass whose
	// onChange(DocumentsWriterFlushControl, DocumentsWriterPerThread) calls
	// control.setApplyAllDeletes(); the writer's first assertion reads
	// w.docWriter.flushControl.getDeleteBytesUsed().
	t.Fatal(documentsWriterAccessMissing)
}

func TestIndexWriterDeletesAppliedOnFlush(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfig())
	defer mustClose(t, w, dir)
	doc := document.NewDocument()
	doc.Add(newField(t, "id", "1", testIndexWriterStoredTextType))
	mustAddDocument(t, w, doc)
	mustUpdateDocument(t, w, index.NewTerm("id", "1"), doc)
	// w.docWriter.flushControl.getDeleteBytesUsed()
	t.Fatal(documentsWriterAccessMissing)
}

func TestIndexWriterHoldLockOnLargestWriter(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfig())
	defer mustClose(t, w, dir)
	testIndexWriterIndexDocsForMultipleDWPTs(t, w)
	// w.docWriter.flushControl.findLargestNonPendingWriter()
	t.Fatal(documentsWriterAccessMissing)
}

func TestIndexWriterCheckPendingFlushPostUpdate(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	// The Failure's eval calls
	// callStackContains(DocumentsWriterPerThread.class, "flush").
	t.Fatal(callStackContainsClassMissing)
}

func TestIndexWriterSoftUpdateDocuments(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfig()
	conf.SetMergePolicy(index.NewNoMergePolicy())
	conf.SetSoftDeletesField("soft_delete")
	writer := mustNewIndexWriter(t, dir, conf)
	softDelete := func() []*document.Field {
		return []*document.Field{numericDVField(t, "soft_delete", 1).Field}
	}
	if _, err := writer.SoftUpdateDocument(nil, document.NewDocument(), softDelete()); err == nil {
		t.Fatal("expected IllegalArgumentException from softUpdateDocument(null, doc, field)")
	}

	if _, err := writer.SoftUpdateDocument(index.NewTerm("id", "1"), document.NewDocument(), nil); err == nil {
		t.Fatal("expected IllegalArgumentException from softUpdateDocument(term, doc)")
	}

	if _, err := writer.SoftUpdateDocuments(nil, []*document.Document{document.NewDocument()}, softDelete()); err == nil {
		t.Fatal("expected IllegalArgumentException from softUpdateDocuments(null, docs, field)")
	}

	if _, err := writer.SoftUpdateDocuments(index.NewTerm("id", "1"), []*document.Document{document.NewDocument()}, nil); err == nil {
		t.Fatal("expected IllegalArgumentException from softUpdateDocuments(term, docs)")
	}

	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "1", true))
	doc.Add(newStringFieldNoRandom(t, "version", "1", true))
	mustAddDocument(t, writer, doc)
	doc = document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "1", true))
	doc.Add(newStringFieldNoRandom(t, "version", "2", true))
	if _, err := writer.SoftUpdateDocument(index.NewTerm("id", "1"), doc, softDelete()); err != nil {
		t.Fatalf("softUpdateDocument: %v", err)
	}
	reader := openReaderFromWriter(t, writer)
	defer mustClose(t, writer, reader, dir)
	// assertEquals(2, reader.docFreq(new Term("id", "1")))
	t.Fatal(indexReaderDocFreqMissing)
}

func TestIndexWriterSoftUpdatesConcurrently(t *testing.T) {
	testIndexWriterSoftUpdatesConcurrently(t, false)
}

func TestIndexWriterSoftUpdatesConcurrentlyMixedDeletes(t *testing.T) {
	testIndexWriterSoftUpdatesConcurrently(t, true)
}

// testIndexWriterSoftUpdatesConcurrently renders the public
// softUpdatesConcurrently(boolean).
func testIndexWriterSoftUpdatesConcurrently(t *testing.T, mixDeletes bool) {
	dir := newDirectory()
	indexWriterConfig := newIndexWriterConfig()
	indexWriterConfig.SetSoftDeletesField("soft_delete")
	if !mixDeletes {
		mustClose(t, dir)
		// new OneMergeWrappingMergePolicy(..., towrap -> new
		// MergePolicy.OneMerge(towrap.segments) { wrapForMerge(...) }) { numDeletesToMerge(...) }
		t.Fatal(oneMergeOverrideMissing)
	}
	writer := mustNewIndexWriter(t, dir, indexWriterConfig)
	threads := 2 + rand.Intn(3)
	startLatch := newCountDownLatch()
	var started sync.WaitGroup
	started.Add(threads)
	updateSeveralDocs := rand.Intn(2) == 0
	var idsMu sync.Mutex
	ids := make(map[string]struct{})
	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			started.Done()
			if !startLatch.awaitFromGoroutine(t, "startLatch") {
				return
			}
			for d := 0; d < 100; d++ {
				id := strconv.Itoa(rand.Intn(10))
				doc := document.NewDocument()
				f, err := document.NewStringField("id", id, true)
				if err != nil {
					t.Errorf("new StringField: %v", err)
					return
				}
				doc.Add(f)
				if updateSeveralDocs {
					if mixDeletes && rand.Intn(2) == 0 {
						if rand.Intn(2) == 0 {
							if _, err := writer.UpdateDocuments(index.NewTerm("id", id), []*document.Document{doc, doc}); err != nil {
								t.Errorf("updateDocuments: %v", err)
								return
							}
						} else {
							t.Errorf("%s", indexWriterUpdateDocumentsQueryMissing)
							return
						}
					} else {
						sd, err := document.NewNumericDocValuesField("soft_delete", 1)
						if err != nil {
							t.Errorf("new NumericDocValuesField: %v", err)
							return
						}
						if _, err := writer.SoftUpdateDocuments(index.NewTerm("id", id), []*document.Document{doc, doc}, []*document.Field{sd.Field}); err != nil {
							t.Errorf("softUpdateDocuments: %v", err)
							return
						}
					}
				} else {
					if mixDeletes && rand.Intn(2) == 0 {
						if _, err := writer.UpdateDocument(index.NewTerm("id", id), doc); err != nil {
							t.Errorf("updateDocument: %v", err)
							return
						}
					} else {
						sd, err := document.NewNumericDocValuesField("soft_delete", 1)
						if err != nil {
							t.Errorf("new NumericDocValuesField: %v", err)
							return
						}
						if _, err := writer.SoftUpdateDocument(index.NewTerm("id", id), doc, []*document.Field{sd.Field}); err != nil {
							t.Errorf("softUpdateDocument: %v", err)
							return
						}
					}
				}
				idsMu.Lock()
				ids[id] = struct{}{}
				idsMu.Unlock()
			}
		}()
	}
	started.Wait()
	startLatch.countDown()

	wg.Wait()
	if t.Failed() {
		mustClose(t, writer, dir)
		t.FailNow()
	}
	reader := openReaderFromWriter(t, writer)
	searcher := search.NewIndexSearcher(reader)
	for id := range ids {
		topDocs, err := searcher.Search(search.NewTermQuery(index.NewTerm("id", id)), 10)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if updateSeveralDocs {
			if topDocs.TotalHits.Value != 2 {
				t.Fatalf("id=%s: totalHits: expected 2, got %d", id, topDocs.TotalHits.Value)
			}
			diff := topDocs.ScoreDocs[0].Doc - topDocs.ScoreDocs[1].Doc
			if diff != 1 && diff != -1 {
				t.Fatalf("id=%s: |doc0 - doc1| = %d, expected 1", id, diff)
			}
		} else if topDocs.TotalHits.Value != 1 {
			t.Fatalf("id=%s: totalHits: expected 1, got %d", id, topDocs.TotalHits.Value)
		}
	}
	mustAddDocument(t, writer, document.NewDocument()) // add a dummy doc to trigger a segment here
	mustFlush(t, writer)
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	oldReader := reader
	newReader, err := index.OpenIfChangedFromWriter(reader, writer)
	if err != nil {
		t.Fatalf("openIfChanged(reader, writer): %v", err)
	}
	if newReader != nil {
		mustClose(t, oldReader)
		if newReader == oldReader {
			t.Fatal("assertNotSame(oldReader, reader)")
		}
		reader = newReader
	}
	defer mustClose(t, reader, writer, dir)
	// assertEquals(2 or 1, reader.docFreq(new Term("id", id)))
	t.Fatal(indexReaderDocFreqMissing)
}

func TestIndexWriterDeleteHappensBeforeWhileFlush(t *testing.T) {
	// The FilterDirectory's createOutput override calls
	// callStackContains(IndexingChain.class, "flush").
	t.Fatal(callStackContainsClassMissing)
}

func TestIndexWriterFullyDeletedSegmentsReleaseFiles(t *testing.T) {
	dir := newDirectory()
	config := newIndexWriterConfig()
	config.SetRAMBufferSizeMB(math.MaxInt32)
	config.SetMaxBufferedDocs(2) // no auto flush
	writer := mustNewIndexWriter(t, dir, config)
	defer mustClose(t, writer, dir)
	d := document.NewDocument()
	d.Add(newStringFieldNoRandom(t, "id", "doc-0", true))
	mustAddDocument(t, writer, d)
	mustFlush(t, writer)
	d = document.NewDocument()
	d.Add(newStringFieldNoRandom(t, "id", "doc-1", true))
	mustAddDocument(t, writer, d)
	mustDeleteTerm(t, writer, "id", "doc-1")
	t.Fatal(indexWriterCloneSegmentInfosMissing)
}

func TestIndexWriterSegmentInfoIsSnapshot(t *testing.T) {
	dir := newDirectory()
	config := newIndexWriterConfig()
	config.SetRAMBufferSizeMB(math.MaxInt32)
	config.SetMaxBufferedDocs(2) // no auto flush
	writer := mustNewIndexWriter(t, dir, config)
	d := document.NewDocument()
	d.Add(newStringFieldNoRandom(t, "id", "doc-0", true))
	mustAddDocument(t, writer, d)
	d = document.NewDocument()
	d.Add(newStringFieldNoRandom(t, "id", "doc-1", true))
	mustAddDocument(t, writer, d)
	reader := openReaderFromWriter(t, writer)
	defer mustClose(t, reader, writer, dir)
	leafSegmentReader(t, mustLeaves(t, reader)[0]).GetSegmentCommitInfo()
	t.Fatal(segmentReaderGetOriginalSegmentInfoMissing)
}

// expectNewIndexWriterIAE renders
// assertEquals(message, expectThrows(IllegalArgumentException.class, () -> new IndexWriter(dir, conf)).getMessage()).
func expectNewIndexWriterIAE(t testing.TB, dir store.Directory, conf *index.IndexWriterConfig, message string) {
	t.Helper()
	w, err := index.NewIndexWriter(dir, conf)
	if err == nil {
		mustClose(t, w)
		t.Fatalf("expected IllegalArgumentException %q", message)
	}
	if err.Error() != message {
		t.Fatalf("getMessage(): expected %q, got %q", message, err.Error())
	}
}

func assertSoftDeletesFieldInfos(t testing.TB, si *index.SegmentCommitInfo, field string) {
	t.Helper()
	fieldInfos, err := index.IndexWriterReadFieldInfos(si)
	if err != nil {
		t.Fatalf("IndexWriter.readFieldInfos: %v", err)
	}
	if got := fieldInfos.GetSoftDeletesField(); got != field {
		t.Fatalf("getSoftDeletesField(): expected %q, got %q", field, got)
	}
	if fi := fieldInfos.FieldInfo(field); fi == nil || !fi.IsSoftDeletesField() {
		t.Fatalf("assertTrue(fieldInfos.fieldInfo(%q).isSoftDeletesField())", field)
	}
}

func TestIndexWriterPreventChangingSoftDeletesField(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfig()
	conf.SetSoftDeletesField("my_deletes")
	writer := mustNewIndexWriter(t, dir, conf)
	v1 := document.NewDocument()
	v1.Add(newStringFieldNoRandom(t, "id", "1", true))
	v1.Add(newStringFieldNoRandom(t, "version", "1", true))
	mustAddDocument(t, writer, v1)
	v2 := document.NewDocument()
	v2.Add(newStringFieldNoRandom(t, "id", "1", true))
	v2.Add(newStringFieldNoRandom(t, "version", "2", true))
	if _, err := writer.SoftUpdateDocument(index.NewTerm("id", "1"), v2, []*document.Field{numericDVField(t, "my_deletes", 1).Field}); err != nil {
		t.Fatalf("softUpdateDocument: %v", err)
	}
	mustCommit(t, writer)
	mustClose(t, writer)
	for si := range mustReadLatestCommit(t, dir).Iterator() {
		assertSoftDeletesFieldInfos(t, si, "my_deletes")
	}

	conf = newIndexWriterConfig()
	conf.SetSoftDeletesField("your_deletes")
	expectNewIndexWriterIAE(t, dir, conf,
		"cannot configure [your_deletes] as soft-deletes; this index uses [my_deletes] as soft-deletes already")

	defer mustClose(t, dir)
	t.Fatal(softDeletesRetentionSupplierMissing)
}

func TestIndexWriterPreventAddingIndexesWithDifferentSoftDeletesField(t *testing.T) {
	dir1 := newDirectory()
	conf := newIndexWriterConfig()
	conf.SetSoftDeletesField("soft_deletes_1")
	w1 := mustNewIndexWriter(t, dir1, conf)
	for i := 0; i < 2; i++ {
		d := document.NewDocument()
		d.Add(newStringFieldNoRandom(t, "id", "1", true))
		d.Add(newStringFieldNoRandom(t, "version", strconv.Itoa(i), true))
		if _, err := w1.SoftUpdateDocument(index.NewTerm("id", "1"), d, []*document.Field{numericDVField(t, "soft_deletes_1", 1).Field}); err != nil {
			t.Fatalf("softUpdateDocument: %v", err)
		}
	}
	mustCommit(t, w1)
	mustClose(t, w1)

	dir2 := newDirectory()
	conf = newIndexWriterConfig()
	conf.SetSoftDeletesField("soft_deletes_2")
	w2 := mustNewIndexWriter(t, dir2, conf)
	_, err := w2.AddIndexes(dir1)
	if err == nil {
		t.Fatal("expected IllegalArgumentException from w2.addIndexes(dir1)")
	}
	const message = "cannot configure [soft_deletes_2] as soft-deletes; this index uses [soft_deletes_1] as soft-deletes already"
	if err.Error() != message {
		t.Fatalf("getMessage(): expected %q, got %q", message, err.Error())
	}
	mustClose(t, w2)

	dir3 := newDirectory()
	config := newIndexWriterConfig()
	config.SetSoftDeletesField("soft_deletes_1")
	w3 := mustNewIndexWriter(t, dir3, config)
	if _, err := w3.AddIndexes(dir1); err != nil {
		t.Fatalf("w3.addIndexes(dir1): %v", err)
	}
	defer mustClose(t, w3, dir1, dir2, dir3)
	t.Fatal(indexWriterCloneSegmentInfosMissing)
}

func TestIndexWriterNotAllowUsingExistingFieldAsSoftDeletes(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	for i := 0; i < 2; i++ {
		d := document.NewDocument()
		d.Add(newStringFieldNoRandom(t, "id", "1", true))
		if rand.Intn(2) == 0 {
			d.Add(numericDVField(t, "dv_field", 1))
			mustUpdateDocument(t, w, index.NewTerm("id", "1"), d)
		} else {
			if _, err := w.SoftUpdateDocument(index.NewTerm("id", "1"), d, []*document.Field{numericDVField(t, "dv_field", 1).Field}); err != nil {
				t.Fatalf("softUpdateDocument: %v", err)
			}
		}
	}
	mustCommit(t, w)
	mustClose(t, w)
	softDeletesField := "dv_field"
	if rand.Intn(2) == 0 {
		softDeletesField = "id"
	}
	config := newIndexWriterConfig()
	config.SetSoftDeletesField(softDeletesField)
	expectNewIndexWriterIAE(t, dir, config,
		"cannot configure ["+softDeletesField+"] as soft-deletes; this index uses ["+softDeletesField+"] as non-soft-deletes already")
	config = newIndexWriterConfig()
	config.SetSoftDeletesField("non-existing-field")
	w = mustNewIndexWriter(t, dir, config)
	mustClose(t, w, dir)
}

func TestIndexWriterBrokenPayload(t *testing.T) {
	d := newDirectory()
	w := mustNewIndexWriter(t, d, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	defer mustClose(t, w, d)
	t.Fatal(tokenOutOfBoundsPayloadMissing)
}

func TestIndexWriterSoftAndHardLiveDocs(t *testing.T) {
	dir := newDirectory()
	indexWriterConfig := newIndexWriterConfig()
	softDeletesField := "soft_delete"
	indexWriterConfig.SetSoftDeletesField(softDeletesField)
	writer := mustNewIndexWriter(t, dir, indexWriterConfig)
	uniqueDocs := make(map[int]struct{})
	for i := 0; i < 100; i++ {
		docID := rand.Intn(5)
		uniqueDocs[docID] = struct{}{}
		doc := document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "id", strconv.Itoa(docID), true))
		if docID%2 == 0 {
			mustUpdateDocument(t, writer, index.NewTerm("id", strconv.Itoa(docID)), doc)
		} else {
			if _, err := writer.SoftUpdateDocument(index.NewTerm("id", strconv.Itoa(docID)), doc,
				[]*document.Field{numericDVField(t, softDeletesField, 0).Field}); err != nil {
				t.Fatalf("softUpdateDocument: %v", err)
			}
		}
		if rand.Intn(2) == 0 {
			testIndexWriterAssertHardLiveDocs(t, writer, uniqueDocs)
		}
	}

	if rand.Intn(2) == 0 {
		mustCommit(t, writer)
	}
	testIndexWriterAssertHardLiveDocs(t, writer, uniqueDocs)

	mustClose(t, writer, dir)
}

func TestIndexWriterAbortFullyDeletedSegment(t *testing.T) {
	// new OneMergeWrappingMergePolicy(newMergePolicy(), toWrap -> new
	// MergePolicy.OneMerge(toWrap.segments) { onMergeComplete() }) { keepFullyDeletedSegment }
	t.Fatal(oneMergeOverrideMissing)
}

// testIndexWriterAssertHardLiveDocs renders the private
// assertHardLiveDocs(IndexWriter, Set<Integer>).
func testIndexWriterAssertHardLiveDocs(t testing.TB, writer *index.IndexWriter, uniqueDocs map[int]struct{}) {
	t.Helper()
	reader := openReaderFromWriter(t, writer)
	defer mustClose(t, reader)
	assertReaderNumDocs(t, len(uniqueDocs), reader)
	for _, ctx := range mustLeaves(t, reader) {
		sr := leafSegmentReader(t, ctx)
		if sr.GetHardLiveDocs() == nil {
			continue
		}
		id, err := sr.Terms("id")
		if err != nil {
			t.Fatalf("terms(id): %v", err)
		}
		iterator, err := id.Iterator()
		if err != nil {
			t.Fatalf("iterator: %v", err)
		}
		hardLiveDocs := sr.GetHardLiveDocs()
		liveDocs := sr.GetLiveDocs()
		for dID := range uniqueDocs {
			mustBeHardDeleted := dID%2 == 0
			found, err := iterator.SeekExact(index.NewTerm("id", strconv.Itoa(dID)))
			if err != nil {
				t.Fatalf("seekExact: %v", err)
			}
			if !found {
				continue
			}
			postings, err := iterator.Postings(index.PostingsFlagFreqs)
			if err != nil {
				t.Fatalf("postings: %v", err)
			}
			for {
				doc, err := postings.NextDoc()
				if err != nil {
					t.Fatalf("nextDoc: %v", err)
				}
				if doc == spi.NO_MORE_DOCS {
					break
				}
				switch {
				case liveDocs.Get(doc):
					if !hardLiveDocs.Get(doc) {
						t.Fatalf("assertTrue(hardLiveDocs.get(%d))", doc)
					}
				case mustBeHardDeleted:
					if hardLiveDocs.Get(doc) {
						t.Fatalf("assertFalse(hardLiveDocs.get(%d))", doc)
					}
				default:
					if !hardLiveDocs.Get(doc) {
						t.Fatalf("assertTrue(hardLiveDocs.get(%d))", doc)
					}
				}
			}
		}
	}
}

func TestIndexWriterSetIndexCreatedVersion(t *testing.T) {
	expectThrowsPanic(t, "new IndexWriterConfig().setIndexCreatedVersionMajor(Version.LATEST.major + 1)", func() {
		index.NewIndexWriterConfig().SetIndexCreatedVersionMajor(util.Latest.Major + 1)
	})
	expectThrowsPanic(t, "new IndexWriterConfig().setIndexCreatedVersionMajor(Version.LATEST.major - 2)", func() {
		index.NewIndexWriterConfig().SetIndexCreatedVersionMajor(util.Latest.Major - 2)
	})

	for previousMajor := util.Latest.Major - 1; previousMajor <= util.Latest.Major; previousMajor++ {
		for newMajor := util.Latest.Major - 1; newMajor <= util.Latest.Major; newMajor++ {
			for _, openMode := range []index.OpenMode{index.Create, index.Append, index.CreateOrAppend} {
				dir := newDirectory()
				conf := newIndexWriterConfig()
				conf.SetIndexCreatedVersionMajor(previousMajor)
				mustClose(t, mustNewIndexWriter(t, dir, conf))
				infos := mustReadLatestCommit(t, dir)
				if int(infos.GetIndexCreatedVersionMajor()) != previousMajor {
					t.Fatalf("getIndexCreatedVersionMajor(): expected %d, got %d", previousMajor, infos.GetIndexCreatedVersionMajor())
				}
				conf = newIndexWriterConfig()
				conf.SetOpenMode(openMode)
				conf.SetIndexCreatedVersionMajor(newMajor)
				mustClose(t, mustNewIndexWriter(t, dir, conf))
				infos = mustReadLatestCommit(t, dir)
				want := previousMajor
				if openMode == index.Create {
					want = newMajor
				}
				if int(infos.GetIndexCreatedVersionMajor()) != want {
					t.Fatalf("openMode=%v: getIndexCreatedVersionMajor(): expected %d, got %d", openMode, want, infos.GetIndexCreatedVersionMajor())
				}
				mustClose(t, dir)
			}
		}
	}
}

// see LUCENE-8639
func TestIndexWriterFlushWhileStartingNewThreads(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfig())
	defer mustClose(t, w, dir)
	mustAddDocument(t, w, document.NewDocument())
	// assertEquals(1, w.docWriter.perThreadPool.size())
	t.Fatal(documentsWriterAccessMissing)
}

// isAlreadyClosed renders `catch (AlreadyClosedException e)`.
func isAlreadyClosed(err error) bool {
	var ace *store.AlreadyClosedException
	return errors.As(err, &ace)
}

func TestIndexWriterRefreshAndRollbackConcurrently(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	var stopped atomic.Bool
	indexedDocs := make(chan struct{}, 1<<16) // Semaphore(0): one token per release(1)
	var threads sync.WaitGroup
	indexer := func() {
		defer threads.Done()
		for !stopped.Load() {
			id := strconv.Itoa(rand.Intn(100))
			doc := document.NewDocument()
			f, err := document.NewStringField("id", id, true)
			if err != nil {
				t.Errorf("new StringField: %v", err)
				return
			}
			doc.Add(f)
			if _, err := w.UpdateDocument(index.NewTerm("id", id), doc); err != nil {
				if isAlreadyClosed(err) {
					return
				}
				t.Errorf("AssertionError: %v", err)
				return
			}
			indexedDocs <- struct{}{}
		}
	}

	sm, err := search.NewSearcherManager(w, search.NewSearcherFactory())
	if err != nil {
		t.Fatalf("new SearcherManager: %v", err)
	}
	refresher := func() {
		defer threads.Done()
		for !stopped.Load() {
			if err := sm.MaybeRefreshBlocking(); err != nil {
				if isAlreadyClosed(err) {
					return
				}
				t.Errorf("AssertionError: %v", err)
				return
			}
		}
	}

	func() {
		defer func() {
			stopped.Store(true)
			threads.Wait() // indexer.join(); refresher.join();
			// assertNull("should not consider ACE a tragedy on a closed IW: " + ..., w.getTragicException());
			t.Error(indexWriterGetTragicExceptionMissing)
			mustClose(t, sm, dir)
		}()
		threads.Add(2)
		go indexer()
		go refresher()
		for i := 1 + rand.Intn(100); i > 0; i-- {
			<-indexedDocs
		}
		if err := w.Rollback(); err != nil {
			t.Errorf("rollback: %v", err)
		}
	}()
}

func TestIndexWriterCloseableQueue(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	defer mustClose(t, writer, dir)
	t.Fatal(indexWriterEventQueueMissing)
}

// randomOperationsPolicy renders the anonymous FilterMergePolicy of
// testRandomOperations, whose keepFullyDeletedSegment answers a value
// drawn once.
type randomOperationsPolicy struct {
	*index.FilterMergePolicy
	keepFullyDeletedSegment bool
}

func (p *randomOperationsPolicy) KeepFullyDeletedSegment(*index.SegmentCommitInfo) bool {
	return p.keepFullyDeletedSegment
}

// javaSemaphore renders the java.util.concurrent.Semaphore permits the
// random-operations tests draw from with tryAcquire().
type javaSemaphore struct{ permits atomic.Int64 }

func (s *javaSemaphore) tryAcquire() bool {
	for {
		p := s.permits.Load()
		if p <= 0 {
			return false
		}
		if s.permits.CompareAndSwap(p, p-1) {
			return true
		}
	}
}

func TestIndexWriterRandomOperations(t *testing.T) {
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(&randomOperationsPolicy{
		FilterMergePolicy:       index.NewFilterMergePolicy(newMergePolicy(t)),
		keepFullyDeletedSegment: rand.Intn(2) == 0,
	})
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, iwc)
	sm, err := search.NewSearcherManager(writer, search.NewSearcherFactory())
	if err != nil {
		t.Fatalf("new SearcherManager: %v", err)
	}
	var numOperations javaSemaphore
	numOperations.permits.Store(int64(10 + rand.Intn(1000)))
	singleDoc := rand.Intn(2) == 0
	threads := 1 + rand.Intn(4)
	var latch sync.WaitGroup
	latch.Add(threads)
	var wg sync.WaitGroup
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			latch.Done()
			latch.Wait()
			for numOperations.tryAcquire() {
				id := "1"
				if !singleDoc {
					id = strconv.Itoa(rand.Intn(10))
				}
				doc := document.NewDocument()
				f, err := document.NewStringField("id", id, true)
				if err != nil {
					t.Errorf("new StringField: %v", err)
					return
				}
				doc.Add(f)
				if rand.Intn(10) <= 2 {
					_, err = writer.UpdateDocument(index.NewTerm("id", id), doc)
				} else if rand.Intn(10) <= 2 {
					_, err = writer.DeleteDocuments([]index.Term{*index.NewTerm("id", id)})
				} else {
					_, err = writer.AddDocument(doc)
				}
				if err != nil {
					t.Errorf("%v", err)
					return
				}
				if rand.Intn(100) < 10 {
					if err := sm.MaybeRefreshBlocking(); err != nil {
						t.Errorf("maybeRefreshBlocking: %v", err)
						return
					}
				}
				if rand.Intn(100) < 5 {
					if _, err := writer.Commit(); err != nil {
						t.Errorf("commit: %v", err)
						return
					}
				}
				if rand.Intn(100) < 1 {
					if _, err := writer.ForceMergeWithObserver(1+rand.Intn(10), rand.Intn(2) == 0); err != nil {
						t.Errorf("forceMerge: %v", err)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	mustClose(t, sm, writer, dir)
}

func TestIndexWriterRandomOperationsWithSoftDeletes(t *testing.T) {
	iwc := newIndexWriterConfig()
	iwc.SetSoftDeletesField("soft_deletes")
	t.Fatal(softDeletesRetentionSupplierMissing)
}

func TestIndexWriterMaxCompletedSequenceNumber(t *testing.T) {
	{
		dir := newDirectory()
		writer := mustNewIndexWriter(t, dir, index.NewIndexWriterConfig())
		assertSeqNo := func(what string, expected int64, got int64, err error) {
			t.Helper()
			if err != nil {
				t.Fatalf("%s: %v", what, err)
			}
			if got != expected {
				t.Fatalf("%s: expected %d, got %d", what, expected, got)
			}
		}
		seqNo, err := writer.AddDocument(document.NewDocument())
		assertSeqNo("addDocument", 1, seqNo, err)
		seqNo, err = writer.UpdateDocument(index.NewTerm("foo", "bar"), document.NewDocument())
		assertSeqNo("updateDocument", 2, seqNo, err)
		if _, err := writer.FlushNextBuffer(); err != nil {
			t.Fatalf("flushNextBuffer: %v", err)
		}
		seqNo, err = writer.Commit()
		assertSeqNo("commit", 3, seqNo, err)
		seqNo, err = writer.AddDocument(document.NewDocument())
		assertSeqNo("addDocument", 4, seqNo, err)
		assertSeqNo("getMaxCompletedSequenceNumber", 4, writer.GetMaxCompletedSequenceNumber(), nil)
		// commit moves seqNo by 2 since there is one DWPT that could still be in-flight
		seqNo, err = writer.Commit()
		assertSeqNo("commit", 6, seqNo, err)
		assertSeqNo("getMaxCompletedSequenceNumber", 6, writer.GetMaxCompletedSequenceNumber(), nil)
		seqNo, err = writer.AddDocument(document.NewDocument())
		assertSeqNo("addDocument", 7, seqNo, err)
		mustClose(t, openReaderFromWriter(t, writer))
		// getReader moves seqNo by 2 since there is one DWPT that could still be in-flight
		assertSeqNo("getMaxCompletedSequenceNumber", 9, writer.GetMaxCompletedSequenceNumber(), nil)
		mustClose(t, writer, dir)
	}
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	manager, err := search.NewSearcherManager(writer, search.NewSearcherFactory())
	if err != nil {
		t.Fatalf("new SearcherManager: %v", err)
	}
	defer mustClose(t, dir, writer, manager)
	start := make(chan struct{}) // CountDownLatch(1)
	numDocs := nextInt(10, 60)   // TEST_NIGHTLY ? nextInt(100, 600) : nextInt(10, 60)
	var maxCompletedSeqID atomic.Int64
	maxCompletedSeqID.Store(-1)
	var threads sync.WaitGroup
	numThreads := 2 + rand.Intn(2)
	for i := 0; i < numThreads; i++ {
		idx := i
		threads.Add(1)
		go func() {
			defer threads.Done()
			<-start
			for j := 0; j < numDocs; j++ {
				doc := document.NewDocument()
				id := strconv.Itoa(idx) + "-" + strconv.Itoa(j)
				f, err := document.NewStringField("id", id, false)
				if err != nil {
					t.Errorf("AssertionError: %v", err)
					return
				}
				doc.Add(f)
				seqNo, err := writer.AddDocument(doc)
				if err != nil {
					t.Errorf("AssertionError: %v", err)
					return
				}
				if maxCompletedSeqID.Load() < seqNo {
					maxCompletedSequenceNumber := writer.GetMaxCompletedSequenceNumber()
					if err := manager.MaybeRefreshBlocking(); err != nil {
						t.Errorf("AssertionError: %v", err)
						return
					}
					for {
						oldVal := maxCompletedSeqID.Load()
						if maxCompletedSeqID.CompareAndSwap(oldVal, max(oldVal, maxCompletedSequenceNumber)) {
							break
						}
					}
				}
				acquire, err := manager.Acquire()
				if err != nil {
					t.Errorf("AssertionError: %v", err)
					return
				}
				td, err := acquire.Search(search.NewTermQuery(index.NewTerm("id", id)), 10)
				releaseErr := manager.Release(acquire)
				if err != nil {
					t.Errorf("AssertionError: %v", err)
					return
				}
				if releaseErr != nil {
					t.Errorf("AssertionError: %v", releaseErr)
					return
				}
				if td.TotalHits.Value != 1 {
					t.Errorf("AssertionError: expected 1 hit for id=%s, got %d", id, td.TotalHits.Value)
					return
				}
			}
		}()
	}
	close(start)
	threads.Wait()
}

func TestIndexWriterEnsureMaxSeqNoIsAccurateDuringFlush(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	// The writer is an IndexWriter subclass overriding isEnableTestPoints().
	t.Fatal(isEnableTestPointsOverrideMissing)
}

func noMergeConfig() *index.IndexWriterConfig {
	c := index.NewIndexWriterConfig()
	c.SetMergePolicy(index.NewNoMergePolicy())
	return c
}

func numIDDoc(t testing.TB, num int64, id string) *document.Document {
	doc := document.NewDocument()
	doc.Add(numericDVField(t, "num", num))
	doc.Add(newStringFieldNoRandom(t, "id", id, false))
	return doc
}

func sciIDString(sci *index.SegmentCommitInfo) string { return util.IdToString(sci.GetID()) }
func siIDString(sci *index.SegmentCommitInfo) string {
	return util.IdToString(sci.SegmentInfo().GetID())
}

func TestIndexWriterSegmentCommitInfoId(t *testing.T) {
	dir := newDirectory()
	var segmentCommitInfos *index.SegmentInfos
	{
		writer := mustNewIndexWriter(t, dir, noMergeConfig())
		mustAddDocument(t, writer, numIDDoc(t, 1, "1"))
		mustAddDocument(t, writer, numIDDoc(t, 1, "2"))
		mustCommit(t, writer)
		segmentCommitInfos = mustReadLatestCommit(t, dir)
		id := sciIDString(segmentCommitInfos.Get(0))
		segInfoID := siIDString(segmentCommitInfos.Get(0))

		mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "1"), "num", 2)
		mustCommit(t, writer)
		segmentCommitInfos = mustReadLatestCommit(t, dir)
		if segmentCommitInfos.Size() != 1 {
			t.Fatalf("size: expected 1, got %d", segmentCommitInfos.Size())
		}
		if id == sciIDString(segmentCommitInfos.Get(0)) {
			t.Fatal("assertNotEquals: the commit id did not change after a doc-values update")
		}
		if segInfoID != siIDString(segmentCommitInfos.Get(0)) {
			t.Fatal("assertEquals: the segment info id changed after a doc-values update")
		}
		id = sciIDString(segmentCommitInfos.Get(0))
		mustAddDocument(t, writer, document.NewDocument()) // second segment
		mustCommit(t, writer)
		segmentCommitInfos = mustReadLatestCommit(t, dir)
		if segmentCommitInfos.Size() != 2 {
			t.Fatalf("size: expected 2, got %d", segmentCommitInfos.Size())
		}
		if id != sciIDString(segmentCommitInfos.Get(0)) {
			t.Fatal("assertEquals: the commit id changed without a change to the segment")
		}
		if segInfoID != siIDString(segmentCommitInfos.Get(0)) {
			t.Fatal("assertEquals: the segment info id changed")
		}

		mustUpdateDocument(t, writer, index.NewTerm("id", "1"), numIDDoc(t, 5, "1"))
		mustCommit(t, writer)
		segmentCommitInfos = mustReadLatestCommit(t, dir)
		if segmentCommitInfos.Size() != 3 {
			t.Fatalf("size: expected 3, got %d", segmentCommitInfos.Size())
		}
		if id == sciIDString(segmentCommitInfos.Get(0)) {
			t.Fatal("assertNotEquals: the commit id did not change after a delete")
		}
		if segInfoID != siIDString(segmentCommitInfos.Get(0)) {
			t.Fatal("assertEquals: the segment info id changed after a delete")
		}
		mustClose(t, writer)
	}

	{
		dir2 := newDirectory()
		writer2 := mustNewIndexWriter(t, dir2, noMergeConfig())
		if _, err := writer2.AddIndexes(dir); err != nil {
			t.Fatalf("addIndexes: %v", err)
		}
		mustCommit(t, writer2)
		infos2 := mustReadLatestCommit(t, dir2)
		if infos2.Size() != segmentCommitInfos.Size() {
			t.Fatalf("size: expected %d, got %d", segmentCommitInfos.Size(), infos2.Size())
		}
		for i := 0; i < infos2.Size(); i++ {
			if sciIDString(infos2.Get(i)) != sciIDString(segmentCommitInfos.Get(i)) {
				t.Fatalf("segment %d: commit ids differ after addIndexes", i)
			}
			if siIDString(infos2.Get(i)) != siIDString(segmentCommitInfos.Get(i)) {
				t.Fatalf("segment %d: segment info ids differ after addIndexes", i)
			}
		}
		mustClose(t, writer2, dir2)
	}
	mustClose(t, dir)

	ids := make(map[string]struct{})
	for i := 0; i < 2; i++ {
		dir := newDirectory()
		writer := mustNewIndexWriter(t, dir, noMergeConfig())
		mustAddDocument(t, writer, numIDDoc(t, 1, "1"))
		mustCommit(t, writer)
		id := sciIDString(mustReadLatestCommit(t, dir).Get(0))
		if _, dup := ids[id]; dup {
			t.Fatalf("assertTrue(ids.add(%s))", id)
		}
		ids[id] = struct{}{}
		mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "1"), "num", 2)
		mustCommit(t, writer)
		id = sciIDString(mustReadLatestCommit(t, dir).Get(0))
		if _, dup := ids[id]; dup {
			t.Fatalf("assertTrue(ids.add(%s))", id)
		}
		ids[id] = struct{}{}
		mustClose(t, writer, dir)
	}
}

func TestIndexWriterMergeZeroDocsMergeIsClosedOnce(t *testing.T) {
	// new OneMergeWrappingMergePolicy(keepAllSegments, merge -> new
	// MergePolicy.OneMerge(merge.segments) { mergeFinished(boolean, boolean) })
	t.Fatal(oneMergeOverrideMissing)
}

// mergeOnCommitKeepFullyDeletedPolicy renders the anonymous
// FilterMergePolicy of testMergeOnCommitKeepFullyDeletedSegments.
type mergeOnCommitKeepFullyDeletedPolicy struct {
	*index.FilterMergePolicy
}

func (mergeOnCommitKeepFullyDeletedPolicy) KeepFullyDeletedSegment(*index.SegmentCommitInfo) bool {
	return true
}

func (mergeOnCommitKeepFullyDeletedPolicy) FindFullFlushMerges(_ index.MergeTrigger, segmentInfos *index.SegmentInfos, _ index.MergeContext) (*index.MergeSpecification, error) {
	var fullyDeletedSegments []*index.SegmentCommitInfo
	for s := range segmentInfos.Iterator() {
		if s.SegmentInfo().MaxDoc()-s.GetDelCount() == 0 {
			fullyDeletedSegments = append(fullyDeletedSegments, s)
		}
	}
	if len(fullyDeletedSegments) == 0 {
		return nil, nil
	}
	spec := index.NewMergeSpecification()
	spec.Add(index.NewOneMerge(fullyDeletedSegments))
	return spec, nil
}

func TestIndexWriterMergeOnCommitKeepFullyDeletedSegments(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMaxFullFlushMergeWaitMillis(30 * 1000)
	iwc.SetMergePolicy(mergeOnCommitKeepFullyDeletedPolicy{FilterMergePolicy: index.NewFilterMergePolicy(newMergePolicy(t))})
	w := mustNewIndexWriter(t, dir, iwc)
	d := document.NewDocument()
	d.Add(newStringFieldNoRandom(t, "id", "1", true))
	mustAddDocument(t, w, d)
	mustCommit(t, w)
	mustUpdateDocument(t, w, index.NewTerm("id", "1"), d)
	mustCommit(t, w)
	reader := openReaderFromWriter(t, w)
	assertReaderNumDocs(t, 1, reader)
	mustClose(t, reader, w, dir)
}

func TestIndexWriterPendingNumDocs(t *testing.T) {
	dir := newDirectory()
	numDocs := rand.Intn(100)
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	for i := 0; i < numDocs; i++ {
		d := document.NewDocument()
		d.Add(newStringFieldNoRandom(t, "id", strconv.Itoa(i), true))
		mustAddDocument(t, writer, d)
		if got := writer.GetPendingNumDocs(); got != i+1 {
			t.Fatalf("getPendingNumDocs(): expected %d, got %d", i+1, got)
		}
	}
	if got := writer.GetPendingNumDocs(); got != numDocs {
		t.Fatalf("getPendingNumDocs(): expected %d, got %d", numDocs, got)
	}
	mustFlush(t, writer)
	if got := writer.GetPendingNumDocs(); got != numDocs {
		t.Fatalf("getPendingNumDocs(): expected %d, got %d", numDocs, got)
	}
	mustClose(t, writer)
	writer = mustNewIndexWriter(t, dir, newIndexWriterConfig())
	if got := writer.GetPendingNumDocs(); got != numDocs {
		t.Fatalf("getPendingNumDocs(): expected %d, got %d", numDocs, got)
	}
	mustClose(t, writer, dir)
}

func TestIndexWriterIndexWriterBlocksOnStall(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	defer mustClose(t, writer, dir)
	// writer.getDocsWriter().flushControl.stallControl
	t.Fatal(documentsWriterAccessMissing)
}

func TestIndexWriterGetFieldNames(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	defer mustClose(t, writer, dir)
	t.Fatal(indexWriterGetFieldNamesMissing)
}

func TestIndexWriterParentAndSoftDeletesAreTheSame(t *testing.T) {
	dir := newDirectory()
	indexWriterConfig := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	indexWriterConfig.SetSoftDeletesField("foo")
	indexWriterConfig.SetParentField("foo")
	expectNewIndexWriterIAE(t, dir, indexWriterConfig,
		"parent document and soft-deletes field can't be the same field \"foo\"")
	mustClose(t, dir)
}

func parentFieldConfig(mode index.OpenMode, parentField string) *index.IndexWriterConfig {
	c := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	c.SetOpenMode(mode)
	c.SetParentField(parentField)
	return c
}

func TestIndexWriterParentFieldExistingIndex(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, iwc)
	d := document.NewDocument()
	d.Add(mustNewTextFieldNoRandom(t, "f", "a", false))
	mustAddDocument(t, writer, d)
	mustClose(t, writer)
	expectNewIndexWriterIAE(t, dir, parentFieldConfig(index.Append, "foo"),
		"can't add a parent field to an already existing index without a parent field")
	expectNewIndexWriterIAE(t, dir, parentFieldConfig(index.CreateOrAppend, "foo"),
		"can't add a parent field to an already existing index without a parent field")

	writer = mustNewIndexWriter(t, dir, parentFieldConfig(index.Create, "foo"))
	mustAddDocument(t, writer, document.NewDocument())
	mustClose(t, writer, dir)
}

func TestIndexWriterIndexWithParentFieldIsCongruent(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetParentField("parent")
	writer := mustNewIndexWriter(t, dir, iwc)
	if rand.Intn(2) == 0 {
		child1 := stringFieldStoredDoc(t, "id", "1")
		child2 := stringFieldStoredDoc(t, "id", "1")
		parent := stringFieldStoredDoc(t, "id", "1")
		if _, err := writer.AddDocuments([]*document.Document{child1, child2, parent}); err != nil {
			t.Fatalf("addDocuments: %v", err)
		}
		mustFlush(t, writer)
		if rand.Intn(2) == 0 {
			if _, err := writer.AddDocuments([]*document.Document{child1, child2, parent}); err != nil {
				t.Fatalf("addDocuments: %v", err)
			}
		}
	} else {
		mustAddDocument(t, writer, document.NewDocument())
	}
	mustCommit(t, writer)
	mustClose(t, writer)
	config := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	config.SetParentField("someOtherField")
	expectNewIndexWriterIAE(t, dir, config,
		"can't add field [parent] as parent document field; this IndexWriter is configured with [someOtherField] as parent document field")
	expectNewIndexWriterIAE(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()),
		"can't add field [parent] as parent document field; this IndexWriter has no parent document field configured")
	mustClose(t, dir)
}

func stringFieldStoredDoc(t testing.TB, name, value string) *document.Document {
	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, name, value, true))
	return doc
}

func TestIndexWriterParentFieldIsAlreadyUsed(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, iwc)
	mustAddDocument(t, writer, stringFieldStoredDoc(t, "parent", "1"))
	mustCommit(t, writer)
	mustClose(t, writer)
	config := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	config.SetParentField("parent")
	expectNewIndexWriterIAE(t, dir, config,
		"can't add [parent] as non parent document field; this IndexWriter is configured with [parent] as parent document field")
	mustClose(t, dir)
}

func TestIndexWriterParentFieldEmptyIndex(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetParentField("parent")
	writer := mustNewIndexWriter(t, dir, iwc)
	mustCommit(t, writer)
	mustClose(t, writer)
	iwc2 := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc2.SetParentField("parent")
	writer = mustNewIndexWriter(t, dir, iwc2)
	mustCommit(t, writer)
	mustClose(t, writer, dir)
}

func TestIndexWriterSingleDocBlockWritesParentField(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetParentField("parent")
	writer := mustNewIndexWriter(t, dir, iwc)
	// Single-document "block" — the lone doc is both first and last in the block
	if _, err := writer.AddDocuments([]*document.Document{stringFieldStoredDoc(t, "id", "s0")}); err != nil {
		t.Fatalf("addDocuments: %v", err)
	}

	// Multi-document block for comparison
	if _, err := writer.AddDocuments([]*document.Document{stringFieldStoredDoc(t, "id", "c0"), stringFieldStoredDoc(t, "id", "p0")}); err != nil {
		t.Fatalf("addDocuments: %v", err)
	}
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	// 3 docs total: doc0=single, doc1=child, doc2=parent
	assertReaderNumDocs(t, 3, reader)
	parentDV := leafNumeric(t, mustLeaves(t, reader)[0].LeafReader(), "parent")

	// doc 0 (the single-doc block) should be a parent
	assertNextNumeric(t, parentDV, 0, -1)

	// doc 2 (last doc of the multi-doc block) should be a parent
	assertNextNumeric(t, parentDV, 2, -1)

	// no more parents
	if doc, err := parentDV.NextDoc(); err != nil || doc != spi.NO_MORE_DOCS {
		t.Fatalf("parentDV.nextDoc(): expected NO_MORE_DOCS, got %d (%v)", doc, err)
	}
	mustClose(t, reader, dir)
}

func TestIndexWriterDocValuesMixedSkippingIndex(t *testing.T) {
	{
		dir := newDirectory()
		writer := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
		f1, err := document.NewSortedNumericDocValuesFieldIndexed("test", []int64{javaNextLong()})
		if err != nil {
			t.Fatalf("SortedNumericDocValuesField.indexedField: %v", err)
		}
		mustAddDocument(t, writer, docOf(f1))

		f2, err := document.NewSortedNumericDocValuesField("test", []int64{javaNextLong()})
		if err != nil {
			t.Fatalf("new SortedNumericDocValuesField: %v", err)
		}
		_, err = writer.AddDocument(docOf(f2))
		expectIAEMessage(t, err,
			"Inconsistency of field data structures across documents for field [test] of doc [1]. doc values skip index type: expected 'RANGE', but it has 'NONE'.")
		mustClose(t, writer, dir)
	}
	{
		dir := newDirectory()
		writer := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
		defer mustClose(t, writer, dir)
		mustAddDocument(t, writer, docOf(sortedSetDVField(t, "test", util.RandomBinaryTerm())))
		t.Fatal("org.apache.lucene.document.SortedSetDocValuesField#indexedField(String, BytesRef) is not ported")
	}
}

func expectIAEMessage(t testing.TB, err error, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected IllegalArgumentException %q", message)
	}
	if err.Error() != message {
		t.Fatalf("getMessage(): expected %q, got %q", message, err.Error())
	}
}

func TestIndexWriterDocValuesSkippingIndexWithoutDocValues(t *testing.T) {
	for _, docValuesType := range []spi.DocValuesType{spi.DocValuesTypeNone, spi.DocValuesTypeBinary} {
		fieldType := document.NewFieldType()
		fieldType.SetStored(true)
		fieldType.SetDocValuesType(docValuesType)
		fieldType.SetDocValuesSkipIndexType(document.DocValuesSkipIndexTypeRange)
		fieldType.Freeze()
		dir := newDirectory()
		writer := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
		doc1 := document.NewDocument()
		f, ferr := document.NewField("test", make([]byte, 10), fieldType)
		if ferr != nil {
			t.Fatalf("new Field: %v", ferr)
		}
		doc1.Add(f)
		_, err := writer.AddDocument(doc1)
		if err == nil {
			t.Fatal("expected IllegalArgumentException from addDocument")
		}
		if !strings.HasPrefix(err.Error(), "field 'test' cannot have docValuesSkipIndexType=RANGE") {
			t.Fatalf("getMessage(): %q", err.Error())
		}
		mustClose(t, writer, dir)
	}
}

func TestIndexWriterAdvanceSegmentInfosCounter(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	defer mustClose(t, writer, dir)

	// add 10 documents
	for i := 0; i < 10; i++ {
		testIndexWriterAddDocWithIndex(t, writer, i)
		mustCommit(t, writer)
	}
	t.Fatal(indexWriterSegmentInfosCounterMissing)
}

func TestIndexWriterAdvanceSegmentCounterInCrashAndRecoveryScenario(t *testing.T) {
	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))

	// add 100 documents
	for i := 0; i < 100; i++ {
		testIndexWriterAddDocWithIndex(t, writer, i)
		if rand.Intn(2) == 0 {
			mustCommit(t, writer)
		}
	}
	assertWriterDocStats(t, writer, 100, 100)
	mustCommit(t, writer)
	mustClose(t, writer)

	// recovery and advance segment counter
	writer = mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	defer mustClose(t, writer, dir)
	if n := iwDocStats(t, writer).NumDocs; n != 100 {
		t.Fatalf("writer.getDocStats().numDocs: expected 100, got %d", n)
	}
	t.Fatal(indexWriterSegmentInfosCounterMissing)
}

// testIndexWriterAddDoc renders the package-private static
// TestIndexWriter.addDoc(IndexWriter) that other Lucene index tests call.
func testIndexWriterAddDoc(t testing.TB, writer *index.IndexWriter) {
	t.Helper()
	doc := document.NewDocument()
	doc.Add(newTextField(t, "content", "aaa", false))
	mustAddDocument(t, writer, doc)
}

// testIndexWriterStoredTextType renders TestIndexWriter's private static
// storedTextType = new FieldType(TextField.TYPE_NOT_STORED).
var testIndexWriterStoredTextType = document.NewFieldTypeFrom(document.TextFieldTypeNotStored)

// testIndexWriterAddDocWithIndex renders the package-private static
// TestIndexWriter.addDocWithIndex(IndexWriter, int).
func testIndexWriterAddDocWithIndex(t testing.TB, writer *index.IndexWriter, index int) {
	t.Helper()
	doc := document.NewDocument()
	doc.Add(newField(t, "content", "aaa "+strconv.Itoa(index), testIndexWriterStoredTextType))
	doc.Add(newField(t, "id", strconv.Itoa(index), testIndexWriterStoredTextType))
	mustAddDocument(t, writer, doc)
}

// assertNoUnreferencedFiles renders the public static
// TestIndexWriter.assertNoUnreferencedFiles(Directory, String): opening and
// rolling back an IndexWriter must not change the directory listing.
func assertNoUnreferencedFiles(t testing.TB, dir store.Directory, message string) {
	t.Helper()
	startFiles, err := dir.ListAll()
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	if err := w.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	endFiles, err := dir.ListAll()
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}

	sort.Strings(startFiles)
	sort.Strings(endFiles)

	if !slices.Equal(startFiles, endFiles) {
		t.Fatalf("%s: before delete:\n    %s\n  after delete:\n    %s",
			message, strings.Join(startFiles, "\n    "), strings.Join(endFiles, "\n    "))
	}
}
