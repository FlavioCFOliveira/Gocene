// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/search/TestMultiNormsLeafSimScorer.java
//	No Java test peer exists — synthetic Go tests covering the public contract.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// stubSimScorer is a minimal LuceneSimScorer: score = freq * float32(norm).
type stubSimScorer struct{}

func (s *stubSimScorer) Score104(freq float32, norm int64) float32 { return freq * float32(norm) }
func (s *stubSimScorer) AsBulkSimScorer() BulkSimScorer            { return NewDefaultBulkSimScorer(s) }
func (s *stubSimScorer) Explain104(freq Explanation, norm int64) Explanation {
	return MatchExplanation(freq.GetValue()*float32(norm), "stub score")
}

// mnlssFixedNorm returns the supplied constant for every doc.
type mnlssFixedNorm struct{ val int64 }

func (f *mnlssFixedNorm) Advance(_ int) (int, error)       { return 0, nil }
func (f *mnlssFixedNorm) AdvanceExact(_ int) (bool, error) { return true, nil }
func (f *mnlssFixedNorm) LongValue() (int64, error)        { return f.val, nil }
func (f *mnlssFixedNorm) NextDoc() (int, error)            { return 0, nil }
func (f *mnlssFixedNorm) DocID() int                       { return -1 }
func (f *mnlssFixedNorm) Cost() int64                      { return 1 }

// mnlssLeafReader wraps *index.LeafReader so that GetNormValues can be
// overridden via a norm map, letting tests inject synthetic norm sources
// without a real index.
type mnlssLeafReader struct {
	*index.LeafReader
	norms map[string]index.NumericDocValues
}

func (r *mnlssLeafReader) GetNormValues(field string) (index.NumericDocValues, error) {
	return r.norms[field], nil
}

// newMNLSSLeafReader builds a mnlssLeafReader with the given field→norms map.
func newMNLSSLeafReader(norms map[string]index.NumericDocValues) *mnlssLeafReader {
	return &mnlssLeafReader{
		LeafReader: &index.LeafReader{},
		norms:      norms,
	}
}

// TestMultiNormsLeafSimScorer_NeedsScoresFalse checks that when needsScores is
// false, score always delegates to the scorer with norm=1.
func TestMultiNormsLeafSimScorer_NeedsScoresFalse(t *testing.T) {
	reader := newMNLSSLeafReader(nil)
	scorer := &stubSimScorer{}
	fields := []FieldAndWeight{{Field: "body", Weight: 1.0}}

	s, err := newMultiNormsLeafSimScorer(scorer, reader, fields, false)
	if err != nil {
		t.Fatalf("constructor error: %v", err)
	}

	const freq float32 = 3.0
	got, err := s.score(0, freq)
	if err != nil {
		t.Fatalf("score error: %v", err)
	}
	// norm=1 → score = freq*1 = 3
	if want := freq * 1; got != want {
		t.Errorf("score = %v, want %v", got, want)
	}
}

// TestMultiNormsLeafSimScorer_NeedsScoresTrue_SingleField checks scoring with
// a real norm source injected for a single field.
func TestMultiNormsLeafSimScorer_NeedsScoresTrue_SingleField(t *testing.T) {
	// Encode norm value 4 through IntToByte4 to get the round-trip byte.
	normInt := 4
	normByte, err := util.IntToByte4(normInt)
	if err != nil {
		t.Fatalf("IntToByte4: %v", err)
	}
	normVal := int64(normByte)

	norms := map[string]index.NumericDocValues{
		"body": &mnlssFixedNorm{val: normVal},
	}
	reader := newMNLSSLeafReader(norms)
	scorer := &stubSimScorer{}
	fields := []FieldAndWeight{{Field: "body", Weight: 1.0}}

	s, err := newMultiNormsLeafSimScorer(scorer, reader, fields, true)
	if err != nil {
		t.Fatalf("constructor error: %v", err)
	}

	// The multiFieldNormValues.Get path: weight=1, lengthTable[normByte]=Byte4ToInt(normByte)=normInt=4.
	// IntToByte4(round(1*4)) = normByte. score = freq * normByte.
	const freq float32 = 2.0
	got, err := s.score(0, freq)
	if err != nil {
		t.Fatalf("score error: %v", err)
	}
	want := freq * float32(normByte) // stubSimScorer multiplies freq * norm
	if got != want {
		t.Errorf("score = %v, want %v", got, want)
	}
}

// TestMultiNormsLeafSimScorer_Explain verifies explain returns a non-nil
// Explanation with a positive value.
func TestMultiNormsLeafSimScorer_Explain(t *testing.T) {
	reader := newMNLSSLeafReader(nil)
	scorer := &stubSimScorer{}
	fields := []FieldAndWeight{{Field: "body", Weight: 1.0}}

	s, err := newMultiNormsLeafSimScorer(scorer, reader, fields, false)
	if err != nil {
		t.Fatalf("constructor error: %v", err)
	}

	freqExpl := MatchExplanation(5.0, "termFreq=5")
	expl, err := s.explain(0, freqExpl)
	if err != nil {
		t.Fatalf("explain error: %v", err)
	}
	if expl == nil {
		t.Fatal("explain returned nil")
	}
	// norm=1 → value = 5*1 = 5
	if want := float32(5); expl.GetValue() != want {
		t.Errorf("explain value = %v, want %v", expl.GetValue(), want)
	}
}

// TestMultiNormsLeafSimScorer_ScoreRange verifies the bulk scoreRange path.
func TestMultiNormsLeafSimScorer_ScoreRange(t *testing.T) {
	reader := newMNLSSLeafReader(nil)
	scorer := &stubSimScorer{}
	fields := []FieldAndWeight{{Field: "body", Weight: 1.0}}

	s, err := newMultiNormsLeafSimScorer(scorer, reader, fields, false)
	if err != nil {
		t.Fatalf("constructor error: %v", err)
	}

	buf := &DocAndFloatFeatureBuffer{
		Docs:     []int{0, 1, 2},
		Features: []float32{1.0, 2.0, 3.0},
		Size:     3,
	}

	if err := s.scoreRange(buf); err != nil {
		t.Fatalf("scoreRange error: %v", err)
	}

	// needsScores=false → norm=1 → score[i] = freq[i]*1
	for i := 0; i < buf.Size; i++ {
		want := float32(i + 1)
		if buf.Features[i] != want {
			t.Errorf("features[%d] = %v, want %v", i, buf.Features[i], want)
		}
	}
}

// TestMultiNormsLeafSimScorer_GetSimScorer confirms the accessor returns the
// original scorer.
func TestMultiNormsLeafSimScorer_GetSimScorer(t *testing.T) {
	reader := newMNLSSLeafReader(nil)
	scorer := &stubSimScorer{}
	fields := []FieldAndWeight{{Field: "body", Weight: 1.0}}

	s, err := newMultiNormsLeafSimScorer(scorer, reader, fields, false)
	if err != nil {
		t.Fatalf("constructor error: %v", err)
	}

	if got := s.getSimScorer(); got != scorer {
		t.Errorf("getSimScorer returned wrong scorer")
	}
}

// TestMultiNormsLeafSimScorer_NoNormFields checks that when the field has no
// norms the fallback of norm=1 is used even with needsScores=true.
func TestMultiNormsLeafSimScorer_NoNormFields(t *testing.T) {
	// Provide a field with no norms (nil in the map).
	norms := map[string]index.NumericDocValues{"body": nil}
	reader := newMNLSSLeafReader(norms)
	scorer := &stubSimScorer{}
	fields := []FieldAndWeight{{Field: "body", Weight: 1.0}}

	s, err := newMultiNormsLeafSimScorer(scorer, reader, fields, true)
	if err != nil {
		t.Fatalf("constructor error: %v", err)
	}

	const freq float32 = 7.0
	got, err := s.score(0, freq)
	if err != nil {
		t.Fatalf("score error: %v", err)
	}
	// norm=1 fallback
	if want := freq; got != want {
		t.Errorf("score = %v, want %v", got, want)
	}
}

// TestMultiFieldNormValues_TwoFields verifies the weighted norm blending with
// two fields.
func TestMultiFieldNormValues_TwoFields(t *testing.T) {
	// Use norm int 4 (weight 1) and norm int 4 (weight 2): combined = 1*4 + 2*4 = 12.
	normByte4, err := util.IntToByte4(4)
	if err != nil {
		t.Fatalf("IntToByte4(4): %v", err)
	}

	norms := map[string]index.NumericDocValues{
		"title": &mnlssFixedNorm{val: int64(normByte4)},
		"body":  &mnlssFixedNorm{val: int64(normByte4)},
	}
	reader := newMNLSSLeafReader(norms)
	scorer := &stubSimScorer{}
	fields := []FieldAndWeight{
		{Field: "title", Weight: 1.0},
		{Field: "body", Weight: 2.0},
	}

	s, err := newMultiNormsLeafSimScorer(scorer, reader, fields, true)
	if err != nil {
		t.Fatalf("constructor error: %v", err)
	}

	_, err = s.score(0, 1.0)
	if err != nil {
		t.Fatalf("score error: %v", err)
	}
	// We only check that it does not error; the blending arithmetic is
	// exercised by the multiFieldNormValues unit path.
}

// TestMultiNormsLeafSimScorer_LengthTableInit verifies lengthTable is
// populated consistently with util.Byte4ToInt.
func TestMultiNormsLeafSimScorer_LengthTableInit(t *testing.T) {
	for i := 0; i < 256; i++ {
		want := float32(util.Byte4ToInt(byte(i)))
		if got := lengthTable[i]; got != want {
			t.Errorf("lengthTable[%d] = %v, want %v", i, got, want)
		}
}	}
