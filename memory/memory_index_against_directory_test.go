// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package memory

// Port of lucene/memory/src/test/org/apache/lucene/index/memory/TestMemoryIndexAgainstDirectory.java
// (Apache Lucene 10.5.0): verifies that Lucene MemoryIndex and a RAM-resident
// Directory have the same behaviour, returning the same results for queries on
// some randomish indexes.
//
// The Java class lives in package org.apache.lucene.index.memory and uses the
// package-private constructor MemoryIndex(boolean, boolean, long), so this port
// is an internal test of package memory. The resources testqueries.txt and
// testqueries2.txt are copied verbatim to testdata/.
//
// Test-framework renderings (see memory_index_test.go for the shared ones):
//   - LuceneTestCase.newDirectory() is rendered as its ByteBuffersDirectory
//     choice wrapped in a MockDirectoryWrapper (tests/util.WrapDirectory);
//   - LuceneTestCase.newIndexWriterConfig(Random, Analyzer) is rendered as
//     index.NewIndexWriterConfigWithAnalyzer, without the random settings;
//   - LuceneTestCase.atLeast(int) and TEST_NIGHTLY/RANDOM_MULTIPLIER keep
//     their defaults (not nightly, multiplier 1).

import (
	"bufio"
	"bytes"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	// Registers the default codec, rendering the Java ServiceLoader lookup
	// behind Codec.getDefault() that IndexWriterConfig relies on.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/spans"
	"github.com/FlavioCFOliveira/Gocene/queryparser"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testutil "github.com/FlavioCFOliveira/Gocene/tests/util"
)

// randomMultiplier renders LuceneTestCase.RANDOM_MULTIPLIER (default 1).
const randomMultiplier = 1

// testNightly renders LuceneTestCase.TEST_NIGHTLY (default false).
const testNightly = false

// memoryIndexAgainstDirectoryTest renders the instance state of
// TestMemoryIndexAgainstDirectory.
type memoryIndexAgainstDirectoryTest struct {
	t       *testing.T
	random  *rand.Rand
	queries map[string]struct{}
}

// setUp renders `public void setUp()`.
func setUpMemoryIndexAgainstDirectory(t *testing.T) *memoryIndexAgainstDirectoryTest {
	t.Helper()
	tc := &memoryIndexAgainstDirectoryTest{t: t, random: newTestRandom(t), queries: map[string]struct{}{}}
	for _, resource := range []string{"testqueries.txt", "testqueries2.txt"} {
		for q := range tc.readQueries(resource) {
			tc.queries[q] = struct{}{}
		}
	}
	return tc
}

// readQueries reads a set of queries from a resource file.
func (tc *memoryIndexAgainstDirectoryTest) readQueries(resource string) map[string]struct{} {
	tc.t.Helper()
	queries := map[string]struct{}{}
	stream, err := os.Open(filepath.Join("testdata", resource))
	must(tc.t, err)
	defer func() { must(tc.t, stream.Close()) }()
	reader := bufio.NewScanner(stream)
	for reader.Scan() {
		line := strings.TrimFunc(reader.Text(), func(r rune) bool { return r <= ' ' })
		if len(line) > 0 && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "//") {
			queries[line] = struct{}{}
		}
	}
	must(tc.t, reader.Err())
	return queries
}

// atLeast renders LuceneTestCase.atLeast(int).
func (tc *memoryIndexAgainstDirectoryTest) atLeast(i int) int {
	minimum := i * randomMultiplier
	maximum := minimum + (minimum / 2)
	return minimum + tc.random.Intn(maximum-minimum+1)
}

// newDirectory renders LuceneTestCase.newDirectory().
func newDirectory() store.Directory {
	return testutil.WrapDirectory(store.NewByteBuffersDirectory())
}

// runs random tests, up to ITERATIONS times.
func TestMemoryIndexAgainstDirectory_RandomQueries(t *testing.T) {
	tc := setUpMemoryIndexAgainstDirectory(t)
	mi := tc.randomMemoryIndex()
	iterations := 10 * randomMultiplier
	if testNightly {
		iterations = 100 * randomMultiplier
	}
	for i := 0; i < iterations; i++ {
		tc.assertAgainstDirectory(mi)
	}
}

// assertAgainstDirectory builds a randomish document for both Directory and
// MemoryIndex, and runs all the queries against it.
func (tc *memoryIndexAgainstDirectoryTest) assertAgainstDirectory(memory *MemoryIndex) {
	t := tc.t
	t.Helper()
	memory.Reset()
	var fooField, termField strings.Builder

	// add up to 250 terms to field "foo"
	numFooTerms := tc.random.Intn(250 * randomMultiplier)
	for i := 0; i < numFooTerms; i++ {
		fooField.WriteString(" ")
		fooField.WriteString(tc.randomTerm())
	}

	// add up to 250 terms to field "term"
	numTermTerms := tc.random.Intn(250 * randomMultiplier)
	for i := 0; i < numTermTerms; i++ {
		termField.WriteString(" ")
		termField.WriteString(tc.randomTerm())
	}

	// Java next indexes the document into a ByteBuffersDirectory through an
	// IndexWriterConfig whose codec is
	// TestUtil.alwaysPostingsFormat(TestUtil.getDefaultPostingsFormat()), with
	// fields built by LuceneTestCase.newTextField, and then calls
	// duellReaders and assertAllQueries.
	blocked(t, "TestUtil.alwaysPostingsFormat, TestUtil.getDefaultPostingsFormat and LuceneTestCase.newTextField (lucene/test-framework)")
}

// duellReaders renders `private void duellReaders(CompositeReader, LeafReader)`.
func (tc *memoryIndexAgainstDirectoryTest) duellReaders() {
	tc.t.Helper()
	blocked(tc.t, "FieldInfos.getIndexedFields(IndexReader) (org.apache.lucene.index.FieldInfos)")
}

// assertAllQueries runs all queries against both the Directory and
// MemoryIndex, ensuring they are the same. LuceneTestCase.newSearcher is
// rendered as a plain IndexSearcher.
func (tc *memoryIndexAgainstDirectoryTest) assertAllQueries(memory *MemoryIndex, directory store.Directory, analyzer analysis.Analyzer) {
	t := tc.t
	t.Helper()
	reader, err := index.OpenDirectoryReader(directory)
	must(t, err)
	ram := search.NewIndexSearcher(reader)
	mem := memory.CreateSearcher()
	qp := queryparser.NewQueryParser("foo", analyzer)
	for query := range tc.queries {
		q, err := qp.Parse(query)
		must(t, err)
		ramDocs, err := ram.Search(q, 1)
		must(t, err)
		q, err = qp.Parse(query)
		must(t, err)
		memDocs, err := mem.Search(q, 1)
		must(t, err)
		if ramDocs.TotalHits.Value != memDocs.TotalHits.Value {
			t.Errorf("%s: directory hits %d, memory hits %d", query, ramDocs.TotalHits.Value, memDocs.TotalHits.Value)
		}
	}
	must(t, reader.Close())
}

// randomAnalyzer returns a random analyzer (Simple, Stop, Standard) to
// analyze the terms.
func (tc *memoryIndexAgainstDirectoryTest) randomAnalyzer() analysis.Analyzer {
	switch tc.random.Intn(4) {
	case 0:
		return testanalysis.NewMockAnalyzer(testanalysis.SIMPLE, true, 0, nil, true)
	case 1:
		return testanalysis.NewMockAnalyzer(testanalysis.SIMPLE, true, 0, testanalysis.ENGLISH_STOPSET, true)
	case 2:
		a := analysis.NewAnalyzer(analysis.GlobalReuseStrategy)
		a.CreateComponents = func(fieldName string) *analysis.TokenStreamComponents {
			// new MockTokenizer() is MockTokenizer(WHITESPACE, true).
			tokenizer := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, true, testanalysis.DefaultMaxTokenLength)
			return &analysis.TokenStreamComponents{
				Source: func(r io.Reader) error {
					tokenizer.SetReader(r)
					return nil
				},
				Sink: newCrazyTokenFilter(tokenizer),
			}
		}
		return a
	default:
		return testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, false, 0, nil, true)
	}
}

// crazyTokenFilter is a tokenfilter that makes all terms starting with 't'
// empty strings.
type crazyTokenFilter struct {
	*analysis.BaseTokenFilter
	termAtt analysis.CharTermAttribute
}

func newCrazyTokenFilter(input analysis.TokenStream) *crazyTokenFilter {
	f := &crazyTokenFilter{BaseTokenFilter: analysis.NewBaseTokenFilter(input)}
	f.termAtt = f.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	return f
}

func (f *crazyTokenFilter) IncrementToken() (bool, error) {
	more, err := f.GetInput().IncrementToken()
	if err != nil || !more {
		return false, err
	}
	if f.termAtt.Length() > 0 && f.termAtt.Buffer()[0] == 't' {
		// termAtt.setLength(0)
		f.termAtt.SetEmpty()
	}
	return true, nil
}

// testTerms renders TEST_TERMS: some terms to be indexed, in addition to
// random words. These terms are commonly used in the queries.
var testTerms = []string{
	"term",
	"Term",
	"tErm",
	"TERM",
	"telm",
	"stop",
	"drop",
	"roll",
	"phrase",
	"a",
	"c",
	"bar",
	"blar",
	"gack",
	"weltbank",
	"worlbank",
	"hello",
	"on",
	"the",
	"apache",
	"Apache",
	"copyright",
	"Copyright",
}

// randomTerm returns, half of the time, a random term from TEST_TERMS and,
// the other half of the time, a random unicode string.
func (tc *memoryIndexAgainstDirectoryTest) randomTerm() string {
	if tc.random.Intn(2) == 0 {
		// return a random TEST_TERM
		return testTerms[tc.random.Intn(len(testTerms))]
	}
	// return a random unicode term
	blocked(tc.t, "TestUtil.randomUnicodeString(Random) (lucene/test-framework)")
	return ""
}

func TestMemoryIndexAgainstDirectory_DocsEnumStart(t *testing.T) {
	tc := setUpMemoryIndexAgainstDirectory(t)
	analyzer := newMockAnalyzer()
	memory := newMemoryIndexWithMaxReusedBytes(tc.random.Intn(2) == 0, false, int64(tc.random.Intn(50))*1024*1024)
	must(t, memory.AddFieldFromString("foo", "bar", analyzer))
	reader := memory.CreateSearcher().GetIndexReader().(index.LeafReader)
	blockedCheckReader(t)
	t.Error("blocked: TestUtil.docs(Random, IndexReader, String, BytesRef, PostingsEnum, int) (lucene/test-framework) is not ported")

	// now reuse and check again
	terms, err := reader.Terms("foo")
	must(t, err)
	te, err := terms.Iterator()
	must(t, err)
	if ok, err := te.SeekExact(index.NewTerm("foo", "bar")); err != nil || !ok {
		t.Fatalf("seekExact(bar): got (%v, %v), want true", ok, err)
	}
	disi, err := te.Postings(spi.PostingsFlagNone)
	must(t, err)
	if docid := disi.DocID(); docid != -1 {
		t.Errorf("docID before nextDoc: got %d, want -1", docid)
	}
	if d, err := disi.NextDoc(); err != nil || d == spi.NO_MORE_DOCS {
		t.Errorf("nextDoc: got (%d, %v), want a document", d, err)
	}
	must(t, reader.Close())
}

func (tc *memoryIndexAgainstDirectoryTest) randomMemoryIndex() *MemoryIndex {
	return newMemoryIndexWithMaxReusedBytes(
		tc.random.Intn(2) == 0, tc.random.Intn(2) == 0, int64(tc.random.Intn(50))*1024*1024)
}

func TestMemoryIndexAgainstDirectory_DocsAndPositionsEnumStart(t *testing.T) {
	tc := setUpMemoryIndexAgainstDirectory(t)
	analyzer := newMockAnalyzer()
	numIters := tc.atLeast(3)
	memory := newMemoryIndexWithMaxReusedBytes(true, false, int64(tc.random.Intn(50))*1024*1024)
	for i := 0; i < numIters; i++ { // check reuse
		must(t, memory.AddFieldFromString("foo", "bar", analyzer))
		reader := memory.CreateSearcher().GetIndexReader().(index.LeafReader)
		blockedCheckReader(t)
		terms, err := reader.Terms("foo")
		must(t, err)
		if v, err := terms.GetSumTotalTermFreq(); err != nil || v != 1 {
			t.Errorf("sumTotalTermFreq: got (%d, %v), want 1", v, err)
		}
		disi, err := reader.Postings(*index.NewTerm("foo", "bar"), spi.PostingsFlagAll)
		must(t, err)
		if docid := disi.DocID(); docid != -1 {
			t.Errorf("docID before nextDoc: got %d, want -1", docid)
		}
		if d, err := disi.NextDoc(); err != nil || d == spi.NO_MORE_DOCS {
			t.Errorf("nextDoc: got (%d, %v), want a document", d, err)
		}
		if p, err := disi.NextPosition(); err != nil || p != 0 {
			t.Errorf("nextPosition: got (%d, %v), want 0", p, err)
		}
		if s, err := disi.StartOffset(); err != nil || s != 0 {
			t.Errorf("startOffset: got (%d, %v), want 0", s, err)
		}
		if e, err := disi.EndOffset(); err != nil || e != 3 {
			t.Errorf("endOffset: got (%d, %v), want 3", e, err)
		}

		// now reuse and check again
		te, err := terms.Iterator()
		must(t, err)
		if ok, err := te.SeekExact(index.NewTerm("foo", "bar")); err != nil || !ok {
			t.Fatalf("seekExact(bar): got (%v, %v), want true", ok, err)
		}
		// te.postings(disi) is postings(disi, PostingsEnum.FREQS).
		disi, err = te.Postings(spi.PostingsFlagFreqs)
		must(t, err)
		if docid := disi.DocID(); docid != -1 {
			t.Errorf("docID before nextDoc: got %d, want -1", docid)
		}
		if d, err := disi.NextDoc(); err != nil || d == spi.NO_MORE_DOCS {
			t.Errorf("nextDoc: got (%d, %v), want a document", d, err)
		}
		must(t, reader.Close())
		memory.Reset()
	}
}

// mockTokenStream renders `new MockAnalyzer(random()).tokenStream(field, text)`.
func mockTokenStream(t *testing.T, fieldName, text string) analysis.TokenStream {
	t.Helper()
	ts, err := newMockAnalyzer().TokenStream(fieldName, strings.NewReader(text))
	must(t, err)
	return ts
}

// LUCENE-3831
func TestMemoryIndexAgainstDirectory_NullPointerException(t *testing.T) {
	tc := setUpMemoryIndexAgainstDirectory(t)
	regex := search.NewRegexpQuery(index.NewTerm("field", "worl."))
	wrappedquery := spans.NewSpanMultiTermQueryWrapper(&regex.MultiTermQuery)

	mindex := tc.randomMemoryIndex()
	must(t, mindex.AddField("field", mockTokenStream(t, "field", "hello there")))

	// This throws an NPE
	if got := searchScore(t, mindex, wrappedquery); math.Abs(float64(got)) > 0.00001 {
		t.Errorf("score: got %v, want 0", got)
	}
	blockedCheckReader(t)
}

// LUCENE-3831
func TestMemoryIndexAgainstDirectory_PassesIfWrapped(t *testing.T) {
	tc := setUpMemoryIndexAgainstDirectory(t)
	regex := search.NewRegexpQuery(index.NewTerm("field", "worl."))
	wrappedquery, err := spans.NewSpanOrQuery(spans.NewSpanMultiTermQueryWrapper(&regex.MultiTermQuery))
	must(t, err)

	mindex := tc.randomMemoryIndex()
	must(t, mindex.AddField("field", mockTokenStream(t, "field", "hello there")))

	// This passes though
	if got := searchScore(t, mindex, wrappedquery); math.Abs(float64(got)) > 0.00001 {
		t.Errorf("score: got %v, want 0", got)
	}
	blockedCheckReader(t)
}

func TestMemoryIndexAgainstDirectory_SameFieldAddedMultipleTimes(t *testing.T) {
	tc := setUpMemoryIndexAgainstDirectory(t)
	mindex := tc.randomMemoryIndex()
	mockAnalyzer := newMockAnalyzer()
	must(t, mindex.AddFieldFromString("field", "the quick brown fox", mockAnalyzer))
	must(t, mindex.AddFieldFromString("field", "jumps over the", mockAnalyzer))
	reader := mindex.CreateSearcher().GetIndexReader().(index.LeafReader)
	blockedCheckReader(t)
	terms, err := reader.Terms("field")
	must(t, err)
	if v, err := terms.GetSumTotalTermFreq(); err != nil || v != 7 {
		t.Errorf("sumTotalTermFreq: got (%d, %v), want 7", v, err)
	}
	var query search.Query = search.NewPhraseQuery(0, "field", "fox", "jumps")
	if got := searchScore(t, mindex, query); got <= 0.1 {
		t.Errorf("phrase fox jumps: got %v, want > 0.1", got)
	}
	mindex.Reset()
	mockAnalyzer.SetPositionIncrementGap(1 + tc.random.Intn(10))
	must(t, mindex.AddFieldFromString("field", "the quick brown fox", mockAnalyzer))
	must(t, mindex.AddFieldFromString("field", "jumps over the", mockAnalyzer))
	if got := searchScore(t, mindex, query); math.Abs(float64(got)) > 0.00001 {
		t.Errorf("phrase fox jumps with a position gap: got %v, want 0", got)
	}
	query = search.NewPhraseQuery(10, "field", "fox", "jumps")
	if got := searchScore(t, mindex, query); got <= 0.0001 {
		t.Errorf("posGap%d: sloppy phrase: got %v, want > 0.0001", mockAnalyzer.GetPositionIncrementGap("field"), got)
	}
	blockedCheckReader(t)
}

func TestMemoryIndexAgainstDirectory_NonExistentField(t *testing.T) {
	tc := setUpMemoryIndexAgainstDirectory(t)
	mindex := tc.randomMemoryIndex()
	mockAnalyzer := newMockAnalyzer()
	must(t, mindex.AddFieldFromString("field", "the quick brown fox", mockAnalyzer))
	reader := mindex.CreateSearcher().GetIndexReader().(index.LeafReader)
	blockedCheckReader(t)
	if v, err := reader.GetNumericDocValues("not-in-index"); err != nil || v != nil {
		t.Errorf("numeric doc values: got (%v, %v), want nil", v, err)
	}
	if v, err := reader.GetNormValues("not-in-index"); err != nil || v != nil {
		t.Errorf("norms: got (%v, %v), want nil", v, err)
	}
	// reader.postings(Term) is postings(term, PostingsEnum.FREQS).
	if v, err := reader.Postings(*index.NewTerm("not-in-index", "foo"), spi.PostingsFlagFreqs); err != nil || v != nil {
		t.Errorf("postings(term): got (%v, %v), want nil", v, err)
	}
	if v, err := reader.Postings(*index.NewTerm("not-in-index", "foo"), spi.PostingsFlagAll); err != nil || v != nil {
		t.Errorf("postings(term, ALL): got (%v, %v), want nil", v, err)
	}
	if v, err := reader.Terms("not-in-index"); err != nil || v != nil {
		t.Errorf("terms: got (%v, %v), want nil", v, err)
	}
}

func TestMemoryIndexAgainstDirectory_DocValuesMemoryIndexVsNormalIndex(t *testing.T) {
	tc := setUpMemoryIndexAgainstDirectory(t)
	doc := document.NewDocument()
	randomLong := tc.random.Int63()
	doc.Add(field[*document.NumericDocValuesField](t)(document.NewNumericDocValuesField("numeric", randomLong)))
	numValues := tc.atLeast(5)
	for i := 0; i < numValues; i++ {
		randomLong = tc.random.Int63()
		doc.Add(sortedNumericField(t, "sorted_numeric", randomLong))
		if tc.random.Intn(2) == 0 {
			// randomly duplicate field/value
			doc.Add(sortedNumericField(t, "sorted_numeric", randomLong))
		}
	}
	tc.randomTerm()
	blocked(t, "TestUtil.randomUnicodeString(Random) (lucene/test-framework)")
}

func TestMemoryIndexAgainstDirectory_NormsWithDocValues(t *testing.T) {
	setUpMemoryIndexAgainstDirectory(t)
	mi := NewMemoryIndexWithOffsetsAndPayloads(true, true)
	mockAnalyzer := newMockAnalyzer()

	must(t, mi.AddFieldFromIndexableField(field[*document.BinaryDocValuesField](t)(document.NewBinaryDocValuesField("text", []byte("quick brown fox"))), mockAnalyzer))
	must(t, mi.AddFieldFromIndexableField(field[*document.TextField](t)(document.NewTextField("text", "quick brown fox", false)), mockAnalyzer))
	leafReader := leafReaderOf(t, mi.CreateSearcher())

	doc := document.NewDocument()
	doc.Add(field[*document.BinaryDocValuesField](t)(document.NewBinaryDocValuesField("text", []byte("quick brown fox"))))
	doc.Add(field[*document.TextField](t)(document.NewTextField("text", "quick brown fox", false)))
	dir := newDirectory()
	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfigWithAnalyzer(mockAnalyzer))
	must(t, err)
	_, err = writer.AddDocument(doc)
	must(t, err)
	must(t, writer.Close())

	controlIndexReader, err := index.OpenDirectoryReader(dir)
	must(t, err)
	controlLeaves, err := controlIndexReader.Leaves()
	must(t, err)
	controlLeafReader := controlLeaves[0].LeafReader()

	norms, err := controlLeafReader.GetNormValues("text")
	must(t, err)
	assertNextDoc(t, norms, 0)
	norms2, err := leafReader.GetNormValues("text")
	must(t, err)
	assertNextDoc(t, norms2, 0)
	n1, err := norms.LongValue()
	must(t, err)
	n2, err := norms2.LongValue()
	must(t, err)
	if n1 != n2 {
		t.Errorf("norms: directory %d, memory %d", n1, n2)
	}

	must(t, controlIndexReader.Close())
	must(t, dir.Close())
}

func TestMemoryIndexAgainstDirectory_PointValuesMemoryIndexVsNormalIndex(t *testing.T) {
	tc := setUpMemoryIndexAgainstDirectory(t)
	size := tc.atLeast(12)

	var randomValues []int32

	doc := document.NewDocument()
	for i := 0; i < size; i++ {
		randomInteger := int32(tc.random.Uint32())
		doc.Add(document.NewIntPoint("int", randomInteger))
		randomValues = append(randomValues, randomInteger)
		doc.Add(document.NewLongPoint("long", int64(randomInteger)))
		doc.Add(document.NewFloatPoint("float", float32(randomInteger)))
		doc.Add(document.NewDoublePoint("double", float64(randomInteger)))
	}

	mockAnalyzer := newMockAnalyzer()
	memoryIndex, err := FromDocument(doc.Fields(), mockAnalyzer)
	must(t, err)
	memoryIndex.CreateSearcher()

	dir := newDirectory()
	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfigWithAnalyzer(mockAnalyzer))
	must(t, err)
	_, err = writer.AddDocument(doc)
	must(t, err)
	must(t, writer.Close())
	controlIndexReader, err := index.OpenDirectoryReader(dir)
	must(t, err)
	search.NewIndexSearcher(controlIndexReader)
	must(t, controlIndexReader.Close())
	must(t, dir.Close())
	blocked(t, "IntPoint/LongPoint/FloatPoint/DoublePoint.newExactQuery, newSetQuery and newRangeQuery (org.apache.lucene.document)")
}

func TestMemoryIndexAgainstDirectory_DuellMemIndex(t *testing.T) {
	setUpMemoryIndexAgainstDirectory(t)
	blocked(t, "LineFileDocs (lucene/test-framework/src/java/org/apache/lucene/tests/util/LineFileDocs.java)")
}

// LUCENE-4880
func TestMemoryIndexAgainstDirectory_EmptyString(t *testing.T) {
	setUpMemoryIndexAgainstDirectory(t)
	memory := NewMemoryIndex()
	must(t, memory.AddField("foo", testanalysis.NewCannedTokenStream(testanalysis.NewToken("", 0, 5))))
	searcher := memory.CreateSearcher()
	docs, err := searcher.Search(termQuery("foo", ""), 10)
	must(t, err)
	if docs.TotalHits.Value != 1 {
		t.Errorf("total hits: got %d, want 1", docs.TotalHits.Value)
	}
	blockedCheckReader(t)
}

func TestMemoryIndexAgainstDirectory_DuelMemoryIndexCoreDirectoryWithArrayField(t *testing.T) {
	tc := setUpMemoryIndexAgainstDirectory(t)
	const fieldName = "text"
	mockAnalyzer := newMockAnalyzer()
	if tc.random.Intn(2) == 0 {
		mockAnalyzer.SetOffsetGap(tc.random.Intn(100))
	}
	// index into a random directory
	ft := document.NewFieldTypeFrom(document.TextFieldTYPESTORED)
	ft.SetStoreTermVectorOffsets(true)
	ft.SetStoreTermVectorPayloads(false)
	ft.SetStoreTermVectorPositions(true)
	ft.SetStoreTermVectors(true)
	ft.Freeze()

	doc := document.NewDocument()
	f1, err := document.NewField(fieldName, "la la", ft)
	must(t, err)
	doc.Add(f1)
	f2, err := document.NewField(fieldName, "foo bar foo bar foo", ft)
	must(t, err)
	doc.Add(f2)

	dir := newDirectory()
	writer, err := index.NewIndexWriter(dir, index.NewIndexWriterConfigWithAnalyzer(mockAnalyzer))
	must(t, err)
	_, err = writer.UpdateDocument(index.NewTerm("id", "1"), doc)
	must(t, err)
	_, err = writer.Commit()
	must(t, err)
	must(t, writer.Close())
	reader, err := index.OpenDirectoryReader(dir)
	must(t, err)

	// Index document in Memory index
	memIndex := NewMemoryIndexWithOffsets(true)
	must(t, memIndex.AddFieldFromString(fieldName, "la la", mockAnalyzer))
	must(t, memIndex.AddFieldFromString(fieldName, "foo bar foo bar foo", mockAnalyzer))

	// compare term vectors
	ramTermVectors, err := reader.TermVectors()
	must(t, err)
	ramTv, err := ramTermVectors.GetField(0, fieldName)
	must(t, err)
	memIndexReader := memIndex.CreateSearcher().GetIndexReader()
	blockedCheckReader(t)
	memTermVectors, err := memIndexReader.TermVectors()
	must(t, err)
	memTv, err := memTermVectors.GetField(0, fieldName)
	must(t, err)

	compareTermVectors(t, ramTv, memTv, fieldName)
	must(t, memIndexReader.Close())
	must(t, reader.Close())
	must(t, dir.Close())
}

// payloadEquals renders assertEquals(BytesRef, BytesRef): null only equals
// null.
func payloadEquals(a, b []byte) bool {
	return (a == nil) == (b == nil) && bytes.Equal(a, b)
}

func compareTermVectors(t *testing.T, terms, memTerms index.Terms, fieldName string) {
	t.Helper()
	if terms == nil || memTerms == nil {
		t.Fatalf("term vectors: directory %v, memory %v; want both non-nil", terms, memTerms)
	}
	termEnum, err := terms.Iterator()
	must(t, err)
	memTermEnum, err := memTerms.Iterator()
	must(t, err)

	for {
		next, err := termEnum.Next()
		must(t, err)
		if next == nil {
			break
		}
		memNext, err := memTermEnum.Next()
		must(t, err)
		if memNext == nil {
			t.Fatal("memory term vector ended early")
		}
		memTotal, err := memTermEnum.TotalTermFreq()
		must(t, err)
		total, err := termEnum.TotalTermFreq()
		must(t, err)
		if memTotal != total {
			t.Errorf("totalTermFreq: memory %d, directory %d", memTotal, total)
		}

		docsPosEnum, err := termEnum.Postings(spi.PostingsFlagPositions)
		must(t, err)
		memDocsPosEnum, err := memTermEnum.Postings(spi.PostingsFlagPositions)
		must(t, err)
		currentTerm := termEnum.Term().Text()

		if got := memTermEnum.Term().Text(); got != currentTerm {
			t.Errorf("Token mismatch for field: %s: memory %q, directory %q", fieldName, got, currentTerm)
		}

		_, err = docsPosEnum.NextDoc()
		must(t, err)
		_, err = memDocsPosEnum.NextDoc()
		must(t, err)

		freq, err := docsPosEnum.Freq()
		must(t, err)
		memFreq, err := memDocsPosEnum.Freq()
		must(t, err)
		if memFreq != freq {
			t.Errorf("freq: memory %d, directory %d", memFreq, freq)
		}
		for i := 0; i < freq; i++ {
			failDesc := " (field:" + fieldName + " term:" + currentTerm + ")"
			memPos, err := memDocsPosEnum.NextPosition()
			must(t, err)
			pos, err := docsPosEnum.NextPosition()
			must(t, err)
			if pos != memPos {
				t.Errorf("Position test failed%s: directory %d, memory %d", failDesc, pos, memPos)
			}
			start, err := docsPosEnum.StartOffset()
			must(t, err)
			memStart, err := memDocsPosEnum.StartOffset()
			must(t, err)
			if start != memStart {
				t.Errorf("Start offset test failed%s: directory %d, memory %d", failDesc, start, memStart)
			}
			end, err := docsPosEnum.EndOffset()
			must(t, err)
			memEnd, err := memDocsPosEnum.EndOffset()
			must(t, err)
			if end != memEnd {
				t.Errorf("End offset test failed%s: directory %d, memory %d", failDesc, end, memEnd)
			}
			payload, err := docsPosEnum.GetPayload()
			must(t, err)
			memPayload, err := memDocsPosEnum.GetPayload()
			must(t, err)
			if !payloadEquals(payload, memPayload) {
				t.Errorf("Missing payload test failed%s: directory %v, memory %v", failDesc, payload, memPayload)
			}
		}
	}
	memNext, err := memTermEnum.Next()
	must(t, err)
	if memNext != nil {
		t.Errorf("Still some tokens not processed: %v", memNext)
	}
}
