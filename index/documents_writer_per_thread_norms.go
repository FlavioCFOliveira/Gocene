// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FieldInvertState captures the inversion counters for a field in a document.
// This is the Go port of Lucene's org.apache.lucene.index.FieldInvertState.
type FieldInvertState struct {
	// Length is the total number of terms in this field.
	Length int

	// NumOverlap is the number of terms whose position increment is zero.
	NumOverlap int

	// UniqueTermCount is the number of distinct terms encountered in this
	// field.
	UniqueTermCount int

	// MaxTermFrequency is the highest term frequency of any term in this
	// field. A field holding "the quick brown fox jumps over the lazy dog" has
	// a value of 2, because "the" occurs twice.
	MaxTermFrequency int

	// indexCreatedVersionMajor is the major version the index was created
	// with, or 6 when it predates 7.0.
	indexCreatedVersionMajor int

	// name is the field's name.
	name string

	// indexOptions records what the field indexes.
	indexOptions IndexOptions

	// position is the last processed term position.
	position int

	// offset is the end offset of the last processed term.
	offset int

	// lastStartOffset and lastPosition are carried across field instances, so
	// a multi-valued field keeps advancing rather than restarting.
	lastStartOffset int
	lastPosition    int
}

// NewFieldInvertState creates the inversion state for the named field.
// Mirrors FieldInvertState(int, String, IndexOptions).
func NewFieldInvertState(indexCreatedVersionMajor int, name string, indexOptions IndexOptions) *FieldInvertState {
	return &FieldInvertState{
		indexCreatedVersionMajor: indexCreatedVersionMajor,
		name:                     name,
		indexOptions:             indexOptions,
	}
}

// Reset clears the per-document counters, keeping the field identity.
// Mirrors FieldInvertState.reset.
func (s *FieldInvertState) Reset() {
	s.position = -1
	s.Length = 0
	s.NumOverlap = 0
	s.offset = 0
	s.MaxTermFrequency = 0
	s.UniqueTermCount = 0
	s.lastStartOffset = 0
	s.lastPosition = 0
}

// Position returns the last processed term position. Mirrors getPosition.
func (s *FieldInvertState) Position() int { return s.position }

// SetPosition sets the last processed term position.
func (s *FieldInvertState) SetPosition(position int) { s.position = position }

// GetLength returns the total number of terms in this field. Mirrors
// getLength.
func (s *FieldInvertState) GetLength() int { return s.Length }

// SetLength sets the total number of terms in this field. Mirrors setLength.
func (s *FieldInvertState) SetLength(length int) { s.Length = length }

// GetNumOverlap returns the number of terms with a zero position increment.
// Mirrors getNumOverlap.
func (s *FieldInvertState) GetNumOverlap() int { return s.NumOverlap }

// SetNumOverlap sets the number of terms with a zero position increment.
// Mirrors setNumOverlap.
func (s *FieldInvertState) SetNumOverlap(numOverlap int) { s.NumOverlap = numOverlap }

// Offset returns the end offset of the last processed term. Mirrors getOffset.
func (s *FieldInvertState) Offset() int { return s.offset }

// SetOffset sets the end offset of the last processed term.
func (s *FieldInvertState) SetOffset(offset int) { s.offset = offset }

// GetMaxTermFrequency returns the highest term frequency in this field.
// Mirrors getMaxTermFrequency.
func (s *FieldInvertState) GetMaxTermFrequency() int { return s.MaxTermFrequency }

// SetMaxTermFrequency sets the highest term frequency in this field.
func (s *FieldInvertState) SetMaxTermFrequency(maxTermFrequency int) {
	s.MaxTermFrequency = maxTermFrequency
}

// GetUniqueTermCount returns the number of distinct terms in this field.
// Mirrors getUniqueTermCount.
func (s *FieldInvertState) GetUniqueTermCount() int { return s.UniqueTermCount }

// SetUniqueTermCount sets the number of distinct terms in this field.
func (s *FieldInvertState) SetUniqueTermCount(uniqueTermCount int) {
	s.UniqueTermCount = uniqueTermCount
}

// LastStartOffset returns the start offset carried over from the previous
// value of a multi-valued field.
func (s *FieldInvertState) LastStartOffset() int { return s.lastStartOffset }

// SetLastStartOffset sets the start offset carried over from the previous
// value of a multi-valued field.
func (s *FieldInvertState) SetLastStartOffset(lastStartOffset int) {
	s.lastStartOffset = lastStartOffset
}

// LastPosition returns the position carried over from the previous value of a
// multi-valued field.
func (s *FieldInvertState) LastPosition() int { return s.lastPosition }

// SetLastPosition sets the position carried over from the previous value of a
// multi-valued field.
func (s *FieldInvertState) SetLastPosition(lastPosition int) { s.lastPosition = lastPosition }

// Name returns the field's name. Mirrors getName.
func (s *FieldInvertState) Name() string { return s.name }

// IndexOptions returns what the field indexes.
func (s *FieldInvertState) IndexOptions() IndexOptions { return s.indexOptions }

// IndexCreatedVersionMajor returns the major version the index was created
// with. Mirrors getIndexCreatedVersionMajor.
func (s *FieldInvertState) IndexCreatedVersionMajor() int { return s.indexCreatedVersionMajor }

// NormsBuffer holds the per-document norm value for a single field, in
// document order. It is the live-path counterpart of Lucene's NormValuesWriter
// accumulator. docIDs is strictly increasing because ProcessDocument assigns
// monotonically increasing docIDs and a field's norm is recorded at most once
// per document (in finalizeNorms, after the field loop completes).
type NormsBuffer struct {
	docIDs []int
	values []int64
}

// normsAccumulator holds the in-progress field-inversion counters for the
// document currently being processed. It mirrors the subset of
// org.apache.lucene.index.FieldInvertState that Similarity.computeNorm reads:
// the running field length (sum of per-token term frequencies), the overlap
// count (tokens with a zero position increment) and the set of distinct terms
// (consulted only for DOCS-only fields). A fresh accumulator is created per
// (field, document) and discarded once finalizeNorms consumes it.
type normsAccumulator struct {
	indexOptions IndexOptions
	length       int
	numOverlap   int
	maxTermFreq  int
	uniqueTerms  map[string]struct{}
}

// newNormsAccumulator returns an empty accumulator for a field with the given
// index options.
func newNormsAccumulator(indexOptions IndexOptions) *normsAccumulator {
	return &normsAccumulator{
		indexOptions: indexOptions,
		uniqueTerms:  make(map[string]struct{}),
	}
}

// addToken folds one indexed token into the inversion counters. termFreq is the
// token's term frequency (the custom term frequency when positive, else the
// default of 1, matching addTermWithFreq); posIncr is its position increment.
// Mirrors IndexingChain.invert: invertState.length += termFreq, and
// invertState.numOverlap++ when posIncr == 0.
func (a *normsAccumulator) addToken(term string, termFreq, posIncr int) error {
	tf := termFreq
	if tf <= 0 {
		tf = 1
	}
	if a.length > math.MaxInt32-tf {
		return fmt.Errorf("term frequency overflow")
	}
	a.length += tf
	if tf > a.maxTermFreq {
		a.maxTermFreq = tf
	}
	if posIncr == 0 {
		a.numOverlap++
	}
	a.uniqueTerms[term] = struct{}{}
	return nil
}

// ToFieldInvertState returns a snapshot of the current inversion counters.
func (a *normsAccumulator) ToFieldInvertState() FieldInvertState {
	return FieldInvertState{
		Length:           a.length,
		NumOverlap:       a.numOverlap,
		UniqueTermCount:  len(a.uniqueTerms),
		MaxTermFrequency: a.maxTermFreq,
	}
}

// computeNorm returns the per-document norm value for the accumulated field
// state, byte-identical to org.apache.lucene.search.similarities.Similarity
// .computeNorm(FieldInvertState) under the default similarity
// (discountOverlaps=true):
//
//	numTerms = uniqueTermCount                 (IndexOptions == DOCS)
//	numTerms = length - numOverlap             (otherwise, discountOverlaps)
//	norm     = SmallFloat.intToByte4(numTerms)
//
// The result occupies the low 8 bits (the encoded byte) and is carried as
// int64 to match the NumericDocValues surface the norms codec consumes. A
// non-empty field always yields a non-zero norm because intToByte4(n) is
// non-zero for every n >= 1 (it is the identity for the first NUM_FREE_VALUES
// values).
//
// Gocene computes the value directly with util.IntToByte4 rather than calling
// into package search (which imports index and so cannot be imported here);
// the transform is identical and is covered by search's similarity tests
// (DefaultComputeNormFromInvertState) and util's small-float tests.
func (a *normsAccumulator) computeNorm() (int64, error) {
	var numTerms int
	switch {
	case a.indexOptions == IndexOptionsDocs:
		numTerms = len(a.uniqueTerms)
	default:
		numTerms = a.length - a.numOverlap
	}
	if numTerms < 0 {
		numTerms = 0
	}
	b, err := util.IntToByte4(numTerms)
	if err != nil {
		return 0, err
	}
	return int64(b), nil
}

// normsAccumulatorFor returns the accumulator for fieldName within the current
// document, lazily creating it on first sight, or nil when the field omits
// norms (omitNorms=true) — in which case no norm is computed or buffered,
// matching Lucene which never calls similarity.computeNorm for such fields.
func (dwpt *DocumentsWriterPerThread) normsAccumulatorFor(fieldName string, fieldInfo *FieldInfo) *normsAccumulator {
	if fieldInfo == nil || fieldInfo.OmitNorms() {
		return nil
	}
	if acc, ok := dwpt.normsAcc[fieldName]; ok {
		return acc
	}
	acc := newNormsAccumulator(fieldInfo.IndexOptions())
	dwpt.normsAcc[fieldName] = acc
	return acc
}

// finalizeNorms buffers one norm value per field that appeared in the document
// with norms enabled, then clears the per-document accumulators. It is the
// live-path counterpart of IndexingChain.PerField.finish, which computes the
// norm from the field's FieldInvertState and calls norms.addValue(docID, norm).
//
// A field with norms that produced no indexed tokens (length == 0) still has
// an accumulator (it was created when the field was first inverted) and yields
// computeNorm()==intToByte4(0)==0; Lucene buffers 0 in that case too, so the
// reader observes a value-less norm for the empty field. Fields with norms
// that did not appear in this document simply have no accumulator and are not
// buffered, matching the sparse NumericDocValues the norms codec writes.
//
// Must be called with dwpt.mu held (write lock); ProcessDocument holds it.
func (dwpt *DocumentsWriterPerThread) finalizeNorms(docID int) error {
	if len(dwpt.normsAcc) == 0 {
		return nil
	}
	for fieldName, acc := range dwpt.normsAcc {
		norm, err := acc.computeNorm()
		if err != nil {
			return fmt.Errorf("index: compute norm for field %q doc %d: %w", fieldName, docID, err)
		}
		buf, ok := dwpt.norms[fieldName]
		if !ok {
			buf = &NormsBuffer{}
			dwpt.norms[fieldName] = buf
		}
		buf.docIDs = append(buf.docIDs, docID)
		buf.values = append(buf.values, norm)
	}
	// Reset the per-document accumulators for the next document.
	dwpt.normsAcc = make(map[string]*normsAccumulator)
	return nil
}

// flushNorms writes the buffered per-document norm values for every field with
// norms to the codec's NormsConsumer, serialising the per-segment .nvd / .nvm
// files.
//
// It mirrors the writeNorms step of Lucene's IndexingChain.flush: a single
// NormsConsumer is opened for the segment and AddNormsField is invoked once per
// norms field, in field-number order, replaying the buffered per-document
// values (a NumericDocValues stream) in document order.
//
// The FieldInfo objects are taken from state.FieldInfos — the same instances
// flushFieldInfos serialises to the .fnm — so the FieldInfo "indexed with
// norms" bit reaches disk and FieldInfos.HasNorms() reports true on reopen,
// lighting up the codec NormsProducer.
//
// No-op when the codec has no NormsFormat or no norms fields were buffered.
func (dwpt *DocumentsWriterPerThread) flushNorms(codec Codec, state *SegmentWriteState) error {
	if codec == nil || codec.NormsFormat() == nil {
		return nil
	}
	if len(dwpt.norms) == 0 {
		return nil
	}

	// Collect the norms fields from the on-disk FieldInfos, preserving
	// field-number order so the AddNormsField sequence (and thus the per-field
	// meta records) is deterministic across runs — matching the field-ordered
	// iteration Lucene's writeNorms performs.
	type normField struct {
		fieldInfo *FieldInfo
		buf       *NormsBuffer
	}
	var normFields []normField
	it := state.FieldInfos.Iterator()
	for {
		fi := it.Next()
		if fi == nil {
			break
		}
		if !fi.HasNorms() {
			continue
		}
		buf, ok := dwpt.norms[fi.Name()]
		if !ok {
			continue
		}
		normFields = append(normFields, normField{fieldInfo: fi, buf: buf})
	}
	if len(normFields) == 0 {
		return nil
	}

	consumer, err := codec.NormsFormat().NormsConsumer(state)
	if err != nil {
		return fmt.Errorf("norms NormsConsumer: %w", err)
	}
	defer consumer.Close()

	for _, nf := range normFields {
		iter := &bufferedNormsIter{docIDs: nf.buf.docIDs, values: nf.buf.values, pos: -1}
		if err := consumer.AddNormsField(nf.fieldInfo, iter); err != nil {
			return fmt.Errorf("norms AddNormsField %q: %w", nf.fieldInfo.Name(), err)
		}
	}
	return nil
}

// bufferedNormsIter replays a field's buffered per-document norm values. It
// satisfies the NormsIterator (spi.NormsIterator) contract the codec's
// NormsConsumer.AddNormsField consumes: Next advances the single-pass cursor,
// DocID / LongValue read the current entry. docIDs is strictly increasing.
type bufferedNormsIter struct {
	docIDs []int
	values []int64
	pos    int
}

func (it *bufferedNormsIter) Next() bool {
	it.pos++
	return it.pos < len(it.docIDs)
}
func (it *bufferedNormsIter) DocID() int       { return it.docIDs[it.pos] }
func (it *bufferedNormsIter) LongValue() int64 { return it.values[it.pos] }
