// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.MultiNormsLeafSimScorer.
package search

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// sandboxLengthTable caches SmallFloat.byte4ToInt for all 256 byte values.
var sandboxLengthTable [256]float32

func init() {
	for i := range sandboxLengthTable {
		sandboxLengthTable[i] = float32(util.Byte4ToInt(byte(i)))
	}
}

// NormValuesReader is the narrow surface of an index.LeafReader required by
// MultiNormsLeafSimScorer: the ability to retrieve per-field norm values.
type NormValuesReader interface {
	GetNormValues(field string) (index.NumericDocValues, error)
}

// MultiNormsLeafSimScorer scores documents by combining norms from multiple
// fields, weighted by the supplied FieldAndWeight list. All norm fields must
// be encoded with SmallFloat.intToByte4.
//
// Mirrors org.apache.lucene.sandbox.search.MultiNormsLeafSimScorer
// (package-private in Java).
type MultiNormsLeafSimScorer struct {
	scorer search.LuceneSimScorer
	norms  index.NumericDocValues // nil when needsScores is false
}

// NewMultiNormsLeafSimScorer constructs a scorer. normFields lists the fields
// whose norms contribute to the combined score. needsScores must be true when
// actual scores are required; pass false for filter-only use.
func NewMultiNormsLeafSimScorer(
	scorer search.LuceneSimScorer,
	reader NormValuesReader,
	normFields []search.FieldAndWeight,
	needsScores bool,
) (*MultiNormsLeafSimScorer, error) {
	s := &MultiNormsLeafSimScorer{scorer: scorer}
	if needsScores {
		var normsArr []index.NumericDocValues
		var weightArr []float32
		seen := make(map[string]struct{}, len(normFields))

		for _, fw := range normFields {
			if _, dup := seen[fw.Field]; dup {
				continue // skip duplicates, matching Java's assertion-only guard
			}
			seen[fw.Field] = struct{}{}

			nv, err := reader.GetNormValues(fw.Field)
			if err != nil {
				return nil, err
			}
			if nv != nil {
				normsArr = append(normsArr, nv)
				weightArr = append(weightArr, fw.Weight)
			}
		}

		if len(normsArr) > 0 {
			s.norms = &sandboxMultiFieldNormValues{
				normsArr:  normsArr,
				weightArr: weightArr,
			}
		}
	}
	return s, nil
}

// GetSimScorer returns the underlying LuceneSimScorer.
func (s *MultiNormsLeafSimScorer) GetSimScorer() search.LuceneSimScorer {
	return s.scorer
}

// blendedValue computes the weighted norm blend for docID, positioning
// each underlying NumericDocValues via AdvanceExact (Lucene's iterator
// contract for MultiFieldNormValues). Callers MUST advance
// monotonically.
func (m *sandboxMultiFieldNormValues) blendedValue(docID int) (int64, error) {
	var normValue float32
	found := false

	for i, nv := range m.normsArr {
		ok, err := nv.AdvanceExact(docID)
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		raw, err := nv.LongValue()
		if err != nil {
			return 0, err
		}
		if raw != 0 {
			normValue += m.weightArr[i] * sandboxLengthTable[uint8(raw)]
			found = true
		}
	}

	if !found {
		return 1, nil
	}

	encoded, _ := util.IntToByte4(int(math.Round(float64(normValue))))
	return int64(encoded), nil
}

// getNormValue returns the combined norm for doc. Returns 1 when no norms are
// configured, matching Lucene's unit-length document fallback.
func (s *MultiNormsLeafSimScorer) getNormValue(doc int) (int64, error) {
	if s.norms != nil {
		ok, err := s.norms.AdvanceExact(doc)
		if err != nil || !ok {
			return 1, err
		}
		return s.norms.LongValue()
	}
	return 1, nil
}

// Score scores document doc with the given term frequency.
// This method must be called on non-decreasing document ID sequences.
func (s *MultiNormsLeafSimScorer) Score(doc int, freq float32) (float32, error) {
	norm, err := s.getNormValue(doc)
	if err != nil {
		return 0, err
	}
	return s.scorer.Score104(freq, norm), nil
}

// Explain returns an Explanation for the given document and frequency explanation.
// This method must be called on non-decreasing document ID sequences.
func (s *MultiNormsLeafSimScorer) Explain(doc int, freqExpl search.Explanation) (search.Explanation, error) {
	norm, err := s.getNormValue(doc)
	if err != nil {
		return nil, err
	}
	return s.scorer.Explain104(freqExpl, norm), nil
}

// sandboxMultiFieldNormValues blends norms from multiple norm sources into a
// single synthetic NumericDocValues via random-access Get.
type sandboxMultiFieldNormValues struct {
	normsArr  []index.NumericDocValues
	weightArr []float32
	doc       int
}

// Advance is unsupported; MultiNormsLeafSimScorer uses AdvanceExact.
func (m *sandboxMultiFieldNormValues) Advance(_ int) (int, error) {
	panic("sandboxMultiFieldNormValues: Advance not supported")
}

// AdvanceExact captures the target document and reports that the blender
// always has a value (the unit-length fallback collapses to 1 when no
// source field carries a norm). Mirrors MultiFieldNormValues.advanceExact
// in Lucene 10.4.0.
func (m *sandboxMultiFieldNormValues) AdvanceExact(target int) (bool, error) {
	m.doc = target
	return true, nil
}

// LongValue returns the blended norm bound to the current cursor.
// Mirrors MultiFieldNormValues.longValue in Lucene 10.4.0.
func (m *sandboxMultiFieldNormValues) LongValue() (int64, error) {
	return m.blendedValue(m.doc)
}

// NextDoc is unsupported.
func (m *sandboxMultiFieldNormValues) NextDoc() (int, error) {
	panic("sandboxMultiFieldNormValues: NextDoc not supported")
}

// DocID returns the most recently advanced target.
func (m *sandboxMultiFieldNormValues) DocID() int {
	if m.doc == 0 {
		return -1
	}
	return m.doc
}

// Cost is unknown on the synthetic blender; report 0 to match the
// conservative estimate Lucene uses for derived NumericDocValues.
func (m *sandboxMultiFieldNormValues) Cost() int64 { return 0 }

var _ index.NumericDocValues = (*sandboxMultiFieldNormValues)(nil)
