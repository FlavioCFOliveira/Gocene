// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/java/org/apache/lucene/search/MultiNormsLeafSimScorer.java

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// lengthTable mirrors the static LENGTH_TABLE from MultiNormsLeafSimScorer.
// Entry i holds SmallFloat.byte4ToInt((byte) i), precomputed at init time.
var lengthTable [256]float32

func init() {
	for i := range lengthTable {
		lengthTable[i] = float32(util.Byte4ToInt(byte(i)))
	}
}

// FieldAndWeight pairs a field name with a scoring weight.
//
// It mirrors the Java record FieldAndWeight(String field, float weight)
// nested inside CombinedFieldQuery.
type FieldAndWeight struct {
	// Field is the name of the indexed field.
	Field string
	// Weight is the relative weight applied to this field's norm contribution.
	Weight float32
}

// normValuesReader is the narrow surface of a leaf reader that
// multiNormsLeafSimScorer requires: the ability to look up norm values for a
// named field.
type normValuesReader interface {
	GetNormValues(field string) (index.NumericDocValues, error)
}

// multiNormsLeafSimScorer scores a single leaf segment across multiple norm
// fields, combining their encoded length norms through a weighted sum before
// delegating to a LuceneSimScorer.
//
// Mirrors org.apache.lucene.search.MultiNormsLeafSimScorer (package-private).
type multiNormsLeafSimScorer struct {
	scorer     LuceneSimScorer
	bulkScorer BulkSimScorer
	norms      index.NumericDocValues // nil when needsScores is false
	normValues []int64                // scratch buffer for scoreRange
}

// newMultiNormsLeafSimScorer constructs a scorer for the given leaf reader.
//
// normFields lists the fields whose norms contribute to the combined score.
// needsScores controls whether norms are loaded; pass false for pure filter
// use where actual scores are not required (results in a nil norms source and
// getNormValue always returns 1).
func newMultiNormsLeafSimScorer(
	scorer LuceneSimScorer,
	reader normValuesReader,
	normFields []FieldAndWeight,
	needsScores bool,
) (*multiNormsLeafSimScorer, error) {
	if scorer == nil {
		panic("multiNormsLeafSimScorer: scorer must not be nil")
	}

	s := &multiNormsLeafSimScorer{
		scorer:     scorer,
		bulkScorer: scorer.AsBulkSimScorer(),
	}

	if needsScores {
		var normsArr []index.NumericDocValues
		var weightArr []float32

		for _, fw := range normFields {
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
			s.norms = &multiFieldNormValues{
				normsArr:  normsArr,
				weightArr: weightArr,
			}
		}
		// if normsArr is empty, s.norms remains nil — getNormValue returns 1.
	}

	return s, nil
}

// getSimScorer exposes the underlying LuceneSimScorer.
func (s *multiNormsLeafSimScorer) getSimScorer() LuceneSimScorer { return s.scorer }

// getNormValue returns the combined norm for doc. When norms is nil (no
// needsScores, or no fields had norms) it returns 1, matching Lucene's
// fallback to a unit-length document.
func (s *multiNormsLeafSimScorer) getNormValue(doc int) (int64, error) {
	if s.norms != nil {
		// NumericDocValues is iterator-shaped after rmp #4710: position
		// the iterator on doc via AdvanceExact and read the value via
		// LongValue. Lucene's MultiNormsLeafSimScorer.getNormValue uses
		// the same pattern.
		ok, err := s.norms.AdvanceExact(doc)
		if err != nil || !ok {
			return 1, err
		}
		return s.norms.LongValue()
	}
	return 1, nil
}

// score scores document doc with the supplied term frequency.
func (s *multiNormsLeafSimScorer) score(doc int, freq float32) (float32, error) {
	norm, err := s.getNormValue(doc)
	if err != nil {
		return 0, err
	}
	return s.scorer.Score104(freq, norm), nil
}

// scoreRange scores a batch of (doc, freq) pairs described by buffer.
// The buffer's Features slice is used as both input (frequencies) and output
// (scores), matching the Lucene convention of reusing the same array.
func (s *multiNormsLeafSimScorer) scoreRange(buffer *DocAndFloatFeatureBuffer) error {
	// Grow scratch buffer without copying — old content is always overwritten.
	if cap(s.normValues) < buffer.Size {
		s.normValues = make([]int64, buffer.Size)
	}
	norms := s.normValues[:buffer.Size]

	if s.norms != nil {
		mfnv, ok := s.norms.(*multiFieldNormValues)
		if ok {
			if err := mfnv.longValues(buffer.Size, buffer.Docs, norms, 1); err != nil {
				return err
			}
		} else {
			// Fallback for any other NumericDocValues implementation.
			// buffer.Docs is monotonic ascending per the bulk-scoring
			// contract, so AdvanceExact + LongValue is safe.
			for i := 0; i < buffer.Size; i++ {
				ok, err := s.norms.AdvanceExact(buffer.Docs[i])
				if err != nil {
					return err
				}
				if !ok {
					norms[i] = 1
					continue
				}
				v, err := s.norms.LongValue()
				if err != nil {
					return err
				}
				norms[i] = v
			}
		}
	} else {
		for i := range norms {
			norms[i] = 1
		}
	}

	s.bulkScorer.ScoreBulk(buffer.Size, buffer.Features, norms, buffer.Features)
	return nil
}

// explain returns an Explanation for document doc given the supplied frequency
// explanation.
func (s *multiNormsLeafSimScorer) explain(doc int, freqExpl Explanation) (Explanation, error) {
	norm, err := s.getNormValue(doc)
	if err != nil {
		return nil, err
	}
	return s.scorer.Explain104(freqExpl, norm), nil
}

// multiFieldNormValues blends norms from multiple fields into a single
// synthetic NumericDocValues stream.
//
// Mirrors the private static inner class MultiFieldNormValues. Lucene's
// reference overrides advanceExact / longValue / cost on the inner
// NumericDocValues; the Gocene port mirrors that exactly. doc is the
// monotonically advanced target captured at AdvanceExact time and
// consumed by LongValue.
type multiFieldNormValues struct {
	normsArr  []index.NumericDocValues
	weightArr []float32
	accBuf    []float32 // scratch for longValues bulk path
	doc       int
}

// blendedValue computes the weighted norm blend for docID. The
// underlying NumericDocValues iterators are positioned via AdvanceExact
// per Lucene's iterator contract; callers MUST drive docID
// monotonically (docID >= the previous docID seen by this blender).
func (m *multiFieldNormValues) blendedValue(docID int) (int64, error) {
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
		// raw == 0 means "no value" for this field — skip the contribution.
		if raw != 0 {
			normValue += m.weightArr[i] * lengthTable[uint8(raw)]
			found = true
		}
	}

	if !found {
		return 1, nil
	}

	encoded, _ := util.IntToByte4(int(math.Round(float64(normValue))))
	return int64(encoded), nil
}

// longValues bulk-populates values[0:size] using the docs slice as doc IDs.
// When a document has no norm contribution across all fields, defaultValue is
// used. This mirrors the Java longValues override on MultiFieldNormValues.
func (m *multiFieldNormValues) longValues(size int, docs []int, values []int64, defaultValue int64) error {
	// Ensure accBuf has capacity; zero-initialise the live portion.
	if cap(m.accBuf) < size {
		m.accBuf = make([]float32, size)
	} else {
		acc := m.accBuf[:size]
		for i := range acc {
			acc[i] = 0
		}
	}
	acc := m.accBuf[:size]

	// Accumulate weighted lengths from each norm source. docs[] is the
	// per-batch slice the scorer feeds the bulk path with; we drive each
	// underlying iterator via AdvanceExact + LongValue, the iterator-
	// shaped equivalent of the legacy random-access Get(docID) accessor.
	for i, nv := range m.normsArr {
		for j := 0; j < size; j++ {
			ok, err := nv.AdvanceExact(docs[j])
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			raw, err := nv.LongValue()
			if err != nil {
				return err
			}
			acc[j] += m.weightArr[i] * lengthTable[uint8(raw)]
		}
	}

	// Encode results back through IntToByte4.
	for i := 0; i < size; i++ {
		if acc[i] == 0 {
			values[i] = defaultValue
		} else {
			encoded, _ := util.IntToByte4(int(math.Round(float64(acc[i]))))
			values[i] = int64(encoded)
		}
	}
	return nil
}

// Advance is unsupported on the random-access blender; callers drive
// iteration via AdvanceExact, matching Lucene's MultiFieldNormValues.
func (m *multiFieldNormValues) Advance(_ int) (int, error) {
	panic("multiFieldNormValues: Advance not supported; use AdvanceExact")
}

// AdvanceExact captures the target document and reports that the
// blender always has a value (the unit-length fallback collapses to 1
// when no source field carries a norm for the target). Mirrors
// MultiFieldNormValues.advanceExact in Lucene 10.4.0, which also
// always returns true.
func (m *multiFieldNormValues) AdvanceExact(target int) (bool, error) {
	m.doc = target
	return true, nil
}

// LongValue returns the blended norm bound to the current cursor.
// Mirrors MultiFieldNormValues.longValue in Lucene 10.4.0.
func (m *multiFieldNormValues) LongValue() (int64, error) {
	return m.blendedValue(m.doc)
}

// NextDoc is unsupported on the random-access blender.
func (m *multiFieldNormValues) NextDoc() (int, error) {
	panic("multiFieldNormValues: NextDoc not supported")
}

// DocID returns the most recently advanced target, or -1 before
// AdvanceExact is called.
func (m *multiFieldNormValues) DocID() int {
	if m.doc == 0 {
		return -1
	}
	return m.doc
}

// Cost is unknown on the synthetic blender; report 0 to match the
// conservative estimate Lucene uses for derived NumericDocValues.
func (m *multiFieldNormValues) Cost() int64 { return 0 }

// Ensure multiFieldNormValues satisfies index.NumericDocValues.
var _ index.NumericDocValues = (*multiFieldNormValues)(nil)
