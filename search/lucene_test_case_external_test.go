// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"testing"
	"unicode/utf16"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file renders, for the external search_test package, the members of
// org.apache.lucene.tests.util.LuceneTestCase, TestUtil and the test-framework
// classes that the ported Lucene 10.5.0 search tests call. Test-framework
// classes that are not ported are rendered as helpers that fail the test
// naming the missing class; the test body that follows them is kept so the
// port is complete once the class exists.

// queryUtilsBlocker names org.apache.lucene.tests.search.QueryUtils.
const queryUtilsBlocker = "requires org.apache.lucene.tests.search.QueryUtils (not ported)"

// newSearcherBlocker names the class LuceneTestCase.newSearcher builds.
const newSearcherBlocker = "requires org.apache.lucene.tests.search.AssertingIndexSearcher " +
	"(built by LuceneTestCase.newSearcher(IndexReader)) (not ported)"

// random renders LuceneTestCase.random(): an independent generator seeded
// from the shared source, safe to call from any goroutine.
func random() *rand.Rand {
	return rand.New(rand.NewSource(rand.Int63()))
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

// newRandomIndexWriter renders new RandomIndexWriter(random(), dir).
func newRandomIndexWriter(t testing.TB, dir store.Directory) *testindex.RandomIndexWriter {
	t.Helper()
	w, err := testindex.NewRandomIndexWriter(random(), dir)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}
	return w
}

// newRandomIndexWriterWithConfig renders new RandomIndexWriter(random(), dir, iwc).
func newRandomIndexWriterWithConfig(t testing.TB, dir store.Directory, iwc *index.IndexWriterConfig) *testindex.RandomIndexWriter {
	t.Helper()
	w, err := testindex.NewRandomIndexWriterWithConfig(random(), dir, iwc)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}
	return w
}

// newRandomIndexWriterWithAnalyzer renders new RandomIndexWriter(random(), dir, analyzer).
func newRandomIndexWriterWithAnalyzer(t testing.TB, dir store.Directory, a analysis.Analyzer) *testindex.RandomIndexWriter {
	t.Helper()
	w, err := testindex.NewRandomIndexWriterWithAnalyzer(random(), dir, a)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}
	return w
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

// documentAdder is the addDocument half of IndexWriter and RandomIndexWriter.
type documentAdder interface {
	AddDocument(doc *document.Document) (int64, error)
}

// mustAddDocument renders writer.addDocument(doc).
func mustAddDocument(t testing.TB, w documentAdder, doc *document.Document) {
	t.Helper()
	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
}

// mustGetReader renders RandomIndexWriter.getReader().
func mustGetReader(t testing.TB, w *testindex.RandomIndexWriter) *index.DirectoryReader {
	t.Helper()
	r, err := w.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	return r
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

// mustClose renders IOUtils.close(closeables...).
func mustClose(t testing.TB, closers ...interface{ Close() error }) {
	t.Helper()
	for _, c := range closers {
		if err := c.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
}

// newTestDocument renders `new Document()` followed by `doc.add(field)` for
// every field.
func newTestDocument(fields ...document.IndexableField) *document.Document {
	doc := document.NewDocument()
	for _, f := range fields {
		doc.Add(f)
	}
	return doc
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

// newStringFieldBytes renders LuceneTestCase.newStringField(String, BytesRef, Store).
func newStringFieldBytes(t testing.TB, name string, value []byte, stored bool) *document.StringField {
	t.Helper()
	f, err := document.NewStringFieldFromBytesRef(name, value, stored)
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

// newSearcher renders LuceneTestCase.newSearcher(IndexReader): with its
// default wrapWithAssertions=true it always builds an
// org.apache.lucene.tests.search.AssertingIndexSearcher, which is not ported.
func newSearcher(t testing.TB, r index.IndexReaderInterface) *search.IndexSearcher {
	t.Helper()
	t.Fatal(newSearcherBlocker)
	return nil
}

// newSearcherMaybeWrap renders LuceneTestCase.newSearcher(IndexReader,
// boolean maybeWrap), which delegates to newSearcher(r, maybeWrap, true).
func newSearcherMaybeWrap(t testing.TB, r index.IndexReaderInterface, maybeWrap bool) *search.IndexSearcher {
	t.Helper()
	t.Fatal(newSearcherBlocker)
	return nil
}

// newSearcherWithOptions renders LuceneTestCase.newSearcher(IndexReader,
// boolean maybeWrap, boolean wrapWithAssertions, boolean useThreads). Every
// path wraps the reader with maybeWrapReader and sets the randomized class
// similarity; the asserting path builds an AssertingIndexSearcher.
func newSearcherWithOptions(t testing.TB, r index.IndexReaderInterface, maybeWrap, wrapWithAssertions, useThreads bool) *search.IndexSearcher {
	t.Helper()
	t.Fatal(newSearcherOptionsBlocker)
	return nil
}

// newSearcherOptionsBlocker names what LuceneTestCase.newSearcher(IndexReader,
// boolean, boolean, ...) needs.
const newSearcherOptionsBlocker = "requires LuceneTestCase.newSearcher(IndexReader, boolean, boolean, ...) support: " +
	"org.apache.lucene.tests.search.AssertingIndexSearcher, LuceneTestCase.maybeWrapReader and the randomized " +
	"class similarity (not ported)"

// assertingQueryBlocker names org.apache.lucene.tests.search.AssertingQuery.
const assertingQueryBlocker = "requires org.apache.lucene.tests.search.AssertingQuery (not ported)"

// blockScoreQueryWrapperBlocker names
// org.apache.lucene.tests.search.BlockScoreQueryWrapper.
const blockScoreQueryWrapperBlocker = "requires org.apache.lucene.tests.search.BlockScoreQueryWrapper (not ported)"

// queryUtilsCheck renders QueryUtils.check(Query).
func queryUtilsCheck(t testing.TB, q search.Query) {
	t.Helper()
	t.Fatal(queryUtilsBlocker)
}

// queryUtilsCheckSearcher renders QueryUtils.check(Random, Query, IndexSearcher).
func queryUtilsCheckSearcher(t testing.TB, q search.Query, s *search.IndexSearcher) {
	t.Helper()
	t.Fatal(queryUtilsBlocker)
}

// queryUtilsCheckEqual renders QueryUtils.checkEqual(Query, Query).
func queryUtilsCheckEqual(t testing.TB, q1, q2 search.Query) {
	t.Helper()
	t.Fatal(queryUtilsBlocker)
}

// queryUtilsCheckUnequal renders QueryUtils.checkUnequal(Query, Query).
func queryUtilsCheckUnequal(t testing.TB, q1, q2 search.Query) {
	t.Helper()
	t.Fatal(queryUtilsBlocker)
}

// mustSearch renders IndexSearcher.search(Query, int).
func mustSearch(t testing.TB, s *search.IndexSearcher, q search.Query, n int) *search.TopDocs {
	t.Helper()
	td, err := s.Search(q, n)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return td
}

// mustCount renders IndexSearcher.count(Query).
func mustCount(t testing.TB, s *search.IndexSearcher, q search.Query) int {
	t.Helper()
	c, err := s.Count(q)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	return c
}

// mustRewrite renders IndexSearcher.rewrite(Query).
func mustRewrite(t testing.TB, s *search.IndexSearcher, q search.Query) search.Query {
	t.Helper()
	r, err := s.Rewrite(q)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	return r
}

// mustLeaves renders IndexReader.leaves().
func mustLeaves(t testing.TB, r interface {
	Leaves() ([]*index.LeafReaderContext, error)
}) []*index.LeafReaderContext {
	t.Helper()
	leaves, err := r.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	return leaves
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

// filterDirectoryReaderSubReaderWrapperBlocker names the production member a
// FilterDirectoryReader subclass with wrapped leaves needs: Gocene's
// FilterDirectoryReader has no FilterDirectoryReader(DirectoryReader,
// SubReaderWrapper) constructor and no SubReaderWrapper.
const filterDirectoryReaderSubReaderWrapperBlocker = "requires org.apache.lucene.index.FilterDirectoryReader(" +
	"DirectoryReader, FilterDirectoryReader.SubReaderWrapper) (not ported)"

// storedFieldsProvider is the storedFields() member of IndexReader and
// IndexSearcher.
type storedFieldsProvider interface {
	StoredFields() (index.StoredFields, error)
}

// mustStoredFields renders reader.storedFields() / searcher.storedFields().
func mustStoredFields(t testing.TB, p storedFieldsProvider) index.StoredFields {
	t.Helper()
	sf, err := p.StoredFields()
	if err != nil {
		t.Fatalf("storedFields: %v", err)
	}
	return sf
}

// storedDocument renders StoredFields.document(int): it visits the stored
// fields of docID with a DocumentStoredFieldVisitor.
func storedDocument(t testing.TB, storedFields index.StoredFields, docID int) *document.Document {
	t.Helper()
	visitor := document.NewDocumentStoredFieldVisitor()
	if err := storedFields.Document(docID, visitor); err != nil {
		t.Fatalf("document(%d): %v", docID, err)
	}
	return visitor.GetDocument()
}

// newMultiReader renders new MultiReader(IndexReader...).
func newMultiReader(t testing.TB, subReaders ...index.IndexReaderInterface) *index.MultiReader {
	t.Helper()
	mr, err := index.NewMultiReader(subReaders)
	if err != nil {
		t.Fatalf("new MultiReader: %v", err)
	}
	return mr
}

// expectThrowsPanic renders expectThrows(IllegalArgumentException.class, ...)
// for Go constructors that have no error result and signal the Java exception
// by panicking. It returns the panic message.
func expectThrowsPanic(t testing.TB, fn func()) (msg string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
		switch v := r.(type) {
		case error:
			msg = v.Error()
		case string:
			msg = v
		default:
			msg = fmt.Sprint(v)
		}
	}()
	fn()
	return ""
}

// randomUnicodeString renders TestUtil.randomUnicodeString(Random): a random
// string of 0..20 UTF-16 chars, surrogate pairs included, decoded to Go.
func randomUnicodeString(r *rand.Rand) string {
	return randomUnicodeStringWithMaxLength(r, 20)
}

// randomUnicodeStringWithMaxLength renders
// TestUtil.randomUnicodeString(Random, int maxLength).
func randomUnicodeStringWithMaxLength(r *rand.Rand, maxLength int) string {
	end := r.Intn(maxLength + 1)
	if end == 0 {
		// allow 0 length
		return ""
	}
	buffer := make([]uint16, end)
	randomFixedLengthUnicodeString(r, buffer, 0, len(buffer))
	return string(utf16.Decode(buffer))
}

// randomFixedLengthUnicodeString renders
// TestUtil.randomFixedLengthUnicodeString(Random, char[], int, int).
func randomFixedLengthUnicodeString(r *rand.Rand, chars []uint16, offset, length int) {
	i := offset
	end := offset + length
	for i < end {
		t := r.Intn(5)
		if 0 == t && i < length-1 {
			// Make a surrogate pair
			// High surrogate
			chars[i] = uint16(0xd800 + r.Intn(0xdbff-0xd800+1))
			i++
			// Low surrogate
			chars[i] = uint16(0xdc00 + r.Intn(0xdfff-0xdc00+1))
			i++
		} else if t <= 1 {
			chars[i] = uint16(r.Intn(0x80))
			i++
		} else if 2 == t {
			chars[i] = uint16(0x80 + r.Intn(0x7ff-0x80+1))
			i++
		} else if 3 == t {
			chars[i] = uint16(0x800 + r.Intn(0xd7ff-0x800+1))
			i++
		} else if 4 == t {
			chars[i] = uint16(0xe000 + r.Intn(0xffff-0xe000+1))
			i++
		}
	}
}

// newMergePolicyNoMock renders LuceneTestCase.newMergePolicy(random(), false):
// a TieredMergePolicy, (rarely) an AlcoholicMergePolicy, or a LogMergePolicy.
// AlcoholicMergePolicy (org.apache.lucene.tests.index) is not ported.
func newMergePolicyNoMock(t testing.TB) index.MergePolicy {
	t.Helper()
	if rand.Intn(2) == 0 {
		return newTieredMergePolicy()
	} else if rarely() {
		t.Fatal("requires org.apache.lucene.tests.index.AlcoholicMergePolicy (not ported)")
	}
	return newLogMergePolicy()
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

// mustCreateWeight renders IndexSearcher.createWeight(Query, ScoreMode, float).
func mustCreateWeight(t testing.TB, s *search.IndexSearcher, q search.Query, scoreMode search.ScoreMode, boost float32) search.Weight {
	t.Helper()
	w, err := s.CreateWeight(q, scoreMode, boost)
	if err != nil {
		t.Fatalf("createWeight: %v", err)
	}
	return w
}

// mustScorer renders Weight.scorer(LeafReaderContext).
func mustScorer(t testing.TB, w search.Weight, ctx *index.LeafReaderContext) search.Scorer {
	t.Helper()
	s, err := w.Scorer(ctx)
	if err != nil {
		t.Fatalf("scorer: %v", err)
	}
	return s
}

// mustNextDoc renders DocIdSetIterator.nextDoc().
func mustNextDoc(t testing.TB, it search.DocIdSetIterator) int {
	t.Helper()
	doc, err := it.NextDoc()
	if err != nil {
		t.Fatalf("nextDoc: %v", err)
	}
	return doc
}

// mustScore renders Scorable.score().
func mustScore(t testing.TB, s search.Scorable) float32 {
	t.Helper()
	score, err := s.Score()
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	return score
}

// mustCommit renders IndexWriter.commit() / RandomIndexWriter.commit().
func mustCommit(t testing.TB, w interface{ Commit() (int64, error) }) {
	t.Helper()
	if _, err := w.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// mustOpenDirectoryReaderFromWriter renders DirectoryReader.open(IndexWriter).
func mustOpenDirectoryReaderFromWriter(t testing.TB, w *index.IndexWriter) *index.DirectoryReader {
	t.Helper()
	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("DirectoryReader.open(writer): %v", err)
	}
	return r
}

// cachedThreadPool renders Executors.newCachedThreadPool(...): every task runs
// on a new goroutine; shutdownAndAwaitTermination renders es.shutdown()
// followed by es.awaitTermination(...).
type cachedThreadPool struct {
	wg sync.WaitGroup
}

// newCachedThreadPool renders Executors.newCachedThreadPool(ThreadFactory).
func newCachedThreadPool() *cachedThreadPool {
	return &cachedThreadPool{}
}

// Execute renders Executor.execute(Runnable).
func (p *cachedThreadPool) Execute(runnable func()) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		runnable()
	}()
}

func (p *cachedThreadPool) shutdownAndAwaitTermination() {
	p.wg.Wait()
}

// getOnlyLeafReader renders LuceneTestCase.getOnlyLeafReader(IndexReader): the
// reader must have exactly one segment, whose leaf reader is returned.
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

// newBytesRef renders LuceneTestCase.newBytesRef(byte[]): a copy of b that
// sometimes uses a non-zero offset and non-zero end padding, to tickle latent
// bugs that fail to look at BytesRef.offset.
func newBytesRef(b []byte) *util.BytesRef {
	return newBytesRefSlice(b, 0, len(b))
}

// newBytesRefSlice renders LuceneTestCase.newBytesRef(byte[], int, int).
func newBytesRefSlice(bytesIn []byte, offset, length int) *util.BytesRef {
	if util.AssertsEnabled() && !(len(bytesIn) >= offset+length) {
		panic(util.NewAssertionError(fmt.Sprintf("got offset=%d length=%d bytesIn.length=%d", offset, length, len(bytesIn))))
	}

	// randomly set a non-zero offset
	var startOffset int
	if random().Intn(2) == 0 {
		startOffset = 1 + random().Intn(20)
	} else {
		startOffset = 0
	}

	// also randomly set an end padding:
	var endPadding int
	if random().Intn(2) == 0 {
		endPadding = 1 + random().Intn(20)
	} else {
		endPadding = 0
	}

	bytes := make([]byte, startOffset+length+endPadding)

	copy(bytes[startOffset:], bytesIn[offset:offset+length])

	it := &util.BytesRef{Bytes: bytes, Offset: startOffset, Length: length}

	if 1+random().Intn(17) == 7 {
		// try to ferret out bugs in this method too!
		return newBytesRefSlice(it.Bytes, it.Offset, it.Length)
	}

	return it
}
