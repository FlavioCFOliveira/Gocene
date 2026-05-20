// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermVectorsAttributeProvider exposes the analysis-time attributes that
// TermVectorsConsumerPerField pulls off FieldInvertState in Lucene. Each
// getter returns the current per-token value, mirroring the Java attribute
// calls in the reference implementation:
//
//   - StartOffset / EndOffset: OffsetAttribute.startOffset() / endOffset();
//     consumed only when the field stores term-vector offsets.
//   - Payload: PayloadAttribute.getPayload(); consumed only when the field
//     stores term-vector payloads. A nil return means "no payload".
//   - TermFrequency: TermFrequencyAttribute.getTermFrequency(); returns 1
//     when the field surfaces no custom attribute, mirroring the
//     "termFreqAtt == null" branch in Lucene's getTermFreq.
//
// All getters are invoked synchronously while a token is being inverted; the
// provider must reflect the values of the token currently held by the
// analyzer. Getters for streams the field does not encode may be left nil.
//
// This mirrors the FreqProxAttributeProvider design (sprint decision b):
// Gocene's FieldInvertState does not yet carry the analysis attribute
// objects Lucene caches on it, so the per-field writer pulls per-token data
// through plain function getters. The pipeline glue (DocumentsWriterPerThread)
// is responsible for bridging the active TokenStream to the provider.
type TermVectorsAttributeProvider struct {
	// StartOffset returns the current token's start offset.
	// Must be non-nil when the field stores term-vector offsets.
	StartOffset func() int
	// EndOffset returns the current token's end offset.
	// Must be non-nil when the field stores term-vector offsets.
	EndOffset func() int
	// Payload returns the current token's payload, or nil when absent.
	// Must be non-nil when the field stores term-vector payloads.
	Payload func() *util.BytesRef
	// TermFrequency returns the current token's term frequency.
	// May be nil; the writer treats nil as "always 1".
	TermFrequency func() int
}

// TermVectorsPostingsArray extends ParallelPostingsArray with the per-term
// counters TermVectorsConsumerPerField needs: term frequencies, last offset
// and last position. It is the Go port of the static inner class
// org.apache.lucene.index.TermVectorsConsumerPerField.TermVectorsPostingsArray.
type TermVectorsPostingsArray struct {
	*ParallelPostingsArray

	// Freqs records how many times the term occurred in the current doc.
	Freqs []int
	// LastOffsets records the last end-offset seen for the term.
	LastOffsets []int
	// LastPositions records the last position the term occurred at.
	LastPositions []int
}

// NewTermVectorsPostingsArray allocates a TermVectorsPostingsArray sized for
// the given number of term slots. All side arrays are zero-initialised,
// mirroring the Java constructor (new int[size] for each array).
func NewTermVectorsPostingsArray(size int) *TermVectorsPostingsArray {
	return &TermVectorsPostingsArray{
		ParallelPostingsArray: NewParallelPostingsArray(size),
		Freqs:                 make([]int, size),
		LastOffsets:           make([]int, size),
		LastPositions:         make([]int, size),
	}
}

// BytesPerPosting returns the byte cost of a single posting slot, including
// the three int side arrays (freqs, lastOffsets, lastPositions). Mirrors
// Lucene's bytesPerPosting() override (super + 3 * Integer.BYTES).
func (a *TermVectorsPostingsArray) BytesPerPosting() int {
	return a.ParallelPostingsArray.BytesPerPosting() + 3*4
}

// CopyTo copies the first numToCopy slots into dst. Mirrors Lucene's copyTo;
// the Java original copies the full size for the side arrays, so the port
// does the same to preserve byte-for-byte parity of the grown array.
func (a *TermVectorsPostingsArray) CopyTo(dst *TermVectorsPostingsArray, numToCopy int) {
	a.ParallelPostingsArray.CopyTo(dst.ParallelPostingsArray, numToCopy)
	size := a.ParallelPostingsArray.Size
	copy(dst.Freqs[:size], a.Freqs[:size])
	copy(dst.LastOffsets[:size], a.LastOffsets[:size])
	copy(dst.LastPositions[:size], a.LastPositions[:size])
}

// TermVectorsConsumerPerField writes the per-document term vectors for a
// single field. It is the Go port of
// org.apache.lucene.index.TermVectorsConsumerPerField from Apache Lucene
// 10.4.0.
//
// The Java class is final and extends TermsHashPerField, overriding the
// abstract hooks (newTerm, addTerm, newPostingsArray, createPostingsArray)
// and the non-abstract finish()/start(); it adds finishDocument() and the
// writeProx helper. Gocene's TermsHashPerField expresses the hook contract as
// function-valued fields; this type wires the four implementations through
// them at construction time and overrides Finish / Start by shadowing the
// embedded methods.
//
// Divergences from Lucene 10.4.0:
//
//   - Self-contained port (sprint decision b): instead of depending on the
//     analysis token-attribute interfaces (OffsetAttribute, PayloadAttribute,
//     TermFrequencyAttribute) cached on FieldInvertState, the writer pulls
//     per-token data through a TermVectorsAttributeProvider whose fields are
//     plain function getters. Lucene reads these attributes off fieldState in
//     start(); the Gocene FieldInvertState does not yet carry them.
//
//   - FieldInfo.SetStoreTermVectors is the Gocene equivalent of Lucene's
//     package-private FieldInfo.setStoreTermVectors; it bypasses the frozen
//     contract and is invoked from FinishDocument exactly as Java does.
//
//   - Lucene's finishDocument hands the raw position / offset byte slices to
//     TermVectorsWriter.addProx(numProx, ByteSliceReader, ByteSliceReader),
//     which re-parses the VInt stream. Gocene's index.TermVectorsWriter
//     exposes the higher-level AddPosition(position, startOffset, endOffset,
//     payload) instead. FinishDocument therefore decodes the position and
//     offset streams itself (the inverse of writeProx) and replays one
//     AddPosition call per occurrence. The encoded byte layout the writer
//     produces is unchanged; only the consumer-to-codec hand-off differs.
//
//   - Lucene's TermsHashPerField wraps termBytePool in a BytesRefBlockPool
//     to resolve the flush term in finishDocument. Gocene's BytesRefHash
//     already owns the equivalent BytesRefBlockPool; the port resolves the
//     term bytes via the base TermsHashPerField.BytesHashGet helper rather
//     than constructing a second pool wrapper.
//
//   - getTermFreq throws IllegalArgumentException in Java. The newTerm /
//     addTerm hooks return errors, but writeProx (their callee) has no error
//     channel, so the invalid-state condition surfaces as a panic, matching
//     the FreqProx port's documented treatment of the same Java exception.
type TermVectorsConsumerPerField struct {
	*TermsHashPerField

	postingsArray *TermVectorsPostingsArray

	// lastCreated holds the TermVectorsPostingsArray most recently returned
	// by createPostingsArray, so newPostingsArray can recover the typed
	// wrapper after the base machinery allocates or grows the array. Mirrors
	// the FreqProx port's bridge field; see its newPostingsArray comment.
	lastCreated *TermVectorsPostingsArray

	termsWriter *TermVectorsConsumer
	fieldState  *FieldInvertState
	fieldInfo   *FieldInfo
	attrs       TermVectorsAttributeProvider

	doVectors         bool
	doVectorPositions bool
	doVectorOffsets   bool
	doVectorPayloads  bool

	// hasPayloads is set to true the first time a non-empty payload is seen
	// for this field in the current document. It feeds the hasPayloads
	// argument of TermVectorsWriter.StartField.
	hasPayloads bool
}

// Compile-time guarantee that *TermVectorsConsumerPerField satisfies the
// narrow handle interface the parent TermVectorsConsumer manipulates.
var _ TermVectorsPerFieldHandle = (*TermVectorsConsumerPerField)(nil)

// NewTermVectorsConsumerPerField wires a term-vectors per-field writer over
// the supplied FieldInvertState, parent TermVectorsConsumer and FieldInfo.
//
// Mirrors the Java constructor TermVectorsConsumerPerField(FieldInvertState,
// TermVectorsConsumer, FieldInfo): the base TermsHashPerField is created with
// streamCount 2 (one stream for positions, one for offsets) and a nil
// next-in-chain handler, exactly as Lucene passes.
//
// invertState must be the FieldInvertState associated with fieldInfo. attrs
// holds the per-token getters; getters for streams the field does not encode
// may be left nil.
//
// The returned writer owns its embedded TermsHashPerField; the embedded
// type's exported Add method is the per-token entry point, and Start / Finish
// / FinishDocument are the document-lifecycle entry points.
func NewTermVectorsConsumerPerField(
	invertState *FieldInvertState,
	termsHash *TermVectorsConsumer,
	fieldInfo *FieldInfo,
	attrs TermVectorsAttributeProvider,
) (*TermVectorsConsumerPerField, error) {
	if invertState == nil {
		return nil, fmt.Errorf("TermVectorsConsumerPerField: invertState must not be nil")
	}
	if termsHash == nil {
		return nil, fmt.Errorf("TermVectorsConsumerPerField: termsHash must not be nil")
	}
	if fieldInfo == nil {
		return nil, fmt.Errorf("TermVectorsConsumerPerField: fieldInfo must not be nil")
	}

	indexOpts := fieldInfo.IndexOptions()
	if indexOpts == IndexOptionsNone {
		return nil, fmt.Errorf("TermVectorsConsumerPerField: field %q has IndexOptionsNone", fieldInfo.Name())
	}

	w := &TermVectorsConsumerPerField{
		termsWriter: termsHash,
		fieldState:  invertState,
		fieldInfo:   fieldInfo,
		attrs:       attrs,
	}

	hooks := TermsHashPerFieldHooks{
		NewTerm:             w.newTerm,
		AddTerm:             w.addTerm,
		NewPostingsArray:    w.newPostingsArray,
		CreatePostingsArray: w.createPostingsArray,
	}

	// Lucene passes streamCount 2: stream 0 carries positions+payloads,
	// stream 1 carries offsets. termBytePool is shared with the parent.
	base, err := NewTermsHashPerField(
		2,
		termsHash.intPool,
		termsHash.bytePool,
		termsHash.bytePool,
		termsHash.bytesUsed,
		nil,
		fieldInfo.Name(),
		indexOpts,
		hooks,
	)
	if err != nil {
		return nil, err
	}
	w.TermsHashPerField = base
	// BytesRefHash.Init may have run the NewPostingsArray hook before
	// w.TermsHashPerField was assigned; mirror lastCreated onto postingsArray
	// so the first Add can read it without re-triggering the hook. Same
	// pattern as the FreqProx per-field constructor.
	w.postingsArray = w.lastCreated
	return w, nil
}

// Finish is called once per field per document when term vectors are enabled.
// When the field saw at least one term, it registers the writer with the
// parent consumer's flush list. Mirrors Lucene's finish() override.
//
// The base TermsHashPerField.Finish is a chain pass-through; this type is
// always the tail of its (single-element) chain, so the override fully
// replaces it rather than delegating.
func (w *TermVectorsConsumerPerField) Finish() error {
	if !w.doVectors || w.GetNumTerms() == 0 {
		return nil
	}
	w.termsWriter.AddFieldToFlush(w)
	return nil
}

// CompareName orders two per-field handles by field name, satisfying the
// TermVectorsPerFieldHandle contract the parent consumer's introSort uses.
func (w *TermVectorsConsumerPerField) CompareName(other TermVectorsPerFieldHandle) int {
	o, ok := other.(*TermVectorsConsumerPerField)
	if !ok {
		// Defensive: the parent only ever stores this concrete type. Fall
		// back to an arbitrary-but-stable ordering rather than panicking.
		return 0
	}
	switch {
	case w.fieldInfo.Name() < o.fieldInfo.Name():
		return -1
	case w.fieldInfo.Name() > o.fieldInfo.Name():
		return 1
	default:
		return 0
	}
}

// FinishDocument flushes this field's term vectors for the current document
// into the parent consumer's active TermVectorsWriter. Mirrors Lucene's
// finishDocument().
//
// It sorts the field's terms, opens a term-vector field section, and for each
// term emits its frequency followed by the decoded positions / offsets /
// payloads. After the field is closed the per-field state is reset and the
// owning FieldInfo is marked as carrying term vectors.
//
// Divergence: Lucene replays the raw position / offset byte slices through
// TermVectorsWriter.addProx; Gocene's TermVectorsWriter exposes AddPosition,
// so this method decodes the streams (the inverse of writeProx) and issues
// one AddPosition call per term occurrence. See the type-doc note.
func (w *TermVectorsConsumerPerField) FinishDocument() error {
	if !w.doVectors {
		return nil
	}
	w.doVectors = false

	numPostings := w.GetNumTerms()
	if numPostings < 0 {
		return fmt.Errorf("TermVectorsConsumerPerField: negative numPostings %d", numPostings)
	}

	postings := w.postingsArray
	tv := w.termsWriter.Writer
	if tv == nil {
		return fmt.Errorf("TermVectorsConsumerPerField: parent consumer has no active TermVectorsWriter")
	}

	w.SortTerms()
	termIDs := w.GetSortedTermIDs()

	if err := tv.StartField(w.fieldInfo, numPostings, w.doVectorPositions, w.doVectorOffsets, w.hasPayloads); err != nil {
		return fmt.Errorf("TermVectorsConsumerPerField: start field %q: %w", w.fieldInfo.Name(), err)
	}

	flushTerm := w.termsWriter.FlushTerm

	for j := 0; j < numPostings; j++ {
		termID := termIDs[j]
		freq := postings.Freqs[termID]

		w.fillFlushTerm(flushTerm, termID)
		if err := tv.StartTerm(flushTerm.Bytes[flushTerm.Offset : flushTerm.Offset+flushTerm.Length]); err != nil {
			return fmt.Errorf("TermVectorsConsumerPerField: start term in field %q: %w", w.fieldInfo.Name(), err)
		}

		if w.doVectorPositions || w.doVectorOffsets {
			if err := w.replayProx(tv, termID, freq); err != nil {
				return err
			}
		}

		if err := tv.FinishTerm(); err != nil {
			return fmt.Errorf("TermVectorsConsumerPerField: finish term in field %q: %w", w.fieldInfo.Name(), err)
		}
	}
	if err := tv.FinishField(); err != nil {
		return fmt.Errorf("TermVectorsConsumerPerField: finish field %q: %w", w.fieldInfo.Name(), err)
	}

	w.Reset()

	w.fieldInfo.SetStoreTermVectors()
	return nil
}

// fillFlushTerm resolves the interned term bytes for termID into ref. Lucene
// uses a dedicated BytesRefBlockPool wrapper here; Gocene's BytesRefHash
// already owns an equivalent pool, so the port resolves through the base
// helper instead (see the type-doc divergence note).
func (w *TermVectorsConsumerPerField) fillFlushTerm(ref *util.BytesRef, termID int) {
	w.bytesHash.Get(termID, ref)
}

// replayProx decodes the position and offset streams written by writeProx for
// a single term and replays them as freq AddPosition calls on tv. It is the
// inverse of writeProx; Lucene avoids this decode by handing the raw slices to
// addProx, an interface Gocene's TermVectorsWriter does not expose.
func (w *TermVectorsConsumerPerField) replayProx(tv TermVectorsWriter, termID, freq int) error {
	var posReader, offReader *ByteSliceReader
	if w.doVectorPositions {
		posReader = w.termsWriter.VectorSliceReaderPos
		if err := w.InitReader(posReader, termID, 0); err != nil {
			return fmt.Errorf("TermVectorsConsumerPerField: init position reader: %w", err)
		}
	}
	if w.doVectorOffsets {
		offReader = w.termsWriter.VectorSliceReaderOff
		if err := w.InitReader(offReader, termID, 1); err != nil {
			return fmt.Errorf("TermVectorsConsumerPerField: init offset reader: %w", err)
		}
	}

	position := 0
	endOffset := 0
	for i := 0; i < freq; i++ {
		pos := -1
		startOffset := -1
		curEndOffset := -1
		var payload []byte

		if posReader != nil {
			code, err := readVIntFrom(posReader)
			if err != nil {
				return fmt.Errorf("TermVectorsConsumerPerField: read position: %w", err)
			}
			position += int(code) >> 1
			pos = position
			if code&1 != 0 {
				payloadLen, err := readVIntFrom(posReader)
				if err != nil {
					return fmt.Errorf("TermVectorsConsumerPerField: read payload length: %w", err)
				}
				payload = make([]byte, payloadLen)
				if err := posReader.ReadBytes(payload); err != nil {
					return fmt.Errorf("TermVectorsConsumerPerField: read payload bytes: %w", err)
				}
			}
		}

		if offReader != nil {
			startDelta, err := readVIntFrom(offReader)
			if err != nil {
				return fmt.Errorf("TermVectorsConsumerPerField: read start offset: %w", err)
			}
			length, err := readVIntFrom(offReader)
			if err != nil {
				return fmt.Errorf("TermVectorsConsumerPerField: read end offset: %w", err)
			}
			startOffset = endOffset + int(startDelta)
			curEndOffset = startOffset + int(length)
			endOffset = curEndOffset
		}

		if err := tv.AddPosition(pos, startOffset, curEndOffset, payload); err != nil {
			return fmt.Errorf("TermVectorsConsumerPerField: add position: %w", err)
		}
	}
	return nil
}

// Start begins a new occurrence of field within the current document. It
// derives the per-field doVectors* flags from the FieldInfo's term-vector
// settings, validates the cross-field consistency rules, and returns whether
// term vectors should be collected for this field. Mirrors Lucene's start()
// override.
//
// Lucene reads the storeTermVector* booleans off field.fieldType(); Gocene's
// FieldInfo already exposes the resolved settings (the four StoreTermVector*
// accessors), so the port reads them there. The boolean combination guards
// reproduce the IllegalArgumentException checks of Lucene's start().
func (w *TermVectorsConsumerPerField) Start(field IndexableField, first bool) bool {
	w.TermsHashPerField.Start(field, first)

	if first {
		if w.GetNumTerms() != 0 {
			// Only reached if a previous doc hit a non-aborting error while
			// writing this field's vectors; clear the stale state.
			w.Reset()
		}
		w.ReinitHash()

		w.hasPayloads = false

		w.doVectors = w.fieldInfo.StoreTermVectors()

		if w.doVectors {
			w.doVectorPositions = w.fieldInfo.StoreTermVectorPositions()
			// Unlike postings, term-vector offsets may be indexed without
			// term-vector positions.
			w.doVectorOffsets = w.fieldInfo.StoreTermVectorOffsets()

			if w.doVectorPositions {
				w.doVectorPayloads = w.fieldInfo.StoreTermVectorPayloads()
			} else {
				w.doVectorPayloads = false
				if w.fieldInfo.StoreTermVectorPayloads() {
					panic(fmt.Sprintf(
						"cannot index term vector payloads without term vector positions (field=%q)",
						w.fieldInfo.Name()))
				}
			}
		} else {
			if w.fieldInfo.StoreTermVectorOffsets() {
				panic(fmt.Sprintf(
					"cannot index term vector offsets when term vectors are not indexed (field=%q)",
					w.fieldInfo.Name()))
			}
			if w.fieldInfo.StoreTermVectorPositions() {
				panic(fmt.Sprintf(
					"cannot index term vector positions when term vectors are not indexed (field=%q)",
					w.fieldInfo.Name()))
			}
			if w.fieldInfo.StoreTermVectorPayloads() {
				panic(fmt.Sprintf(
					"cannot index term vector payloads when term vectors are not indexed (field=%q)",
					w.fieldInfo.Name()))
			}
		}
	}
	// Lucene re-checks that every instance of a field name agrees on its
	// term-vector settings. In Gocene the four doVector* flags are derived
	// from the single shared FieldInfo, which is immutable for the field's
	// term-vector booleans, so the per-instance settings cannot diverge and
	// the four "settings changed" guards are unreachable. They are omitted
	// intentionally; the invariant is enforced upstream by FieldInfo.

	return w.doVectors
}

// writeProx encodes one token occurrence of term termID into the position
// (stream 0) and offset (stream 1) byte slices. Mirrors Lucene's writeProx.
//
// The offset block is written first, then the position block, matching the
// Java order so the encoded byte stream is identical.
func (w *TermVectorsConsumerPerField) writeProx(termID int) {
	postings := w.postingsArray

	if w.doVectorOffsets {
		if w.attrs.StartOffset == nil || w.attrs.EndOffset == nil {
			panic(fmt.Sprintf(
				"TermVectorsConsumerPerField: field %q indexes term vector offsets but provider has no StartOffset/EndOffset",
				w.fieldInfo.Name()))
		}
		startOffset := w.fieldState.Offset() + w.attrs.StartOffset()
		endOffset := w.fieldState.Offset() + w.attrs.EndOffset()

		w.WriteStreamVInt(1, int32(startOffset-postings.LastOffsets[termID]))
		w.WriteStreamVInt(1, int32(endOffset-startOffset))
		postings.LastOffsets[termID] = endOffset
	}

	if w.doVectorPositions {
		var payload *util.BytesRef
		if w.attrs.Payload != nil {
			payload = w.attrs.Payload()
		}

		pos := w.fieldState.Position() - postings.LastPositions[termID]
		if payload != nil && payload.Length > 0 {
			w.WriteStreamVInt(0, int32((pos<<1)|1))
			w.WriteStreamVInt(0, int32(payload.Length))
			w.WriteStreamBytes(0, payload.Bytes, payload.Offset, payload.Length)
			w.hasPayloads = true
		} else {
			w.WriteStreamVInt(0, int32(pos<<1))
		}
		postings.LastPositions[termID] = w.fieldState.Position()
	}
}

// newTerm initialises a freshly observed term's posting slot. Mirrors
// Lucene's newTerm.
func (w *TermVectorsConsumerPerField) newTerm(termID, docID int) error {
	_ = docID // term vectors are per-document; docID is unused, as in Lucene
	postings := w.postingsArray

	postings.Freqs[termID] = w.getTermFreq()
	postings.LastOffsets[termID] = 0
	postings.LastPositions[termID] = 0

	w.writeProx(termID)
	return nil
}

// addTerm updates the posting slot for a previously observed term. Mirrors
// Lucene's addTerm.
func (w *TermVectorsConsumerPerField) addTerm(termID, docID int) error {
	_ = docID // unused, as in Lucene
	postings := w.postingsArray

	postings.Freqs[termID] += w.getTermFreq()

	w.writeProx(termID)
	return nil
}

// getTermFreq reads the current token's term frequency. Mirrors Lucene's
// getTermFreq, including the guards that reject a custom term frequency when
// the field also stores term-vector positions or offsets.
//
// Lucene throws IllegalArgumentException. The newTerm / addTerm hooks return
// errors, but getTermFreq is called from writeProx (their callee), which has
// no error channel; the invalid-state condition therefore surfaces as a
// panic, matching the FreqProx port's treatment of the same Java exception.
func (w *TermVectorsConsumerPerField) getTermFreq() int {
	if w.attrs.TermFrequency == nil {
		return 1
	}
	freq := w.attrs.TermFrequency()
	if freq != 1 {
		if w.doVectorPositions {
			panic(fmt.Sprintf(
				"field %q: cannot index term vector positions while using custom TermFrequencyAttribute",
				w.GetFieldName()))
		}
		if w.doVectorOffsets {
			panic(fmt.Sprintf(
				"field %q: cannot index term vector offsets while using custom TermFrequencyAttribute",
				w.GetFieldName()))
		}
	}
	return freq
}

// newPostingsArray refreshes the cached typed pointer after the base
// TermsHashPerField allocates or grows the underlying postings array. It
// simply mirrors lastCreated; createPostingsArray is the single source of
// truth for the current wrapper. See the FreqProx port's newPostingsArray
// comment for the three base-machinery firing conditions.
func (w *TermVectorsConsumerPerField) newPostingsArray() {
	w.postingsArray = w.lastCreated
}

// createPostingsArray returns a freshly-sized TermVectorsPostingsArray and
// records the wrapper so newPostingsArray can recover it.
func (w *TermVectorsConsumerPerField) createPostingsArray(size int) *ParallelPostingsArray {
	pa := NewTermVectorsPostingsArray(size)
	w.lastCreated = pa
	return pa.ParallelPostingsArray
}

// readVIntFrom decodes a variable-length integer from a ByteSliceReader,
// matching the encoding produced by TermsHashPerField.WriteStreamVInt.
func readVIntFrom(r *ByteSliceReader) (int32, error) {
	var result int32
	shift := 0
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= int32(b&0x7F) << shift
		if b&0x80 == 0 {
			return result, nil
		}
		shift += 7
		if shift >= 35 {
			return 0, fmt.Errorf("TermVectorsConsumerPerField: corrupted VInt")
		}
	}
}
