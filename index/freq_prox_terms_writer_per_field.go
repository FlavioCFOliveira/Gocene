// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// FreqProxTermsHash bundles the four shared inversion pools that Lucene's
// org.apache.lucene.index.TermsHash exposes to its per-field consumers.
// Gocene has not yet ported the full TermsHash aggregator (tracked under the
// indexing-pipeline backlog), so FreqProxTermsWriterPerField accepts this
// minimal value type instead of a *TermsHash. The fields mirror the four
// constructor arguments TermsHashPerField pulls off termsHash in Java.
//
// All four fields are required; NewFreqProxTermsWriterPerField rejects a
// zero-valued FreqProxTermsHash.
type FreqProxTermsHash struct {
	// IntPool is the shared IntBlockPool used to store per-term stream
	// addresses across the inversion pipeline.
	IntPool *util.IntBlockPool
	// BytePool is the shared ByteBlockPool used to encode the per-stream
	// posting bytes (doc+freq and prox+offset).
	BytePool *util.ByteBlockPool
	// TermBytePool is the shared ByteBlockPool used to intern the raw term
	// bytes inside the per-field BytesRefHash.
	TermBytePool *util.ByteBlockPool
	// BytesUsed accumulates the memory consumed by the postings side arrays
	// and is shared with the surrounding DocumentsWriterPerThread.
	BytesUsed *util.Counter
}

// FreqProxAttributeProvider exposes the analysis-time attributes that the
// FreqProx writer pulls off FieldInvertState in Lucene. Each getter returns
// the current per-token value, mirroring the Java attribute calls in the
// reference implementation:
//
//   - TermFrequency: TermFrequencyAttribute.getTermFrequency(); returns 1
//     when the field does not surface a custom attribute, mirroring the
//     "termFreqAtt == null" branch in Lucene.
//   - HasTermFreqAttribute: true if a TermFrequencyAttribute is wired in,
//     i.e. equivalent to "termFreqAtt != null". Lucene relies on the null
//     check to guard the custom-frequency validation in addTerm.
//   - StartOffset / EndOffset: OffsetAttribute.startOffset() / endOffset(),
//     consumed only when the field indexes offsets.
//   - Payload: PayloadAttribute.getPayload(), consumed only when the field
//     indexes positions. A nil return means "no payload" and triggers the
//     "no payload" branch of writeProx.
//
// All getters are invoked synchronously during indexField; the provider
// must reflect the values of the token currently held by the analyzer.
//
// Decision (b) of this sprint keeps the FreqProx port self-contained: the
// concrete index package never depends on a particular attribute API. The
// pipeline glue layer (DocumentsWriterPerThread) is responsible for
// constructing a provider that bridges its TokenStream attribute source.
type FreqProxAttributeProvider struct {
	// TermFrequency returns the current token's term frequency.
	// May be nil; the writer treats nil as "always 1".
	TermFrequency func() int
	// HasTermFreqAttribute reports whether a TermFrequencyAttribute is wired.
	// May be nil; the writer treats nil as "false" (i.e. no attribute).
	HasTermFreqAttribute func() bool
	// StartOffset returns the current token's start offset.
	// Must be non-nil when the field indexes offsets.
	StartOffset func() int
	// EndOffset returns the current token's end offset.
	// Must be non-nil when the field indexes offsets.
	EndOffset func() int
	// Payload returns the current token's payload, or nil when absent.
	// Must be non-nil when the field indexes positions.
	Payload func() *util.BytesRef
}

// FreqProxPostingsArray extends ParallelPostingsArray with the per-term
// counters needed by the FreqProx writer: term frequencies, last docID,
// last docCode, last position, and last offset. The optional slices are
// allocated only when the field indexes the corresponding stream
// (freqs / positions / offsets), mirroring Lucene's
// FreqProxTermsWriterPerField.FreqProxPostingsArray.
type FreqProxPostingsArray struct {
	*ParallelPostingsArray

	// TermFreqs counts how many times the term occurs in the current doc;
	// nil when the field does not index frequencies.
	TermFreqs []int
	// LastDocIDs records the last docID where the term occurred.
	LastDocIDs []int
	// LastDocCodes caches the encoded doc-delta for the prior document.
	LastDocCodes []int
	// LastPositions records the last position where the term occurred;
	// nil when the field does not index positions.
	LastPositions []int
	// LastOffsets records the last end-offset where the term occurred;
	// nil when the field does not index offsets.
	LastOffsets []int

	writeFreqs   bool
	writeProx    bool
	writeOffsets bool
}

// NewFreqProxPostingsArray allocates a FreqProxPostingsArray sized for the
// given index options. writeFreqs / writeProx / writeOffsets follow Lucene's
// boolean trio derived from FieldInfo.indexOptions.
func NewFreqProxPostingsArray(size int, writeFreqs, writeProx, writeOffsets bool) *FreqProxPostingsArray {
	if writeOffsets && !writeProx {
		panic("FreqProxPostingsArray: cannot write offsets without positions")
	}
	a := &FreqProxPostingsArray{
		ParallelPostingsArray: NewParallelPostingsArray(size),
		LastDocIDs:            make([]int, size),
		LastDocCodes:          make([]int, size),
		writeFreqs:            writeFreqs,
		writeProx:             writeProx,
		writeOffsets:          writeOffsets,
	}
	if writeFreqs {
		a.TermFreqs = make([]int, size)
	}
	if writeProx {
		a.LastPositions = make([]int, size)
		if writeOffsets {
			a.LastOffsets = make([]int, size)
		}
	}
	return a
}

// BytesPerPosting returns the byte cost of a single posting slot, including
// the optional freq / position / offset sidearms.
func (a *FreqProxPostingsArray) BytesPerPosting() int {
	bytes := a.ParallelPostingsArray.BytesPerPosting() + 2*4
	if a.LastPositions != nil {
		bytes += 4
	}
	if a.LastOffsets != nil {
		bytes += 4
	}
	if a.TermFreqs != nil {
		bytes += 4
	}
	return bytes
}

// CopyTo copies the first numToCopy slots into dst.
func (a *FreqProxPostingsArray) CopyTo(dst *FreqProxPostingsArray, numToCopy int) {
	a.ParallelPostingsArray.CopyTo(dst.ParallelPostingsArray, numToCopy)
	copy(dst.LastDocIDs[:numToCopy], a.LastDocIDs[:numToCopy])
	copy(dst.LastDocCodes[:numToCopy], a.LastDocCodes[:numToCopy])
	if a.LastPositions != nil {
		copy(dst.LastPositions[:numToCopy], a.LastPositions[:numToCopy])
	}
	if a.LastOffsets != nil {
		copy(dst.LastOffsets[:numToCopy], a.LastOffsets[:numToCopy])
	}
	if a.TermFreqs != nil {
		copy(dst.TermFreqs[:numToCopy], a.TermFreqs[:numToCopy])
	}
}

// FreqProxTermsWriterPerField encodes per-document term frequencies,
// positions, offsets and payloads for a single field. It is the Go port of
// org.apache.lucene.index.FreqProxTermsWriterPerField from Apache Lucene
// 10.4.0.
//
// The Java class is final and extends TermsHashPerField, overriding the four
// abstract hooks (newTerm, addTerm, newPostingsArray, createPostingsArray)
// and adding writeProx / writeOffsets helpers. Gocene's TermsHashPerField
// expresses the same contract as function-valued hooks; this type wires the
// four FreqProx-specific implementations through them at construction time.
//
// Divergences from Lucene 10.4.0:
//
//   - Self-contained port (sprint decision b): instead of depending on the
//     analysis token-attribute interfaces, the writer pulls per-token data
//     through a FreqProxAttributeProvider whose fields are plain function
//     getters. The pipeline glue is responsible for bridging the active
//     TokenStream to the provider.
//   - Local FreqProxTermsHash stub: the Lucene TermsHash aggregator is not
//     yet ported, so the constructor accepts FreqProxTermsHash carrying the
//     same four pools (intPool, bytePool, termBytePool, bytesUsed).
//   - FieldInfo.SetStorePayloads is the Gocene equivalent of Lucene's
//     package-private setStorePayloads; it bypasses the frozen contract.
//   - assertDocID is enforced by TermsHashPerField in release builds (Lucene
//     relies on a Java assertion), so addTerm receives docIDs guaranteed to
//     be monotonic. The lastDocIDs comparison inside addTerm therefore only
//     decides between "same doc" and "new doc", not "ordering violation".
type FreqProxTermsWriterPerField struct {
	*TermsHashPerField

	postingsArray *FreqProxPostingsArray

	// lastCreated holds the FreqProxPostingsArray most recently returned
	// by createPostingsArray. The base TermsHashPerField machinery only
	// retains the embedded *ParallelPostingsArray; this field is the
	// Go-side bridge that lets newPostingsArray recover the wrapper
	// without needing a back-pointer on ParallelPostingsArray itself.
	lastCreated *FreqProxPostingsArray

	fieldState *FieldInvertState
	fieldInfo  *FieldInfo
	attrs      FreqProxAttributeProvider

	hasFreq    bool
	hasProx    bool
	hasOffsets bool

	// sawPayloads is set to true the first time the writer sees a non-empty
	// payload during the current segment. It is consulted by Finish to call
	// FieldInfo.SetStorePayloads exactly once, mirroring Lucene.
	sawPayloads bool
}

// NewFreqProxTermsWriterPerField wires a FreqProx per-field writer over the
// supplied FieldInvertState, TermsHash bundle and next-in-chain handler.
//
// fieldInfo must be the FieldInfo associated with invertState. attrs holds
// the per-token getters; getters for streams the field does not encode may
// be left nil. nextPerField may be nil; when non-nil it is invoked with the
// pool offset of each newly observed term (mirroring Lucene's secondary
// entry point used by the term-vectors layer).
//
// The returned writer owns its embedded TermsHashPerField and may be used
// directly; the embedded type's exported Add / Finish / Start methods are
// the public entry points.
func NewFreqProxTermsWriterPerField(
	invertState *FieldInvertState,
	termsHash FreqProxTermsHash,
	fieldInfo *FieldInfo,
	nextPerField *TermsHashPerField,
	attrs FreqProxAttributeProvider,
) (*FreqProxTermsWriterPerField, error) {
	if invertState == nil {
		return nil, fmt.Errorf("FreqProxTermsWriterPerField: invertState must not be nil")
	}
	if fieldInfo == nil {
		return nil, fmt.Errorf("FreqProxTermsWriterPerField: fieldInfo must not be nil")
	}
	if termsHash.IntPool == nil || termsHash.BytePool == nil || termsHash.TermBytePool == nil {
		return nil, fmt.Errorf("FreqProxTermsWriterPerField: TermsHash pools must not be nil")
	}
	if termsHash.BytesUsed == nil {
		termsHash.BytesUsed = util.NewCounter()
	}

	indexOpts := fieldInfo.IndexOptions()
	if indexOpts == IndexOptionsNone {
		return nil, fmt.Errorf("FreqProxTermsWriterPerField: field %q has IndexOptionsNone", fieldInfo.Name())
	}

	streamCount := 1
	if indexOpts >= IndexOptionsDocsAndFreqsAndPositions {
		streamCount = 2
	}

	w := &FreqProxTermsWriterPerField{
		fieldState: invertState,
		fieldInfo:  fieldInfo,
		attrs:      attrs,
		hasFreq:    indexOpts >= IndexOptionsDocsAndFreqs,
		hasProx:    indexOpts >= IndexOptionsDocsAndFreqsAndPositions,
		hasOffsets: indexOpts >= IndexOptionsDocsAndFreqsAndPositionsAndOffsets,
	}

	hooks := TermsHashPerFieldHooks{
		NewTerm:             w.newTerm,
		AddTerm:             w.addTerm,
		NewPostingsArray:    w.newPostingsArray,
		CreatePostingsArray: w.createPostingsArray,
	}

	base, err := NewTermsHashPerField(
		streamCount,
		termsHash.IntPool,
		termsHash.BytePool,
		termsHash.TermBytePool,
		termsHash.BytesUsed,
		nextPerField,
		fieldInfo.Name(),
		indexOpts,
		hooks,
	)
	if err != nil {
		return nil, err
	}
	w.TermsHashPerField = base
	// BytesRefHash.Init may have run the NewPostingsArray hook before we
	// assigned w.TermsHashPerField. lastCreated is set by
	// createPostingsArray in that path; mirror it onto postingsArray now
	// so the first Add can read it without re-triggering the hook.
	w.postingsArray = w.lastCreated
	return w, nil
}

// Finish closes the current document. When a non-empty payload was seen
// during the segment, the field's FieldInfo is marked as carrying payloads
// before the call is propagated down the chain (mirroring Lucene's
// FreqProxTermsWriterPerField.finish ordering).
//
// In Lucene, FreqProxTermsWriterPerField is always the head of the chain
// (its NextPerField is the term-vectors writer), so the dispatch goes
// through this method even when invoked generically. Callers in Gocene
// must respect the same invariant: the FreqProx writer is the inversion
// pipeline entry point, never a downstream link.
func (w *FreqProxTermsWriterPerField) Finish() error {
	if w.sawPayloads {
		w.fieldInfo.SetStorePayloads()
	}
	return w.TermsHashPerField.Finish()
}

// Start begins a new occurrence of f within the current document. The base
// TermsHashPerField.Start handles the doNextCall coordination with the next
// handler; this override is a marker that mirrors Lucene's reading of the
// FieldInvertState attribute caches (kept for parity with the Java code
// path even though Gocene's attribute getters are pulled per token via the
// FreqProxAttributeProvider rather than cached on entry).
func (w *FreqProxTermsWriterPerField) Start(f IndexableField, first bool) bool {
	return w.TermsHashPerField.Start(f, first)
}

// writeProx encodes proxCode (and an optional payload) into stream 1, then
// updates the term's lastPositions slot. Mirrors Lucene's writeProx.
func (w *FreqProxTermsWriterPerField) writeProx(termID, proxCode int) {
	var payload *util.BytesRef
	if w.attrs.Payload != nil {
		payload = w.attrs.Payload()
	}
	if payload == nil || payload.Length == 0 {
		w.WriteStreamVInt(1, int32(proxCode<<1))
	} else {
		w.WriteStreamVInt(1, int32((proxCode<<1)|1))
		w.WriteStreamVInt(1, int32(payload.Length))
		w.WriteStreamBytes(1, payload.Bytes, payload.Offset, payload.Length)
		w.sawPayloads = true
	}
	w.postingsArray.LastPositions[termID] = w.fieldState.Position()
}

// writeOffsets encodes the delta-encoded start/end offsets into stream 1
// and refreshes the term's lastOffsets slot. Mirrors Lucene's writeOffsets.
func (w *FreqProxTermsWriterPerField) writeOffsets(termID, offsetAccum int) {
	if w.attrs.StartOffset == nil || w.attrs.EndOffset == nil {
		panic(fmt.Sprintf("FreqProxTermsWriterPerField: field %q indexes offsets but provider has no StartOffset/EndOffset", w.fieldInfo.Name()))
	}
	startOffset := offsetAccum + w.attrs.StartOffset()
	endOffset := offsetAccum + w.attrs.EndOffset()
	if delta := startOffset - w.postingsArray.LastOffsets[termID]; delta < 0 {
		panic(fmt.Sprintf("FreqProxTermsWriterPerField: field %q: startOffset went backwards (last=%d, current=%d)",
			w.fieldInfo.Name(), w.postingsArray.LastOffsets[termID], startOffset))
	}
	w.WriteStreamVInt(1, int32(startOffset-w.postingsArray.LastOffsets[termID]))
	w.WriteStreamVInt(1, int32(endOffset-startOffset))
	w.postingsArray.LastOffsets[termID] = startOffset
}

// newTerm initialises a freshly observed term's posting slot. Mirrors
// Lucene's newTerm.
func (w *FreqProxTermsWriterPerField) newTerm(termID, docID int) error {
	postings := w.postingsArray
	postings.LastDocIDs[termID] = docID
	if !w.hasFreq {
		postings.LastDocCodes[termID] = docID
		if w.fieldState.MaxTermFrequency() < 1 {
			w.fieldState.SetMaxTermFrequency(1)
		}
	} else {
		postings.LastDocCodes[termID] = docID << 1
		freq := w.getTermFreq()
		postings.TermFreqs[termID] = freq
		if w.hasProx {
			w.writeProx(termID, w.fieldState.Position())
			if w.hasOffsets {
				w.writeOffsets(termID, w.fieldState.Offset())
			}
		}
		if freq > w.fieldState.MaxTermFrequency() {
			w.fieldState.SetMaxTermFrequency(freq)
		}
	}
	w.fieldState.SetUniqueTermCount(w.fieldState.UniqueTermCount() + 1)
	return nil
}

// addTerm updates the posting slot for a previously observed term. Mirrors
// Lucene's addTerm; the TermsHashPerField base guarantees docID is
// monotonic with respect to the previous Add call, so the comparison below
// only distinguishes "same doc" from "new doc".
func (w *FreqProxTermsWriterPerField) addTerm(termID, docID int) error {
	postings := w.postingsArray
	if !w.hasFreq {
		if w.attrs.HasTermFreqAttribute != nil && w.attrs.HasTermFreqAttribute() &&
			w.attrs.TermFrequency != nil && w.attrs.TermFrequency() != 1 {
			return fmt.Errorf(
				"field %q: must index term freq while using custom TermFrequencyAttribute",
				w.GetFieldName())
		}
		if docID != postings.LastDocIDs[termID] {
			w.WriteStreamVInt(0, int32(postings.LastDocCodes[termID]))
			postings.LastDocCodes[termID] = docID - postings.LastDocIDs[termID]
			postings.LastDocIDs[termID] = docID
			w.fieldState.SetUniqueTermCount(w.fieldState.UniqueTermCount() + 1)
		}
		return nil
	}
	if docID != postings.LastDocIDs[termID] {
		if postings.TermFreqs[termID] == 1 {
			w.WriteStreamVInt(0, int32(postings.LastDocCodes[termID]|1))
		} else {
			w.WriteStreamVInt(0, int32(postings.LastDocCodes[termID]))
			w.WriteStreamVInt(0, int32(postings.TermFreqs[termID]))
		}
		freq := w.getTermFreq()
		postings.TermFreqs[termID] = freq
		if freq > w.fieldState.MaxTermFrequency() {
			w.fieldState.SetMaxTermFrequency(freq)
		}
		postings.LastDocCodes[termID] = (docID - postings.LastDocIDs[termID]) << 1
		postings.LastDocIDs[termID] = docID
		if w.hasProx {
			w.writeProx(termID, w.fieldState.Position())
			if w.hasOffsets {
				postings.LastOffsets[termID] = 0
				w.writeOffsets(termID, w.fieldState.Offset())
			}
		}
		w.fieldState.SetUniqueTermCount(w.fieldState.UniqueTermCount() + 1)
		return nil
	}
	freq := w.getTermFreq()
	sum, err := addExact(postings.TermFreqs[termID], freq)
	if err != nil {
		return fmt.Errorf("field %q: %w", w.GetFieldName(), err)
	}
	postings.TermFreqs[termID] = sum
	if sum > w.fieldState.MaxTermFrequency() {
		w.fieldState.SetMaxTermFrequency(sum)
	}
	if w.hasProx {
		w.writeProx(termID, w.fieldState.Position()-postings.LastPositions[termID])
		if w.hasOffsets {
			w.writeOffsets(termID, w.fieldState.Offset())
		}
	}
	return nil
}

// getTermFreq reads the current token's term frequency. Mirrors Lucene's
// getTermFreq, including the "cannot index positions with custom freq"
// guard which becomes a Go error rather than an unchecked exception.
//
// Note: Lucene panics here via IllegalStateException. Gocene's TermsHash
// hook contract requires returning errors, but getTermFreq is called from
// newTerm / addTerm in positions where Lucene also throws. We surface the
// invalid-state condition by panicking too, since the hook would have to
// return the error up through writeProx callers that have no error
// channel. The panic is intentional and documents the same bug.
func (w *FreqProxTermsWriterPerField) getTermFreq() int {
	freq := 1
	if w.attrs.TermFrequency != nil {
		freq = w.attrs.TermFrequency()
	}
	if freq != 1 && w.hasProx {
		panic(fmt.Sprintf(
			"field %q: cannot index positions while using custom TermFrequencyAttribute",
			w.GetFieldName()))
	}
	return freq
}

// newPostingsArray refreshes the cached typed pointer after the base
// TermsHashPerField allocates or grows the underlying postings array. The
// hook fires under three conditions in the base machinery:
//   - postingsBytesStartArray.Init: PostingsArray was just created. The
//     Init path also fires during NewTermsHashPerField before we have a
//     chance to wire w.TermsHashPerField; the constructor mirrors
//     lastCreated onto postingsArray manually after the base returns.
//   - postingsBytesStartArray.Grow: PostingsArray was just resized.
//   - postingsBytesStartArray.Clear: PostingsArray was just set to nil;
//     lastCreated is left dangling because the next createPostingsArray
//     overwrites it before postingsArray is consulted again.
//
// postingsArray simply mirrors lastCreated; the createPostingsArray hook
// is the single source of truth for the current wrapper.
func (w *FreqProxTermsWriterPerField) newPostingsArray() {
	w.postingsArray = w.lastCreated
}

// createPostingsArray returns a freshly-sized FreqProxPostingsArray and
// records the wrapper so newPostingsArray can recover it.
func (w *FreqProxTermsWriterPerField) createPostingsArray(size int) *ParallelPostingsArray {
	pa := NewFreqProxPostingsArray(size, w.hasFreq, w.hasProx, w.hasOffsets)
	w.lastCreated = pa
	return pa.ParallelPostingsArray
}

// addExact returns x+y or an overflow error, matching the semantics of
// Java's Math.addExact used by Lucene's FreqProxTermsWriterPerField.
func addExact(x, y int) (int, error) {
	sum := x + y
	if (x > 0 && y > 0 && sum < 0) ||
		(x < 0 && y < 0 && sum > 0) ||
		(x == math.MinInt && y < 0) {
		return 0, fmt.Errorf("integer overflow: %d + %d", x, y)
	}
	return sum, nil
}
