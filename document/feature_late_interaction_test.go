// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestFeatureField_Basic(t *testing.T) {
	f, err := NewFeatureField("scores", "pagerank", 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if f.GetFeatureName() != "pagerank" || f.GetFeatureValue() != 0.5 {
		t.Fatalf("attrs wrong")
	}
	if f.FieldType().GetIndexOptions() != index.IndexOptionsDocsAndFreqs {
		t.Fatalf("indexOptions wrong: %v", f.FieldType().GetIndexOptions())
	}
}

func TestFeatureField_RejectInvalidValues(t *testing.T) {
	cases := []float32{0, -1, float32(math.NaN()), float32(math.Inf(1))}
	for _, v := range cases {
		if _, err := NewFeatureField("scores", "x", v); err == nil {
			t.Errorf("expected error for value %v", v)
		}
	}
}

func TestFeatureField_EncodeDecodeTermFreq(t *testing.T) {
	values := []float32{0.1, 1, 100, 1e6}
	for _, v := range values {
		freq := EncodeFeatureValueAsTermFreq(v)
		decoded := DecodeFeatureValueFromTermFreq(freq)
		// Precision: upper-16-bits encoding gives ~3 significant digits.
		// Upper-16-bits encoding gives ~2-3 significant digits.
		rel := math.Abs(float64(decoded-v) / float64(v))
		if rel > 2e-3 {
			t.Errorf("round-trip %v -> %v (rel err %v)", v, decoded, rel)
		}
	}
}

func TestLateInteractionField_RoundTrip(t *testing.T) {
	m := [][]float32{{1, 2, 3}, {4, 5, 6}}
	f, err := NewLateInteractionField("li", m)
	if err != nil {
		t.Fatal(err)
	}
	got := f.GetValue()
	if len(got) != 2 || len(got[0]) != 3 || got[1][2] != 6 {
		t.Fatalf("matrix mismatch: %v", got)
	}
}

func TestLateInteractionField_DimMismatchErrors(t *testing.T) {
	if _, err := NewLateInteractionField("li", [][]float32{{1, 2}, {3, 4, 5}}); err == nil {
		t.Fatalf("expected error for non-uniform dim")
	}
}

func TestLateInteraction_EmptyErrors(t *testing.T) {
	if _, err := EncodeLateInteraction(nil); err == nil {
		t.Fatalf("expected error for nil")
	}
	if _, err := EncodeLateInteraction([][]float32{{}}); err == nil {
		t.Fatalf("expected error for empty inner vector")
	}
}

func TestLateInteraction_DecodeInvalid(t *testing.T) {
	if _, err := DecodeLateInteraction([]byte{0, 0}); err == nil {
		t.Fatalf("expected error for short payload")
	}
	if _, err := DecodeLateInteraction([]byte{0, 0, 0, 0}); err == nil {
		t.Fatalf("expected error for zero-dim payload")
	}
}
