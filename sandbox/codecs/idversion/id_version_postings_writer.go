// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.codecs.idversion.IDVersionPostingsWriter.
package idversion

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const (
	// TermsCodec is the codec name written in the terms file header.
	TermsCodec = "IDVersionPostingsWriterTerms"

	// VersionStart is the first wire format version.
	VersionStart = 1

	// VersionCurrent is the current wire format version.
	VersionCurrent = VersionStart

	// MinVersion is the minimum allowed version value.
	MinVersion int64 = 0

	// MaxVersion is the maximum allowed version value (ZigZag-safe).
	MaxVersion int64 = 0x3fffffff_ffffffff
)

// idVersionExtra holds IDVersion-specific fields that extend BlockTermState.
// A sidecar map keyed by *BlockTermState allows the interface-mandated
// NewTermState() return type to remain *codecs.BlockTermState while still
// carrying codec-specific data.
type idVersionExtra struct {
	IDVersion int64
	DocID     int
}

// termStateRegistry is a package-level sidecar that maps each *BlockTermState
// allocated by IDVersionPostingsWriter/Reader to its extra codec data.
// The map is guarded by a mutex to allow concurrent readers.
type termStateRegistry struct {
	mu    sync.Mutex
	table map[*codecs.BlockTermState]*idVersionExtra
}

var globalTermStateRegistry = &termStateRegistry{
	table: make(map[*codecs.BlockTermState]*idVersionExtra),
}

// register creates a new sidecar entry for state and returns it.
func (r *termStateRegistry) register(state *codecs.BlockTermState) *idVersionExtra {
	extra := &idVersionExtra{}
	r.mu.Lock()
	r.table[state] = extra
	r.mu.Unlock()
	return extra
}

// lookup retrieves the sidecar entry for state. Returns nil when not found.
func (r *termStateRegistry) lookup(state *codecs.BlockTermState) *idVersionExtra {
	r.mu.Lock()
	extra := r.table[state]
	r.mu.Unlock()
	return extra
}

// BytesToLong decodes an 8-byte big-endian payload into a long.
//
// Mirrors IDVersionPostingsFormat.bytesToLong(BytesRef).
func BytesToLong(b []byte) int64 {
	return int64(binary.BigEndian.Uint64(b))
}

// LongToBytes encodes v as an 8-byte big-endian slice.
//
// Mirrors IDVersionPostingsFormat.longToBytes(long, BytesRef).
// Panics if v is outside [MinVersion, MaxVersion].
func LongToBytes(v int64, dst []byte) {
	if v > MaxVersion || v < MinVersion {
		panic(fmt.Sprintf(
			"version must be >= MIN_VERSION=%d and <= MAX_VERSION=%d (got: %d)",
			MinVersion, MaxVersion, v,
		))
	}
	binary.BigEndian.PutUint64(dst, uint64(v))
}

// IDVersionPostingsWriter implements PushPostingsWriterBase for the
// IDVersionPostingsFormat. Each term must appear in exactly one document
// with exactly one position whose 8-byte payload encodes the version.
//
// Mirrors org.apache.lucene.sandbox.codecs.idversion.IDVersionPostingsWriter.
type IDVersionPostingsWriter struct {
	lastState *idVersionExtra

	lastDocID    int
	lastPosition int
	lastVersion  int64

	liveDocs util.Bits

	lastEncodedVersion int64
}

// NewIDVersionPostingsWriter constructs a writer. liveDocs may be nil (all
// documents live).
func NewIDVersionPostingsWriter(liveDocs util.Bits) *IDVersionPostingsWriter {
	return &IDVersionPostingsWriter{
		liveDocs: liveDocs,
	}
}

// NewTermState allocates a fresh BlockTermState with IDVersion sidecar.
func (w *IDVersionPostingsWriter) NewTermState() *codecs.BlockTermState {
	state := codecs.NewBlockTermState()
	globalTermStateRegistry.register(state)
	return state
}

// Init writes the codec header into the terms output.
func (w *IDVersionPostingsWriter) Init(termsOut store.IndexOutput, state *codecs.SegmentWriteState) error {
	return codecs.WriteIndexHeader(
		termsOut,
		TermsCodec,
		VersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
}

// SetField validates field options and resets per-field state.
func (w *IDVersionPostingsWriter) SetField(fieldInfo *index.FieldInfo) (int, error) {
	if fieldInfo.IndexOptions() != index.IndexOptionsDocsAndFreqsAndPositions {
		return 0, errors.New("field must be indexed using IndexOptions.DOCS_AND_FREQS_AND_POSITIONS")
	}
	if fieldInfo.HasTermVectors() {
		return 0, errors.New("field cannot index term vectors: CheckIndex will report this as index corruption")
	}
	return 0, nil
}

// StartTerm resets the per-term last-doc tracker.
func (w *IDVersionPostingsWriter) StartTerm(_ index.NumericDocValues) error {
	w.lastDocID = -1
	return nil
}

// StartDoc validates constraints and records the document.
func (w *IDVersionPostingsWriter) StartDoc(docID, termDocFreq int) error {
	if w.liveDocs != nil && !w.liveDocs.Get(docID) {
		// Deleted doc; mark as skipped.
		w.lastDocID = -1
		return nil
	}
	if w.lastDocID != -1 {
		return fmt.Errorf("term appears in more than one document: %d and %d", w.lastDocID, docID)
	}
	if termDocFreq != 1 {
		return fmt.Errorf("term appears more than once in the document (freq=%d)", termDocFreq)
	}
	w.lastDocID = docID
	w.lastPosition = -1
	w.lastVersion = -1
	return nil
}

// AddPosition records the version payload from the single position token.
func (w *IDVersionPostingsWriter) AddPosition(position int, payload []byte, _, _ int) error {
	if w.lastDocID == -1 {
		// Deleted doc; skip.
		return nil
	}
	if w.lastPosition != -1 {
		return errors.New("term appears more than once in document")
	}
	w.lastPosition = position
	if payload == nil {
		return errors.New("token doesn't have a payload")
	}
	if len(payload) != 8 {
		return fmt.Errorf("payload.length != 8 (got %d)", len(payload))
	}

	ver := BytesToLong(payload)
	if ver < MinVersion {
		return fmt.Errorf("version must be >= MIN_VERSION=%d (got: %d)", MinVersion, ver)
	}
	if ver > MaxVersion {
		return fmt.Errorf("version must be <= MAX_VERSION=%d (got: %d)", MaxVersion, ver)
	}
	w.lastVersion = ver
	return nil
}

// FinishDoc validates that the expected position was seen.
func (w *IDVersionPostingsWriter) FinishDoc() error {
	if w.lastDocID == -1 {
		// Deleted doc; skip.
		return nil
	}
	if w.lastPosition == -1 {
		return errors.New("missing AddPosition")
	}
	return nil
}

// FinishTerm populates state with the doc/version recorded for this term.
func (w *IDVersionPostingsWriter) FinishTerm(state *codecs.BlockTermState) error {
	if w.lastDocID == -1 {
		return nil
	}
	extra := globalTermStateRegistry.lookup(state)
	if extra == nil {
		// State was not allocated by this writer; treat as plain BlockTermState.
		return nil
	}
	extra.DocID = w.lastDocID
	extra.IDVersion = w.lastVersion
	return nil
}

// EncodeTerm serializes the docID and version delta into out.
func (w *IDVersionPostingsWriter) EncodeTerm(
	out store.IndexOutput,
	_ *index.FieldInfo,
	state *codecs.BlockTermState,
	absolute bool,
) error {
	extra := globalTermStateRegistry.lookup(state)
	if extra == nil {
		return errors.New("EncodeTerm: state not registered with IDVersionPostingsWriter")
	}
	if err := store.WriteVInt(out, int32(extra.DocID)); err != nil {
		return err
	}
	if absolute {
		if err := store.WriteVLong(out, extra.IDVersion); err != nil {
			return err
		}
	} else {
		delta := extra.IDVersion - w.lastEncodedVersion
		if err := store.WriteVLong(out, util.ZigZagEncodeInt64(delta)); err != nil {
			return err
		}
	}
	w.lastEncodedVersion = extra.IDVersion
	return nil
}

// Close is a no-op (no owned file handles).
func (w *IDVersionPostingsWriter) Close() error { return nil }

var _ codecs.PushPostingsWriterBase = (*IDVersionPostingsWriter)(nil)
