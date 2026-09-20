// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

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
		length:           a.length,
		numOverlap:       a.numOverlap,
		uniqueTermCount:  len(a.uniqueTerms),
		maxTermFrequency: a.maxTermFreq,
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
		// NormsConsumer.addNormsField is a pull API: the consumer may ask the
		// producer for the NumericDocValues more than once, so the producer
		// hands out a fresh cursor on every call. Mirrors the anonymous
		// NormsProducer built by NormValuesWriter.flush
		// (NormValuesWriter.java:88-106).
		producer := &bufferedNormsProducer{
			fieldInfo: nf.fieldInfo,
			docIDs:    nf.buf.docIDs,
			values:    nf.buf.values,
		}
		if err := consumer.AddNormsField(nf.fieldInfo, producer); err != nil {
			return fmt.Errorf("norms AddNormsField %q: %w", nf.fieldInfo.Name(), err)
		}
	}
	return nil
}

// bufferedNormsProducer is the NormsProducer the flush hands to the codec
// NormsConsumer. Mirrors the anonymous NormsProducer of
// NormValuesWriter.flush: getNorms rejects a FieldInfo other than the one
// being flushed, checkIntegrity and close are no-ops.
type bufferedNormsProducer struct {
	fieldInfo *FieldInfo
	docIDs    []int
	values    []int64
}

// GetNorms returns a fresh cursor over the buffered values. Mirrors
// NormValuesWriter.flush's getNorms(FieldInfo), which raises
// IllegalArgumentException("wrong fieldInfo") for any other field.
func (p *bufferedNormsProducer) GetNorms(field *FieldInfo) (NumericDocValues, error) {
	if field != p.fieldInfo {
		return nil, errors.New("wrong fieldInfo")
	}
	return &bufferedNormsValues{docIDs: p.docIDs, values: p.values, pos: -1, doc: -1}, nil
}

func (p *bufferedNormsProducer) CheckIntegrity() error               { return nil }
func (p *bufferedNormsProducer) GetMergeInstance() spi.NormsProducer { return p }
func (p *bufferedNormsProducer) Close() error                        { return nil }

// bufferedNormsValues replays a field's buffered per-document norm values.
// docIDs is strictly increasing. Mirrors NormValuesWriter.BufferedNorms, whose
// advance / advanceExact throw UnsupportedOperationException.
type bufferedNormsValues struct {
	docIDs []int
	values []int64
	pos    int
	doc    int
}

func (it *bufferedNormsValues) DocID() int { return it.doc }

func (it *bufferedNormsValues) NextDoc() (int, error) {
	it.pos++
	if it.pos >= len(it.docIDs) {
		it.doc = NO_MORE_DOCS
		return it.doc, nil
	}
	it.doc = it.docIDs[it.pos]
	return it.doc, nil
}

func (it *bufferedNormsValues) Advance(int) (int, error) {
	return 0, errBufferedNormsAdvance
}

func (it *bufferedNormsValues) AdvanceExact(int) (bool, error) {
	return false, errBufferedNormsAdvance
}

func (it *bufferedNormsValues) LongValue() (int64, error) {
	return it.values[it.pos], nil
}

// Cost returns the number of value-bearing documents in the buffered stream,
// mirroring BufferedNorms.cost().
func (it *bufferedNormsValues) Cost() int64 { return int64(len(it.docIDs)) }

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int), which
// NormValuesWriter.BufferedNorms does not override.
func (it *bufferedNormsValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() —
// docID() + 1 — which NormValuesWriter.BufferedNorms does not override.
func (it *bufferedNormsValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(it)
}
