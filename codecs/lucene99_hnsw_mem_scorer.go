// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//   Licensed to the Apache Software Foundation (ASF) under one or more
//   contributor license agreements. See the NOTICE file distributed with
//   this work for additional information regarding copyright ownership.
//   The ASF licenses this file to You under the Apache License, Version 2.0
//   (the "License"); you may not use this file except in compliance with
//   the License. You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//   implied. See the License for the specific language governing
//   permissions and limitations under the License.

// lucene99_hnsw_mem_scorer.go — in-memory scorer supplier for HNSW graph
// building at index time. This is a codecs-internal implementation that
// avoids importing codecs/hnsw (which would create an import cycle) by
// inlining the minimal float/byte similarity scoring needed during
// Lucene99HnswVectorsWriter.finish().
//
// The Java equivalent is DefaultFlatVectorScorer + inner supplier classes,
// but those live in codecs.hnsw. This package-private Go adaptation
// re-implements only the graph-build path.

package codecs

import (
	"errors"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// ---------------------------------------------------------------------------
// In-memory float32 KnnVectorValues
// ---------------------------------------------------------------------------

// memFloat32VectorValues wraps a [][]float32 slice and implements
// util/hnsw.KnnVectorValues. It is used to feed the HNSW graph builder
// from the vectors collected in lucene99HnswFieldWriter.
type memFloat32VectorValues struct {
	vecs [][]float32
}

func newMemFloat32VectorValues(vecs [][]float32) *memFloat32VectorValues {
	return &memFloat32VectorValues{vecs: vecs}
}

func (m *memFloat32VectorValues) Dimension() int       { return len(m.vecs[0]) }
func (m *memFloat32VectorValues) Size() int            { return len(m.vecs) }
func (m *memFloat32VectorValues) OrdToDoc(ord int) int { return ord }
func (m *memFloat32VectorValues) GetAcceptOrds(_ util.Bits) util.Bits {
	return nil // nil → accept all
}

// VectorValue returns the float32 vector at ordinal ord.
func (m *memFloat32VectorValues) VectorValue(ord int) ([]float32, error) {
	if ord < 0 || ord >= len(m.vecs) {
		return nil, errors.New("memFloat32VectorValues: ordinal out of range")
	}
	return m.vecs[ord], nil
}

// CopyFloat returns an independent copy (used by the scorer supplier
// to create a targetVectors buffer).
func (m *memFloat32VectorValues) CopyFloat() (*memFloat32VectorValues, error) {
	cp := make([][]float32, len(m.vecs))
	for i, v := range m.vecs {
		cp[i] = make([]float32, len(v))
		copy(cp[i], v)
	}
	return &memFloat32VectorValues{vecs: cp}, nil
}

// Iterator returns a simple sequential iterator.
func (m *memFloat32VectorValues) Iterator() utilhnsw.DocIndexIterator {
	return &seqDocIndexIterator{size: len(m.vecs), cur: -1}
}

// ---------------------------------------------------------------------------
// In-memory byte KnnVectorValues
// ---------------------------------------------------------------------------

// memByteVectorValues wraps a [][]byte slice.
type memByteVectorValues struct {
	vecs [][]byte
}

func newMemByteVectorValues(vecs [][]byte) *memByteVectorValues {
	return &memByteVectorValues{vecs: vecs}
}

func (m *memByteVectorValues) Dimension() int       { return len(m.vecs[0]) }
func (m *memByteVectorValues) Size() int            { return len(m.vecs) }
func (m *memByteVectorValues) OrdToDoc(ord int) int { return ord }
func (m *memByteVectorValues) GetAcceptOrds(_ util.Bits) util.Bits {
	return nil
}

// VectorValue returns the byte vector at ordinal ord.
func (m *memByteVectorValues) VectorValue(ord int) ([]byte, error) {
	if ord < 0 || ord >= len(m.vecs) {
		return nil, errors.New("memByteVectorValues: ordinal out of range")
	}
	return m.vecs[ord], nil
}

// CopyByte returns an independent copy.
func (m *memByteVectorValues) CopyByte() (*memByteVectorValues, error) {
	cp := make([][]byte, len(m.vecs))
	for i, v := range m.vecs {
		cp[i] = make([]byte, len(v))
		copy(cp[i], v)
	}
	return &memByteVectorValues{vecs: cp}, nil
}

// Iterator returns a simple sequential iterator.
func (m *memByteVectorValues) Iterator() utilhnsw.DocIndexIterator {
	return &seqDocIndexIterator{size: len(m.vecs), cur: -1}
}

// ---------------------------------------------------------------------------
// Sequential DocIndexIterator — identity ordinal → docID mapping.
// ---------------------------------------------------------------------------

type seqDocIndexIterator struct {
	size int
	cur  int
}

func (it *seqDocIndexIterator) NextDoc() (int, error) {
	it.cur++
	if it.cur >= it.size {
		it.cur = it.size // clamp at exhaustion
		return util.NO_MORE_DOCS, nil
	}
	return it.cur, nil
}

func (it *seqDocIndexIterator) Index() int { return it.cur }

// ---------------------------------------------------------------------------
// Float32 scorer supplier
// ---------------------------------------------------------------------------

// memFloat32ScorerSupplier implements util/hnsw.RandomVectorScorerSupplier
// for in-memory float32 vectors during graph building.
type memFloat32ScorerSupplier struct {
	vecs   *memFloat32VectorValues
	target *memFloat32VectorValues
	sim    index.VectorSimilarityFunction
}

func newMemFloat32ScorerSupplier(
	vecs *memFloat32VectorValues,
	sim index.VectorSimilarityFunction,
) (utilhnsw.RandomVectorScorerSupplier, error) {
	tgt, err := vecs.CopyFloat()
	if err != nil {
		return nil, err
	}
	return &memFloat32ScorerSupplier{vecs: vecs, target: tgt, sim: sim}, nil
}

func (s *memFloat32ScorerSupplier) Scorer() (utilhnsw.UpdateableRandomVectorScorer, error) {
	buf := make([]float32, s.vecs.Dimension())
	base := utilhnsw.NewAbstractUpdateableRandomVectorScorer(s.vecs)
	return &memFloat32Scorer{
		AbstractUpdateableRandomVectorScorer: base,
		supplier:                             s,
		buf:                                  buf,
	}, nil
}

func (s *memFloat32ScorerSupplier) Copy() (utilhnsw.RandomVectorScorerSupplier, error) {
	return newMemFloat32ScorerSupplier(s.vecs, s.sim)
}

// memFloat32Scorer is the per-Scorer instance.
type memFloat32Scorer struct {
	*utilhnsw.AbstractUpdateableRandomVectorScorer
	supplier *memFloat32ScorerSupplier
	buf      []float32 // target vector copy
}

func (s *memFloat32Scorer) SetScoringOrdinal(node int) error {
	v, err := s.supplier.target.VectorValue(node)
	if err != nil {
		return err
	}
	copy(s.buf, v)
	return nil
}

func (s *memFloat32Scorer) Score(node int) (float32, error) {
	v, err := s.supplier.vecs.VectorValue(node)
	if err != nil {
		return 0, err
	}
	return memComputeFloatSimilarity(s.supplier.sim, s.buf, v), nil
}

func (s *memFloat32Scorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return utilhnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

// ---------------------------------------------------------------------------
// Byte scorer supplier
// ---------------------------------------------------------------------------

// memByteScorerSupplier implements util/hnsw.RandomVectorScorerSupplier
// for in-memory byte vectors during graph building.
type memByteScorerSupplier struct {
	vecs   *memByteVectorValues
	target *memByteVectorValues
	sim    index.VectorSimilarityFunction
}

func newMemByteScorerSupplier(
	vecs *memByteVectorValues,
	sim index.VectorSimilarityFunction,
) (utilhnsw.RandomVectorScorerSupplier, error) {
	tgt, err := vecs.CopyByte()
	if err != nil {
		return nil, err
	}
	return &memByteScorerSupplier{vecs: vecs, target: tgt, sim: sim}, nil
}

func (s *memByteScorerSupplier) Scorer() (utilhnsw.UpdateableRandomVectorScorer, error) {
	buf := make([]byte, s.vecs.Dimension())
	base := utilhnsw.NewAbstractUpdateableRandomVectorScorer(s.vecs)
	return &memByteScorer{
		AbstractUpdateableRandomVectorScorer: base,
		supplier:                             s,
		buf:                                  buf,
	}, nil
}

func (s *memByteScorerSupplier) Copy() (utilhnsw.RandomVectorScorerSupplier, error) {
	return newMemByteScorerSupplier(s.vecs, s.sim)
}

// memByteScorer is the per-Scorer instance for byte vectors.
type memByteScorer struct {
	*utilhnsw.AbstractUpdateableRandomVectorScorer
	supplier *memByteScorerSupplier
	buf      []byte
}

func (s *memByteScorer) SetScoringOrdinal(node int) error {
	v, err := s.supplier.target.VectorValue(node)
	if err != nil {
		return err
	}
	copy(s.buf, v)
	return nil
}

func (s *memByteScorer) Score(node int) (float32, error) {
	v, err := s.supplier.vecs.VectorValue(node)
	if err != nil {
		return 0, err
	}
	return memComputeByteSimilarity(s.supplier.sim, s.buf, v), nil
}

func (s *memByteScorer) BulkScore(nodes []int, scores []float32, numNodes int) (float32, error) {
	return utilhnsw.BulkScoreDefault(s, nodes, scores, numNodes)
}

// ---------------------------------------------------------------------------
// Similarity functions (inlined from codecs/hnsw to avoid import cycle)
// ---------------------------------------------------------------------------

// memComputeFloatSimilarity is a private copy of the float similarity
// dispatch in codecs/hnsw/default_flat_vector_scorer.go. Duplicated here
// to avoid the codecs → codecs/hnsw → codecs import cycle.
func memComputeFloatSimilarity(sim index.VectorSimilarityFunction, a, b []float32) float32 {
	switch sim {
	case index.VectorSimilarityFunctionEuclidean:
		var sum float32
		for i := range a {
			d := a[i] - b[i]
			sum += d * d
		}
		return 1.0 / (1.0 + sum)
	case index.VectorSimilarityFunctionDotProduct:
		var dot float32
		for i := range a {
			dot += a[i] * b[i]
		}
		return (dot + 1.0) / 2.0
	case index.VectorSimilarityFunctionCosine:
		var dot, na, nb float32
		for i := range a {
			dot += a[i] * b[i]
			na += a[i] * a[i]
			nb += b[i] * b[i]
		}
		if na == 0 || nb == 0 {
			return 0
		}
		return (dot/(memSqrt32(na)*memSqrt32(nb)) + 1.0) / 2.0
	case index.VectorSimilarityFunctionMaximumInnerProduct:
		var dot float32
		for i := range a {
			dot += a[i] * b[i]
		}
		if dot < 0 {
			return 1.0 / (1.0 - dot)
		}
		return dot + 1.0
	default:
		return 0
	}
}

// memComputeByteSimilarity is a private copy of the byte similarity
// dispatch.
func memComputeByteSimilarity(sim index.VectorSimilarityFunction, a, b []byte) float32 {
	switch sim {
	case index.VectorSimilarityFunctionEuclidean:
		var sum int32
		for i := range a {
			d := int32(a[i]) - int32(b[i])
			sum += d * d
		}
		return 1.0 / (1.0 + float32(sum))
	case index.VectorSimilarityFunctionDotProduct:
		var dot int32
		for i := range a {
			dot += int32(a[i]) * int32(b[i])
		}
		maxDot := float32(127 * 127 * len(a))
		if maxDot == 0 {
			return 0
		}
		return (float32(dot) + maxDot) / (2.0 * maxDot)
	case index.VectorSimilarityFunctionCosine:
		var dot, na, nb int32
		for i := range a {
			dot += int32(a[i]) * int32(b[i])
			na += int32(a[i]) * int32(a[i])
			nb += int32(b[i]) * int32(b[i])
		}
		if na == 0 || nb == 0 {
			return 0.5
		}
		cos := float32(dot) / (memSqrt32(float32(na)) * memSqrt32(float32(nb)))
		return (cos + 1.0) / 2.0
	case index.VectorSimilarityFunctionMaximumInnerProduct:
		var dot int32
		for i := range a {
			dot += int32(a[i]) * int32(b[i])
		}
		d := float32(dot)
		if d < 0 {
			return 1.0 / (1.0 - d)
		}
		return d + 1.0
	default:
		return 0
	}
}

func memSqrt32(x float32) float32 { return float32(math.Sqrt(float64(x))) }
