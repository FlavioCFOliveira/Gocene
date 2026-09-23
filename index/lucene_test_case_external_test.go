// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"errors"
	"io/fs"
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

// This file renders, for the external index_test package, the members of
// org.apache.lucene.tests.util.LuceneTestCase and TestUtil that the ported
// Lucene index tests call, plus helpers for recurring Lucene test idioms.

// newDirectory renders LuceneTestCase.newDirectory(): a MockDirectoryWrapper
// over an in-memory directory.
func newDirectory() *store.MockDirectoryWrapper {
	return store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
}

// newMockAnalyzer renders new MockAnalyzer(random()): MockTokenizer.WHITESPACE,
// lower-casing, no stop words.
func newMockAnalyzer() analysis.Analyzer {
	return testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true)
}

// newIndexWriterConfig renders LuceneTestCase.newIndexWriterConfig(), which
// uses new MockAnalyzer(random()).
func newIndexWriterConfig() *index.IndexWriterConfig {
	return index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
}

// newIndexWriterConfigWithAnalyzer renders
// LuceneTestCase.newIndexWriterConfig(Analyzer).
func newIndexWriterConfigWithAnalyzer(a analysis.Analyzer) *index.IndexWriterConfig {
	return index.NewIndexWriterConfigWithAnalyzer(a)
}

// atLeast renders LuceneTestCase.atLeast(int) with RANDOM_MULTIPLIER = 1 and
// TEST_NIGHTLY = false: a random value in [i, i + i/2].
func atLeast(i int) int {
	return i + rand.Intn(i/2+1)
}

// rarely renders LuceneTestCase.rarely() (1% with the default multipliers).
func rarely() bool {
	return rand.Intn(100) >= 99
}

// usually renders LuceneTestCase.usually(): !rarely().
func usually() bool {
	return !rarely()
}

// nextInt renders TestUtil.nextInt(random(), start, end): a value in
// [start, end].
func nextInt(start, end int) int {
	return start + rand.Intn(end-start+1)
}

// getOnlyLeafReader renders LuceneTestCase.getOnlyLeafReader(IndexReader).
func getOnlyLeafReader(t testing.TB, reader interface {
	Leaves() ([]*index.LeafReaderContext, error)
}) index.LeafReader {
	t.Helper()
	subReaders, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	if len(subReaders) != 1 {
		t.Fatalf("reader has %d segments instead of exactly one", len(subReaders))
	}
	return subReaders[0].LeafReader()
}

// newStringField renders LuceneTestCase.newStringField(String, String, Store).
func newStringField(t testing.TB, name, value string, stored bool) *document.StringField {
	t.Helper()
	f, err := document.NewStringField(name, value, stored)
	if err != nil {
		t.Fatalf("newStringField: %v", err)
	}
	return f
}

// newTextField renders LuceneTestCase.newTextField(String, String, Store).
func newTextField(t testing.TB, name, value string, stored bool) *document.TextField {
	t.Helper()
	f, err := document.NewTextField(name, value, stored)
	if err != nil {
		t.Fatalf("newTextField: %v", err)
	}
	return f
}

// newField renders LuceneTestCase.newField(String, String, FieldType).
func newField(t testing.TB, name, value string, ft *document.FieldType) *document.Field {
	t.Helper()
	f, err := document.NewField(name, value, ft)
	if err != nil {
		t.Fatalf("newField: %v", err)
	}
	return f
}

// mustNewIndexWriter renders new IndexWriter(dir, conf).
func mustNewIndexWriter(t testing.TB, dir store.Directory, conf *index.IndexWriterConfig) *index.IndexWriter {
	t.Helper()
	w, err := index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	return w
}

// mustAddDocument renders writer.addDocument(doc).
func mustAddDocument(t testing.TB, w *index.IndexWriter, doc *document.Document) {
	t.Helper()
	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
}

// mustCommit renders writer.commit().
func mustCommit(t testing.TB, w *index.IndexWriter) {
	t.Helper()
	if _, err := w.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// mustClose renders IOUtils.close(closeables...).
func mustClose(t testing.TB, closers ...interface{ Close() error }) {
	t.Helper()
	for _, c := range closers {
		if err := c.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
}

// mustOpenDirectoryReader renders DirectoryReader.open(dir).
func mustOpenDirectoryReader(t testing.TB, dir store.Directory) *index.DirectoryReader {
	t.Helper()
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("DirectoryReader.open: %v", err)
	}
	return r
}

// iwDocStats renders IndexWriter.getDocStats(), failing the test on error.
func iwDocStats(t testing.TB, w *index.IndexWriter) index.DocStats {
	t.Helper()
	stats, err := w.GetDocStats()
	if err != nil {
		t.Fatalf("getDocStats: %v", err)
	}
	return stats
}

// newTestDocument renders `new Document()` followed by `doc.add(field)` for
// every field.
func newTestDocument(fields ...any) *document.Document {
	doc := document.NewDocument()
	for _, f := range fields {
		doc.Add(f.(document.IndexableField))
	}
	return doc
}

// newFSDirectory renders LuceneTestCase.newFSDirectory(createTempDir()): a
// MockDirectoryWrapper over a file-system directory in a fresh temporary
// directory.
func newFSDirectory(t testing.TB) *store.MockDirectoryWrapper {
	t.Helper()
	dir, err := store.NewNIOFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("newFSDirectory: %v", err)
	}
	return store.NewMockDirectoryWrapper(dir)
}

// logMergePolicy is the part of org.apache.lucene.index.LogMergePolicy that
// LuceneTestCase.newLogMergePolicy configures; LogDocMergePolicy and
// LogByteSizeMergePolicy both provide it.
type logMergePolicy interface {
	index.MergePolicy
	SetCalibrateSizeByDeletes(bool)
	SetTargetSearchConcurrency(int)
	SetMergeFactor(int)
	GetMergeFactor() int
	SetNoCFSRatio(float64)
	SetMaxCFSSegmentSizeMB(float64)
}

// newLogMergePolicy renders LuceneTestCase.newLogMergePolicy(random()).
func newLogMergePolicy() logMergePolicy {
	var logmp logMergePolicy
	if rand.Intn(2) == 0 {
		logmp = index.NewLogDocMergePolicy()
	} else {
		logmp = index.NewLogByteSizeMergePolicy()
	}
	logmp.SetCalibrateSizeByDeletes(rand.Intn(2) == 0)
	logmp.SetTargetSearchConcurrency(nextInt(1, 16))
	if rarely() {
		logmp.SetMergeFactor(nextInt(2, 9))
	} else {
		logmp.SetMergeFactor(nextInt(10, 50))
	}
	configureRandomMergePolicy(logmp)
	return logmp
}

// newLogMergePolicyWithCFS renders LuceneTestCase.newLogMergePolicy(boolean useCFS, int mergeFactor).
func newLogMergePolicyWithCFS(useCFS bool, mergeFactor int) logMergePolicy {
	logmp := newLogMergePolicy()
	if useCFS {
		logmp.SetNoCFSRatio(1.0)
	} else {
		logmp.SetNoCFSRatio(0.0)
	}
	logmp.SetMergeFactor(mergeFactor)
	return logmp
}

// newLogMergePolicyWithMergeFactor renders LuceneTestCase.newLogMergePolicy(int mergeFactor).
func newLogMergePolicyWithMergeFactor(mergeFactor int) logMergePolicy {
	logmp := newLogMergePolicy()
	logmp.SetMergeFactor(mergeFactor)
	return logmp
}

// configureRandomMergePolicy renders the private
// LuceneTestCase.configureRandom(Random, MergePolicy).
func configureRandomMergePolicy(mergePolicy interface {
	SetNoCFSRatio(float64)
	SetMaxCFSSegmentSizeMB(float64)
}) {
	if rand.Intn(2) == 0 {
		mergePolicy.SetNoCFSRatio(0.1 + rand.Float64()*0.8)
	} else if rand.Intn(2) == 0 {
		mergePolicy.SetNoCFSRatio(1.0)
	} else {
		mergePolicy.SetNoCFSRatio(0.0)
	}
	if rarely() {
		mergePolicy.SetMaxCFSSegmentSizeMB(0.2 + rand.Float64()*2.0)
	} else {
		mergePolicy.SetMaxCFSSegmentSizeMB(math.Inf(1))
	}
}

// newRandomIndexWriterWithConfig renders new RandomIndexWriter(random(), dir, iwc).
func newRandomIndexWriterWithConfig(t testing.TB, dir store.Directory, iwc *index.IndexWriterConfig) *testindex.RandomIndexWriter {
	t.Helper()
	w, err := testindex.NewRandomIndexWriterWithConfig(rand.New(rand.NewSource(rand.Int63())), dir, iwc)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}
	return w
}

// newRandomIndexWriter renders new RandomIndexWriter(random(), dir).
func newRandomIndexWriter(t testing.TB, dir store.Directory) *testindex.RandomIndexWriter {
	t.Helper()
	w, err := testindex.NewRandomIndexWriter(rand.New(rand.NewSource(rand.Int63())), dir)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}
	return w
}

// storedDocument renders StoredFields.document(int): it visits the stored
// fields of docID with a DocumentStoredFieldVisitor.
func storedDocument(t testing.TB, storedFields interface {
	Document(docID int, visitor spi.StoredFieldVisitor) error
}, docID int) *document.Document {
	t.Helper()
	visitor := document.NewDocumentStoredFieldVisitor()
	if err := storedFields.Document(docID, visitor); err != nil {
		t.Fatalf("document(%d): %v", docID, err)
	}
	return visitor.GetDocument()
}

// docGet renders Document.get(String): the string value of the first field
// with the given name, or nil when absent.
func docGet(doc *document.Document, name string) *string {
	f := doc.Get(name)
	if f == nil {
		return nil
	}
	v := f.StringValue()
	return &v
}

// slowFileExists renders LuceneTestCase.slowFileExists(Directory, String):
// opens the file with IOContext.READONCE and closes it; only a missing-file
// error (NoSuchFileException/FileNotFoundException) answers false, any other
// error propagates.
func slowFileExists(t testing.TB, dir store.Directory, fileName string) bool {
	t.Helper()
	in, err := dir.OpenInput(fileName, store.IOContextReadOnce)
	if err != nil {
		if errors.Is(err, store.ErrFileNotFound) || errors.Is(err, fs.ErrNotExist) {
			return false
		}
		t.Fatalf("slowFileExists(%s): %v", fileName, err)
	}
	if err := in.Close(); err != nil {
		t.Fatalf("slowFileExists(%s): close: %v", fileName, err)
	}
	return true
}

// newTieredMergePolicy renders LuceneTestCase.newTieredMergePolicy(random()).
func newTieredMergePolicy() *index.TieredMergePolicy {
	tmp := index.NewTieredMergePolicy()
	if rarely() {
		tmp.SetMaxMergeAtOnce(nextInt(2, 9))
	} else {
		tmp.SetMaxMergeAtOnce(nextInt(10, 50))
	}
	if rarely() {
		tmp.SetMaxMergedSegmentMB(0.2 + rand.Float64()*2.0)
	} else {
		tmp.SetMaxMergedSegmentMB(10 + rand.Float64()*100)
	}
	tmp.SetFloorSegmentMB(0.2 + rand.Float64()*2.0)
	tmp.SetForceMergeDeletesPctAllowed(0.0 + rand.Float64()*30.0)
	if rarely() {
		tmp.SetSegmentsPerTier(float64(nextInt(2, 20)))
	} else {
		tmp.SetSegmentsPerTier(float64(nextInt(10, 50)))
	}
	if rarely() {
		tmp.SetTargetSearchConcurrency(nextInt(10, 50))
	} else {
		tmp.SetTargetSearchConcurrency(nextInt(2, 20))
	}

	configureRandomMergePolicy(tmp)
	tmp.SetDeletesPctAllowed(20 + rand.Float64()*30)
	return tmp
}

// newMergePolicy renders LuceneTestCase.newMergePolicy(random()), that is
// newMergePolicy(random(), true). The MockRandomMergePolicy and
// AlcoholicMergePolicy branches name the test-framework classes that are not
// ported.
func newMergePolicy(t testing.TB) index.MergePolicy {
	t.Helper()
	if rarely() {
		t.Fatal("org.apache.lucene.tests.index.MockRandomMergePolicy is not ported")
	} else if rand.Intn(2) == 0 {
		return newTieredMergePolicy()
	} else if rarely() {
		t.Fatal("org.apache.lucene.tests.index.AlcoholicMergePolicy is not ported")
	}
	return newLogMergePolicy()
}

// awaitLatch renders CountDownLatch.await() on a channel closed by the
// countDown. Lucene's suite timeout bounds every await; here the bound is one
// minute, after which the test fails naming what never happened.
func awaitLatch(t testing.TB, latch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-latch:
	case <-time.After(time.Minute):
		t.Fatalf("timed out awaiting: %s", what)
	}
}

// countDownLatch renders java.util.concurrent.CountDownLatch(1): countDown
// releases every await.
type countDownLatch struct {
	ch   chan struct{}
	once sync.Once
}

func newCountDownLatch() *countDownLatch { return &countDownLatch{ch: make(chan struct{})} }

func (l *countDownLatch) countDown() { l.once.Do(func() { close(l.ch) }) }

func (l *countDownLatch) await(t testing.TB, what string) {
	t.Helper()
	awaitLatch(t, l.ch, what)
}

// newFSDirectoryAt renders LuceneTestCase.newFSDirectory(Path): a
// MockDirectoryWrapper over a file-system directory at path.
func newFSDirectoryAt(t testing.TB, path string) *store.MockDirectoryWrapper {
	t.Helper()
	dir, err := store.NewNIOFSDirectory(path)
	if err != nil {
		t.Fatalf("newFSDirectory(%s): %v", path, err)
	}
	return store.NewMockDirectoryWrapper(dir)
}

// awaitFromGoroutine is await for goroutines other than the test's own: it
// reports the timeout with t.Errorf and returns false.
func (l *countDownLatch) awaitFromGoroutine(t testing.TB, what string) bool {
	select {
	case <-l.ch:
		return true
	case <-time.After(time.Minute):
		t.Errorf("timed out awaiting: %s", what)
		return false
	}
}

// expectThrowsPanic renders expectThrows(IllegalArgumentException.class, ...)
// and expectThrows(IllegalStateException.class, ...) for Go setters that have
// no error result and signal the Java exception by panicking.
func expectThrowsPanic(t testing.TB, what string, fn func()) {
	t.Helper()
	panicked := func() (p bool) {
		defer func() {
			if recover() != nil {
				p = true
			}
		}()
		fn()
		return false
	}()
	if !panicked {
		t.Fatalf("expected an exception from %s", what)
	}
}

// newLogMergePolicyUseCFS renders LuceneTestCase.newLogMergePolicy(boolean useCFS).
func newLogMergePolicyUseCFS(useCFS bool) logMergePolicy {
	logmp := newLogMergePolicy()
	if useCFS {
		logmp.SetNoCFSRatio(1.0)
	} else {
		logmp.SetNoCFSRatio(0.0)
	}
	return logmp
}

// newRandomIndexWriterOrError renders new RandomIndexWriter(random(), dir, iwc)
// for call sites that expect the constructor to throw.
func newRandomIndexWriterOrError(dir store.Directory, iwc *index.IndexWriterConfig) (*testindex.RandomIndexWriter, error) {
	return testindex.NewRandomIndexWriterWithConfig(rand.New(rand.NewSource(rand.Int63())), dir, iwc)
}

// newIOContext renders LuceneTestCase.newIOContext(random()), that is
// newIOContext(random, IOContext.DEFAULT): a totally random IOContext other
// than READONCE.
func newIOContext() store.IOContext {
	randomNumDocs := rand.Intn(4192)
	size := int64(rand.Intn(512)) * int64(randomNumDocs)
	switch rand.Intn(3) {
	case 1:
		return store.IOContextMerge(&spi.MergeInfo{TotalMaxDoc: randomNumDocs, EstimatedMergeBytes: size, IsExternal: true, MergeMaxNumSegments: -1})
	case 2:
		return store.IOContextFlush(store.NewFlushInfo(randomNumDocs, size))
	default:
		return store.IOContextDefault
	}
}

// testUtilDocs renders TestUtil.docs(Random, TermsEnum, PostingsEnum, int):
// a PostingsEnum with random features available. Gocene's TermsEnum.Postings
// takes no reuse argument.
func testUtilDocs(termsEnum index.TermsEnum, flags int) (index.PostingsEnum, error) {
	if rand.Intn(2) == 0 {
		if rand.Intn(2) == 0 {
			var posFlags int
			switch rand.Intn(4) {
			case 0:
				posFlags = spi.PostingsFlagPositions
			case 1:
				posFlags = spi.PostingsFlagOffsets
			case 2:
				posFlags = spi.PostingsFlagPayloads
			default:
				posFlags = spi.PostingsFlagAll
			}
			return termsEnum.Postings(posFlags)
		}
		flags |= spi.PostingsFlagFreqs
	}
	return termsEnum.Postings(flags)
}

// randomFixedByteLengthUnicodeString renders
// TestUtil.randomFixedByteLengthUnicodeString(Random, int): a random valid
// unicode string whose UTF-8 encoding is exactly length bytes. Java builds
// UTF-16 chars; a surrogate pair becomes one supplementary rune here.
func randomFixedByteLengthUnicodeString(length int) string {
	runes := make([]rune, 0, length)
	bytes := length
	for bytes != 0 {
		var t int
		if bytes >= 4 {
			t = rand.Intn(5)
		} else if bytes == 3 {
			t = rand.Intn(4)
		} else if bytes == 2 {
			t = rand.Intn(2)
		} else {
			t = 0
		}
		switch t {
		case 0:
			runes = append(runes, rune(rand.Intn(0x80)))
			bytes--
		case 1:
			runes = append(runes, rune(nextInt(0x80, 0x7ff)))
			bytes -= 2
		case 2:
			runes = append(runes, rune(nextInt(0x800, 0xd7ff)))
			bytes -= 3
		case 3:
			runes = append(runes, rune(nextInt(0xe000, 0xffff)))
			bytes -= 3
		case 4:
			// Make a surrogate pair
			high := nextInt(0xd800, 0xdbff)
			low := nextInt(0xdc00, 0xdfff)
			runes = append(runes, rune(0x10000+((high-0xd800)<<10)+(low-0xdc00)))
			bytes -= 4
		}
	}
	return string(runes)
}

// newRandomIndexWriterWithAnalyzer renders new RandomIndexWriter(random(), dir, analyzer).
func newRandomIndexWriterWithAnalyzer(t testing.TB, dir store.Directory, a analysis.Analyzer) *testindex.RandomIndexWriter {
	t.Helper()
	w, err := testindex.NewRandomIndexWriterWithAnalyzer(rand.New(rand.NewSource(rand.Int63())), dir, a)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}
	return w
}

// closeWhileHandlingException renders IOUtils.closeWhileHandlingException:
// every closer is closed and every error suppressed, as Java does on the
// failure path it guards.
func closeWhileHandlingException(closers ...interface{ Close() error }) {
	for _, c := range closers {
		if c != nil {
			// The Java helper swallows the exception by contract: it runs
			// while another exception is already propagating.
			if err := c.Close(); err != nil {
				continue
			}
		}
	}
}

// setIndexWriterMaxDocs renders LuceneTestCase.setIndexWriterMaxDocs(int);
// call restoreIndexWriterMaxDocs once the test is done.
func setIndexWriterMaxDocs(t testing.TB, limit int) {
	t.Helper()
	if err := index.SetMaxDocs(limit); err != nil {
		t.Fatalf("setIndexWriterMaxDocs(%d): %v", limit, err)
	}
}

// restoreIndexWriterMaxDocs renders LuceneTestCase.restoreIndexWriterMaxDocs().
func restoreIndexWriterMaxDocs(t testing.TB) {
	t.Helper()
	if err := index.SetMaxDocs(index.MaxDocs); err != nil {
		t.Fatalf("restoreIndexWriterMaxDocs: %v", err)
	}
}

// newSearcher renders LuceneTestCase.newSearcher(IndexReader): with its
// default wrapWithAssertions=true it always builds an
// org.apache.lucene.tests.search.AssertingIndexSearcher, which is not ported.
func newSearcher(t testing.TB, r index.IndexReaderInterface) *search.IndexSearcher {
	t.Helper()
	t.Fatal("org.apache.lucene.tests.search.AssertingIndexSearcher (built by LuceneTestCase.newSearcher(IndexReader)) is not ported")
	return nil
}

// reduceOpenFiles renders TestUtil.reduceOpenFiles(IndexWriter): keep the
// number of open files lowish.
func reduceOpenFiles(w *index.IndexWriter) {
	mp := w.GetConfig().GetMergePolicy()
	mp.(interface{ SetNoCFSRatio(float64) }).SetNoCFSRatio(1.0)
	switch p := mp.(type) {
	case interface {
		GetMergeFactor() int
		SetMergeFactor(int)
	}:
		p.SetMergeFactor(min(5, p.GetMergeFactor()))
	case *index.TieredMergePolicy:
		p.SetMaxMergeAtOnce(min(5, p.GetMaxMergeAtOnce()))
		p.SetSegmentsPerTier(min(5, p.GetSegmentsPerTier()))
	}
	if cms, ok := w.GetConfig().GetMergeScheduler().(*index.ConcurrentMergeScheduler); ok {
		// wtf... shouldn't it be even lower since it's 1 by default?!?!
		if err := cms.SetMaxMergesAndThreads(3, 2); err != nil {
			panic(err)
		}
	}
}

// javaNextInt renders random().nextInt() widened to long: a uniformly
// distributed int over the whole 32-bit range.
func javaNextInt() int64 { return int64(int32(rand.Uint32())) }

// javaNextLong renders random().nextLong(): a uniformly distributed long over
// the whole 64-bit range.
func javaNextLong() int64 { return int64(rand.Uint64()) }

// testUtilDocsForTerm renders TestUtil.docs(Random, IndexReader, String,
// BytesRef, PostingsEnum, int): nil when the field or term is absent.
func testUtilDocsForTerm(r index.IndexReader, field, text string, flags int) (index.PostingsEnum, error) {
	terms, err := index.MultiTermsGetTerms(r, field)
	if err != nil || terms == nil {
		return nil, err
	}
	termsEnum, err := terms.Iterator()
	if err != nil {
		return nil, err
	}
	found, err := termsEnum.SeekExact(spi.NewTerm(field, text))
	if err != nil || !found {
		return nil, err
	}
	return testUtilDocs(termsEnum, flags)
}
