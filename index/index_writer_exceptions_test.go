// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterExceptions.java
// (Apache Lucene 10.5.0). The @Nightly tests live in
// index_writer_exceptions_monster_test.go.
//
// Most tests inject faults through RandomIndexWriter.mockIndexWriter test
// points, IndexWriter subclasses overriding isEnableTestPoints(), or
// MockDirectoryWrapper.Failure call-stack inspection; none of these is
// ported, so each such test runs up to that point and fails naming it.

package index_test

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testutil "github.com/FlavioCFOliveira/Gocene/tests/util"
)

// Missing members the Java tests reach.
const (
	mockIndexWriterMissing = "org.apache.lucene.tests.index.RandomIndexWriter#mockIndexWriter(...) with a " +
		"RandomIndexWriter.TestPoint is not ported (IndexWriter#testPoint(String) and #isEnableTestPoints() are missing)"
	callStackContainsClassMissing     = "MockDirectoryWrapper.Failure#callStackContains(Class<?>, String) is not ported"
	isEnableTestPointsOverrideMissing = "overriding org.apache.lucene.index.IndexWriter#isEnableTestPoints() in an " +
		"IndexWriter subclass is not ported"
	indexWriterGetTragicExceptionMissing = "org.apache.lucene.index.IndexWriter#getTragicException() is not ported"
	indexReaderDocFreqMissing            = "org.apache.lucene.index.IndexReader#docFreq(Term) is not ported for " +
		"composite readers (DirectoryReader)"
	setCheckIndexOnCloseMissing         = "org.apache.lucene.tests.store.BaseDirectoryWrapper#setCheckIndexOnClose(boolean) is not ported"
	softDeletesRetentionSupplierMissing = "org.apache.lucene.index.SoftDeletesRetentionMergePolicy(String, " +
		"Supplier<Query>, MergePolicy) is not ported"
)

// docCopyIteratorCustom5 renders DocCopyIterator.custom5, the only one of the
// record's field types the reachable code uses.
var docCopyIteratorCustom5 = func() *document.FieldType {
	ft := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorPositions(true)
	ft.SetStoreTermVectorOffsets(true)
	return ft
}()

const crashFailMessage = "I'm experiencing problems"

// crashingFilter renders the private static CrashingFilter.
type crashingFilter struct {
	*analysis.BaseTokenFilter
	fieldName string
	count     int
}

func newCrashingFilter(fieldName string, input analysis.TokenStream) *crashingFilter {
	return &crashingFilter{BaseTokenFilter: analysis.NewBaseTokenFilter(input), fieldName: fieldName}
}

func (f *crashingFilter) IncrementToken() (bool, error) {
	if f.fieldName == "crash" {
		c := f.count
		f.count++
		if c >= 4 {
			return false, errors.New(crashFailMessage)
		}
	}
	return f.GetInput().IncrementToken()
}

func (f *crashingFilter) Reset() error {
	if err := f.BaseTokenFilter.Reset(); err != nil {
		return err
	}
	f.count = 0
	return nil
}

// crashingAnalyzer renders the anonymous Analyzer(PER_FIELD_REUSE_STRATEGY)
// whose components are a WHITESPACE MockTokenizer, with checks disabled,
// wrapped by a CrashingFilter when crash reports true.
func crashingAnalyzer(crash func() bool) analysis.Analyzer {
	a := analysis.NewAnalyzer(analysis.PerFieldReuseStrategy)
	a.CreateComponents = func(fieldName string) *analysis.TokenStreamComponents {
		tokenizer := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, false, testanalysis.DefaultMaxTokenLength)
		tokenizer.SetEnableChecks(false) // disable workflow checking as we forcefully close() in exceptional cases.
		var stream analysis.TokenStream = tokenizer
		if crash() {
			stream = newCrashingFilter(fieldName, stream)
		}
		return &analysis.TokenStreamComponents{
			Source: func(r io.Reader) error {
				tokenizer.SetReader(r)
				return nil
			},
			Sink: stream,
		}
	}
	return a
}

func alwaysCrash() bool { return true }

// newMockAnalyzerNoChecks renders new MockAnalyzer(random()) followed by
// analyzer.setEnableChecks(false).
func newMockAnalyzerNoChecks() analysis.Analyzer {
	a := testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true)
	a.SetEnableChecks(false) // disable workflow checking as we forcefully close() in exceptional cases.
	return a
}

func TestIndexWriterExceptionsRandomExceptions(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	analyzer := newMockAnalyzerNoChecks()
	conf := newIndexWriterConfigWithAnalyzer(analyzer)
	conf.SetRAMBufferSizeMB(0.1)
	conf.SetMergeScheduler(index.NewConcurrentMergeScheduler())
	t.Fatal(mockIndexWriterMissing)
}

func TestIndexWriterExceptionsRandomExceptionsThreads(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	analyzer := newMockAnalyzerNoChecks()
	conf := newIndexWriterConfigWithAnalyzer(analyzer)
	conf.SetRAMBufferSizeMB(0.2)
	conf.SetMergeScheduler(index.NewConcurrentMergeScheduler())
	t.Fatal(mockIndexWriterMissing)
}

func TestIndexWriterExceptionsExceptionDocumentsWriterInit(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal(mockIndexWriterMissing)
}

// LUCENE-1208
func TestIndexWriterExceptionsExceptionJustBeforeFlush(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)

	var doCrash atomic.Bool
	analyzer := crashingAnalyzer(doCrash.Load)
	conf := newIndexWriterConfigWithAnalyzer(analyzer)
	conf.SetMaxBufferedDocs(2)
	t.Fatal(mockIndexWriterMissing)
}

// LUCENE-1210
func TestIndexWriterExceptionsExceptionOnMergeInit(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	conf.SetMergePolicy(newLogMergePolicy())
	cms := index.NewConcurrentMergeScheduler()
	cms.SetSuppressExceptions()
	conf.SetMergeScheduler(cms)
	lmp := conf.GetMergePolicy().(logMergePolicy)
	lmp.SetMergeFactor(2)
	lmp.SetTargetSearchConcurrency(1)
	t.Fatal(mockIndexWriterMissing)
}

// exceptionFromTokenStreamFilter renders the anonymous TokenFilter of
// testExceptionFromTokenStream.
type exceptionFromTokenStreamFilter struct {
	*analysis.BaseTokenFilter
	count int
}

func (f *exceptionFromTokenStreamFilter) IncrementToken() (bool, error) {
	c := f.count
	f.count++
	if c == 5 {
		return false, errors.New("IOException")
	}
	return f.GetInput().IncrementToken()
}

func (f *exceptionFromTokenStreamFilter) Reset() error {
	if err := f.BaseTokenFilter.Reset(); err != nil {
		return err
	}
	f.count = 0
	return nil
}

// LUCENE-1072
func TestIndexWriterExceptionsExceptionFromTokenStream(t *testing.T) {
	dir := newDirectory()
	analyzer := analysis.NewAnalyzer(nil)
	analyzer.CreateComponents = func(string) *analysis.TokenStreamComponents {
		tokenizer := testanalysis.NewMockTokenizer(testanalysis.SIMPLE, true, testanalysis.DefaultMaxTokenLength)
		tokenizer.SetEnableChecks(false) // disable workflow checking as we forcefully close() in exceptional cases.
		return &analysis.TokenStreamComponents{
			Source: func(r io.Reader) error {
				tokenizer.SetReader(r)
				return nil
			},
			Sink: &exceptionFromTokenStreamFilter{BaseTokenFilter: analysis.NewBaseTokenFilter(tokenizer)},
		}
	}
	conf := newIndexWriterConfigWithAnalyzer(analyzer)
	conf.SetMaxBufferedDocs(max(3, conf.GetMaxBufferedDocs()))
	conf.SetMergePolicy(index.NewNoMergePolicy())

	writer := mustNewIndexWriter(t, dir, conf)

	brokenDoc := document.NewDocument()
	contents := "aa bb cc dd ee ff gg hh ii jj kk"
	brokenDoc.Add(newTextField(t, "content", contents, false))
	if _, err := writer.AddDocument(brokenDoc); err == nil {
		t.Fatal("expected an exception from addDocument(brokenDoc)")
	}

	// Make sure we can add another normal document
	doc := document.NewDocument()
	doc.Add(newTextField(t, "content", "aa bb cc dd", false))
	mustAddDocument(t, writer, doc)

	// Make sure we can add another normal document
	doc = document.NewDocument()
	doc.Add(newTextField(t, "content", "aa bb cc dd", false))
	mustAddDocument(t, writer, doc)

	mustClose(t, writer)
	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader, dir)
	// assertEquals(3, reader.docFreq(new Term("content", "aa")))
	t.Fatal(indexReaderDocFreqMissing)
}

// make sure an aborting exception closes the writer:
func TestIndexWriterExceptionsDocumentsWriterAbort(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	// FailOnlyOnFlush.eval calls callStackContainsAnyOf("flush") and
	// callStackContainsAnyOf("finishDocument").
	t.Fatal(callStackContainsMissing)
}

// exceptionsLogMergePolicy returns the writer's LogMergePolicy, as the Java
// cast (LogMergePolicy) writer.getConfig().getMergePolicy() does.
func exceptionsLogMergePolicy(t testing.TB, w *index.IndexWriter) logMergePolicy {
	t.Helper()
	lmp, ok := w.GetConfig().GetMergePolicy().(logMergePolicy)
	if !ok {
		t.Fatalf("merge policy %T is not a LogMergePolicy", w.GetConfig().GetMergePolicy())
	}
	return lmp
}

func TestIndexWriterExceptionsDocumentsWriterExceptions(t *testing.T) {
	analyzer := crashingAnalyzer(alwaysCrash)

	for i := 0; i < 2; i++ {
		dir := newDirectory()
		conf := newIndexWriterConfigWithAnalyzer(analyzer)
		conf.SetMergePolicy(newLogMergePolicy())
		writer := mustNewIndexWriter(t, dir, conf)

		// don't allow a sudden merge to clean up the deleted
		// doc below:
		lmp := exceptionsLogMergePolicy(t, writer)
		lmp.SetMergeFactor(max(lmp.GetMergeFactor(), 5))

		doc := document.NewDocument()
		doc.Add(newField(t, "contents", "here are some contents", docCopyIteratorCustom5))
		mustAddDocument(t, writer, doc)
		mustAddDocument(t, writer, doc)
		doc.Add(newField(t, "crash", "this should crash after 4 terms", docCopyIteratorCustom5))
		doc.Add(newField(t, "other", "this will not get indexed", docCopyIteratorCustom5))
		if _, err := writer.AddDocument(doc); err == nil {
			t.Fatal("expected IOException from addDocument(crash doc)")
		}

		if i == 0 {
			doc = document.NewDocument()
			doc.Add(newField(t, "contents", "here are some contents", docCopyIteratorCustom5))
			mustAddDocument(t, writer, doc)
			mustAddDocument(t, writer, doc)
		}
		mustClose(t, writer)

		reader := mustOpenDirectoryReader(t, dir)
		if i == 0 {
			mustClose(t, reader, dir)
			// assertEquals(expected, reader.docFreq(new Term("contents", "here")))
			t.Fatal(indexReaderDocFreqMissing)
		}
		mustClose(t, reader, dir)
	}
}

// keepFullyDeletedSegmentsPolicy renders the anonymous
// FilterMergePolicy(NoMergePolicy.INSTANCE) overriding
// keepFullyDeletedSegment to return true. Gocene's
// MergePolicy.KeepFullyDeletedSegment takes the SegmentCommitInfo instead of
// the IOSupplier<CodecReader>; the override ignores its argument, so the
// rendering is unaffected.
type keepFullyDeletedSegmentsPolicy struct {
	*index.FilterMergePolicy
}

func (keepFullyDeletedSegmentsPolicy) KeepFullyDeletedSegment(*index.SegmentCommitInfo) bool {
	return true
}

func newKeepFullyDeletedSegmentsPolicy() keepFullyDeletedSegmentsPolicy {
	return keepFullyDeletedSegmentsPolicy{FilterMergePolicy: index.NewFilterMergePolicy(index.NewNoMergePolicy())}
}

func TestIndexWriterExceptionsDocumentsWriterExceptionFailOneDoc(t *testing.T) {
	analyzer := crashingAnalyzer(alwaysCrash)
	for i := 0; i < 10; i++ {
		dir := newDirectory()
		conf := newIndexWriterConfigWithAnalyzer(analyzer)
		conf.SetMaxBufferedDocs(-1)
		if rand.Intn(2) == 0 {
			conf.SetRAMBufferSizeMB(0.00001)
		} else {
			conf.SetRAMBufferSizeMB(math.MaxInt32)
		}
		conf.SetMergePolicy(newKeepFullyDeletedSegmentsPolicy())
		writer := mustNewIndexWriter(t, dir, conf)
		doc := document.NewDocument()
		doc.Add(newField(t, "contents", "here are some contents", docCopyIteratorCustom5))
		mustAddDocument(t, writer, doc)
		doc.Add(newField(t, "crash", "this should crash after 4 terms", docCopyIteratorCustom5))
		doc.Add(newField(t, "other", "this will not get indexed", docCopyIteratorCustom5))
		if _, err := writer.AddDocument(doc); err == nil {
			t.Fatal("expected IOException from addDocument(crash doc)")
		}
		mustCommit(t, writer)
		reader := mustOpenDirectoryReader(t, dir)
		mustClose(t, reader, writer, dir)
		// assertEquals(2, reader.docFreq(new Term("contents", "here")))
		t.Fatal(indexReaderDocFreqMissing)
	}
}

func TestIndexWriterExceptionsDocumentsWriterExceptionThreads(t *testing.T) {
	analyzer := crashingAnalyzer(alwaysCrash)

	const numThread = 3
	numIter := atLeast(10)

	for i := 0; i < 2; i++ {
		dir := newDirectory()
		{
			conf := newIndexWriterConfigWithAnalyzer(analyzer)
			conf.SetMaxBufferedDocs(math.MaxInt32)
			conf.SetRAMBufferSizeMB(-1) // we don't want to flush automatically
			// don't use a merge policy here they depend on the DWPThreadPool
			// and its max thread states etc. we also need to keep fully
			// deleted segments since otherwise we clean up fully deleted ones
			// and if we flush the one that has only the failed document the
			// docFreq checks will be off below.
			conf.SetMergePolicy(newKeepFullyDeletedSegmentsPolicy())
			writer := mustNewIndexWriter(t, dir, conf)

			finalI := i

			var wg sync.WaitGroup
			for th := 0; th < numThread; th++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for iter := 0; iter < numIter; iter++ {
						doc := document.NewDocument()
						f, err := document.NewField("contents", "here are some contents", docCopyIteratorCustom5)
						if err != nil {
							t.Errorf("new Field: %v", err)
							return
						}
						doc.Add(f)
						if _, err := writer.AddDocument(doc); err != nil {
							t.Errorf("ERROR: hit unexpected exception: %v", err)
							return
						}
						if _, err := writer.AddDocument(doc); err != nil {
							t.Errorf("ERROR: hit unexpected exception: %v", err)
							return
						}
						crash, err := document.NewField("crash", "this should crash after 4 terms", docCopyIteratorCustom5)
						if err != nil {
							t.Errorf("new Field: %v", err)
							return
						}
						doc.Add(crash)
						other, err := document.NewField("other", "this will not get indexed", docCopyIteratorCustom5)
						if err != nil {
							t.Errorf("new Field: %v", err)
							return
						}
						doc.Add(other)
						if _, err := writer.AddDocument(doc); err == nil {
							t.Errorf("expected IOException from addDocument(crash doc)")
							return
						}

						if finalI == 0 {
							extraDoc := document.NewDocument()
							extra, err := document.NewField("contents", "here are some contents", docCopyIteratorCustom5)
							if err != nil {
								t.Errorf("new Field: %v", err)
								return
							}
							extraDoc.Add(extra)
							if _, err := writer.AddDocument(extraDoc); err != nil {
								t.Errorf("ERROR: hit unexpected exception: %v", err)
								return
							}
							if _, err := writer.AddDocument(extraDoc); err != nil {
								t.Errorf("ERROR: hit unexpected exception: %v", err)
								return
							}
						}
					}
				}()
			}

			wg.Wait()

			mustClose(t, writer)
		}

		reader := mustOpenDirectoryReader(t, dir)
		mustClose(t, reader, dir)
		// assertEquals("i=" + i, expected, reader.docFreq(new Term("contents", "here")))
		t.Fatal(indexReaderDocFreqMissing)
	}
}

// LUCENE-1044: test exception during sync
func TestIndexWriterExceptionsExceptionDuringSync(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	// FailOnlyInSync.eval calls callStackContains(MockDirectoryWrapper.class, "sync").
	t.Fatal(callStackContainsClassMissing)
}

func TestIndexWriterExceptionsExceptionsDuringCommit(t *testing.T) {
	// FailOnlyInCommit.eval calls callStackContains(SegmentInfos.class, stage)
	// and callStackContains(MockDirectoryWrapper.class, ...).
	t.Fatal(callStackContainsClassMissing)
}

func TestIndexWriterExceptionsForceMergeExceptions(t *testing.T) {
	startDir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	conf.SetMergePolicy(newLogMergePolicy())
	conf.GetMergePolicy().(logMergePolicy).SetMergeFactor(100)
	w := mustNewIndexWriter(t, startDir, conf)
	for i := 0; i < 27; i++ {
		testIndexWriterAddDoc(t, w)
	}
	mustClose(t, w)

	iter := 10
	if testNightly {
		iter = 200
	}
	for i := 0; i < iter; i++ {
		ramCopy, err := testutil.RamCopyOf(startDir)
		if err != nil {
			t.Fatalf("TestUtil.ramCopyOf: %v", err)
		}
		dir := store.NewMockDirectoryWrapper(ramCopy)
		conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		cms := index.NewConcurrentMergeScheduler()
		cms.SetSuppressExceptions()
		conf.SetMergeScheduler(cms)
		w = mustNewIndexWriter(t, dir, conf)
		dir.SetRandomIOExceptionRate(0.5)
		if err := w.ForceMerge(1); err != nil {
			if !errors.Is(err, store.ErrIllegalState) && errors.Unwrap(err) == nil {
				t.Fatalf("forceMerge threw IOException without root cause: %v", err)
			}
		}
		dir.SetRandomIOExceptionRate(0)
		if err := w.Close(); err != nil && !errors.Is(err, store.ErrIllegalState) {
			t.Fatalf("close: %v", err)
		}
		mustClose(t, dir)
	}
	mustClose(t, startDir)
}

// fakeOutOfMemoryError renders the java.lang.OutOfMemoryError the evil
// InfoStreams throw; an Error thrown from InfoStream#message propagates in
// Go as a panic, because InfoStream.Message has no error result.
type fakeOutOfMemoryError struct{ msg string }

// throwingInfoStream renders the anonymous InfoStreams that throw from
// message(String, String).
type throwingInfoStream struct {
	enabled func(component string) bool
	message func(component, message string)
}

func (s *throwingInfoStream) Message(component, message string) { s.message(component, message) }
func (s *throwingInfoStream) IsEnabled(component string) bool   { return s.enabled(component) }
func (s *throwingInfoStream) Close() error                      { return nil }

func allComponentsEnabled(string) bool { return true }

// expectThrowsOutOfMemoryError renders expectThrows(OutOfMemoryError.class, fn).
func expectThrowsOutOfMemoryError(t testing.TB, what string, fn func() error) {
	t.Helper()
	thrown := func() (oome bool) {
		defer func() {
			if p := recover(); p != nil {
				if _, ok := p.(fakeOutOfMemoryError); !ok {
					panic(p)
				}
				oome = true
			}
		}()
		if err := fn(); err != nil {
			t.Fatalf("%s: expected OutOfMemoryError, got %v", what, err)
		}
		return false
	}()
	if !thrown {
		t.Fatalf("%s: expected OutOfMemoryError", what)
	}
}

// LUCENE-1429
func TestIndexWriterExceptionsOutOfMemoryErrorCausesCloseToFail(t *testing.T) {
	var thrown atomic.Bool
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetInfoStream(&throwingInfoStream{
		enabled: allComponentsEnabled,
		message: func(component, message string) {
			if strings.HasPrefix(message, "now flush at close") && thrown.CompareAndSwap(false, true) {
				panic(fakeOutOfMemoryError{msg: "fake OOME at " + message})
			}
		},
	})
	writer := mustNewIndexWriter(t, dir, conf)

	expectThrowsOutOfMemoryError(t, "writer.close()", writer.Close)

	// throws IllegalStateEx w/o bug fix
	mustClose(t, writer, dir)
}

// If IW hits OOME during indexing, it should refuse to commit any further changes.
func TestIndexWriterExceptionsOutOfMemoryErrorRollback(t *testing.T) {
	var thrown atomic.Bool
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetInfoStream(&throwingInfoStream{
		enabled: allComponentsEnabled,
		message: func(component, message string) {
			if strings.Contains(message, "startFullFlush") && thrown.CompareAndSwap(false, true) {
				panic(fakeOutOfMemoryError{msg: "fake OOME at " + message})
			}
		},
	})
	writer := mustNewIndexWriter(t, dir, conf)
	mustAddDocument(t, writer, document.NewDocument())

	expectThrowsOutOfMemoryError(t, "writer.commit()", func() error {
		_, err := writer.Commit()
		return err
	})

	// Java tolerates an IllegalArgumentException here; Gocene has no typed
	// IllegalArgumentException, so any close failure is reported.
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	_, err := writer.AddDocument(document.NewDocument())
	dwdqExpectAlreadyClosed(t, err)

	// IW should have done rollback() during close, since it hit OOME, and so
	// no index should exist:
	exists, err := index.IndexExists(dir)
	if err != nil {
		t.Fatalf("indexExists: %v", err)
	}
	if exists {
		t.Fatal("assertFalse(DirectoryReader.indexExists(dir))")
	}

	mustClose(t, dir)
}

// LUCENE-1347
func TestIndexWriterExceptionsRollbackExceptionHang(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal(mockIndexWriterMissing)
}

// LUCENE-1044: Simulate checksum error in segments_N
func TestIndexWriterExceptionsSegmentsChecksumError(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal(setCheckIndexOnCloseMissing) // we corrupt the index
}

// Simulate a corrupt index by removing last byte of
// latest segments file and make sure we get an
// IOException trying to open the index:
func TestIndexWriterExceptionsSimulatedCorruptIndex1(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal(setCheckIndexOnCloseMissing) // we are corrupting it!
}

// Simulate a corrupt index by removing one of the
// files and make sure we get an IOException trying to
// open the index:
func TestIndexWriterExceptionsSimulatedCorruptIndex2(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal(setCheckIndexOnCloseMissing) // we are corrupting it!
}

func TestIndexWriterExceptionsTermVectorExceptions(t *testing.T) {
	// FailOnTermVectors.eval calls callStackContains(TermVectorsConsumer.class, stage).
	t.Fatal(callStackContainsClassMissing)
}

// crashTokenStreamField renders the "crash" field of the non-aborting tests:
// new Field("crash", new CrashingFilter("crash", tokenizer),
// TextField.TYPE_NOT_STORED) over a WHITESPACE MockTokenizer reading
// "crash me on the 4th token".
func crashTokenStreamField(t testing.TB) *document.Field {
	t.Helper()
	tokenizer := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, false, testanalysis.DefaultMaxTokenLength)
	tokenizer.SetReader(strings.NewReader("crash me on the 4th token"))
	tokenizer.SetEnableChecks(false) // disable workflow checking as we forcefully close() in exceptional cases.
	f, err := document.NewField("crash", newCrashingFilter("crash", tokenizer), document.TextFieldTypeNotStored)
	if err != nil {
		t.Fatalf("new Field(crash): %v", err)
	}
	return f
}

// expectCrashIOException renders
// assertEquals(CRASH_FAIL_MESSAGE, expectThrows(IOException.class, ...).getMessage()).
func expectCrashIOException(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected IOException")
	}
	if err.Error() != crashFailMessage {
		t.Fatalf("expected.getMessage(): expected %q, got %q", crashFailMessage, err.Error())
	}
}

func addGoodContentDocs(t testing.TB, w interface {
	AddDocument(*document.Document) (int64, error)
}, n int) {
	t.Helper()
	for docCount := 0; docCount < n; docCount++ {
		doc := document.NewDocument()
		doc.Add(newTextField(t, "content", "good content", false))
		if _, err := w.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
}

func TestIndexWriterExceptionsAddDocsNonAbortingException(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	numDocs1 := rand.Intn(25)
	addGoodContentDocs(t, w, numDocs1)

	docs := make([]*document.Document, 0, 7)
	for docCount := 0; docCount < 7; docCount++ {
		doc := document.NewDocument()
		docs = append(docs, doc)
		doc.Add(newStringField(t, "id", strconv.Itoa(docCount), false))
		doc.Add(newTextField(t, "content", "silly content "+strconv.Itoa(docCount), false))
		if docCount == 4 {
			doc.Add(crashTokenStreamField(t))
		}
	}

	_, err := w.AddDocuments(docs)
	expectCrashIOException(t, err)

	numDocs2 := rand.Intn(25)
	addGoodContentDocs(t, w, numDocs2)

	r, err := w.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	mustClose(t, w)
	defer mustClose(t, r, dir)

	newSearcher(t, r)
}

func TestIndexWriterExceptionsUpdateDocsNonAbortingException(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	numDocs1 := rand.Intn(25)
	addGoodContentDocs(t, w, numDocs1)

	// Use addDocs (no exception) to get docs in the index:
	var docs []*document.Document
	numDocs2 := rand.Intn(25)
	for docCount := 0; docCount < numDocs2; docCount++ {
		doc := document.NewDocument()
		docs = append(docs, doc)
		doc.Add(newStringField(t, "subid", "subs", false))
		doc.Add(newStringField(t, "id", strconv.Itoa(docCount), false))
		doc.Add(newTextField(t, "content", "silly content "+strconv.Itoa(docCount), false))
	}
	if _, err := w.AddDocuments(docs); err != nil {
		t.Fatalf("addDocuments: %v", err)
	}

	numDocs3 := rand.Intn(25)
	addGoodContentDocs(t, w, numDocs3)

	docs = docs[:0]
	limit := nextInt(2, 25)
	crashAt := rand.Intn(limit)
	for docCount := 0; docCount < limit; docCount++ {
		doc := document.NewDocument()
		docs = append(docs, doc)
		doc.Add(newStringField(t, "id", strconv.Itoa(docCount), false))
		doc.Add(newTextField(t, "content", "silly content "+strconv.Itoa(docCount), false))
		if docCount == crashAt {
			doc.Add(crashTokenStreamField(t))
		}
	}

	_, err := w.UpdateDocuments(index.NewTerm("subid", "subs"), docs)
	expectCrashIOException(t, err)

	numDocs4 := rand.Intn(25)
	addGoodContentDocs(t, w, numDocs4)

	r, err := w.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	mustClose(t, w)
	defer mustClose(t, r, dir)

	newSearcher(t, r)
}

// storedFieldNullStringMissing names the Java constructors and setters that
// accept a null String, which Gocene's string-typed API cannot express.
const (
	storedFieldNullStringMissing   = "org.apache.lucene.document.StoredField(String, String) accepting a null value is not ported"
	fieldSetStringValueNullMissing = "org.apache.lucene.document.Field#setStringValue(String) accepting a null value is not ported"
)

// expectThrowsErrorOrPanic renders expectThrows(...) around a block whose
// Java exception Gocene reports either as a returned error or, for the
// setters without an error result, as a panic.
func expectThrowsErrorOrPanic(t testing.TB, what string, fn func() error) {
	t.Helper()
	thrown := func() (p bool) {
		defer func() {
			if recover() != nil {
				p = true
			}
		}()
		return fn() != nil
	}()
	if !thrown {
		t.Fatalf("expected an exception from %s", what)
	}
}

// nullStoredFieldWriter renders the common prologue of the testNullStored*
// tests: an IndexWriter over new IndexWriterConfig(new MockAnalyzer(random()))
// that has indexed one good document.
func nullStoredFieldWriter(t *testing.T, dir store.Directory, doc *document.Document) *index.IndexWriter {
	t.Helper()
	iw := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	// add good document
	mustAddDocument(t, iw, doc)
	return iw
}

// test a null string value doesn't abort the entire segment
func TestIndexWriterExceptionsNullStoredField(t *testing.T) {
	dir := newDirectory()
	iw := nullStoredFieldWriter(t, dir, document.NewDocument())
	defer mustClose(t, iw, dir)
	// set to null value
	t.Fatal(storedFieldNullStringMissing)
}

// test a null string value doesn't abort the entire segment
func TestIndexWriterExceptionsNullStoredFieldReuse(t *testing.T) {
	dir := newDirectory()
	doc := document.NewDocument()
	theField, err := document.NewStoredFieldFromStringWithType("foo", "hello", document.StoredFieldType)
	if err != nil {
		t.Fatalf("new StoredField: %v", err)
	}
	doc.Add(theField)
	iw := nullStoredFieldWriter(t, dir, doc)
	defer mustClose(t, iw, dir)
	// set to null value
	t.Fatal(fieldSetStringValueNullMissing)
}

// nullStoredBytesTail renders the shared tail of the null byte[]/BytesRef
// tests: the good document must survive, which the Java test checks after
// assertNull(iw.getTragicException()).
func nullStoredBytesTail(t *testing.T) {
	t.Helper()
	t.Fatal(indexWriterGetTragicExceptionMissing)
}

// test a null byte[] value doesn't abort the entire segment
func TestIndexWriterExceptionsNullStoredBytesField(t *testing.T) {
	dir := newDirectory()
	doc := document.NewDocument()
	iw := nullStoredFieldWriter(t, dir, doc)
	defer mustClose(t, iw, dir)

	expectThrowsErrorOrPanic(t, "new StoredField(\"foo\", (byte[]) null) + addDocument", func() error {
		// set to null value
		var v []byte
		theField, err := document.NewStoredFieldFromBytes("foo", v)
		if err != nil {
			return err
		}
		doc.Add(theField)
		_, err = iw.AddDocument(doc)
		return err
	})

	nullStoredBytesTail(t)
}

// test a null byte[] value doesn't abort the entire segment
func TestIndexWriterExceptionsNullStoredBytesFieldReuse(t *testing.T) {
	dir := newDirectory()
	doc := document.NewDocument()
	theField, err := document.NewStoredFieldFromBytes("foo", []byte("hello"))
	if err != nil {
		t.Fatalf("new StoredField: %v", err)
	}
	doc.Add(theField)
	iw := nullStoredFieldWriter(t, dir, doc)
	defer mustClose(t, iw, dir)

	expectThrowsErrorOrPanic(t, "theField.setBytesValue((byte[]) null) + addDocument", func() error {
		// set to null value
		var v []byte
		theField.SetBytesValue(v)
		_, err := iw.AddDocument(doc)
		return err
	})

	nullStoredBytesTail(t)
}

// test a null bytesref value doesn't abort the entire segment
func TestIndexWriterExceptionsNullStoredBytesRefField(t *testing.T) {
	dir := newDirectory()
	doc := document.NewDocument()
	iw := nullStoredFieldWriter(t, dir, doc)
	defer mustClose(t, iw, dir)

	// Gocene renders StoredField(String, BytesRef) with the []byte constructor.
	expectThrowsErrorOrPanic(t, "new StoredField(\"foo\", (BytesRef) null) + addDocument", func() error {
		// set to null value
		var v []byte
		theField, err := document.NewStoredFieldFromBytes("foo", v)
		if err != nil {
			return err
		}
		doc.Add(theField)
		_, err = iw.AddDocument(doc)
		return err
	})

	nullStoredBytesTail(t)
}

// test a null bytesref value doesn't abort the entire segment
func TestIndexWriterExceptionsNullStoredBytesRefFieldReuse(t *testing.T) {
	dir := newDirectory()
	doc := document.NewDocument()
	theField, err := document.NewStoredFieldFromBytes("foo", []byte("hello"))
	if err != nil {
		t.Fatalf("new StoredField: %v", err)
	}
	doc.Add(theField)
	iw := nullStoredFieldWriter(t, dir, doc)
	defer mustClose(t, iw, dir)

	expectThrowsErrorOrPanic(t, "theField.setBytesValue((BytesRef) null) + addDocument", func() error {
		// set to null value
		var v []byte
		theField.SetBytesValue(v)
		_, err := iw.AddDocument(doc)
		return err
	})

	nullStoredBytesTail(t)
}

// test a null data input value doesn't abort the entire segment
func TestIndexWriterExceptionsNullStoredDataInputField(t *testing.T) {
	dir := newDirectory()
	doc := document.NewDocument()
	iw := nullStoredFieldWriter(t, dir, doc)
	defer mustClose(t, iw, dir)

	expectThrowsErrorOrPanic(t, "new StoredField(\"foo\", (StoredFieldDataInput) null) + addDocument", func() error {
		// set to null value
		theField, err := document.NewStoredFieldFromDataInput("foo", nil)
		if err != nil {
			return err
		}
		doc.Add(theField)
		_, err = iw.AddDocument(doc)
		return err
	})

	nullStoredBytesTail(t)
}

// crazyPositionIncrementGapAnalyzer renders the anonymous Analyzer of
// testCrazyPositionIncrementGap.
type crazyPositionIncrementGapAnalyzer struct {
	*analysis.BaseAnalyzer
}

func (crazyPositionIncrementGapAnalyzer) GetPositionIncrementGap(string) int { return -2 }

func TestIndexWriterExceptionsCrazyPositionIncrementGap(t *testing.T) {
	dir := newDirectory()
	base := analysis.NewAnalyzer(nil)
	base.CreateComponents = func(string) *analysis.TokenStreamComponents {
		tokenizer := testanalysis.NewMockTokenizer(testanalysis.KEYWORD, false, testanalysis.DefaultMaxTokenLength)
		return &analysis.TokenStreamComponents{
			Source: func(r io.Reader) error {
				tokenizer.SetReader(r)
				return nil
			},
			Sink: tokenizer,
		}
	}
	analyzer := crazyPositionIncrementGapAnalyzer{BaseAnalyzer: base}
	iw := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(analyzer))
	defer mustClose(t, iw, dir)
	// add good document
	doc := document.NewDocument()
	mustAddDocument(t, iw, doc)
	doc.Add(newTextField(t, "foo", "bar", false))
	doc.Add(newTextField(t, "foo", "bar", false))
	if _, err := iw.AddDocument(doc); err == nil {
		t.Fatal("expected IllegalArgumentException from addDocument")
	}

	t.Fatal(indexWriterGetTragicExceptionMissing)
}

// uoeDirectory renders the static UOEDirectory: a FilterDirectory over a
// ByteBuffersDirectory whose openInput, when doFail is set, throws
// UnsupportedOperationException for segments_N files opened from
// readCommit/readLatestCommit — a check that needs
// callStackContainsAnyOf(String...), which is not ported.
type uoeDirectory struct {
	*store.FilterDirectory
	doFail bool
}

func TestIndexWriterExceptionsExceptionOnCtor(t *testing.T) {
	uoe := &uoeDirectory{FilterDirectory: store.NewFilterDirectory(store.NewByteBuffersDirectory())}
	d := store.NewMockDirectoryWrapper(uoe)
	iw := mustNewIndexWriter(t, d, newIndexWriterConfigWithAnalyzer(nil))
	mustAddDocument(t, iw, document.NewDocument())
	mustClose(t, iw)
	uoe.doFail = true
	defer mustClose(t, d)
	// new IndexWriter(d, ...) now reaches UOEDirectory#openInput, whose
	// condition is callStackContainsAnyOf("readCommit", "readLatestCommit").
	t.Fatal(callStackContainsMissing)
}

var assertFilesExistPattern = regexp.MustCompile(`^file .* does not exist; files=\[.*\]$`)

// See LUCENE-4870 TooManyOpenFiles errors are thrown as
// FNFExceptions which can trigger data loss.
func TestIndexWriterExceptionsTooManyFileException(t *testing.T) {
	// Create failure that throws Too many open files exception randomly
	failure := &store.Failure{}
	failure.SetEval(func(*store.MockDirectoryWrapper) error {
		if failure.DoFail() {
			if rand.Intn(2) == 0 {
				return fmt.Errorf("%w: some/file/name.ext (Too many open files)", store.ErrFileNotFound)
			}
		}
		return nil
	})

	dir := newDirectory()
	// The exception is only thrown on open input
	dir.SetFailOnOpenInput(true)
	dir.FailOn(failure)

	// Create an index with one document
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iw := mustNewIndexWriter(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "foo", "bar", false))
	mustAddDocument(t, iw, doc) // add a document
	mustCommit(t, iw)
	ir := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 1, ir)
	mustClose(t, ir, iw)

	// Open and close the index a few times
	for i := 0; i < 10; i++ {
		failure.SetDoFail()
		iwc = index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		newWriter, err := index.NewIndexWriter(dir, iwc)
		if err != nil {
			var cie *index.CorruptIndexException
			switch {
			case assertFilesExistPattern.MatchString(err.Error()):
				// This is fine: we tripped IW's assert that all files it's
				// about to fsync do exist
			case errors.As(err, &cie):
				// Exceptions are fine - we are running out of file handlers here
				continue
			case isFileNotFoundOrNoSuchFile(err):
				continue
			default:
				t.Fatalf("new IndexWriter: %v", err)
			}
		} else {
			iw = newWriter
		}
		failure.ClearDoFail()
		mustClose(t, iw)
		ir = mustOpenDirectoryReader(t, dir)
		if ir.NumDocs() != 1 {
			t.Fatalf("lost document after iteration: %d: numDocs=%d", i, ir.NumDocs())
		}
		mustClose(t, ir)
	}

	// Check if document is still there
	failure.ClearDoFail()
	ir = mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 1, ir)
	mustClose(t, ir, dir)
}

func TestIndexWriterExceptionsExceptionDuringRollback(t *testing.T) {
	// currently: fail in two different places
	messageToFailOn := "rollback before checkpoint"
	if rand.Intn(2) == 0 {
		messageToFailOn = "rollback: done finish merges"
	}

	// infostream that throws exception during rollback
	evilInfoStream := &throwingInfoStream{
		enabled: allComponentsEnabled,
		message: func(component, message string) {
			if messageToFailOn == message {
				panic("BOOM!")
			}
		},
	}

	dir := newDirectory() // we want to ensure we don't leak any locks or file handles
	defer mustClose(t, dir)
	iwc := index.NewIndexWriterConfigWithAnalyzer(nil)
	iwc.SetInfoStream(evilInfoStream)
	t.Fatal(isEnableTestPointsOverrideMissing)
}

func TestIndexWriterExceptionsRandomExceptionDuringRollback(t *testing.T) {
	// fail in random places on i/o; the first of the RANDOM_MULTIPLIER * 75
	// iterations installs a Failure whose eval calls
	// callStackContainsAnyOf("rollbackInternal").
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal(callStackContainsMissing)
}

func TestIndexWriterExceptionsOnlyRollbackOnceOnException(t *testing.T) {
	var once atomic.Bool
	stream := &throwingInfoStream{
		enabled: func(component string) bool { return component == "TP" },
		message: func(component, message string) {
			if component == "TP" && message == "rollback before checkpoint" {
				if once.CompareAndSwap(false, true) {
					panic("boom")
				}
				panic("has been rolled back twice")
			}
		},
	}
	dir := newDirectory()
	defer mustClose(t, dir)
	conf := newIndexWriterConfig()
	conf.SetInfoStream(stream)
	t.Fatal(isEnableTestPointsOverrideMissing)
}

func TestIndexWriterExceptionsExceptionOnSyncMetadata(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfig()
	conf.SetCommitOnClose(false)
	writer := mustNewIndexWriter(t, dir, conf)
	mustCommit(t, writer)
	defer mustClose(t, writer, dir)
	// The Failure's eval calls callStackContains(MockDirectoryWrapper.class,
	// "syncMetaData") and callStackContains(SegmentInfos.class, "finishCommit").
	t.Fatal(callStackContainsClassMissing)
}

func TestIndexWriterExceptionsExceptionJustBeforeFlushWithPointValues(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	analyzer := crashingAnalyzer(alwaysCrash)
	iwc := newIndexWriterConfigWithAnalyzer(analyzer)
	iwc.SetCommitOnClose(false)
	iwc.SetMaxBufferedDocs(3)
	t.Fatal(softDeletesRetentionSupplierMissing)
}
