// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestCustomTermFreq.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// indexOptionsDocsAndCustomFreqsMissing names the IndexOptions constant
// getTermDocFieldType() sets.
const indexOptionsDocsAndCustomFreqsMissing = "org.apache.lucene.index.IndexOptions#DOCS_AND_CUSTOM_FREQS is not ported"

// cannedTermFreqs is the private CannedTermFreqs TokenStream.
type cannedTermFreqs struct {
	*analysis.BaseTokenStream
	terms       []string
	termFreqs   []int
	termAtt     analysis.CharTermAttribute
	termFreqAtt analysis.TermFrequencyAttribute
	upto        int
}

func newCannedTermFreqs(terms []string, termFreqs []int) *cannedTermFreqs {
	c := &cannedTermFreqs{BaseTokenStream: analysis.NewBaseTokenStream(), terms: terms, termFreqs: termFreqs}
	c.termAtt = c.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	c.termFreqAtt = c.AddAttribute(analysis.TermFrequencyAttributeType).(analysis.TermFrequencyAttribute)
	if util.AssertsEnabled() && !(len(terms) == len(termFreqs)) {
		panic(util.NewAssertionError("terms.length == termFreqs.length"))
	}
	return c
}

func (c *cannedTermFreqs) IncrementToken() (bool, error) {
	if c.upto == len(c.terms) {
		return false, nil
	}

	c.ClearAttributes()

	c.termAtt.AppendString(c.terms[c.upto])
	c.termFreqAtt.SetTermFrequency(c.termFreqs[c.upto])

	c.upto++
	return true, nil
}

func (c *cannedTermFreqs) Reset() error {
	c.upto = 0
	return nil
}

// getLegacyFieldType renders the private getLegacyFieldType(). It is
// possible to store custom term frequencies in fields that are *not* labeled
// as term-doc fields. These will sum term frequencies rather than counts and
// have more places where they may overflow these aggregates.
func getLegacyFieldType() *document.FieldType {
	fieldType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	fieldType.SetIndexOptions(index.IndexOptionsDocsAndFreqs)
	return fieldType
}

func cannedField(t testing.TB, terms []string, freqs []int, ft *document.FieldType) *document.Field {
	t.Helper()
	f, err := document.NewField("field", newCannedTermFreqs(terms, freqs), ft)
	if err != nil {
		t.Fatalf("Field: %v", err)
	}
	return f
}

func customTermFreqWriter(t *testing.T) (*index.IndexWriter, func()) {
	t.Helper()
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	return w, func() { mustClose(t, w, dir) }
}

func TestCustomTermFreqSingletonTermsOneDoc(t *testing.T) {
	_, closeAll := customTermFreqWriter(t)
	defer closeAll()
	t.Fatal(indexOptionsDocsAndCustomFreqsMissing)
}

func TestCustomTermFreqRepeatedTerms(t *testing.T) {
	_, closeAll := customTermFreqWriter(t)
	defer closeAll()
	t.Fatal(indexOptionsDocsAndCustomFreqsMissing)
}

func TestCustomTermFreqSingletonTermsTwoDocs(t *testing.T) {
	_, closeAll := customTermFreqWriter(t)
	defer closeAll()
	t.Fatal(indexOptionsDocsAndCustomFreqsMissing)
}

func TestCustomTermFreqRepeatTermsOneDoc(t *testing.T) {
	// With legacy field type, we sum the individual term freqs
	w, closeAll := customTermFreqWriter(t)
	defer closeAll()
	doc := document.NewDocument()
	doc.Add(cannedField(t, []string{"foo", "bar", "foo", "bar"}, []int{42, 128, 17, 100}, getLegacyFieldType()))
	mustAddDocument(t, w, doc)
	r := openReaderFromWriter(t, w)
	defer mustClose(t, r)
	t.Fatal(multiTermsGetTermPostingsEnumMissing)
}

func TestCustomTermFreqRepeatTermsTwoDocs(t *testing.T) {
	w, closeAll := customTermFreqWriter(t)
	defer closeAll()
	fieldType := getLegacyFieldType()
	doc := document.NewDocument()
	doc.Add(cannedField(t, []string{"foo", "bar", "foo", "bar"}, []int{42, 128, 17, 100}, fieldType))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(cannedField(t, []string{"foo", "bar", "foo", "bar"}, []int{50, 60, 70, 80}, fieldType))
	mustAddDocument(t, w, doc)

	r := openReaderFromWriter(t, w)
	defer mustClose(t, r)
	t.Fatal(multiTermsGetTermPostingsEnumMissing)
}

// fieldTermsEnum renders MultiTerms.getTerms(r, "field").iterator().
func fieldTermsEnum(t testing.TB, r index.IndexReader) index.TermsEnum {
	t.Helper()
	terms, err := index.MultiTermsGetTerms(r, "field")
	if err != nil {
		t.Fatalf("MultiTerms.getTerms: %v", err)
	}
	termsEnum, err := terms.Iterator()
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	return termsEnum
}

// assertSeekTotalTermFreq renders assertTrue(termsEnum.seekExact(text)) and
// assertEquals(expected, termsEnum.totalTermFreq()).
func assertSeekTotalTermFreq(t testing.TB, termsEnum index.TermsEnum, text string, expected int64) {
	t.Helper()
	found, err := termsEnum.SeekExact(spi.NewTerm("field", text))
	if err != nil || !found {
		t.Fatalf("seekExact(%q): %v (%v)", text, found, err)
	}
	ttf, err := termsEnum.TotalTermFreq()
	if err != nil || ttf != expected {
		t.Fatalf("totalTermFreq(%q): expected %d, got %d (%v)", text, expected, ttf, err)
	}
}

func TestCustomTermFreqTotalTermFreq(t *testing.T) {
	w, closeAll := customTermFreqWriter(t)
	defer closeAll()
	fieldType := getLegacyFieldType()
	doc := document.NewDocument()
	doc.Add(cannedField(t, []string{"foo", "bar", "foo", "bar", "baz"}, []int{42, 128, 17, 100, 99}, fieldType))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(cannedField(t, []string{"foo", "bar", "foo", "bar"}, []int{50, 60, 70, 80}, fieldType))
	mustAddDocument(t, w, doc)

	r := openReaderFromWriter(t, w)
	defer mustClose(t, r)

	termsEnum := fieldTermsEnum(t, r)
	assertSeekTotalTermFreq(t, termsEnum, "foo", 179)
	assertSeekTotalTermFreq(t, termsEnum, "bar", 368)
}

func TestCustomTermFreqTotalTermFreqTermDoc(t *testing.T) {
	_, closeAll := customTermFreqWriter(t)
	defer closeAll()
	t.Fatal(indexOptionsDocsAndCustomFreqsMissing)
}

// assertAddDocumentMessage renders the expectThrows + assertEquals(message)
// pairs of the invalid-configuration tests.
func assertAddDocumentMessage(t testing.TB, w *index.IndexWriter, doc *document.Document, message string) {
	t.Helper()
	_, err := w.AddDocument(doc)
	if err == nil {
		t.Fatalf("expected an exception with message %q", message)
	}
	if err.Error() != message {
		t.Fatalf("message: expected %q, got %q", message, err.Error())
	}
}

// you can't index proximity with custom term freqs:
func TestCustomTermFreqInvalidProx(t *testing.T) {
	w, closeAll := customTermFreqWriter(t)
	defer closeAll()

	doc := document.NewDocument()
	fieldType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	doc.Add(cannedField(t, []string{"foo", "bar", "foo", "bar"}, []int{42, 128, 17, 100}, fieldType))
	assertAddDocumentMessage(t, w, doc, "field \"field\": cannot index positions while using custom TermFrequencyAttribute")
}

// you can't index DOCS_ONLY with custom term freq
func TestCustomTermFreqInvalidDocsOnly(t *testing.T) {
	w, closeAll := customTermFreqWriter(t)
	defer closeAll()

	doc := document.NewDocument()
	fieldType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	fieldType.SetIndexOptions(index.IndexOptionsDocs)
	doc.Add(cannedField(t, []string{"foo", "bar", "foo", "bar"}, []int{42, 128, 17, 100}, fieldType))
	assertAddDocumentMessage(t, w, doc, "field \"field\": must index term freq while using custom TermFrequencyAttribute")
}

// sum of term freqs must fit in an int
func TestCustomTermFreqOverflowInt(t *testing.T) {
	w, closeAll := customTermFreqWriter(t)
	defer closeAll()

	fieldType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	fieldType.SetIndexOptions(index.IndexOptionsDocs)

	doc := document.NewDocument()
	f, err := document.NewField("field", "this field should be indexed", fieldType)
	if err != nil {
		t.Fatalf("Field: %v", err)
	}
	doc.Add(f)
	mustAddDocument(t, w, doc)

	doc2 := document.NewDocument()
	doc2.Add(cannedField(t, []string{"foo", "bar"}, []int{3, math.MaxInt32}, fieldType))
	if _, err := w.AddDocument(doc2); err == nil {
		t.Fatal("expected IllegalArgumentException from addDocument")
	}

	r := openReaderFromWriter(t, w)
	assertNumDocs(t, 1, r)
	mustClose(t, r)
}

func TestCustomTermFreqNoOverflowInt(t *testing.T) {
	_, closeAll := customTermFreqWriter(t)
	defer closeAll()
	// Using the termdoc field type enables us to store large term frequencies
	// that would otherwise overflow totalTermFreq
	t.Fatal(indexOptionsDocsAndCustomFreqsMissing)
}

func TestCustomTermFreqInvalidTermVectorPositions(t *testing.T) {
	_, closeAll := customTermFreqWriter(t)
	defer closeAll()
	t.Fatal(indexOptionsDocsAndCustomFreqsMissing)
}

func TestCustomTermFreqInvalidTermVectorOffsets(t *testing.T) {
	_, closeAll := customTermFreqWriter(t)
	defer closeAll()
	t.Fatal(indexOptionsDocsAndCustomFreqsMissing)
}

func TestCustomTermFreqTermVectors(t *testing.T) {
	_, closeAll := customTermFreqWriter(t)
	defer closeAll()
	t.Fatal(indexOptionsDocsAndCustomFreqsMissing)
}

// TestCustomTermFreqFieldInvertState ports testFieldInvertState, which
// installs the private NeverForgetsSimilarity and then indexes a
// getTermDocFieldType() field.
func TestCustomTermFreqFieldInvertState(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetSimilarity(neverForgetsSimilarityInstance)
	w := mustNewIndexWriter(t, dir, iwc)
	defer mustClose(t, w, dir)
	t.Fatal(indexOptionsDocsAndCustomFreqsMissing)
}

// neverForgetsSimilarity is the private NeverForgetsSimilarity: it holds onto
// the FieldInvertState for subsequent verification.
type neverForgetsSimilarity struct {
	lastState *index.FieldInvertState
}

var neverForgetsSimilarityInstance = &neverForgetsSimilarity{}

// GetDiscountOverlaps carries Similarity's default discountOverlaps = true.
func (s *neverForgetsSimilarity) GetDiscountOverlaps() bool { return true }

func (s *neverForgetsSimilarity) ComputeNormFromInvertState(state *index.FieldInvertState) int64 {
	s.lastState = state
	return 1
}
