// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package word2vec

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────────

// buildModel constructs a Word2VecModel from parallel slices of term strings
// and [x,y] vectors.
func buildModel(t *testing.T, terms []string, vecs [][2]float32) *Word2VecModel {
	t.Helper()
	m := NewWord2VecModel(len(terms), 2)
	for i, term := range terms {
		m.AddTermAndVector(util.NewTermAndVector(util.NewBytesRef([]byte(term)), []float32{vecs[i][0], vecs[i][1]}))
	}
	return m
}

// tokenResults drains a Word2VecSynonymFilter and returns the token texts,
// types, posIncrements, and posLengths observed.
type tokenResults struct {
	terms      []string
	types      []string
	posIncrs   []int
	posLengths []int
}

func drainFilter(t *testing.T, input analysis.TokenStream, f *Word2VecSynonymFilter) tokenResults {
	t.Helper()

	src := f.GetAttributeSource()
	termAttr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	typeAttr := src.GetAttribute(analysis.TypeAttributeType).(analysis.TypeAttribute)
	posIncrAttr := src.GetAttribute(analysis.PositionIncrementAttributeType).(analysis.PositionIncrementAttribute)
	posLenAttr := src.GetAttribute(analysis.PositionLengthAttributeType).(analysis.PositionLengthAttribute)

	var res tokenResults
	for {
		ok, err := f.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		res.terms = append(res.terms, termAttr.String())
		res.types = append(res.types, typeAttr.GetType())
		res.posIncrs = append(res.posIncrs, posIncrAttr.GetPositionIncrement())
		res.posLengths = append(res.posLengths, posLenAttr.GetPositionLength())
	}
	return res
}

// whitespaceTokenizer returns a simple analysis.TokenStream that emits
// whitespace-split tokens with default attributes.
func whitespaceTokenizer(text string) analysis.TokenStream {
	words := strings.Fields(text)
	return &mockTokenStream{words: words, pos: -1}
}

// mockTokenStream is a minimal TokenStream over a string slice.
type mockTokenStream struct {
	analysis.BaseTokenStream
	words    []string
	pos      int
	termAttr analysis.CharTermAttribute
	piAttr   analysis.PositionIncrementAttribute
	plAttr   analysis.PositionLengthAttribute
	tyAttr   analysis.TypeAttribute
}

func newMockTokenStream(words []string) *mockTokenStream {
	m := &mockTokenStream{
		BaseTokenStream: *analysis.NewBaseTokenStream(),
		words:           words,
		pos:             -1,
	}
	src := m.GetAttributeSource()

	// Register concrete impls (AddAttributeImpl) so the attribute source
	// knows about analysis attributes without relying on the default factory.
	termImpl := analysis.NewCharTermAttribute()
	posIncrImpl := analysis.NewPositionIncrementAttribute()
	posLenImpl := analysis.NewPositionLengthAttribute()
	typeImpl := analysis.NewTypeAttribute()

	src.AddAttributeImpl(termImpl.(util.AttributeImpl))
	src.AddAttributeImpl(posIncrImpl.(util.AttributeImpl))
	src.AddAttributeImpl(posLenImpl.(util.AttributeImpl))
	src.AddAttributeImpl(typeImpl.(util.AttributeImpl))

	m.termAttr = src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	m.piAttr = src.GetAttribute(analysis.PositionIncrementAttributeType).(analysis.PositionIncrementAttribute)
	m.plAttr = src.GetAttribute(analysis.PositionLengthAttributeType).(analysis.PositionLengthAttribute)
	m.tyAttr = src.GetAttribute(analysis.TypeAttributeType).(analysis.TypeAttribute)
	return m
}

func (m *mockTokenStream) IncrementToken() (bool, error) {
	m.pos++
	if m.pos >= len(m.words) {
		return false, nil
	}
	m.ClearAttributes()
	m.termAttr.SetValue(m.words[m.pos])
	m.piAttr.SetPositionIncrement(1)
	m.plAttr.SetPositionLength(1)
	m.tyAttr.SetType(analysis.DefaultTypeAttributeValue)
	return true, nil
}

func (m *mockTokenStream) Reset() error {
	m.pos = -1
	return nil
}

func (m *mockTokenStream) End() error   { return nil }
func (m *mockTokenStream) Close() error { return nil }

// buildFilter builds a Word2VecSynonymFilter on top of a mock token stream
// over the given whitespace-split text.
func buildFilter(
	t *testing.T,
	model *Word2VecModel,
	text string,
	maxSynonyms int,
	minSim float32,
) (*Word2VecSynonymFilter, error) {
	t.Helper()
	provider, err := NewWord2VecSynonymProvider(model)
	if err != nil {
		return nil, err
	}
	input := newMockTokenStream(strings.Fields(text))
	return NewWord2VecSynonymFilter(input, provider, maxSynonyms, minSim)
}

// ─────────────────────────────────────────────────────────────────────────────
// TermAndBoost
// ─────────────────────────────────────────────────────────────────────────────

func TestTermAndBoost_DeepCopy(t *testing.T) {
	orig := util.NewBytesRef([]byte("hello"))
	tab := NewTermAndBoost(orig, 0.9)
	// Mutate the original bytes; the copy must not change.
	orig.Bytes[0] = 'X'
	if tab.Term.String() != "hello" {
		t.Errorf("deep copy violated: got %q", tab.Term.String())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Word2VecModel
// ─────────────────────────────────────────────────────────────────────────────

func TestWord2VecModel_NormalizedVectors(t *testing.T) {
	terms := []string{"a", "b", "c", "f"}
	vecs := [][2]float32{{10, 10}, {10, 8}, {9, 10}, {-1, 10}}
	m := buildModel(t, terms, vecs)

	vA := m.VectorValueByTerm(util.NewBytesRef([]byte("a")))
	if vA == nil {
		t.Fatal("vector for 'a' is nil")
	}
	if abs32(vA[0]-0.70710) > 0.001 || abs32(vA[1]-0.70710) > 0.001 {
		t.Errorf("normalized vector for 'a': got %v, want ~[0.70710, 0.70710]", vA)
	}

	vF := m.VectorValueByTerm(util.NewBytesRef([]byte("f")))
	if vF == nil {
		t.Fatal("vector for 'f' is nil")
	}
	if abs32(vF[0]-(-0.0995)) > 0.001 || abs32(vF[1]-0.99503) > 0.001 {
		t.Errorf("normalized vector for 'f': got %v, want ~[-0.0995, 0.99503]", vF)
	}
}

func TestWord2VecModel_UnknownTermReturnsNil(t *testing.T) {
	m := buildModel(t, []string{"a"}, [][2]float32{{1, 0}})
	v := m.VectorValueByTerm(util.NewBytesRef([]byte("z")))
	if v != nil {
		t.Errorf("expected nil for unknown term, got %v", v)
	}
}

func TestWord2VecModel_IteratorCoversAllDocs(t *testing.T) {
	n := 5
	terms := make([]string, n)
	vecs := make([][2]float32, n)
	for i := range terms {
		terms[i] = string(rune('a' + i))
		vecs[i] = [2]float32{float32(i + 1), 1}
	}
	m := buildModel(t, terms, vecs)
	it := m.Iterator()
	count := 0
	for {
		doc, err := it.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc == util.NO_MORE_DOCS {
			break
		}
		count++
	}
	if count != n {
		t.Errorf("iterator count: got %d, want %d", count, n)
	}
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// ─────────────────────────────────────────────────────────────────────────────
// Word2VecSynonymProvider
// ─────────────────────────────────────────────────────────────────────────────

func TestWord2VecSynonymProvider_NilTermReturnsError(t *testing.T) {
	m := buildModel(t, []string{"a", "b"}, [][2]float32{{0.24, 0.78}, {0.44, 0.01}})
	provider, err := NewWord2VecSynonymProvider(m)
	if err != nil {
		t.Fatalf("NewWord2VecSynonymProvider: %v", err)
	}
	_, err = provider.GetSynonyms(nil, 10, 0.85)
	if err == nil {
		t.Fatal("expected error for nil term, got nil")
	}
}

func TestWord2VecSynonymProvider_GetSynonymsBasedOnMinSimilarity(t *testing.T) {
	terms := []string{"a", "b", "c", "d", "e", "f"}
	vecs := [][2]float32{{10, 10}, {10, 8}, {9, 10}, {1, 1}, {99, 101}, {-1, 10}}
	m := buildModel(t, terms, vecs)
	provider, err := NewWord2VecSynonymProvider(m)
	if err != nil {
		t.Fatalf("NewWord2VecSynonymProvider: %v", err)
	}

	synonyms, err := provider.GetSynonyms(util.NewBytesRef([]byte("a")), 10, 0.85)
	if err != nil {
		t.Fatalf("GetSynonyms: %v", err)
	}
	// Lucene's test expects 4 synonyms: d, e, c, b (in HNSW result order)
	if len(synonyms) != 4 {
		t.Errorf("expected 4 synonyms, got %d: %v", len(synonyms), termsFromSynonyms(synonyms))
	}
	expected := map[string]bool{"b": true, "c": true, "d": true, "e": true}
	for _, s := range synonyms {
		if !expected[s.Term.String()] {
			t.Errorf("unexpected synonym %q", s.Term.String())
		}
	}
}

func TestWord2VecSynonymProvider_NoSynonymsWithinSimilarity(t *testing.T) {
	terms := []string{"a", "b", "c", "d"}
	vecs := [][2]float32{{10, 10}, {-10, -8}, {-9, -10}, {6, -6}}
	m := buildModel(t, terms, vecs)
	provider, err := NewWord2VecSynonymProvider(m)
	if err != nil {
		t.Fatalf("NewWord2VecSynonymProvider: %v", err)
	}

	synonyms, err := provider.GetSynonyms(util.NewBytesRef([]byte("a")), 10, 0.85)
	if err != nil {
		t.Fatalf("GetSynonyms: %v", err)
	}
	if len(synonyms) != 0 {
		t.Errorf("expected 0 synonyms, got %d", len(synonyms))
	}
}

func TestWord2VecSynonymProvider_UnknownTermReturnsNil(t *testing.T) {
	m := buildModel(t, []string{"a"}, [][2]float32{{1, 0}})
	provider, err := NewWord2VecSynonymProvider(m)
	if err != nil {
		t.Fatalf("NewWord2VecSynonymProvider: %v", err)
	}
	synonyms, err := provider.GetSynonyms(util.NewBytesRef([]byte("z")), 10, 0.8)
	if err != nil {
		t.Fatalf("GetSynonyms: %v", err)
	}
	if synonyms != nil {
		t.Errorf("expected nil for unknown term, got %v", synonyms)
	}
}

func termsFromSynonyms(ss []*TermAndBoost) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Term.String()
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// Dl4jModelReader
// ─────────────────────────────────────────────────────────────────────────────

func openTestData(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("open testdata/%s: %v", name, err)
	}
	return data
}

func TestDl4jModelReader_DictionarySize(t *testing.T) {
	data := openTestData(t, "word2vec-model.zip")
	reader, err := NewDl4jModelReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("NewDl4jModelReader: %v", err)
	}
	model, err := reader.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if model.Size() != 235 {
		t.Errorf("dictionary size: got %d, want 235", model.Size())
	}
}

func TestDl4jModelReader_VectorDimension(t *testing.T) {
	data := openTestData(t, "word2vec-model.zip")
	reader, err := NewDl4jModelReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("NewDl4jModelReader: %v", err)
	}
	model, err := reader.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if model.Dimension() != 100 {
		t.Errorf("vector dimension: got %d, want 100", model.Dimension())
	}
}

func TestDl4jModelReader_FirstTermDecoded(t *testing.T) {
	data := openTestData(t, "word2vec-model.zip")
	reader, err := NewDl4jModelReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("NewDl4jModelReader: %v", err)
	}
	model, err := reader.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	first := model.TermValue(0)
	if first.String() != "it" {
		t.Errorf("first term: got %q, want %q", first.String(), "it")
	}
}

func TestDl4jModelReader_EmptyZipReturnsError(t *testing.T) {
	data := openTestData(t, "word2vec-empty-model.zip")
	reader, err := NewDl4jModelReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("NewDl4jModelReader: %v", err)
	}
	_, err = reader.Read()
	if err == nil {
		t.Fatal("expected error for empty model, got nil")
	}
}

func TestDl4jModelReader_CorruptedDimensionReturnsError(t *testing.T) {
	data := openTestData(t, "word2vec-corrupted-vector-dimension-model.zip")
	reader, err := NewDl4jModelReader(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("NewDl4jModelReader: %v", err)
	}
	_, err = reader.Read()
	if err == nil {
		t.Fatal("expected error for corrupted dimension, got nil")
	}
}

func TestDl4jModelReader_DecodeB64Term(t *testing.T) {
	// "lucene" base64-encoded
	encoded := base64.StdEncoding.EncodeToString([]byte("lucene"))
	token := "B64:" + encoded
	got := decodeB64Term(token)
	if got.String() != "lucene" {
		t.Errorf("decodeB64Term: got %q, want %q", got.String(), "lucene")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Word2VecSynonymFilter
// ─────────────────────────────────────────────────────────────────────────────

func TestWord2VecSynonymFilter_OneCandidate_WithinThreshold(t *testing.T) {
	terms := []string{"a", "b", "c", "d", "e", "f"}
	vecs := [][2]float32{{10, 10}, {10, 8}, {9, 10}, {1, 1}, {99, 101}, {-1, 10}}
	m := buildModel(t, terms, vecs)

	f, err := buildFilter(t, m, "pre a post", 10, 0.9)
	if err != nil {
		t.Fatalf("buildFilter: %v", err)
	}

	res := drainFilter(t, nil, f)

	// "pre" and "post" have no close synonyms in this model; "a" has 4 synonyms
	// (d, e, c, b) within 0.9 threshold.
	if len(res.terms) < 3 {
		t.Fatalf("too few tokens: %v", res.terms)
	}

	// Verify the original token "a" is present and the synonyms have posIncr=0
	aIdx := indexOf(res.terms, "a")
	if aIdx < 0 {
		t.Fatalf("token 'a' not found in %v", res.terms)
	}
	// All tokens after "a" up to next posIncr>0 should be synonyms
	synonymCount := 0
	for i := aIdx + 1; i < len(res.terms) && res.posIncrs[i] == 0; i++ {
		if res.types[i] != typeSynonym {
			t.Errorf("token %q at idx %d should have type SYNONYM, got %q", res.terms[i], i, res.types[i])
		}
		synonymCount++
	}
	if synonymCount == 0 {
		t.Errorf("expected synonyms after 'a', got none; all tokens: %v", res.terms)
	}
}

func TestWord2VecSynonymFilter_NoSynonymsAboveThreshold(t *testing.T) {
	terms := []string{"a", "b", "c", "f"}
	vecs := [][2]float32{{10, 10}, {-10, -8}, {-9, -10}, {-1, -10}}
	m := buildModel(t, terms, vecs)

	f, err := buildFilter(t, m, "pre a post", 10, 0.8)
	if err != nil {
		t.Fatalf("buildFilter: %v", err)
	}

	res := drainFilter(t, nil, f)
	// All 3 tokens, none should be SYNONYM type
	if len(res.terms) != 3 {
		t.Errorf("expected 3 tokens (no synonyms), got %d: %v", len(res.terms), res.terms)
	}
	for _, tp := range res.types {
		if tp == typeSynonym {
			t.Errorf("unexpected SYNONYM token in %v", res.types)
			break
		}
	}
}

func TestWord2VecSynonymFilter_MaxSynonymsPerTerm(t *testing.T) {
	terms := []string{"a", "b", "c", "d", "e"}
	vecs := [][2]float32{{10, 10}, {10, 8}, {9, 10}, {1, 1}, {99, 101}}
	m := buildModel(t, terms, vecs)

	f, err := buildFilter(t, m, "pre a post", 2, 0.9)
	if err != nil {
		t.Fatalf("buildFilter: %v", err)
	}

	res := drainFilter(t, nil, f)

	aIdx := indexOf(res.terms, "a")
	if aIdx < 0 {
		t.Fatalf("token 'a' not found")
	}
	synonymCount := 0
	for i := aIdx + 1; i < len(res.terms) && res.posIncrs[i] == 0; i++ {
		synonymCount++
	}
	if synonymCount > 2 {
		t.Errorf("maxSynonymsPerTerm=2 violated: got %d synonyms", synonymCount)
	}
}

func TestWord2VecSynonymFilter_NilProviderReturnsError(t *testing.T) {
	input := newMockTokenStream([]string{"a"})
	_, err := NewWord2VecSynonymFilter(input, nil, 5, 0.8)
	if err == nil {
		t.Fatal("expected error for nil provider, got nil")
	}
}

func TestWord2VecSynonymFilter_Reset(t *testing.T) {
	terms := []string{"a", "b", "c"}
	vecs := [][2]float32{{10, 10}, {10, 8}, {9, 10}}
	m := buildModel(t, terms, vecs)

	provider, err := NewWord2VecSynonymProvider(m)
	if err != nil {
		t.Fatalf("NewWord2VecSynonymProvider: %v", err)
	}
	input := newMockTokenStream(strings.Fields("a"))
	f, err := NewWord2VecSynonymFilter(input, provider, 10, 0.5)
	if err != nil {
		t.Fatalf("NewWord2VecSynonymFilter: %v", err)
	}

	// First pass
	res1 := drainFilter(t, nil, f)

	// Reset and replay
	if err := f.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	res2 := drainFilter(t, nil, f)

	if len(res1.terms) != len(res2.terms) {
		t.Errorf("after reset: first pass %d tokens, second pass %d tokens", len(res1.terms), len(res2.terms))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Word2VecSynonymFilterFactory
// ─────────────────────────────────────────────────────────────────────────────

func TestWord2VecSynonymFilterFactory_MissingModelParam(t *testing.T) {
	_, err := NewWord2VecSynonymFilterFactory(map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing 'model' param")
	}
}

func TestWord2VecSynonymFilterFactory_InvalidMinSimilarity(t *testing.T) {
	_, err := NewWord2VecSynonymFilterFactory(map[string]string{
		"model":                 "m.zip",
		"minAcceptedSimilarity": "1.5",
	})
	if err == nil {
		t.Fatal("expected error for minAcceptedSimilarity > 1")
	}
}

func TestWord2VecSynonymFilterFactory_InvalidMaxSynonyms(t *testing.T) {
	_, err := NewWord2VecSynonymFilterFactory(map[string]string{
		"model":              "m.zip",
		"maxSynonymsPerTerm": "0",
	})
	if err == nil {
		t.Fatal("expected error for maxSynonymsPerTerm = 0")
	}
}

func TestWord2VecSynonymFilterFactory_UnknownParam(t *testing.T) {
	_, err := NewWord2VecSynonymFilterFactory(map[string]string{
		"model":   "m.zip",
		"unknown": "val",
	})
	if err == nil {
		t.Fatal("expected error for unknown param")
	}
}

func TestWord2VecSynonymFilterFactory_UnsupportedFormat(t *testing.T) {
	_, err := NewWord2VecSynonymFilterFactory(map[string]string{
		"model":  "m.zip",
		"format": "word2vec_native",
	})
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestWord2VecSynonymFilterFactory_DefaultParams(t *testing.T) {
	f, err := NewWord2VecSynonymFilterFactory(map[string]string{"model": "m.zip"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.maxSynonymsPerTerm != DefaultMaxSynonymsPerTerm {
		t.Errorf("maxSynonymsPerTerm: got %d, want %d", f.maxSynonymsPerTerm, DefaultMaxSynonymsPerTerm)
	}
	if f.minAcceptedSimilarity != DefaultMinAcceptedSimilarity {
		t.Errorf("minAcceptedSimilarity: got %v, want %v", f.minAcceptedSimilarity, DefaultMinAcceptedSimilarity)
	}
}

func TestWord2VecSynonymFilterFactory_CreatePassthroughWithoutInform(t *testing.T) {
	factory, err := NewWord2VecSynonymFilterFactory(map[string]string{"model": "m.zip"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	input := newMockTokenStream([]string{"hello"})
	ts := factory.Create(input)
	if ts == nil {
		t.Fatal("Create returned nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

func indexOf(slice []string, s string) int {
	for i, v := range slice {
		if v == s {
			return i
		}
	}
	return -1
}
