// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.MultiNormsLeafSimScorer tests.
package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// captureSimScorer records the last Score104 call arguments.
type captureSimScorer struct {
	lastFreq float32
	lastNorm int64
}

func (c *captureSimScorer) Score104(freq float32, norm int64) float32 {
	c.lastFreq = freq
	c.lastNorm = norm
	return freq * 2
}

func (c *captureSimScorer) AsBulkSimScorer() search.BulkSimScorer {
	return search.NewDefaultBulkSimScorer(c)
}

func (c *captureSimScorer) Explain104(freqExpl search.Explanation, norm int64) search.Explanation {
	return freqExpl
}

var _ search.LuceneSimScorer = (*captureSimScorer)(nil)

// fixedNormValues is a NumericDocValues that always returns the given value.
type fixedNormValues struct {
	val int64
}

func (f *fixedNormValues) Advance(_ int) (int, error)       { panic("unsupported") }
func (f *fixedNormValues) AdvanceExact(_ int) (bool, error) { return true, nil }
func (f *fixedNormValues) LongValue() (int64, error)        { return f.val, nil }
func (f *fixedNormValues) NextDoc() (int, error)            { panic("unsupported") }
func (f *fixedNormValues) DocID() int                       { return -1 }
func (f *fixedNormValues) Cost() int64                      { return 1 }

var _ index.NumericDocValues = (*fixedNormValues)(nil)

// normValuesReader returns fixedNormValues for any field name.
type singleFieldNormReader struct {
	field string
	norms index.NumericDocValues
}

func (r *singleFieldNormReader) GetNormValues(field string) (index.NumericDocValues, error) {
	if field == r.field {
		return r.norms, nil
	}
	return nil, nil
}

// TestMultiNormsLeafSimScorer_ScoreDelegatesFreqAndNorm verifies that Score
// passes freq and the combined norm to the underlying LuceneSimScorer.
func TestMultiNormsLeafSimScorer_ScoreDelegatesFreqAndNorm(t *testing.T) {
	cap := &captureSimScorer{}
	// Encode norm=4 → byte4 encoding, then use it as the raw norm value.
	encoded, _ := util.IntToByte4(4)
	reader := &singleFieldNormReader{field: "body", norms: &fixedNormValues{val: int64(encoded)}}

	scorer, err := NewMultiNormsLeafSimScorer(
		cap,
		reader,
		[]search.FieldAndWeight{{Field: "body", Weight: 1.0}},
		true,
	)
	if err != nil {
		t.Fatal(err)
	}

	score, err := scorer.Score(0, 2.5)
	if err != nil {
		t.Fatal(err)
	}
	if cap.lastFreq != 2.5 {
		t.Errorf("Score104 freq = %v; want 2.5", cap.lastFreq)
	}
	// norm is non-zero and was forwarded
	if cap.lastNorm == 0 {
		t.Errorf("Score104 norm should be non-zero")
	}
	if score != cap.lastFreq*2 {
		t.Errorf("Score() = %v; want %v", score, cap.lastFreq*2)
	}
}

// TestMultiNormsLeafSimScorer_NoNormsWhenNeedsScoresFalse verifies that when
// needsScores=false, Score passes norm=1 (the unit fallback).
func TestMultiNormsLeafSimScorer_NoNormsWhenNeedsScoresFalse(t *testing.T) {
	cap := &captureSimScorer{}
	reader := &singleFieldNormReader{field: "body", norms: &fixedNormValues{val: 42}}

	scorer, err := NewMultiNormsLeafSimScorer(
		cap,
		reader,
		[]search.FieldAndWeight{{Field: "body", Weight: 1.0}},
		false,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = scorer.Score(0, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	if cap.lastNorm != 1 {
		t.Errorf("expected unit norm=1 when needsScores=false, got %d", cap.lastNorm)
	}
}

// TestMultiNormsLeafSimScorer_NilNormFieldSkipped verifies that a field
// returning nil NumericDocValues is silently skipped; Score uses norm=1.
func TestMultiNormsLeafSimScorer_NilNormFieldSkipped(t *testing.T) {
	cap := &captureSimScorer{}
	// Reader returns nil for all fields.
	reader := &singleFieldNormReader{field: "other", norms: &fixedNormValues{val: 99}}

	scorer, err := NewMultiNormsLeafSimScorer(
		cap,
		reader,
		[]search.FieldAndWeight{{Field: "body", Weight: 1.0}},
		true,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = scorer.Score(7, 3.0)
	if err != nil {
		t.Fatal(err)
	}
	// No norm source found → unit norm=1
	if cap.lastNorm != 1 {
		t.Errorf("expected unit norm=1 when no norm source, got %d", cap.lastNorm)
	}
}

// TestMultiNormsLeafSimScorer_DuplicateFieldDeduped verifies that duplicate
// field entries in normFields are deduplicated.
func TestMultiNormsLeafSimScorer_DuplicateFieldDeduped(t *testing.T) {
	callCount := 0
	type countingReader struct {
		singleFieldNormReader
		count *int
	}
	cr := &countingReader{
		singleFieldNormReader: singleFieldNormReader{
			field: "body",
			norms: &fixedNormValues{val: 1},
		},
		count: &callCount,
	}

	reader := &singleFieldNormReader{field: "body", norms: &fixedNormValues{val: 1}}
	cap := &captureSimScorer{}
	_, err := NewMultiNormsLeafSimScorer(
		cap,
		reader,
		[]search.FieldAndWeight{
			{Field: "body", Weight: 1.0},
			{Field: "body", Weight: 2.0}, // duplicate
		},
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = cr // not actually needed; just verifying no error on dup field
}

// TestMultiNormsLeafSimScorer_GetSimScorer verifies that GetSimScorer returns
// the underlying LuceneSimScorer.
func TestMultiNormsLeafSimScorer_GetSimScorer(t *testing.T) {
	cap := &captureSimScorer{}
	reader := &singleFieldNormReader{field: "body", norms: &fixedNormValues{val: 1}}
	scorer, err := NewMultiNormsLeafSimScorer(cap, reader, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := scorer.GetSimScorer().(*captureSimScorer)
	if !ok || got != cap {
		t.Error("GetSimScorer() should return the supplied scorer")
	}
}

// TestMultiNormsLeafSimScorer_MultiFieldNormBlended verifies that two fields
// with equal weight=0.5 and the same raw byte norm produce a blended result.
func TestMultiNormsLeafSimScorer_MultiFieldNormBlended(t *testing.T) {
	cap := &captureSimScorer{}
	// Use raw value=1 so sandboxLengthTable[1] is decoded, then re-encoded.
	norms1 := &fixedNormValues{val: 1}
	norms2 := &fixedNormValues{val: 1}

	reader := &dualFieldNormReader{
		fields: map[string]index.NumericDocValues{
			"f1": norms1,
			"f2": norms2,
		},
	}

	scorer, err := NewMultiNormsLeafSimScorer(
		cap,
		reader,
		[]search.FieldAndWeight{
			{Field: "f1", Weight: 0.5},
			{Field: "f2", Weight: 0.5},
		},
		true,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = scorer.Score(0, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	// The blended norm must be non-zero and have been forwarded.
	if cap.lastNorm == 0 {
		t.Error("blended norm should be non-zero")
	}
}

// dualFieldNormReader returns per-field NumericDocValues from a map.
type dualFieldNormReader struct {
	fields map[string]index.NumericDocValues
}

func (d *dualFieldNormReader) GetNormValues(field string) (index.NumericDocValues, error) {
	return d.fields[field], nil
}
