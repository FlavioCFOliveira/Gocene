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
		return s.norms.Get(doc)
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
			for i := 0; i < buffer.Size; i++ {
				v, err := s.norms.Get(buffer.Docs[i])
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
// Mirrors the private static inner class MultiFieldNormValues.
type multiFieldNormValues struct {
	normsArr  []index.NumericDocValues
	weightArr []float32
	accBuf    []float32 // scratch for longValues bulk path
}

// Get returns the blended norm for doc. It iterates over all norm sources,
// accumulates weighted Byte4ToInt values, and encodes the rounded sum back
// through IntToByte4. When no field has a norm for this doc, Get returns
// (1, nil) to maintain the "unit-length document" invariant.
//
// This satisfies index.NumericDocValues.Get(docID int) (int64, error).
func (m *multiFieldNormValues) Get(docID int) (int64, error) {
	var normValue float32
	found := false

	for i, nv := range m.normsArr {
		raw, err := nv.Get(docID)
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

	// Accumulate weighted lengths from each norm source.
	for i, nv := range m.normsArr {
		// Borrow values[] as scratch for this field's raw norms.
		for j := 0; j < size; j++ {
			raw, err := nv.Get(docs[j])
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

// Advance, NextDoc, DocID, and Cost are unused on the single-doc path that
// drives multiNormsLeafSimScorer; only Get and longValues are called.

// Advance is unsupported.
func (m *multiFieldNormValues) Advance(_ int) (int, error) {
	panic("multiFieldNormValues: Advance not supported")
}

// NextDoc is unsupported.
func (m *multiFieldNormValues) NextDoc() (int, error) {
	panic("multiFieldNormValues: NextDoc not supported")
}

// DocID returns -1 (multiFieldNormValues uses random-access Get only).
func (m *multiFieldNormValues) DocID() int { return -1 }

// Ensure multiFieldNormValues satisfies index.NumericDocValues.
var _ index.NumericDocValues = (*multiFieldNormValues)(nil)
