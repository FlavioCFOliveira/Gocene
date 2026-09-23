// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
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
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
)

// This file renders, for the internal search/grouping test package, the members of
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

// queryUtilsCheckExplanations renders QueryUtils.checkExplanations(Query,
// IndexSearcher), whose body is CheckHits.checkExplanations(q, null, s, true).
func queryUtilsCheckExplanations(t testing.TB, q search.Query, s *search.IndexSearcher) {
	t.Helper()
	testsearch.CheckExplanations(t, q, "", s, true)
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

// mustScorerScore renders Scorable.score().
func mustScorerScore(t testing.TB, s search.Scorable) float32 {
	t.Helper()
	score, err := s.Score()
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	return score
}

// newSearcherOptionsBlocker names what LuceneTestCase.newSearcher(IndexReader,
// boolean, boolean, ...) needs.
const newSearcherOptionsBlocker = "requires LuceneTestCase.newSearcher(IndexReader, boolean, boolean, ...) support: " +
	"org.apache.lucene.tests.search.AssertingIndexSearcher, LuceneTestCase.maybeWrapReader and the randomized " +
	"class similarity (not ported)"

// newSearcherWithConcurrency renders LuceneTestCase.newSearcher(IndexReader,
// boolean maybeWrap, boolean wrapWithAssertions, Concurrency). Every path
// wraps the reader with maybeWrapReader and sets the randomized class
// similarity; the asserting path builds an AssertingIndexSearcher.
func newSearcherWithConcurrency(t testing.TB, r index.IndexReaderInterface, maybeWrap, wrapWithAssertions bool) *search.IndexSearcher {
	t.Helper()
	t.Fatal(newSearcherOptionsBlocker)
	return nil
}

// newSearcherMaybeWrap renders LuceneTestCase.newSearcher(IndexReader,
// boolean maybeWrap), which delegates to newSearcher(r, maybeWrap, true).
func newSearcherMaybeWrap(t testing.TB, r index.IndexReaderInterface, maybeWrap bool) *search.IndexSearcher {
	t.Helper()
	t.Fatal(newSearcherBlocker)
	return nil
}

// unicodeBlockStarts renders the private TestUtil.blockStarts table.
var unicodeBlockStarts = []int{
	0x0000, 0x0080, 0x0100, 0x0180, 0x0250, 0x02B0, 0x0300, 0x0370, 0x0400, 0x0500,
	0x0530, 0x0590, 0x0600, 0x0700, 0x0750, 0x0780, 0x07C0, 0x0800, 0x0900, 0x0980,
	0x0A00, 0x0A80, 0x0B00, 0x0B80, 0x0C00, 0x0C80, 0x0D00, 0x0D80, 0x0E00, 0x0E80,
	0x0F00, 0x1000, 0x10A0, 0x1100, 0x1200, 0x1380, 0x13A0, 0x1400, 0x1680, 0x16A0,
	0x1700, 0x1720, 0x1740, 0x1760, 0x1780, 0x1800, 0x18B0, 0x1900, 0x1950, 0x1980,
	0x19E0, 0x1A00, 0x1A20, 0x1B00, 0x1B80, 0x1C00, 0x1C50, 0x1CD0, 0x1D00, 0x1D80,
	0x1DC0, 0x1E00, 0x1F00, 0x2000, 0x2070, 0x20A0, 0x20D0, 0x2100, 0x2150, 0x2190,
	0x2200, 0x2300, 0x2400, 0x2440, 0x2460, 0x2500, 0x2580, 0x25A0, 0x2600, 0x2700,
	0x27C0, 0x27F0, 0x2800, 0x2900, 0x2980, 0x2A00, 0x2B00, 0x2C00, 0x2C60, 0x2C80,
	0x2D00, 0x2D30, 0x2D80, 0x2DE0, 0x2E00, 0x2E80, 0x2F00, 0x2FF0, 0x3000, 0x3040,
	0x30A0, 0x3100, 0x3130, 0x3190, 0x31A0, 0x31C0, 0x31F0, 0x3200, 0x3300, 0x3400,
	0x4DC0, 0x4E00, 0xA000, 0xA490, 0xA4D0, 0xA500, 0xA640, 0xA6A0, 0xA700, 0xA720,
	0xA800, 0xA830, 0xA840, 0xA880, 0xA8E0, 0xA900, 0xA930, 0xA960, 0xA980, 0xAA00,
	0xAA60, 0xAA80, 0xABC0, 0xAC00, 0xD7B0, 0xE000, 0xF900, 0xFB00, 0xFB50, 0xFE00,
	0xFE10, 0xFE20, 0xFE30, 0xFE50, 0xFE70, 0xFF00, 0xFFF0, 0x10000, 0x10080, 0x10100,
	0x10140, 0x10190, 0x101D0, 0x10280, 0x102A0, 0x10300, 0x10330, 0x10380, 0x103A0, 0x10400,
	0x10450, 0x10480, 0x10800, 0x10840, 0x10900, 0x10920, 0x10A00, 0x10A60, 0x10B00, 0x10B40,
	0x10B60, 0x10C00, 0x10E60, 0x11080, 0x12000, 0x12400, 0x13000, 0x1D000, 0x1D100, 0x1D200,
	0x1D300, 0x1D360, 0x1D400, 0x1F000, 0x1F030, 0x1F100, 0x1F200, 0x20000, 0x2A700, 0x2F800,
	0xE0000, 0xE0100, 0xF0000, 0x100000,
}

// unicodeBlockEnds renders the private TestUtil.blockEnds table.
var unicodeBlockEnds = []int{
	0x007F, 0x00FF, 0x017F, 0x024F, 0x02AF, 0x02FF, 0x036F, 0x03FF, 0x04FF, 0x052F,
	0x058F, 0x05FF, 0x06FF, 0x074F, 0x077F, 0x07BF, 0x07FF, 0x083F, 0x097F, 0x09FF,
	0x0A7F, 0x0AFF, 0x0B7F, 0x0BFF, 0x0C7F, 0x0CFF, 0x0D7F, 0x0DFF, 0x0E7F, 0x0EFF,
	0x0FFF, 0x109F, 0x10FF, 0x11FF, 0x137F, 0x139F, 0x13FF, 0x167F, 0x169F, 0x16FF,
	0x171F, 0x173F, 0x175F, 0x177F, 0x17FF, 0x18AF, 0x18FF, 0x194F, 0x197F, 0x19DF,
	0x19FF, 0x1A1F, 0x1AAF, 0x1B7F, 0x1BBF, 0x1C4F, 0x1C7F, 0x1CFF, 0x1D7F, 0x1DBF,
	0x1DFF, 0x1EFF, 0x1FFF, 0x206F, 0x209F, 0x20CF, 0x20FF, 0x214F, 0x218F, 0x21FF,
	0x22FF, 0x23FF, 0x243F, 0x245F, 0x24FF, 0x257F, 0x259F, 0x25FF, 0x26FF, 0x27BF,
	0x27EF, 0x27FF, 0x28FF, 0x297F, 0x29FF, 0x2AFF, 0x2BFF, 0x2C5F, 0x2C7F, 0x2CFF,
	0x2D2F, 0x2D7F, 0x2DDF, 0x2DFF, 0x2E7F, 0x2EFF, 0x2FDF, 0x2FFF, 0x303F, 0x309F,
	0x30FF, 0x312F, 0x318F, 0x319F, 0x31BF, 0x31EF, 0x31FF, 0x32FF, 0x33FF, 0x4DBF,
	0x4DFF, 0x9FFF, 0xA48F, 0xA4CF, 0xA4FF, 0xA63F, 0xA69F, 0xA6FF, 0xA71F, 0xA7FF,
	0xA82F, 0xA83F, 0xA87F, 0xA8DF, 0xA8FF, 0xA92F, 0xA95F, 0xA97F, 0xA9DF, 0xAA5F,
	0xAA7F, 0xAADF, 0xABFF, 0xD7AF, 0xD7FF, 0xF8FF, 0xFAFF, 0xFB4F, 0xFDFF, 0xFE0F,
	0xFE1F, 0xFE2F, 0xFE4F, 0xFE6F, 0xFEFF, 0xFFEF, 0xFFFF, 0x1007F, 0x100FF, 0x1013F,
	0x1018F, 0x101CF, 0x101FF, 0x1029F, 0x102DF, 0x1032F, 0x1034F, 0x1039F, 0x103DF, 0x1044F,
	0x1047F, 0x104AF, 0x1083F, 0x1085F, 0x1091F, 0x1093F, 0x10A5F, 0x10A7F, 0x10B3F, 0x10B5F,
	0x10B7F, 0x10C4F, 0x10E7F, 0x110CF, 0x123FF, 0x1247F, 0x1342F, 0x1D0FF, 0x1D1FF, 0x1D24F,
	0x1D35F, 0x1D37F, 0x1D7FF, 0x1F02F, 0x1F09F, 0x1F1FF, 0x1F2FF, 0x2A6DF, 0x2B73F, 0x2FA1F,
	0xE007F, 0xE01EF, 0xFFFFF, 0x10FFFF,
}

// randomRealisticUnicodeString renders
// TestUtil.randomRealisticUnicodeString(Random): a random string of up to 20
// code points, all within the same unicode block.
func randomRealisticUnicodeString(r *rand.Rand) string {
	return randomRealisticUnicodeStringRange(r, 0, 20)
}

// randomRealisticUnicodeStringRange renders
// TestUtil.randomRealisticUnicodeString(Random, int minLength, int maxLength):
// a random string of length between min and max code points, all code points
// within the same unicode block.
func randomRealisticUnicodeStringRange(r *rand.Rand, minLength, maxLength int) string {
	end := nextIntR(r, minLength, maxLength)
	block := r.Intn(len(unicodeBlockStarts))
	var sb strings.Builder
	for i := 0; i < end; i++ {
		sb.WriteRune(rune(nextIntR(r, unicodeBlockStarts[block], unicodeBlockEnds[block])))
	}
	return sb.String()
}

// nextIntR renders TestUtil.nextInt(Random, int start, int end): a value in
// [start, end].
func nextIntR(r *rand.Rand, start, end int) int {
	return start + r.Intn(end-start+1)
}

// verbose renders LuceneTestCase.VERBOSE (false by default).
const verbose = false
