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

// getNormValue returns the combined norm for doc. Returns 1 when no norms are
// configured, matching Lucene's unit-length document fallback.
func (s *MultiNormsLeafSimScorer) getNormValue(doc int) (int64, error) {
	if s.norms != nil {
		return s.norms.Get(doc)
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
}

// Get returns the blended norm for docID by accumulating weighted Byte4ToInt
// values and re-encoding with IntToByte4. Returns 1 when no field has a norm.
func (m *sandboxMultiFieldNormValues) Get(docID int) (int64, error) {
	var normValue float32
	found := false

	for i, nv := range m.normsArr {
		raw, err := nv.Get(docID)
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

// Advance is unsupported; MultiNormsLeafSimScorer uses random-access Get only.
func (m *sandboxMultiFieldNormValues) Advance(_ int) (int, error) {
	panic("sandboxMultiFieldNormValues: Advance not supported")
}

// NextDoc is unsupported.
func (m *sandboxMultiFieldNormValues) NextDoc() (int, error) {
	panic("sandboxMultiFieldNormValues: NextDoc not supported")
}

// DocID returns -1 (random-access only).
func (m *sandboxMultiFieldNormValues) DocID() int { return -1 }

var _ index.NumericDocValues = (*sandboxMultiFieldNormValues)(nil)
