// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package word2vec

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Word2VecModel holds the parsed Word2Vec model: a dictionary of (term, vector)
// pairs where all vectors are L2-normalized at insertion time. As in Lucene it
// is a FloatVectorValues ([spi.FloatVectorValues]), so it can be handed to a
// FlatVectorsScorer and HnswGraphBuilder directly.
//
// This is the Go port of
// org.apache.lucene.analysis.synonym.word2vec.Word2VecModel from
// Apache Lucene 10.5.0.
type Word2VecModel struct {
	dictionarySize  int
	vectorDimension int
	termsAndVectors []*util.TermAndVector
	word2Vec        *util.BytesRefHash
	loadedCount     int
}

// NewWord2VecModel creates a new Word2VecModel with the given capacity and
// vector dimension. Mirrors Word2VecModel(int, int).
func NewWord2VecModel(dictionarySize, vectorDimension int) *Word2VecModel {
	return &Word2VecModel{
		dictionarySize:  dictionarySize,
		vectorDimension: vectorDimension,
		termsAndVectors: make([]*util.TermAndVector, dictionarySize),
		word2Vec:        util.NewBytesRefHash(),
	}
}

// newSharedWord2VecModel mirrors the private constructor
// Word2VecModel(int, int, TermAndVector[], BytesRefHash) used by copy(): the
// new model shares the terms, vectors and hash, and its loadedCount starts at
// the field initializer value 0.
func newSharedWord2VecModel(
	dictionarySize, vectorDimension int,
	termsAndVectors []*util.TermAndVector,
	word2Vec *util.BytesRefHash,
) *Word2VecModel {
	return &Word2VecModel{
		dictionarySize:  dictionarySize,
		vectorDimension: vectorDimension,
		termsAndVectors: termsAndVectors,
		word2Vec:        word2Vec,
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
// Mirrors the Word2VecModel.vectorValue(BytesRef) overload.
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

// VectorValue returns the vector at the given ordinal. Mirrors
// Word2VecModel.vectorValue(int).
func (m *Word2VecModel) VectorValue(targetOrd int) ([]float32, error) {
	return m.termsAndVectors[targetOrd].Vector, nil
}

// Dimension returns the vector dimensionality. Mirrors
// Word2VecModel.dimension().
func (m *Word2VecModel) Dimension() int { return m.vectorDimension }

// Size returns the number of vectors in the model. Mirrors
// Word2VecModel.size().
func (m *Word2VecModel) Size() int { return m.dictionarySize }

// Copy mirrors Word2VecModel.copy(): a new model built by the private
// constructor over the same terms, vectors and hash.
func (m *Word2VecModel) Copy() (spi.KnnVectorValues, error) {
	return newSharedWord2VecModel(m.dictionarySize, m.vectorDimension, m.termsAndVectors, m.word2Vec), nil
}

// CopyFloatVectorValues is the covariant FloatVectorValues view of [Copy].
func (m *Word2VecModel) CopyFloatVectorValues() (spi.FloatVectorValues, error) {
	return newSharedWord2VecModel(m.dictionarySize, m.vectorDimension, m.termsAndVectors, m.word2Vec), nil
}

// OrdToDoc carries the KnnVectorValues.ordToDoc default, which returns ord.
func (m *Word2VecModel) OrdToDoc(ord int) int { return ord }

// Prefetch carries the KnnVectorValues.prefetch default, which does nothing.
func (m *Word2VecModel) Prefetch(ordsToPrefetch []int, numOrds int) error { return nil }

// GetVectorByteLength carries the KnnVectorValues.getVectorByteLength
// default: dimension() * getEncoding().byteSize.
func (m *Word2VecModel) GetVectorByteLength() int {
	return m.Dimension() * index.VectorEncodingByteSize(m.GetEncoding())
}

// GetEncoding carries the FloatVectorValues.getEncoding override, FLOAT32.
func (m *Word2VecModel) GetEncoding() index.VectorEncoding {
	return index.VectorEncodingFloat32
}

// GetAcceptOrds carries the KnnVectorValues.getAcceptOrds default.
func (m *Word2VecModel) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return spi.DefaultGetAcceptOrds(m, acceptDocs)
}

// Iterator carries the KnnVectorValues.iterator default, which Word2VecModel
// does not override and which throws UnsupportedOperationException. The Go
// signature has no error result, so the unchecked exception is a panic.
func (m *Word2VecModel) Iterator() spi.DocIndexIterator {
	panic("UnsupportedOperationException: Word2VecModel does not override KnnVectorValues.iterator()")
}

// Scorer carries the FloatVectorValues.scorer default, which throws
// UnsupportedOperationException.
func (m *Word2VecModel) Scorer(target []float32) (util.VectorScorer, error) {
	return nil, errors.New("UnsupportedOperationException: FloatVectorValues.scorer")
}

// Rescorer carries the FloatVectorValues.rescorer default, which returns
// scorer(target).
func (m *Word2VecModel) Rescorer(target []float32) (util.VectorScorer, error) {
	return m.Scorer(target)
}

// Compile-time interface assertion: Word2VecModel extends FloatVectorValues.
var _ spi.FloatVectorValues = (*Word2VecModel)(nil)
