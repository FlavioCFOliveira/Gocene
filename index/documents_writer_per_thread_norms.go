// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// This file wires the NORMS write path for the live DocumentsWriterPerThread
// flush pipeline. It mirrors three Lucene collaborators that work together in
// org.apache.lucene.index.IndexingChain (Lucene 10.4.0):
//
//   - FieldInvertState — the per-(field, document) inversion counters
//     (length / numOverlap / uniqueTermCount) accumulated during invert().
//     Gocene tracks them in normsAccumulator, fed token-by-token from
//     indexFieldWithValue.
//   - PerField.finish — computes Similarity.computeNorm(FieldInvertState) once
//     per document and buffers it via NormValuesWriter.addValue(docID, norm).
//     Gocene does this in finalizeNorms, buffering into NormsBuffer.
//   - writeNorms — opens codec.normsFormat().normsConsumer(state) and replays
//     every field's buffered norms through NormsConsumer.addNormsField. Gocene
//     does this in flushNorms.
//
// The norm VALUE is computed exactly as Lucene's default similarity
// (BM25Similarity, discountOverlaps=true): SmallFloat.intToByte4(numTerms),
// where numTerms is the unique-term count for DOCS-only fields and otherwise
// the field length minus the overlap count. See computeNorm below.

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
func (a *normsAccumulator) addToken(term string, termFreq, posIncr int) {
	tf := termFreq
	if tf <= 0 {
		tf = 1
	}
	a.length += tf
	if posIncr == 0 {
		a.numOverlap++
	}
	a.uniqueTerms[term] = struct{}{}
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
