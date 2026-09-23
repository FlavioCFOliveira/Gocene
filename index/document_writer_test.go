// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestDocumentWriter.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"io"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const (
	docHelperMissing                = "org.apache.lucene.tests.index.DocHelper is not ported"
	indexWriterNewestSegmentMissing = "org.apache.lucene.index.IndexWriter#newestSegment() is not ported"
	testUtilCheckIndexMissing       = "TestUtil.checkIndex(Directory) is not ported"
	indexWriterRAMAccountingMissing = "org.apache.lucene.index.IndexWriter#hasChangesInRam() and IndexWriter#ramBytesUsed() are not ported"
)

// documentWriterTest renders the per-test setUp/tearDown of
// TestDocumentWriter: dir is created before and closed after each test.
func documentWriterTest(t *testing.T) store.Directory {
	t.Helper()
	dir := newDirectory()
	t.Cleanup(func() {
		if err := dir.Close(); err != nil {
			t.Errorf("tearDown: close dir: %v", err)
		}
	})
	return dir
}

func TestDocumentWriterAddDocument(t *testing.T) {
	documentWriterTest(t)
	t.Fatal(docHelperMissing + " (DocHelper.setupDoc(Document))")
}

// positionIncrementGapAnalyzer renders the anonymous Analyzer of
// testPositionIncrementGap.
type positionIncrementGapAnalyzer struct {
	*analysis.BaseAnalyzer
}

func (a *positionIncrementGapAnalyzer) GetPositionIncrementGap(string) int { return 500 }

func whitespaceComponents(createFilter func(*testanalysis.MockTokenizer) analysis.TokenStream) func(string) *analysis.TokenStreamComponents {
	return func(string) *analysis.TokenStreamComponents {
		src := testanalysis.NewMockTokenizer(testanalysis.WHITESPACE, false, testanalysis.DefaultMaxTokenLength)
		var sink analysis.TokenStream = src
		if createFilter != nil {
			sink = createFilter(src)
		}
		return &analysis.TokenStreamComponents{
			Source: func(r io.Reader) error {
				src.SetReader(r)
				return nil
			},
			Sink: sink,
		}
	}
}

func TestDocumentWriterPositionIncrementGap(t *testing.T) {
	dir := documentWriterTest(t)
	base := analysis.NewAnalyzer(nil)
	base.CreateComponents = whitespaceComponents(nil)
	analyzer := &positionIncrementGapAnalyzer{BaseAnalyzer: base}

	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(analyzer))

	doc := document.NewDocument()
	doc.Add(newTextField(t, "repeated", "repeated one", true))
	doc.Add(newTextField(t, "repeated", "repeated two", true))

	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)
	defer mustClose(t, writer)
	t.Fatal(indexWriterNewestSegmentMissing)
}

// tokenReuseFilter renders the anonymous TokenFilter of testTokenReuse: it
// indexes a "synonym" b for every token, with a payload on the first
// position only.
type tokenReuseFilter struct {
	*analysis.BaseTokenFilter
	first      bool
	state      *util.AttributeState
	termAtt    analysis.CharTermAttribute
	payloadAtt analysis.PayloadAttribute
	posIncrAtt tokenattributes.PositionIncrementAttribute
}

func newTokenReuseFilter(input analysis.TokenStream) *tokenReuseFilter {
	f := &tokenReuseFilter{BaseTokenFilter: analysis.NewBaseTokenFilter(input), first: true}
	f.termAtt = f.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	f.payloadAtt = f.AddAttribute(analysis.PayloadAttributeType).(analysis.PayloadAttribute)
	f.posIncrAtt = f.AddAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	return f
}

func (f *tokenReuseFilter) IncrementToken() (bool, error) {
	if f.state != nil {
		f.RestoreState(f.state)
		f.payloadAtt.SetPayload(nil)
		f.posIncrAtt.SetPositionIncrement(0)
		f.termAtt.SetEmpty()
		f.termAtt.AppendString("b")
		f.state = nil
		return true, nil
	}

	hasNext, err := f.GetInput().IncrementToken()
	if err != nil || !hasNext {
		return false, err
	}
	if c := f.termAtt.Buffer()[0]; c >= '0' && c <= '9' {
		f.posIncrAtt.SetPositionIncrement(int(c - '0'))
	}
	if f.first {
		// set payload on first position only
		f.payloadAtt.SetPayload([]byte{100})
		f.first = false
	}

	// index a "synonym" for every token
	f.state = f.CaptureState()
	return true, nil
}

func (f *tokenReuseFilter) Reset() error {
	if err := f.BaseTokenFilter.Reset(); err != nil {
		return err
	}
	f.first = true
	f.state = nil
	return nil
}

func TestDocumentWriterTokenReuse(t *testing.T) {
	dir := documentWriterTest(t)
	analyzer := analysis.NewAnalyzer(nil)
	analyzer.CreateComponents = whitespaceComponents(func(tokenizer *testanalysis.MockTokenizer) analysis.TokenStream {
		return newTokenReuseFilter(tokenizer)
	})

	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(analyzer))

	doc := document.NewDocument()
	doc.Add(newTextField(t, "f1", "a 5 a a", true))

	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)
	defer mustClose(t, writer)
	t.Fatal(indexWriterNewestSegmentMissing)
}

// preAnalyzedTokenStream renders the anonymous TokenStream of
// testPreAnalyzedField.
type preAnalyzedTokenStream struct {
	*analysis.BaseTokenStream
	tokens  []string
	index   int
	termAtt analysis.CharTermAttribute
}

func newPreAnalyzedTokenStream() *preAnalyzedTokenStream {
	s := &preAnalyzedTokenStream{BaseTokenStream: analysis.NewBaseTokenStream(), tokens: []string{"term1", "term2", "term3", "term2"}}
	s.termAtt = s.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	return s
}

func (s *preAnalyzedTokenStream) IncrementToken() (bool, error) {
	if s.index == len(s.tokens) {
		return false, nil
	}
	s.ClearAttributes()
	s.termAtt.SetEmpty()
	s.termAtt.AppendString(s.tokens[s.index])
	s.index++
	return true, nil
}

func TestDocumentWriterPreAnalyzedField(t *testing.T) {
	dir := documentWriterTest(t)
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()

	f, err := document.NewField("preanalyzed", newPreAnalyzedTokenStream(), document.TextFieldTypeNotStored)
	if err != nil {
		t.Fatalf("TextField: %v", err)
	}
	doc.Add(f)

	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)
	defer mustClose(t, writer)
	t.Fatal(indexWriterNewestSegmentMissing)
}

// Test adding two fields with the same name, one indexed the other stored
// only. The omitNorms and omitTermFreqAndPositions setting of the stored
// field should not affect the indexed one (LUCENE-1590)
func TestDocumentWriterLUCENE_1590(t *testing.T) {
	dir := documentWriterTest(t)
	doc := document.NewDocument()
	// f1 has no norms
	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetOmitNorms(true)
	customType2 := document.NewFieldType()
	customType2.SetStored(true)
	doc.Add(newField(t, "f1", "v1", customType))
	doc.Add(newField(t, "f1", "v2", customType2))
	// f2 has no TF
	customType3 := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType3.SetIndexOptions(index.IndexOptionsDocs)
	doc.Add(newField(t, "f2", "v1", customType3))
	doc.Add(newField(t, "f2", "v2", customType2))

	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	mustAddDocument(t, writer, doc)
	if err := writer.ForceMerge(1); err != nil { // be sure to have a single segment
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer)

	t.Fatal(testUtilCheckIndexMissing)
}

// doTestRAMUsage renders the private doTestRAMUsage(Function): make sure
// that every new field doesn't increment memory usage by more than 16kB.
func doTestRAMUsage(t *testing.T, fieldSupplier func(string) document.IndexableField) {
	t.Helper()
	dir := newDirectory()
	conf := newIndexWriterConfig()
	conf.SetMaxBufferedDocs(10)
	conf.SetRAMBufferSizeMB(index.DisableAutoFlush)
	w := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, w, dir)
	doc := document.NewDocument()
	const numFields = 100
	for i := 0; i < numFields; i++ {
		doc.Add(fieldSupplier("f" + strconv.Itoa(i)))
	}
	mustAddDocument(t, w, doc)
	t.Fatal(indexWriterRAMAccountingMissing)
}

func TestDocumentWriterRAMUsageStored(t *testing.T) {
	doTestRAMUsage(t, func(field string) document.IndexableField {
		return mustStoredBytesField(t, field, []byte("Lucene"))
	})
}

func TestDocumentWriterRAMUsageIndexed(t *testing.T) {
	doTestRAMUsage(t, func(field string) document.IndexableField {
		f, err := document.NewStringFieldFromBytesRef(field, []byte("Lucene"), false)
		if err != nil {
			t.Fatalf("StringField: %v", err)
		}
		return f
	})
}

func TestDocumentWriterRAMUsagePoint(t *testing.T) {
	doTestRAMUsage(t, func(field string) document.IndexableField { return document.NewIntPoint(field, 42) })
}

func TestDocumentWriterRAMUsageNumericDocValue(t *testing.T) {
	doTestRAMUsage(t, func(field string) document.IndexableField { return numericDVField(t, field, 42) })
}

func TestDocumentWriterRAMUsageSortedDocValue(t *testing.T) {
	doTestRAMUsage(t, func(field string) document.IndexableField { return sortedDVField(t, field, []byte("Lucene")) })
}

func TestDocumentWriterRAMUsageBinaryDocValue(t *testing.T) {
	doTestRAMUsage(t, func(field string) document.IndexableField { return binaryDVField(t, field, []byte("Lucene")) })
}

func TestDocumentWriterRAMUsageSortedNumericDocValue(t *testing.T) {
	doTestRAMUsage(t, func(field string) document.IndexableField {
		f, err := document.NewSortedNumericDocValuesField(field, []int64{42})
		if err != nil {
			t.Fatalf("SortedNumericDocValuesField: %v", err)
		}
		return f
	})
}

func TestDocumentWriterRAMUsageSortedSetDocValue(t *testing.T) {
	doTestRAMUsage(t, func(field string) document.IndexableField { return sortedSetDVField(t, field, []byte("Lucene")) })
}

func TestDocumentWriterRAMUsageVector(t *testing.T) {
	doTestRAMUsage(t, func(field string) document.IndexableField {
		f, err := document.NewKnnFloatVectorField(field, []float32{1, 2, 3, 4}, index.VectorSimilarityFunctionEuclidean)
		if err != nil {
			t.Fatalf("KnnFloatVectorField: %v", err)
		}
		return f
	})
}

// mockIndexableField is the private record MockIndexableField.
type mockIndexableField struct {
	field     string
	value     []byte
	fieldType spi.IndexableFieldType
}

func (f *mockIndexableField) Name() string                       { return f.field }
func (f *mockIndexableField) FieldType() spi.IndexableFieldType  { return f.fieldType }
func (f *mockIndexableField) BinaryValue() []byte                { return f.value }
func (f *mockIndexableField) StringValue() string                { return "" }
func (f *mockIndexableField) ReaderValue() io.Reader             { return nil }
func (f *mockIndexableField) NumericValue() interface{}          { return nil }
func (f *mockIndexableField) StoredValue() *document.StoredValue { return nil }
func (f *mockIndexableField) InvertableType() document.InvertableType {
	return document.InvertableTypeBinary
}

// GetCharSequenceValue is the Go-only IndexableField member; the record's
// stringValue() is null.
func (f *mockIndexableField) GetCharSequenceValue() string { return "" }

func (f *mockIndexableField) TokenStream(analysis.Analyzer, analysis.TokenStream) analysis.TokenStream {
	return nil
}

func frozenFieldType(tokenized bool, options index.IndexOptions, configure func(*document.FieldType)) *document.FieldType {
	ft := document.NewFieldType()
	ft.SetTokenized(tokenized)
	ft.SetIndexOptions(options)
	if configure != nil {
		configure(ft)
	}
	ft.Freeze()
	return ft
}

func createModeWriter(t *testing.T, dir store.Directory) *index.IndexWriter {
	t.Helper()
	conf := newIndexWriterConfig()
	conf.SetOpenMode(index.Create)
	return mustNewIndexWriter(t, dir, conf)
}

func TestDocumentWriterIndexBinaryValueWithoutTokenStream(t *testing.T) {
	dir := documentWriterTest(t)
	illegalFieldTypes := []*document.FieldType{
		// cannot index a tokenized binary field
		frozenFieldType(true, index.IndexOptionsDocs, nil),
		// cannot index positions on a binary field
		frozenFieldType(false, index.IndexOptionsDocsAndFreqsAndPositions, nil),
		// cannot index term vector positions
		frozenFieldType(false, index.IndexOptionsDocs, func(ft *document.FieldType) {
			ft.SetStoreTermVectors(true)
			ft.SetStoreTermVectorPositions(true)
		}),
		// cannot index term vector offsets
		frozenFieldType(false, index.IndexOptionsDocs, func(ft *document.FieldType) {
			ft.SetStoreTermVectors(true)
			ft.SetStoreTermVectorOffsets(true)
		}),
	}

	for _, ft := range illegalFieldTypes {
		w := createModeWriter(t, dir)
		doc := document.NewDocument()
		doc.Add(&mockIndexableField{field: "field", value: []byte("a"), fieldType: ft})
		if _, err := w.AddDocument(doc); err == nil {
			mustClose(t, w)
			t.Fatal("expected IllegalArgumentException from addDocument")
		}
		mustClose(t, w)
	}

	{
		w := createModeWriter(t, dir)
		// Field that has both a null token stream and a null binary value
		doc := document.NewDocument()
		doc.Add(&mockIndexableField{field: "field", value: nil, fieldType: document.StringFieldTypeNotStored})
		if _, err := w.AddDocument(doc); err == nil {
			mustClose(t, w)
			t.Fatal("expected IllegalArgumentException from addDocument")
		}
		mustClose(t, w)
	}

	legalFieldTypes := []*document.FieldType{
		frozenFieldType(false, index.IndexOptionsDocs, func(ft *document.FieldType) { ft.SetOmitNorms(false) }),
		frozenFieldType(false, index.IndexOptionsDocsAndFreqs, func(ft *document.FieldType) { ft.SetOmitNorms(false) }),
		frozenFieldType(false, index.IndexOptionsDocs, func(ft *document.FieldType) { ft.SetOmitNorms(true) }),
		frozenFieldType(false, index.IndexOptionsDocsAndFreqs, func(ft *document.FieldType) { ft.SetOmitNorms(true) }),
		frozenFieldType(false, index.IndexOptionsDocs, func(ft *document.FieldType) { ft.SetStoreTermVectors(true) }),
		frozenFieldType(false, index.IndexOptionsDocsAndFreqs, func(ft *document.FieldType) { ft.SetStoreTermVectors(true) }),
	}

	for _, ft := range legalFieldTypes {
		w := createModeWriter(t, dir)
		field := &mockIndexableField{field: "field", value: []byte("a"), fieldType: ft}
		doc := document.NewDocument()
		doc.Add(field)
		doc.Add(field)
		mustAddDocument(t, w, doc)
		mustClose(t, w)

		reader := mustOpenDirectoryReader(t, dir)
		leafReader := getOnlyLeafReader(t, reader)
		withFreqs := ft.IndexOptions() >= index.IndexOptionsDocsAndFreqs

		terms, err := leafReader.Terms("field")
		if err != nil {
			t.Fatalf("terms: %v", err)
		}
		assertBinaryTerms(t, terms, 2, withFreqs)

		termVectors, err := leafReader.TermVectors()
		if err != nil {
			t.Fatalf("termVectors: %v", err)
		}
		tvFields, err := termVectors.Get(0)
		if err != nil {
			t.Fatalf("termVectors().get(0): %v", err)
		}
		if ft.StoreTermVectors() {
			tvTerms, err := tvFields.Terms("field")
			if err != nil {
				t.Fatalf("tv terms: %v", err)
			}
			assertBinaryTerms(t, tvTerms, 2, true)
		} else if tvFields != nil {
			t.Fatalf("assertNull(leafReader.termVectors().get(0)): got %v", tvFields)
		}
		mustClose(t, reader)
	}
}

// assertBinaryTerms renders the terms/postings checks of
// testIndexBinaryValueWithoutTokenStream for the single term "a" indexed
// twice in doc 0.
func assertBinaryTerms(t *testing.T, terms index.Terms, twice int64, withFreqs bool) {
	t.Helper()
	if terms == nil {
		t.Fatal("terms is null")
	}
	if sdf, err := terms.GetSumDocFreq(); err != nil || sdf != 1 {
		t.Fatalf("getSumDocFreq: expected 1, got %d (%v)", sdf, err)
	}
	expectedTTF := int64(1)
	if withFreqs {
		expectedTTF = twice
	}
	if ttf, err := terms.GetSumTotalTermFreq(); err != nil || ttf != expectedTTF {
		t.Fatalf("getSumTotalTermFreq: expected %d, got %d (%v)", expectedTTF, ttf, err)
	}
	termsEnum, err := terms.Iterator()
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	if found, err := termsEnum.SeekExact(spi.NewTerm("field", "a")); err != nil || !found {
		t.Fatalf("seekExact(a): %v (%v)", found, err)
	}
	pe, err := termsEnum.Postings(spi.PostingsFlagAll)
	if err != nil {
		t.Fatalf("postings: %v", err)
	}
	if doc, err := pe.NextDoc(); err != nil || doc != 0 {
		t.Fatalf("nextDoc: expected 0, got %d (%v)", doc, err)
	}
	expectedFreq := 1
	if withFreqs {
		expectedFreq = int(twice)
	}
	if freq, err := pe.Freq(); err != nil || freq != expectedFreq {
		t.Fatalf("freq: expected %d, got %d (%v)", expectedFreq, freq, err)
	}
	if pos, err := pe.NextPosition(); err != nil || pos != -1 {
		t.Fatalf("nextPosition: expected -1, got %d (%v)", pos, err)
	}
	if doc, err := pe.NextDoc(); err != nil || doc != spi.NO_MORE_DOCS {
		t.Fatalf("nextDoc: expected NO_MORE_DOCS, got %d (%v)", doc, err)
	}
}
