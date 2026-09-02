// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// NormValuesWriter buffers a pending int64 per doc and hands it to a
// NormsConsumer when the owning segment flushes.
//
// This is the Go port of org.apache.lucene.index.NormValuesWriter from
// Apache Lucene 10.4.0.
//
// Gocene divergences:
//
//   - The Java flush() targets codecs.NormsConsumer.addNormsField with an
//     EmptyNormsProducer anonymous subclass. To avoid an import cycle with
//     the codecs package, Flush accepts a local NormsFieldConsumer callback;
//     a higher-level wiring layer in the codecs package adapts the codec
//     NormsConsumer to this callback.
//   - NumericDocValuesWriter.sortDocValues / NumericDVs /
//     SortingNumericDocValues are not yet ported. When sortMap is non-nil
//     Flush returns an error directing the caller to the unsorted path
//     until the numeric-DV bridge lands (tracked alongside the broader
//     DocValuesWriter port).
//   - The Java BufferedNorms.advance / advanceExact throw
//     UnsupportedOperationException; the Go bufferedNorms returns a
//     dedicated sentinel error from each, preserving the contract without
//     panicking.
type NormValuesWriter struct {
	docsWithField *DocsWithFieldSet
	pending       *packed.PackedLongValuesBuilder
	iwBytesUsed   util.CounterAPI
	bytesUsed     int64
	fieldInfo     *FieldInfo
	lastDocID     int
}

// NewNormValuesWriter constructs a writer for fieldInfo. RAM accounting
// updates are reported to iwBytesUsed.
func NewNormValuesWriter(fieldInfo *FieldInfo, iwBytesUsed util.CounterAPI) (*NormValuesWriter, error) {
	if fieldInfo == nil {
		return nil, errors.New("NormValuesWriter: fieldInfo must not be nil")
	}
	if iwBytesUsed == nil {
		return nil, errors.New("NormValuesWriter: iwBytesUsed must not be nil")
	}
	pending, err := packed.DeltaPackedBuilder(packed.PackedLongValuesDefaultPageSize, packed.Compact)
	if err != nil {
		return nil, fmt.Errorf("NormValuesWriter: build pending: %w", err)
	}
	w := &NormValuesWriter{
		docsWithField: NewDocsWithFieldSet(),
		pending:       pending,
		iwBytesUsed:   iwBytesUsed,
		fieldInfo:     fieldInfo,
		lastDocID:     -1,
	}
	w.bytesUsed = w.currentBytesUsed()
	w.iwBytesUsed.AddAndGet(w.bytesUsed)
	return w, nil
}

// AddValue appends a single norm value for docID. docID must be strictly
// greater than any previously seen docID for this field.
func (w *NormValuesWriter) AddValue(docID int, value int64) error {
	if docID <= w.lastDocID {
		return fmt.Errorf(
			"Norm for %q appears more than once in this document (only one value is allowed per field)",
			w.fieldInfo.Name(),
		)
	}
	if err := w.pending.Add(value); err != nil {
		return fmt.Errorf("NormValuesWriter.AddValue: %w", err)
	}
	if err := w.docsWithField.Add(docID); err != nil {
		return fmt.Errorf("NormValuesWriter.AddValue: %w", err)
	}
	w.updateBytesUsed()
	w.lastDocID = docID
	return nil
}

// Finish is the segment-side completion hook. It exists for parity with the
// Java DocValuesWriter contract; the norm writer has no per-segment state to
// finalise here.
func (w *NormValuesWriter) Finish(maxDoc int) {}

// NormsFieldConsumer is the callback used by Flush to hand a frozen
// NumericDocValues view to the underlying codec consumer.
//
// Gocene divergence: replaces the Java NormsConsumer.addNormsField +
// EmptyNormsProducer override with a function-typed boundary. The wiring
// layer in the codecs package adapts codecs.NormsConsumer to this
// signature.
type NormsFieldConsumer func(field *FieldInfo, values NumericDocValues) error

// Flush hands the buffered norms to consumer. When sortMap is nil the values
// are flushed in indexed order. The sort-aware path is not yet implemented
// (see the package doc for the deferred dependency on NumericDocValuesWriter).
func (w *NormValuesWriter) Flush(state *SegmentWriteState, sortMap SorterDocMap, consumer NormsFieldConsumer) error {
	if consumer == nil {
		return errors.New("NormValuesWriter.Flush: consumer must not be nil")
	}
	values := w.pending.Build()
	if sortMap != nil {
		return errors.New(
			"NormValuesWriter.Flush: sort-aware path requires NumericDocValuesWriter.sortDocValues, " +
				"which is not yet ported",
		)
	}
	return consumer(w.fieldInfo, newBufferedNorms(values, w.docsWithFieldDocs()))
}

// currentBytesUsed returns the best-effort RAM size of the in-memory state.
// PackedLongValuesBuilder exposes a Size() (count) helper; we approximate
// bytes-used as count*8 to mirror the Java updateBytesUsed shape. The
// DocsWithFieldSet contribution is approximated from its bitset + counters.
func (w *NormValuesWriter) currentBytesUsed() int64 {
	var pendingBytes int64
	if w.pending != nil {
		pendingBytes = w.pending.Size() * 8
	}
	var dwfBytes int64
	if w.docsWithField != nil {
		// 8 bytes per bitset word + a small fixed header (cardinality,
		// lastDocID, dense flag) — matches the Java best-effort shape.
		dwfBytes = int64(len(w.docsWithField.bits)*8) + 24
	}
	return pendingBytes + dwfBytes
}

func (w *NormValuesWriter) updateBytesUsed() {
	newBytesUsed := w.currentBytesUsed()
	w.iwBytesUsed.AddAndGet(newBytesUsed - w.bytesUsed)
	w.bytesUsed = newBytesUsed
}

// docsWithFieldDocs returns the docIDs in the order they were added. The
// existing DocsWithFieldSet exposes no iterator; we synthesise a slice from
// either its dense prefix or its sparse bitset. Mirrors the helper of the
// same name in SortedSetDocValuesWriter.
func (w *NormValuesWriter) docsWithFieldDocs() []int {
	d := w.docsWithField
	docs := make([]int, 0, d.Cardinality())
	if d.bits == nil {
		for i := 0; i < d.Cardinality(); i++ {
			docs = append(docs, i)
		}
		return docs
	}
	for w64, word := range d.bits {
		for word != 0 {
			bit := word & -word
			docs = append(docs, w64*64+trailingZeros64(uint64(bit)))
			word ^= bit
		}
	}
	return docs
}

// ============================================================================
// bufferedNorms — in-memory NumericDocValues view over the writer's pending
// state. Mirrors NormValuesWriter.BufferedNorms.
// ============================================================================

// errBufferedNormsAdvance signals that random-access advance() is not
// supported by the buffered norms view, mirroring
// UnsupportedOperationException in Lucene.
var errBufferedNormsAdvance = errors.New("bufferedNorms: Advance/AdvanceExact is not supported")

type bufferedNorms struct {
	iter  *packed.PackedLongValuesIterator
	docs  []int
	idx   int
	docID int
	value int64
}

func newBufferedNorms(values *packed.PackedLongValues, docs []int) *bufferedNorms {
	return &bufferedNorms{
		iter:  values.Iterator(),
		docs:  docs,
		docID: -1,
	}
}

// DocID returns the current document, or -1 before the first NextDoc.
func (b *bufferedNorms) DocID() int { return b.docID }

// NextDoc advances to the next document that has a norm.
func (b *bufferedNorms) NextDoc() (int, error) {
	if b.idx >= len(b.docs) {
		b.docID = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	b.docID = b.docs[b.idx]
	b.idx++
	b.value = b.iter.Next()
	return b.docID, nil
}

// Advance is unsupported, matching Lucene's BufferedNorms.advance.
func (b *bufferedNorms) Advance(int) (int, error) {
	return 0, errBufferedNormsAdvance
}

// AdvanceExact is unsupported, matching the buffered writer view; T4709
// shim to satisfy NumericDocValues.
func (b *bufferedNorms) AdvanceExact(int) (bool, error) {
	return false, errBufferedNormsAdvance
}

// LongValue returns the value bound to the current cursor position.
// Mirrors org.apache.lucene.index.NumericDocValues#longValue.
func (b *bufferedNorms) LongValue() (int64, error) {
	return b.value, nil
}

// Cost returns the number of value-bearing documents in the buffered
// stream.
func (b *bufferedNorms) Cost() int64 { return int64(len(b.docs)) }
