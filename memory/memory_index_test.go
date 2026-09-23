// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package memory

// Port of lucene/memory/src/test/org/apache/lucene/index/memory/TestMemoryIndex.java
// (Apache Lucene 10.5.0). The Java class lives in package
// org.apache.lucene.index.memory, so this port is an internal test of package
// memory.
//
// Test-framework renderings:
//   - LuceneTestCase.random() is newTestRandom: a seeded *rand.Rand whose seed
//     is logged, so a failing run can be reproduced.
//   - new MockAnalyzer(random()) is MockAnalyzer(random, WHITESPACE, true):
//     tests/analysis.NewMockAnalyzer(WHITESPACE, true, 0, nil, true).
//   - Java exceptions are Go errors; expectThrows asserts a non-nil error.
//
// Test-framework or production components that Gocene has not ported are
// reported as failures that name the missing component (see blocked); they
// are never skipped.

import (
	"bytes"
	"io"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// newTestRandom renders LuceneTestCase.random().
func newTestRandom(t *testing.T) *rand.Rand {
	t.Helper()
	seed := time.Now().UnixNano()
	t.Logf("random seed: %d", seed)
	return rand.New(rand.NewSource(seed))
}

// newMockAnalyzer renders `new MockAnalyzer(random())`, which is
// `new MockAnalyzer(random, MockTokenizer.WHITESPACE, true)`.
func newMockAnalyzer() *testanalysis.MockAnalyzer {
	return testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true)
}

// blockedCheckReader renders TestUtil.checkReader(IndexReader). TestUtil is
// not ported, and index.CheckIndex's per-part checks only accept an
// *index.SegmentReader, so the check cannot run on a MemoryIndex reader. The
// gap is reported as a failure without stopping the rest of the test.
func blockedCheckReader(t *testing.T) {
	t.Helper()
	t.Error("blocked: TestUtil.checkReader (lucene/test-framework/src/java/org/apache/lucene/tests/util/TestUtil.java) is not ported")
}

// blocked stops a test at a statement whose component Gocene does not port.
func blocked(t *testing.T, component string) {
	t.Helper()
	t.Fatalf("blocked: %s is not ported", component)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// testMemoryIndexSetup renders the @Before method setup().
func testMemoryIndexSetup() *testanalysis.MockAnalyzer {
	analyzer := newMockAnalyzer()
	analyzer.SetEnableChecks(false) // MemoryIndex can close a TokenStream on init error
	return analyzer
}

func leafReaderOf(t *testing.T, searcher *search.IndexSearcher) index.LeafReader {
	t.Helper()
	leaves, err := searcher.GetIndexReader().Leaves()
	must(t, err)
	return leaves[0].LeafReader()
}

func searchScore(t *testing.T, mi *MemoryIndex, q search.Query) float32 {
	t.Helper()
	score, err := mi.Search(q)
	must(t, err)
	return score
}

func termQuery(field, text string) search.Query {
	return search.NewTermQuery(index.NewTerm(field, text))
}

func newDoc(t *testing.T, fields ...document.IndexableField) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	for _, f := range fields {
		doc.Add(f)
	}
	return doc
}

func field[F document.IndexableField](t *testing.T) func(F, error) document.IndexableField {
	return func(f F, err error) document.IndexableField {
		t.Helper()
		must(t, err)
		return f
	}
}

func TestMemoryIndex_FreezeAPI(t *testing.T) {
	analyzer := testMemoryIndexSetup()

	mi := NewMemoryIndex()
	must(t, mi.AddFieldFromString("f1", "some text", analyzer))

	if searchScore(t, mi, search.Instance) == 0.0 {
		t.Error("match all must score")
	}
	if searchScore(t, mi, termQuery("f1", "some")) == 0.0 {
		t.Error("f1:some must score")
	}

	// check we can add a new field after searching
	must(t, mi.AddFieldFromString("f2", "some more text", analyzer))
	if searchScore(t, mi, termQuery("f2", "some")) == 0.0 {
		t.Error("f2:some must score")
	}

	// freeze!
	mi.Freeze()

	err := mi.AddFieldFromString("f3", "and yet more", analyzer)
	if err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Errorf("addField on a frozen index: got %v, want an error containing \"frozen\"", err)
	}

	err = mi.SetSimilarity(search.NewLuceneBM25SimilarityWithParams(1, 1))
	if err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Errorf("setSimilarity on a frozen index: got %v, want an error containing \"frozen\"", err)
	}

	if searchScore(t, mi, termQuery("f1", "some")) == 0.0 {
		t.Error("f1:some must score after freeze")
	}

	mi.Reset()
	must(t, mi.AddFieldFromString("f1", "wibble", analyzer))
	if got := searchScore(t, mi, termQuery("f1", "some")); got != 0.0 {
		t.Errorf("f1:some after reset: got %v, want 0", got)
	}
	if searchScore(t, mi, termQuery("f1", "wibble")) == 0.0 {
		t.Error("f1:wibble must score")
	}

	// check we can set the Similarity again
	must(t, mi.SetSimilarity(search.NewClassicSimilarity()))
}

func TestMemoryIndex_SeekByTermOrd(t *testing.T) {
	analyzer := testMemoryIndexSetup()
	mi := NewMemoryIndex()
	must(t, mi.AddFieldFromString("field", "some terms be here", analyzer))
	searcher := mi.CreateSearcher()
	reader := searcher.GetIndexReader().(index.LeafReader)
	terms, err := reader.Terms("field")
	must(t, err)
	termsEnum, err := terms.Iterator()
	must(t, err)
	// TermsEnum.seekExact(long ord) is rendered as the optional SeekExactOrd.
	must(t, termsEnum.(interface{ SeekExactOrd(int64) error }).SeekExactOrd(0))
	if got := termsEnum.Term().Text(); got != "be" {
		t.Errorf("term at ord 0: got %q, want \"be\"", got)
	}
	blockedCheckReader(t)
}

func TestMemoryIndex_FieldsOnlyReturnsIndexedFields(t *testing.T) {
	analyzer := testMemoryIndexSetup()
	doc := newDoc(t,
		field[*document.NumericDocValuesField](t)(document.NewNumericDocValuesField("numeric", 29)),
		field[*document.TextField](t)(document.NewTextField("text", "some text", false)))

	mi, err := FromDocument(doc.Fields(), analyzer)
	must(t, err)
	searcher := mi.CreateSearcher()
	reader := searcher.GetIndexReader()

	termVectors, err := reader.TermVectors()
	must(t, err)
	fields, err := termVectors.Get(0)
	must(t, err)
	if fields.Size() != 1 {
		t.Errorf("term vector fields: got %d, want 1", fields.Size())
	}
}

func TestMemoryIndex_ReaderConsistency(t *testing.T) {
	analyzer := testanalysis.NewMockPayloadAnalyzer()

	// defaults
	mi := NewMemoryIndex()
	must(t, mi.AddFieldFromString("field", "some terms be here", analyzer))
	blockedCheckReader(t)

	// all combinations of offsets/payloads options
	mi = NewMemoryIndexWithOffsetsAndPayloads(true, true)
	must(t, mi.AddFieldFromString("field", "some terms be here", analyzer))
	blockedCheckReader(t)

	mi = NewMemoryIndexWithOffsetsAndPayloads(true, false)
	must(t, mi.AddFieldFromString("field", "some terms be here", analyzer))
	blockedCheckReader(t)

	mi = NewMemoryIndexWithOffsetsAndPayloads(false, true)
	must(t, mi.AddFieldFromString("field", "some terms be here", analyzer))
	blockedCheckReader(t)

	mi = NewMemoryIndexWithOffsetsAndPayloads(false, false)
	must(t, mi.AddFieldFromString("field", "some terms be here", analyzer))
	blockedCheckReader(t)

	must(t, analyzer.Close())
}

// constantNormSimilarity renders the anonymous Similarity of testSimilarities:
// computeNorm returns 74 and scorer throws UnsupportedOperationException.
type constantNormSimilarity struct{}

func (constantNormSimilarity) GetDiscountOverlaps() bool { return true }

func (constantNormSimilarity) ComputeNormFromInvertState(*index.FieldInvertState) int64 { return 74 }

func (constantNormSimilarity) Scorer104(float32, *search.CollectionStatistics, ...*search.TermStatistics) search.SimScorer {
	panic("UnsupportedOperationException")
}

func TestMemoryIndex_Similarities(t *testing.T) {
	analyzer := testMemoryIndexSetup()

	mi := NewMemoryIndex()
	must(t, mi.AddFieldFromString("f1", "a long text field that contains many many terms", analyzer))

	searcher := mi.CreateSearcher()
	reader := searcher.GetIndexReader().(index.LeafReader)
	norms, err := reader.GetNormValues("f1")
	must(t, err)
	doc, err := norms.NextDoc()
	must(t, err)
	if doc != 0 {
		t.Fatalf("norms.nextDoc: got %d, want 0", doc)
	}
	n1v, err := norms.LongValue()
	must(t, err)
	n1 := float32(n1v)

	// Norms are re-computed when we change the Similarity
	must(t, mi.SetSimilarity(constantNormSimilarity{}))
	norms, err = reader.GetNormValues("f1")
	must(t, err)
	doc, err = norms.NextDoc()
	must(t, err)
	if doc != 0 {
		t.Fatalf("norms.nextDoc: got %d, want 0", doc)
	}
	n2v, err := norms.LongValue()
	must(t, err)
	n2 := float32(n2v)

	if n1 == n2 {
		t.Errorf("norms must change with the Similarity: n1=%v n2=%v", n1, n2)
	}
	blockedCheckReader(t)
}

func TestMemoryIndex_OmitNorms(t *testing.T) {
	analyzer := testMemoryIndexSetup()
	mi := NewMemoryIndex()
	ft := document.NewFieldType()
	ft.SetTokenized(true)
	ft.SetIndexOptions(spi.IndexOptionsDocsAndFreqs)
	ft.SetOmitNorms(true)
	f, err := document.NewField("f1", "some text in here", ft)
	must(t, err)
	must(t, mi.AddFieldFromIndexableField(f, analyzer))
	mi.Freeze()

	leader := mi.CreateSearcher().GetIndexReader().(index.LeafReader)
	norms, err := leader.GetNormValues("f1")
	must(t, err)
	if norms != nil {
		t.Errorf("norms: got %v, want nil", norms)
	}
}

func TestMemoryIndex_BuildFromDocument(t *testing.T) {
	analyzer := testMemoryIndexSetup()

	doc := newDoc(t,
		field[*document.TextField](t)(document.NewTextField("field1", "some text", false)),
		field[*document.TextField](t)(document.NewTextField("field1", "some more text", false)),
		field[*document.StringField](t)(document.NewStringField("field2", "untokenized text", false)))

	analyzer.SetPositionIncrementGap(100)

	mi, err := FromDocument(doc.Fields(), analyzer)
	must(t, err)

	if searchScore(t, mi, termQuery("field1", "text")) == 0.0 {
		t.Error("field1:text must score")
	}
	if got := searchScore(t, mi, termQuery("field2", "text")); got != 0.0 {
		t.Errorf("field2:text: got %v, want 0", got)
	}
	if searchScore(t, mi, termQuery("field2", "untokenized text")) == 0.0 {
		t.Error("field2:\"untokenized text\" must score")
	}

	if got := searchScore(t, mi, termQuery("field1", "some more text")); got != 0.0 {
		t.Errorf("field1:\"some more text\" term: got %v, want 0", got)
	}
	if searchScore(t, mi, search.NewPhraseQuery(0, "field1", "some", "more", "text")) == 0.0 {
		t.Error("phrase \"some more text\" must score")
	}
	if searchScore(t, mi, search.NewPhraseQuery(0, "field1", "some", "text")) == 0.0 {
		t.Error("phrase \"some text\" must score")
	}
	if got := searchScore(t, mi, search.NewPhraseQuery(0, "field1", "text", "some")); got != 0.0 {
		t.Errorf("phrase \"text some\": got %v, want 0", got)
	}
}

func sortedNumericField(t *testing.T, name string, value int64) document.IndexableField {
	t.Helper()
	f, err := document.NewSortedNumericDocValuesField(name, []int64{value})
	must(t, err)
	return f
}

func sortedSetField(t *testing.T, name string, value string) document.IndexableField {
	t.Helper()
	f, err := document.NewSortedSetDocValuesField(name, [][]byte{[]byte(value)})
	must(t, err)
	return f
}

func assertNextDoc(t *testing.T, it interface{ NextDoc() (int, error) }, want int) {
	t.Helper()
	got, err := it.NextDoc()
	must(t, err)
	if got != want {
		t.Fatalf("nextDoc: got %d, want %d", got, want)
	}
}

func TestMemoryIndex_DocValues(t *testing.T) {
	analyzer := testMemoryIndexSetup()
	doc := newDoc(t,
		field[*document.NumericDocValuesField](t)(document.NewNumericDocValuesField("numeric", 29)),
		sortedNumericField(t, "sorted_numeric", 33),
		sortedNumericField(t, "sorted_numeric", 32),
		sortedNumericField(t, "sorted_numeric", 32),
		sortedNumericField(t, "sorted_numeric", 31),
		sortedNumericField(t, "sorted_numeric", 30),
		field[*document.BinaryDocValuesField](t)(document.NewBinaryDocValuesField("binary", []byte("a"))),
		field[*document.SortedDocValuesField](t)(document.NewSortedDocValuesField("sorted", []byte("b"))),
		sortedSetField(t, "sorted_set", "f"),
		sortedSetField(t, "sorted_set", "d"),
		sortedSetField(t, "sorted_set", "d"),
		sortedSetField(t, "sorted_set", "c"))

	mi, err := FromDocument(doc.Fields(), analyzer)
	must(t, err)
	leafReader := leafReaderOf(t, mi.CreateSearcher())

	numericDocValues, err := leafReader.GetNumericDocValues("numeric")
	must(t, err)
	assertNextDoc(t, numericDocValues, 0)
	if v, err := numericDocValues.LongValue(); err != nil || v != 29 {
		t.Errorf("numeric: got (%d, %v), want 29", v, err)
	}
	assertNextDoc(t, numericDocValues, spi.NO_MORE_DOCS)

	sortedNumericDocValues, err := leafReader.GetSortedNumericDocValues("sorted_numeric")
	must(t, err)
	assertNextDoc(t, sortedNumericDocValues, 0)
	if c, err := sortedNumericDocValues.DocValueCount(); err != nil || c != 5 {
		t.Errorf("sorted_numeric docValueCount: got (%d, %v), want 5", c, err)
	}
	for _, want := range []int64{30, 31, 32, 32, 33} {
		if v, err := sortedNumericDocValues.NextValue(); err != nil || v != want {
			t.Errorf("sorted_numeric nextValue: got (%d, %v), want %d", v, err, want)
		}
	}
	assertNextDoc(t, sortedNumericDocValues, spi.NO_MORE_DOCS)

	binaryDocValues, err := leafReader.GetBinaryDocValues("binary")
	must(t, err)
	assertNextDoc(t, binaryDocValues, 0)
	if v, err := binaryDocValues.BinaryValue(); err != nil || string(v) != "a" {
		t.Errorf("binary: got (%q, %v), want \"a\"", v, err)
	}
	assertNextDoc(t, binaryDocValues, spi.NO_MORE_DOCS)

	sortedDocValues, err := leafReader.GetSortedDocValues("sorted")
	must(t, err)
	assertNextDoc(t, sortedDocValues, 0)
	ord, err := sortedDocValues.OrdValue()
	must(t, err)
	if v, err := sortedDocValues.LookupOrd(ord); err != nil || string(v) != "b" {
		t.Errorf("sorted lookupOrd(ordValue): got (%q, %v), want \"b\"", v, err)
	}
	if ord != 0 {
		t.Errorf("sorted ordValue: got %d, want 0", ord)
	}
	if v, err := sortedDocValues.LookupOrd(0); err != nil || string(v) != "b" {
		t.Errorf("sorted lookupOrd(0): got (%q, %v), want \"b\"", v, err)
	}
	assertNextDoc(t, sortedDocValues, spi.NO_MORE_DOCS)

	sortedSetDocValues, err := leafReader.GetSortedSetDocValues("sorted_set")
	must(t, err)
	if c := sortedSetDocValues.GetValueCount(); c != 3 {
		t.Errorf("sorted_set valueCount: got %d, want 3", c)
	}
	assertNextDoc(t, sortedSetDocValues, 0)
	if c := sortedSetDocValues.DocValueCount(); c != 3 {
		t.Errorf("sorted_set docValueCount: got %d, want 3", c)
	}
	for _, want := range []int{0, 1, 2} {
		if o, err := sortedSetDocValues.NextOrd(); err != nil || o != want {
			t.Errorf("sorted_set nextOrd: got (%d, %v), want %d", o, err, want)
		}
	}
	for o, want := range []string{"c", "d", "f"} {
		if v, err := sortedSetDocValues.LookupOrd(o); err != nil || string(v) != want {
			t.Errorf("sorted_set lookupOrd(%d): got (%q, %v), want %q", o, v, err, want)
		}
	}
	assertNextDoc(t, sortedDocValues, spi.NO_MORE_DOCS)
}

func TestMemoryIndex_DocValuesResetIterator(t *testing.T) {
	analyzer := testMemoryIndexSetup()
	doc := newDoc(t,
		sortedSetField(t, "sorted_set", "f"),
		sortedSetField(t, "sorted_set", "d"),
		sortedSetField(t, "sorted_set", "d"),
		sortedSetField(t, "sorted_set", "c"),
		sortedNumericField(t, "sorted_numeric", 33),
		sortedNumericField(t, "sorted_numeric", 32),
		sortedNumericField(t, "sorted_numeric", 32),
		sortedNumericField(t, "sorted_numeric", 31),
		sortedNumericField(t, "sorted_numeric", 30))

	mi, err := FromDocument(doc.Fields(), analyzer)
	must(t, err)
	leafReader := leafReaderOf(t, mi.CreateSearcher())

	sortedSetDocValues, err := leafReader.GetSortedSetDocValues("sorted_set")
	must(t, err)
	if c := sortedSetDocValues.GetValueCount(); c != 3 {
		t.Errorf("sorted_set valueCount: got %d, want 3", c)
	}
	for times := 0; times < 3; times++ {
		if ok, err := sortedSetDocValues.AdvanceExact(0); err != nil || !ok {
			t.Fatalf("sorted_set advanceExact(0): got (%v, %v), want true", ok, err)
		}
		if c := sortedSetDocValues.DocValueCount(); c != 3 {
			t.Errorf("sorted_set docValueCount: got %d, want 3", c)
		}
		for _, want := range []int{0, 1, 2} {
			if o, err := sortedSetDocValues.NextOrd(); err != nil || o != want {
				t.Errorf("sorted_set nextOrd: got (%d, %v), want %d", o, err, want)
			}
		}
	}

	sortedNumericDocValues, err := leafReader.GetSortedNumericDocValues("sorted_numeric")
	must(t, err)
	for times := 0; times < 3; times++ {
		if ok, err := sortedNumericDocValues.AdvanceExact(0); err != nil || !ok {
			t.Fatalf("sorted_numeric advanceExact(0): got (%v, %v), want true", ok, err)
		}
		if c, err := sortedNumericDocValues.DocValueCount(); err != nil || c != 5 {
			t.Errorf("sorted_numeric docValueCount: got (%d, %v), want 5", c, err)
		}
		for _, want := range []int64{30, 31, 32, 32, 33} {
			if v, err := sortedNumericDocValues.NextValue(); err != nil || v != want {
				t.Errorf("sorted_numeric nextValue: got (%d, %v), want %d", v, err, want)
			}
		}
	}
}

// assertFromDocumentError renders the try/catch blocks of
// testInvalidDocValuesUsage: when fromDocument fails, the message must match.
func assertFromDocumentError(t *testing.T, doc *document.Document, analyzer analysis.Analyzer, want string) {
	t.Helper()
	if _, err := FromDocument(doc.Fields(), analyzer); err != nil {
		if err.Error() != want {
			t.Errorf("fromDocument error: got %q, want %q", err.Error(), want)
		}
	}
}

func TestMemoryIndex_InvalidDocValuesUsage(t *testing.T) {
	analyzer := testMemoryIndexSetup()
	doc := newDoc(t,
		field[*document.NumericDocValuesField](t)(document.NewNumericDocValuesField("field", 29)),
		field[*document.BinaryDocValuesField](t)(document.NewBinaryDocValuesField("field", []byte("30"))))
	assertFromDocumentError(t, doc, analyzer,
		"cannot change DocValues type from NUMERIC to BINARY for field \"field\"")

	doc = newDoc(t,
		field[*document.NumericDocValuesField](t)(document.NewNumericDocValuesField("field", 29)),
		field[*document.NumericDocValuesField](t)(document.NewNumericDocValuesField("field", 30)))
	assertFromDocumentError(t, doc, analyzer,
		"Only one value per field allowed for [NUMERIC] doc values field [field]")

	doc = newDoc(t,
		field[*document.TextField](t)(document.NewTextField("field", "a b", false)),
		field[*document.BinaryDocValuesField](t)(document.NewBinaryDocValuesField("field", []byte("a"))),
		field[*document.BinaryDocValuesField](t)(document.NewBinaryDocValuesField("field", []byte("b"))))
	assertFromDocumentError(t, doc, analyzer,
		"Only one value per field allowed for [BINARY] doc values field [field]")

	doc = newDoc(t,
		field[*document.SortedDocValuesField](t)(document.NewSortedDocValuesField("field", []byte("a"))),
		field[*document.SortedDocValuesField](t)(document.NewSortedDocValuesField("field", []byte("b"))),
		field[*document.TextField](t)(document.NewTextField("field", "a b", false)))
	assertFromDocumentError(t, doc, analyzer,
		"Only one value per field allowed for [SORTED] doc values field [field]")
}

// assertPostingsAt renders the next()/postings(OFFSETS) block shared by
// testDocValuesDoNotAffectBoostPositionsOrOffset and
// testPointValuesDoNotAffectPositionsOrOffset.
func assertPostingsAt(t *testing.T, tenum index.TermsEnum, term string, pos, start, end int) {
	t.Helper()
	next, err := tenum.Next()
	must(t, err)
	if next == nil || next.Text() != term {
		t.Fatalf("tenum.next: got %v, want %q", next, term)
	}
	penum, err := tenum.Postings(spi.PostingsFlagOffsets)
	must(t, err)
	assertNextDoc(t, penum, 0)
	if f, err := penum.Freq(); err != nil || f != 1 {
		t.Errorf("%s freq: got (%d, %v), want 1", term, f, err)
	}
	if p, err := penum.NextPosition(); err != nil || p != pos {
		t.Errorf("%s nextPosition: got (%d, %v), want %d", term, p, err, pos)
	}
	if s, err := penum.StartOffset(); err != nil || s != start {
		t.Errorf("%s startOffset: got (%d, %v), want %d", term, s, err, start)
	}
	if e, err := penum.EndOffset(); err != nil || e != end {
		t.Errorf("%s endOffset: got (%d, %v), want %d", term, e, err, end)
	}
}

func textTermsEnum(t *testing.T, leafReader index.LeafReader) index.TermsEnum {
	t.Helper()
	terms, err := leafReader.Terms("text")
	must(t, err)
	tenum, err := terms.Iterator()
	must(t, err)
	return tenum
}

func TestMemoryIndex_DocValuesDoNotAffectBoostPositionsOrOffset(t *testing.T) {
	analyzer := testMemoryIndexSetup()
	doc := newDoc(t,
		field[*document.BinaryDocValuesField](t)(document.NewBinaryDocValuesField("text", []byte("quick brown fox"))),
		field[*document.TextField](t)(document.NewTextField("text", "quick brown fox", false)))
	mi, err := FromDocumentWithOffsetsAndPayloads(doc.Fields(), analyzer, true, true)
	must(t, err)
	leafReader := leafReaderOf(t, mi.CreateSearcher())
	tenum := textTermsEnum(t, leafReader)

	assertPostingsAt(t, tenum, "brown", 1, 6, 11)
	assertPostingsAt(t, tenum, "fox", 2, 12, 15)
	assertPostingsAt(t, tenum, "quick", 0, 0, 5)

	binaryDocValues, err := leafReader.GetBinaryDocValues("text")
	must(t, err)
	assertNextDoc(t, binaryDocValues, 0)
	if v, err := binaryDocValues.BinaryValue(); err != nil || string(v) != "quick brown fox" {
		t.Errorf("binary: got (%q, %v), want \"quick brown fox\"", v, err)
	}
}

func TestMemoryIndex_BigBinaryDocValues(t *testing.T) {
	r := newTestRandom(t)
	analyzer := testMemoryIndexSetup()
	b := make([]byte, 33*1024)
	r.Read(b)
	doc := newDoc(t, field[*document.BinaryDocValuesField](t)(document.NewBinaryDocValuesField("binary", b)))
	mi, err := FromDocumentWithOffsetsAndPayloads(doc.Fields(), analyzer, true, true)
	must(t, err)
	leafReader := leafReaderOf(t, mi.CreateSearcher())
	binaryDocValues, err := leafReader.GetBinaryDocValues("binary")
	must(t, err)
	assertNextDoc(t, binaryDocValues, 0)
	if v, err := binaryDocValues.BinaryValue(); err != nil || !bytes.Equal(v, b) {
		t.Errorf("binary value differs (err %v)", err)
	}
}

func TestMemoryIndex_BigSortedDocValues(t *testing.T) {
	r := newTestRandom(t)
	analyzer := testMemoryIndexSetup()
	b := make([]byte, 33*1024)
	r.Read(b)
	doc := newDoc(t, field[*document.SortedDocValuesField](t)(document.NewSortedDocValuesField("binary", b)))
	mi, err := FromDocumentWithOffsetsAndPayloads(doc.Fields(), analyzer, true, true)
	must(t, err)
	leafReader := leafReaderOf(t, mi.CreateSearcher())
	sortedDocValues, err := leafReader.GetSortedDocValues("binary")
	must(t, err)
	assertNextDoc(t, sortedDocValues, 0)
	if v, err := sortedDocValues.LookupOrd(0); err != nil || !bytes.Equal(v, b) {
		t.Errorf("sorted value differs (err %v)", err)
	}
}

func TestMemoryIndex_PointValues(t *testing.T) {
	analyzer := testMemoryIndexSetup()
	fieldFunctions := []func(int64) document.IndexableField{
		func(v int64) document.IndexableField { return document.NewIntPoint("number", int32(v)) },
		func(v int64) document.IndexableField { return document.NewLongPoint("number", v) },
		func(v int64) document.IndexableField { return document.NewFloatPoint("number", float32(v)) },
		func(v int64) document.IndexableField { return document.NewDoublePoint("number", float64(v)) },
	}
	for i := range fieldFunctions {
		doc := document.NewDocument()
		for number := 1; number < 32; number += 2 {
			doc.Add(fieldFunctions[i](int64(number)))
		}
		mi, err := FromDocument(doc.Fields(), analyzer)
		must(t, err)
		mi.CreateSearcher()
		blocked(t, "IntPoint/LongPoint/FloatPoint/DoublePoint.newExactQuery, newSetQuery and newRangeQuery (org.apache.lucene.document)")
	}
}

func TestMemoryIndex_MissingPoints(t *testing.T) {
	analyzer := testMemoryIndexSetup()
	doc := newDoc(t, field[*document.StoredField](t)(document.NewStoredFieldFromInt("field", 42)))
	mi, err := FromDocument(doc.Fields(), analyzer)
	must(t, err)
	indexSearcher := mi.CreateSearcher()
	// field that exists but does not have points
	pv, err := leafReaderOf(t, indexSearcher).GetPointValues("field")
	must(t, err)
	if pv != nil {
		t.Errorf("point values of a field without points: got %v, want nil", pv)
	}
	// field that does not exist
	pv, err = leafReaderOf(t, indexSearcher).GetPointValues("some_missing_field")
	must(t, err)
	if pv != nil {
		t.Errorf("point values of a missing field: got %v, want nil", pv)
	}
}

func countOf(t *testing.T, searcher *search.IndexSearcher, q search.Query, qerr error) int {
	t.Helper()
	must(t, qerr)
	n, err := searcher.Count(q)
	must(t, err)
	return n
}

func TestMemoryIndex_PointValuesDoNotAffectPositionsOrOffset(t *testing.T) {
	analyzer := testMemoryIndexSetup()
	mi := NewMemoryIndexWithOffsetsAndPayloads(true, true)
	must(t, mi.AddFieldFromIndexableField(field[*document.TextField](t)(document.NewTextField("text", "quick brown fox", false)), analyzer))
	must(t, mi.AddFieldFromIndexableField(document.NewBinaryPoint("text", []byte("quick")), analyzer))
	must(t, mi.AddFieldFromIndexableField(document.NewBinaryPoint("text", []byte("brown")), analyzer))
	leafReader := leafReaderOf(t, mi.CreateSearcher())
	tenum := textTermsEnum(t, leafReader)

	assertPostingsAt(t, tenum, "brown", 1, 6, 11)
	assertPostingsAt(t, tenum, "fox", 2, 12, 15)
	assertPostingsAt(t, tenum, "quick", 0, 0, 5)

	indexSearcher := mi.CreateSearcher()
	q, err := search.NewBinaryPointExactQuery("text", []byte("quick"))
	if got := countOf(t, indexSearcher, q, err); got != 1 {
		t.Errorf("count(text:quick point): got %d, want 1", got)
	}
	q, err = search.NewBinaryPointExactQuery("text", []byte("brown"))
	if got := countOf(t, indexSearcher, q, err); got != 1 {
		t.Errorf("count(text:brown point): got %d, want 1", got)
	}
	q, err = search.NewBinaryPointExactQuery("text", []byte("jumps"))
	if got := countOf(t, indexSearcher, q, err); got != 0 {
		t.Errorf("count(text:jumps point): got %d, want 0", got)
	}
}

func TestMemoryIndex_2DPoints(t *testing.T) {
	analyzer := testMemoryIndexSetup()
	doc := newDoc(t,
		document.NewIntPoint("ints", 0, -100),
		document.NewIntPoint("ints", 20, 20),
		document.NewIntPoint("ints", 100, -100),
		document.NewLongPoint("longs", 0, -100),
		document.NewLongPoint("longs", 20, 20),
		document.NewLongPoint("longs", 100, -100),
		document.NewFloatPoints("floats", 0, -100),
		document.NewFloatPoints("floats", 20, 20),
		document.NewFloatPoints("floats", 100, -100),
		document.NewDoublePoint("doubles", 0, -100),
		document.NewDoublePoint("doubles", 20, 20),
		document.NewDoublePoint("doubles", 100, -100))

	mi, err := FromDocument(doc.Fields(), analyzer)
	must(t, err)
	mi.CreateSearcher()
	blocked(t, "IntPoint/LongPoint/FloatPoint/DoublePoint.newRangeQuery (org.apache.lucene.document)")
}

func TestMemoryIndex_MultiValuedPointsSortedCorrectly(t *testing.T) {
	analyzer := testMemoryIndexSetup()
	doc := newDoc(t,
		document.NewIntPoint("ints", 3),
		document.NewIntPoint("ints", 2),
		document.NewIntPoint("ints", 1),
		document.NewLongPoint("longs", 3),
		document.NewLongPoint("longs", 2),
		document.NewLongPoint("longs", 1),
		document.NewFloatPoint("floats", 3),
		document.NewFloatPoint("floats", 2),
		document.NewFloatPoint("floats", 1),
		document.NewDoublePoint("doubles", 3),
		document.NewDoublePoint("doubles", 2),
		document.NewDoublePoint("doubles", 1))

	mi, err := FromDocument(doc.Fields(), analyzer)
	must(t, err)
	mi.CreateSearcher()
	blocked(t, "IntPoint/LongPoint/FloatPoint/DoublePoint.newSetQuery (org.apache.lucene.document)")
}

// pointAndBinaryDocValuesType renders the FieldType of
// testIndexingPointsAndDocValues and testToStringDebug.
func pointAndBinaryDocValuesType() *document.FieldType {
	ft := document.NewFieldType()
	ft.SetDimensions(1, 4)
	ft.SetDocValuesType(spi.DocValuesTypeBinary)
	ft.Freeze()
	return ft
}

func TestMemoryIndex_IndexingPointsAndDocValues(t *testing.T) {
	analyzer := testMemoryIndexSetup()
	packedPoint := []byte("term")
	doc := newDoc(t, document.NewBinaryPointExpert("field", packedPoint, pointAndBinaryDocValuesType()))
	mi, err := FromDocument(doc.Fields(), analyzer)
	must(t, err)
	leafReader := leafReaderOf(t, mi.CreateSearcher())

	pv, err := leafReader.GetPointValues("field")
	must(t, err)
	if pv == nil {
		t.Fatal("point values: got nil")
	}
	if pv.Size() != 1 {
		t.Errorf("point values size: got %d, want 1", pv.Size())
	}
	if v, err := pv.GetMinPackedValue(); err != nil || !bytes.Equal(v, packedPoint) {
		t.Errorf("min packed value: got (%v, %v), want %v", v, err, packedPoint)
	}
	if v, err := pv.GetMaxPackedValue(); err != nil || !bytes.Equal(v, packedPoint) {
		t.Errorf("max packed value: got (%v, %v), want %v", v, err, packedPoint)
	}

	dvs, err := leafReader.GetBinaryDocValues("field")
	must(t, err)
	assertNextDoc(t, dvs, 0)
	if v, err := dvs.BinaryValue(); err != nil || string(v) != "term" {
		t.Errorf("binary: got (%q, %v), want \"term\"", v, err)
	}
}

func TestMemoryIndex_ToStringDebug(t *testing.T) {
	mi := NewMemoryIndexWithOffsetsAndPayloads(true, true)
	analyzer := testanalysis.NewMockPayloadAnalyzer()

	must(t, mi.AddFieldFromString("analyzedField", "aa bb aa", analyzer))

	must(t, mi.AddFieldFromIndexableField(
		document.NewBinaryPointExpert("pointAndDvField", []byte("term"), pointAndBinaryDocValuesType()),
		analyzer))

	want := "analyzedField:\n" +
		"\t'[61 61]':2: [(0, 0, 2, [70 6f 73 3a 20 30]), (1, 6, 8, [70 6f 73 3a 20 32])]\n" +
		"\t'[62 62]':1: [(1, 3, 5, [70 6f 73 3a 20 31])]\n" +
		"\tterms=2, positions=3\n" +
		"pointAndDvField:\n" +
		"\tterms=0, positions=0\n" +
		"\n" +
		"fields=2, terms=2, positions=3"
	if got := mi.ToStringDebug(); got != want {
		t.Errorf("toStringDebug:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// storedFieldsDocument renders StoredFields.document(int), whose final body
// visits the document with a DocumentStoredFieldVisitor.
func storedFieldsDocument(t *testing.T, searcher *search.IndexSearcher, docID int) *document.Document {
	t.Helper()
	storedFields, err := searcher.StoredFields()
	must(t, err)
	visitor := document.NewDocumentStoredFieldVisitor()
	must(t, storedFields.Document(docID, visitor))
	return visitor.GetDocument()
}

func TestMemoryIndex_StoredFields(t *testing.T) {
	r := newTestRandom(t)
	s := func(f *document.StoredField, err error) document.IndexableField {
		t.Helper()
		must(t, err)
		return f
	}
	fields := []document.IndexableField{
		s(document.NewStoredFieldFromFloat32("float", 1.5)),
		s(document.NewStoredFieldFromFloat32("multifloat", 2.5)),
		s(document.NewStoredFieldFromFloat32("multifloat", 3.5)),
		s(document.NewStoredFieldFromInt64("long", 10)),
		s(document.NewStoredFieldFromInt64("multilong", 15)),
		s(document.NewStoredFieldFromInt64("multilong", 20)),
		s(document.NewStoredFieldFromFloat64("int", 1.7)),
		s(document.NewStoredFieldFromFloat64("multiint", 2.7)),
		s(document.NewStoredFieldFromFloat64("multiint", 2.8)),
		s(document.NewStoredFieldFromFloat64("multiint", 2.9)),
		s(document.NewStoredFieldFromFloat64("double", 3.7)),
		s(document.NewStoredFieldFromFloat64("multidouble", 4.5)),
		s(document.NewStoredFieldFromFloat64("multidouble", 4.6)),
		s(document.NewStoredFieldFromFloat64("multidouble", 4.7)),
		s(document.NewStoredField("string", "foo")),
		s(document.NewStoredField("multistring", "bar")),
		s(document.NewStoredField("multistring", "baz")),
		s(document.NewStoredFieldFromBytes("binary", []byte("bfoo"))),
		s(document.NewStoredFieldFromBytes("multibinary", []byte("bbar"))),
		s(document.NewStoredFieldFromBytes("multibinary", []byte("bbaz"))),
	}

	r.Shuffle(len(fields), func(i, j int) { fields[i], fields[j] = fields[j], fields[i] })
	doc := newDoc(t, fields...)

	mi, err := FromDocument(doc.Fields(), analysis.NewStandardAnalyzer())
	must(t, err)
	d := storedFieldsDocument(t, mi.CreateSearcher(), 0)

	numericValue := func(f document.IndexableField) any { return f.NumericValue() }
	stringValue := func(f document.IndexableField) any { return f.StringValue() }

	assertContains(t, d, "long", int64(10), numericValue)
	assertContains(t, d, "int", 1.7, numericValue)
	assertContains(t, d, "double", 3.7, numericValue)
	assertContains(t, d, "float", float32(1.5), numericValue)
	assertContains(t, d, "string", "foo", stringValue)
	assertBinaryContains(t, d, "binary", []byte("bfoo"))

	assertMultiContains(t, d, "multilong", []any{int64(15), int64(20)}, numericValue)
	assertMultiContains(t, d, "multiint", []any{2.7, 2.8, 2.9}, numericValue)
	assertMultiContains(t, d, "multidouble", []any{4.5, 4.6, 4.7}, numericValue)
	assertMultiContains(t, d, "multifloat", []any{float32(2.5), float32(3.5)}, numericValue)
	assertMultiContains(t, d, "multistring", []any{"bar", "baz"}, stringValue)
	assertBinaryMultiContains(t, d, "multibinary", [][]byte{[]byte("bbar"), []byte("bbaz")})
}

func TestMemoryIndex_KnnFloatVectorOnlyOneVectorAllowed(t *testing.T) {
	doc := newDoc(t,
		field[*document.KnnFloatVectorField](t)(document.NewKnnFloatVectorFieldEuclidean("knnFloatA", []float32{1.0, 2.0})),
		field[*document.KnnFloatVectorField](t)(document.NewKnnFloatVectorFieldEuclidean("knnFloatA", []float32{3.0, 4.0})))
	if _, err := FromDocument(doc.Fields(), analysis.NewStandardAnalyzer()); err == nil {
		t.Error("fromDocument with two vectors for one field: got nil error, want IllegalArgumentException")
	}
}

func knnSearchTotalHits(t *testing.T, mi *MemoryIndex, q search.Query) int64 {
	t.Helper()
	docs, err := mi.CreateSearcher().Search(q, 10)
	must(t, err)
	return docs.TotalHits.Value
}

func TestMemoryIndex_KnnFloatVectors(t *testing.T) {
	r := newTestRandom(t)
	fields := []document.IndexableField{
		field[*document.KnnFloatVectorField](t)(document.NewKnnFloatVectorFieldEuclidean("knnFloatA", []float32{1.0, 2.0})),
		field[*document.KnnFloatVectorField](t)(document.NewKnnFloatVectorFieldEuclidean("knnFloatB", []float32{3.0, 4.0, 5.0, 6.0})),
		field[*document.KnnFloatVectorField](t)(document.NewKnnFloatVectorField("knnFloatC", []float32{7.0, 8.0, 9.0}, util.DotProductSim)),
	}
	r.Shuffle(len(fields), func(i, j int) { fields[i], fields[j] = fields[j], fields[i] })
	doc := newDoc(t, fields...)

	mi, err := FromDocument(doc.Fields(), analysis.NewStandardAnalyzer())
	must(t, err)
	assertFloatVectorValue(t, mi, "knnFloatA", []float32{1.0, 2.0})
	assertFloatVectorValue(t, mi, "knnFloatB", []float32{3.0, 4.0, 5.0, 6.0})
	assertFloatVectorValue(t, mi, "knnFloatC", []float32{7.0, 8.0, 9.0})

	assertFloatVectorScore(t, r, mi, "knnFloatA", []float32{1.0, 1.0}, 0.5)
	assertFloatVectorScore(t, r, mi, "knnFloatB", []float32{3.0, 3.0, 3.0, 3.0}, 0.06666667)
	assertFloatVectorScore(t, r, mi, "knnFloatC", []float32{7.0, 7.0, 7.0}, 84.5)

	if v, err := leafReaderOf(t, mi.CreateSearcher()).GetFloatVectorValues("knnFloatMissing"); err != nil || v != nil {
		t.Errorf("float vectors of a missing field: got (%v, %v), want nil", v, err)
	}
	if v, err := leafReaderOf(t, mi.CreateSearcher()).GetByteVectorValues("knnByteVectorValue"); err != nil || v != nil {
		t.Errorf("byte vectors of a missing field: got (%v, %v), want nil", v, err)
	}
	// we don't really do knn search right now for MemoryIndex
	if got := knnSearchTotalHits(t, mi, search.NewKnnFloatVectorQuery("knnFloatA", []float32{1.0, 1.0}, 1)); got != 0 {
		t.Errorf("knn float search total hits: got %d, want 0", got)
	}
}

func TestMemoryIndex_KnnByteVectorOnlyOneVectorAllowed(t *testing.T) {
	doc := newDoc(t,
		field[*document.KnnByteVectorField](t)(document.NewKnnByteVectorFieldEuclidean("knnByteA", []byte{1, 2})),
		field[*document.KnnByteVectorField](t)(document.NewKnnByteVectorFieldEuclidean("knnByteA", []byte{3, 4})))
	if _, err := FromDocument(doc.Fields(), analysis.NewStandardAnalyzer()); err == nil {
		t.Error("fromDocument with two vectors for one field: got nil error, want IllegalArgumentException")
	}
}

func TestMemoryIndex_KnnByteVectors(t *testing.T) {
	r := newTestRandom(t)
	fields := []document.IndexableField{
		field[*document.KnnByteVectorField](t)(document.NewKnnByteVectorFieldEuclidean("knnByteA", []byte{1, 2})),
		field[*document.KnnByteVectorField](t)(document.NewKnnByteVectorFieldEuclidean("knnByteB", []byte{3, 4, 5, 6})),
		field[*document.KnnByteVectorField](t)(document.NewKnnByteVectorField("knnByteC", []byte{7, 8, 9}, util.DotProductSim)),
	}
	r.Shuffle(len(fields), func(i, j int) { fields[i], fields[j] = fields[j], fields[i] })
	doc := newDoc(t, fields...)

	mi, err := FromDocument(doc.Fields(), analysis.NewStandardAnalyzer())
	must(t, err)
	assertByteVectorValue(t, mi, "knnByteA", []byte{1, 2})
	assertByteVectorValue(t, mi, "knnByteB", []byte{3, 4, 5, 6})
	assertByteVectorValue(t, mi, "knnByteC", []byte{7, 8, 9})

	assertByteVectorScore(t, r, mi, "knnByteA", []byte{1, 1}, 0.5)
	assertByteVectorScore(t, r, mi, "knnByteB", []byte{3, 3, 3, 3}, 0.06666667)
	assertByteVectorScore(t, r, mi, "knnByteC", []byte{7, 7, 7}, 0.501709)

	if v, err := leafReaderOf(t, mi.CreateSearcher()).GetByteVectorValues("knnByteMissing"); err != nil || v != nil {
		t.Errorf("byte vectors of a missing field: got (%v, %v), want nil", v, err)
	}
	if v, err := leafReaderOf(t, mi.CreateSearcher()).GetFloatVectorValues("knnFloatVectorValue"); err != nil || v != nil {
		t.Errorf("float vectors of a missing field: got (%v, %v), want nil", v, err)
	}
	// we don't really do knn search right now for MemoryIndex
	if got := knnSearchTotalHits(t, mi, search.NewKnnByteVectorQuery("knnByteA", []byte{1, 1}, 1)); got != 0 {
		t.Errorf("knn byte search total hits: got %d, want 0", got)
	}
}

func assertFloatVectorValue(t *testing.T, mi *MemoryIndex, fieldName string, expected []float32) {
	t.Helper()
	fvv, err := leafReaderOf(t, mi.CreateSearcher()).GetFloatVectorValues(fieldName)
	must(t, err)
	if fvv == nil {
		t.Fatalf("float vectors of %s: got nil", fieldName)
	}
	iterator := fvv.Iterator()
	assertNextDoc(t, iterator, 0)
	got, err := fvv.VectorValue(0)
	must(t, err)
	if len(got) != len(expected) {
		t.Fatalf("vector of %s: got %v, want %v", fieldName, got, expected)
	}
	for i := range expected {
		if d := got[i] - expected[i]; d > 1e-6 || d < -1e-6 {
			t.Errorf("vector of %s: got %v, want %v", fieldName, got, expected)
			break
		}
	}
	assertNextDoc(t, iterator, spi.NO_MORE_DOCS)
}

func assertFloatVectorScore(t *testing.T, r *rand.Rand, mi *MemoryIndex, fieldName string, queryVector []float32, expectedScore float32) {
	t.Helper()
	fvv, err := leafReaderOf(t, mi.CreateSearcher()).GetFloatVectorValues(fieldName)
	must(t, err)
	if fvv == nil {
		t.Fatalf("float vectors of %s: got nil", fieldName)
	}
	if r.Intn(2) == 0 {
		_, err := fvv.Iterator().NextDoc()
		must(t, err)
	}
	scorer, err := fvv.Scorer(queryVector)
	must(t, err)
	assertNextDoc(t, scorer.Iterator(), 0)
	if s, err := scorer.Score(); err != nil || s != expectedScore {
		t.Errorf("score of %s: got (%v, %v), want %v", fieldName, s, err, expectedScore)
	}
	assertNextDoc(t, scorer.Iterator(), spi.NO_MORE_DOCS)
}

func assertByteVectorValue(t *testing.T, mi *MemoryIndex, fieldName string, expected []byte) {
	t.Helper()
	bvv, err := leafReaderOf(t, mi.CreateSearcher()).GetByteVectorValues(fieldName)
	must(t, err)
	if bvv == nil {
		t.Fatalf("byte vectors of %s: got nil", fieldName)
	}
	iterator := bvv.Iterator()
	assertNextDoc(t, iterator, 0)
	if got, err := bvv.VectorValue(0); err != nil || !bytes.Equal(got, expected) {
		t.Errorf("vector of %s: got (%v, %v), want %v", fieldName, got, err, expected)
	}
	assertNextDoc(t, iterator, spi.NO_MORE_DOCS)
}

func assertByteVectorScore(t *testing.T, r *rand.Rand, mi *MemoryIndex, fieldName string, queryVector []byte, expectedScore float32) {
	t.Helper()
	bvv, err := leafReaderOf(t, mi.CreateSearcher()).GetByteVectorValues(fieldName)
	must(t, err)
	if bvv == nil {
		t.Fatalf("byte vectors of %s: got nil", fieldName)
	}
	if r.Intn(2) == 0 {
		_, err := bvv.Iterator().NextDoc()
		must(t, err)
	}
	scorer, err := bvv.Scorer(queryVector)
	must(t, err)
	assertNextDoc(t, scorer.Iterator(), 0)
	if s, err := scorer.Score(); err != nil || s != expectedScore {
		t.Errorf("score of %s: got (%v, %v), want %v", fieldName, s, err, expectedScore)
	}
	assertNextDoc(t, scorer.Iterator(), spi.NO_MORE_DOCS)
}

func assertContains(t *testing.T, d *document.Document, fieldName string, expected any, value func(document.IndexableField) any) {
	t.Helper()
	if d.GetField(fieldName) == nil {
		t.Errorf("field %s: got nil", fieldName)
		return
	}
	fields := d.GetFieldsArray(fieldName)
	if len(fields) != 1 {
		t.Errorf("fields %s: got %d, want 1", fieldName, len(fields))
		return
	}
	if got := value(fields[0]); got != expected {
		t.Errorf("field %s: got %#v, want %#v", fieldName, got, expected)
	}
}

func assertBinaryContains(t *testing.T, d *document.Document, fieldName string, expected []byte) {
	t.Helper()
	if d.GetField(fieldName) == nil {
		t.Errorf("field %s: got nil", fieldName)
		return
	}
	fields := d.GetFieldsArray(fieldName)
	if len(fields) != 1 {
		t.Errorf("fields %s: got %d, want 1", fieldName, len(fields))
		return
	}
	if bytes.Compare(expected, fields[0].BinaryValue()) != 0 {
		t.Errorf("field %s: got %q, want %q", fieldName, fields[0].BinaryValue(), expected)
	}
}

// assertMultiContains renders the Java helper; Document.get(String) returns
// the first non-null stringValue of the named fields, so its null check is
// rendered through document.Document.Get.
func assertMultiContains(t *testing.T, d *document.Document, fieldName string, expected []any, value func(document.IndexableField) any) {
	t.Helper()
	if d.Get(fieldName) == nil {
		t.Errorf("field %s: got nil", fieldName)
		return
	}
	fields := d.GetFieldsArray(fieldName)
	if len(fields) != len(expected) {
		t.Errorf("fields %s: got %d, want %d", fieldName, len(fields), len(expected))
		return
	}
	for _, f := range fields {
		actual := value(f)
		if !arrayContains(expected, actual) {
			t.Errorf("field %s: %#v not in %#v", fieldName, actual, expected)
		}
	}
}

func assertBinaryMultiContains(t *testing.T, d *document.Document, fieldName string, expected [][]byte) {
	t.Helper()
	fields := d.GetFieldsArray(fieldName)
	if len(fields) != len(expected) {
		t.Errorf("fields %s: got %d, want %d", fieldName, len(fields), len(expected))
		return
	}
	for _, f := range fields {
		actual := f.BinaryValue()
		if !arrayBinaryContains(expected, actual) {
			t.Errorf("field %s: %q not in %q", fieldName, actual, expected)
		}
	}
}

func arrayContains(array []any, value any) bool {
	for _, o := range array {
		if o == value {
			return true
		}
	}
	return false
}

func arrayBinaryContains(array [][]byte, value []byte) bool {
	for _, b := range array {
		if bytes.Compare(b, value) == 0 {
			return true
		}
	}
	return false
}

func TestMemoryIndex_IntegerNumericDocValue(t *testing.T) {
	// MemoryIndex used to fail when doc values are enabled and numericValue() returns an Integer
	// such as with IntField.
	ft := document.NewFieldType()
	ft.SetDocValuesType(spi.DocValuesTypeNumeric)
	ft.Freeze()
	// new Field("field", ft) { { fieldsData = 35; } }
	f, err := document.NewField("field", int32(35), ft)
	must(t, err)

	multiFt := document.NewFieldType()
	multiFt.SetDocValuesType(spi.DocValuesTypeSortedNumeric)
	multiFt.Freeze()
	multiField, err := document.NewField("multi_field", int32(42), multiFt)
	must(t, err)

	intField, err := document.NewIntFieldLucene("int_field", 50, false)
	must(t, err)

	mi, err := FromDocument([]index.IndexableField{f, multiField, intField}, nil)
	must(t, err)
	searcher := mi.CreateSearcher()

	ndv, err := leafReaderOf(t, searcher).GetNumericDocValues("field")
	must(t, err)
	if ok, err := ndv.AdvanceExact(0); err != nil || !ok {
		t.Fatalf("field advanceExact(0): got (%v, %v), want true", ok, err)
	}
	if v, err := ndv.LongValue(); err != nil || v != 35 {
		t.Errorf("field: got (%d, %v), want 35", v, err)
	}

	sndv, err := leafReaderOf(t, searcher).GetSortedNumericDocValues("multi_field")
	must(t, err)
	if ok, err := sndv.AdvanceExact(0); err != nil || !ok {
		t.Fatalf("multi_field advanceExact(0): got (%v, %v), want true", ok, err)
	}
	if c, err := sndv.DocValueCount(); err != nil || c != 1 {
		t.Errorf("multi_field docValueCount: got (%d, %v), want 1", c, err)
	}
	if v, err := sndv.NextValue(); err != nil || v != 42 {
		t.Errorf("multi_field: got (%d, %v), want 42", v, err)
	}

	sndv, err = leafReaderOf(t, searcher).GetSortedNumericDocValues("int_field")
	must(t, err)
	if ok, err := sndv.AdvanceExact(0); err != nil || !ok {
		t.Fatalf("int_field advanceExact(0): got (%v, %v), want true", ok, err)
	}
	if c, err := sndv.DocValueCount(); err != nil || c != 1 {
		t.Errorf("int_field docValueCount: got (%d, %v), want 1", c, err)
	}
	if v, err := sndv.NextValue(); err != nil || v != 50 {
		t.Errorf("int_field: got (%d, %v), want 50", v, err)
	}
}

// mockIndexableField renders the private record MockIndexableField.
type mockIndexableField struct {
	field     string
	value     []byte
	fieldType spi.IndexableFieldType
}

func (f mockIndexableField) Name() string { return f.field }

func (f mockIndexableField) FieldType() spi.IndexableFieldType { return f.fieldType }

func (f mockIndexableField) TokenStream(analysis.Analyzer, analysis.TokenStream) analysis.TokenStream {
	return nil
}

func (f mockIndexableField) BinaryValue() []byte { return f.value }

// StringValue renders `return null`; Gocene renders a null String as "".
func (f mockIndexableField) StringValue() string { return "" }

func (f mockIndexableField) GetCharSequenceValue() string { return f.StringValue() }

func (f mockIndexableField) ReaderValue() io.Reader { return nil }

func (f mockIndexableField) NumericValue() any { return nil }

func (f mockIndexableField) StoredValue() *document.StoredValue { return nil }

func (f mockIndexableField) InvertableType() document.InvertableType {
	return document.InvertableTypeBinary
}

func TestMemoryIndex_KeywordWithoutTokenStream(t *testing.T) {
	var legalFieldTypes []*document.FieldType
	newFT := func(indexOptions spi.IndexOptions, omitNorms, storeTermVectors, setOmitNorms bool) *document.FieldType {
		ft := document.NewFieldType()
		ft.SetTokenized(false)
		ft.SetIndexOptions(indexOptions)
		if setOmitNorms {
			ft.SetOmitNorms(omitNorms)
		}
		if storeTermVectors {
			ft.SetStoreTermVectors(true)
		}
		ft.Freeze()
		return ft
	}
	legalFieldTypes = append(legalFieldTypes,
		newFT(spi.IndexOptionsDocs, false, false, true),
		newFT(spi.IndexOptionsDocsAndFreqs, false, false, true),
		newFT(spi.IndexOptionsDocs, true, false, true),
		newFT(spi.IndexOptionsDocsAndFreqs, true, false, true),
		newFT(spi.IndexOptionsDocs, false, true, false),
		newFT(spi.IndexOptionsDocsAndFreqs, false, true, false))

	for _, ft := range legalFieldTypes {
		f := mockIndexableField{field: "field", value: []byte("a"), fieldType: ft}
		mi, err := FromDocument([]index.IndexableField{f, f}, nil)
		must(t, err)
		leafReader := leafReaderOf(t, mi.CreateSearcher())
		{
			terms, err := leafReader.Terms("field")
			must(t, err)
			assertKeywordTerms(t, terms)
		}

		if ft.StoreTermVectors() {
			termVectors, err := leafReader.TermVectors()
			must(t, err)
			fields, err := termVectors.Get(0)
			must(t, err)
			tvTerms, err := fields.Terms("field")
			must(t, err)
			assertKeywordTerms(t, tvTerms)
		}
	}
}

func assertKeywordTerms(t *testing.T, terms index.Terms) {
	t.Helper()
	if v, err := terms.GetSumDocFreq(); err != nil || v != 1 {
		t.Errorf("sumDocFreq: got (%d, %v), want 1", v, err)
	}
	if v, err := terms.GetSumTotalTermFreq(); err != nil || v != 2 {
		t.Errorf("sumTotalTermFreq: got (%d, %v), want 2", v, err)
	}
	termsEnum, err := terms.Iterator()
	must(t, err)
	if ok, err := termsEnum.SeekExact(index.NewTerm("field", "a")); err != nil || !ok {
		t.Fatalf("seekExact(a): got (%v, %v), want true", ok, err)
	}
	pe, err := termsEnum.Postings(spi.PostingsFlagAll)
	must(t, err)
	assertNextDoc(t, pe, 0)
	if f, err := pe.Freq(); err != nil || f != 2 {
		t.Errorf("freq: got (%d, %v), want 2", f, err)
	}
	for _, want := range []int{0, 1} {
		if p, err := pe.NextPosition(); err != nil || p != want {
			t.Errorf("nextPosition: got (%d, %v), want %d", p, err, want)
		}
	}
	assertNextDoc(t, pe, spi.NO_MORE_DOCS)
}
