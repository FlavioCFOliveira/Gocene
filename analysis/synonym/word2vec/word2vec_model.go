// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package word2vec

import (
	codecshnsw "github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// Word2VecModel holds the parsed Word2Vec model: a dictionary of (term, vector)
// pairs where all vectors are L2-normalized at insertion time. It implements
// codecs/hnsw.FloatVectorValues so it can be used directly with
// DefaultFlatVectorScorer and HnswGraphBuilder.
//
// This is the Go port of
// org.apache.lucene.analysis.synonym.word2vec.Word2VecModel from
// Apache Lucene 10.4.0.
type Word2VecModel struct {
	dictionarySize  int
	vectorDimension int
	termsAndVectors []*util.TermAndVector
	word2Vec        *util.BytesRefHash
	loadedCount     int

	// iterator state
	iterPos int
}

// NewWord2VecModel creates a new Word2VecModel with the given capacity and
// vector dimension.
func NewWord2VecModel(dictionarySize, vectorDimension int) *Word2VecModel {
	return &Word2VecModel{
		dictionarySize:  dictionarySize,
		vectorDimension: vectorDimension,
		termsAndVectors: make([]*util.TermAndVector, dictionarySize),
		word2Vec:        util.NewBytesRefHash(),
		iterPos:         -1,
	}
}

// AddTermAndVector normalizes the vector in the entry and appends it to the
// model.
func (m *Word2VecModel) AddTermAndVector(entry *util.TermAndVector) {
	entry = entry.NormalizeVector()
	m.termsAndVectors[m.loadedCount] = entry
	m.loadedCount++
	_, _ = m.word2Vec.Add(entry.Term) //nolint:errcheck // index only; errors are size-exceeded, irrelevant here
}

// VectorValueByTerm returns the float32 vector for term, or nil if not found.
func (m *Word2VecModel) VectorValueByTerm(term *util.BytesRef) []float32 {
	termOrd := m.word2Vec.Find(term)
	if termOrd < 0 {
		return nil
	}
	entry := m.termsAndVectors[termOrd]
	if entry == nil {
		return nil
	}
	return entry.Vector
}

// TermValue returns the term at the given ordinal.
func (m *Word2VecModel) TermValue(targetOrd int) *util.BytesRef {
	return m.termsAndVectors[targetOrd].Term
}

// ----------------------- codecs/hnsw.FloatVectorValues -------------------

// VectorValue returns the float32 vector at the given ordinal.
// Implements codecs/hnsw.FloatVectorValues.
func (m *Word2VecModel) VectorValue(ord int) ([]float32, error) {
	return m.termsAndVectors[ord].Vector, nil
}

// CopyFloat returns a shallow copy of the model with independent iterator state.
// Implements codecs/hnsw.FloatVectorValues.
func (m *Word2VecModel) CopyFloat() (codecshnsw.FloatVectorValues, error) {
	return &Word2VecModel{
		dictionarySize:  m.dictionarySize,
		vectorDimension: m.vectorDimension,
		termsAndVectors: m.termsAndVectors,
		word2Vec:        m.word2Vec,
		loadedCount:     m.loadedCount,
		iterPos:         -1,
	}, nil
}

// ----------------------- util/hnsw.KnnVectorValues -----------------------

// Dimension returns the vector dimensionality.
func (m *Word2VecModel) Dimension() int { return m.vectorDimension }

// Size returns the number of vectors in the model.
func (m *Word2VecModel) Size() int { return m.dictionarySize }

// OrdToDoc is the identity function (dense model: ord == doc).
func (m *Word2VecModel) OrdToDoc(ord int) int { return ord }

// GetAcceptOrds returns acceptDocs unchanged (no doc-level filtering).
func (m *Word2VecModel) GetAcceptOrds(acceptDocs util.Bits) util.Bits { return acceptDocs }

// Iterator returns a DocIndexIterator that iterates over all ordinals.
func (m *Word2VecModel) Iterator() utilhnsw.DocIndexIterator {
	return &word2vecIterator{model: m, pos: -1}
}

// GetEncoding reports FLOAT32 encoding. Required by codecs/hnsw.HasEncoding.
func (m *Word2VecModel) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingFloat32
}

// Compile-time interface assertion.
var _ codecshnsw.FloatVectorValues = (*Word2VecModel)(nil)

// word2vecIterator is a dense DocIndexIterator over the model.
type word2vecIterator struct {
	model *Word2VecModel
	pos   int
}

func (it *word2vecIterator) NextDoc() (int, error) {
	it.pos++
	if it.pos >= it.model.dictionarySize {
		return util.NO_MORE_DOCS, nil
	}
	return it.pos, nil
}

func (it *word2vecIterator) Index() int { return it.pos }
