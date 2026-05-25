// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import "fmt"

// IntBlockTermState holds the Lucene104-specific term state produced by
// Lucene104PostingsWriter and consumed by Lucene104PostingsReader.
//
// It extends BlockTermState with five extra fields that record where the
// per-document, per-position, and per-payload data for a term begins inside
// the .doc, .pos, and .pay files, plus two optimisation hints.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene104.Lucene104PostingsFormat.IntBlockTermState
// from Apache Lucene 10.4.0.
type IntBlockTermState struct {
	*BlockTermState

	// DocStartFP is the file pointer to the start of the doc-id enumeration
	// inside the .doc file.
	DocStartFP int64

	// PosStartFP is the file pointer to the start of the position enumeration
	// inside the .pos file. Zero when the field does not index positions.
	PosStartFP int64

	// PayStartFP is the file pointer to the start of the payload/offset
	// enumeration inside the .pay file. Zero when the field has neither
	// payloads nor offsets.
	PayStartFP int64

	// LastPosBlockOffset is the file offset of the last position block inside
	// the .pos file when totalTermFreq > ForUtil.BLOCK_SIZE (256). -1 otherwise.
	//
	// The reader uses this to decide whether the trailing positions are packed
	// (PFOR block) or VInt-encoded. Tracking this offset separately — rather
	// than deriving it from totalTermFreq — is necessary because skip-list
	// navigation advances the position pointer without telling the reader how
	// many positions were skipped, so a running count would diverge.
	LastPosBlockOffset int64

	// SingletonDocID is the single document ID when a term appears in exactly
	// one document (docFreq == 1). In this case the writer stores the docID
	// directly in the term dictionary instead of writing a .doc file pointer,
	// saving a seek. -1 when docFreq != 1.
	//
	// When SingletonDocID >= 0, freq is always implicitly totalTermFreq.
	SingletonDocID int
}

// NewIntBlockTermState returns a fresh IntBlockTermState with the Lucene104
// default sentinel values:
//   - LastPosBlockOffset = -1 (no trailing VInt position block)
//   - SingletonDocID     = -1 (not a singleton term)
func NewIntBlockTermState() *IntBlockTermState {
	return &IntBlockTermState{
		BlockTermState:     NewBlockTermState(),
		LastPosBlockOffset: -1,
		SingletonDocID:     -1,
	}
}

// Clone returns a deep copy of the receiver. The embedded *BlockTermState is
// copied field-by-field via CopyFrom to preserve any future extensions.
func (s *IntBlockTermState) Clone() *IntBlockTermState {
	other := NewIntBlockTermState()
	other.CopyFrom(s)
	return other
}

// CopyFrom copies all fields from src into the receiver. Panics if src is nil
// or not an *IntBlockTermState (matching the Java assertion behaviour).
func (s *IntBlockTermState) CopyFrom(src *IntBlockTermState) {
	if src == nil {
		panic("IntBlockTermState.CopyFrom: nil src")
	}
	s.BlockTermState.CopyFrom(src.BlockTermState)
	s.DocStartFP = src.DocStartFP
	s.PosStartFP = src.PosStartFP
	s.PayStartFP = src.PayStartFP
	s.LastPosBlockOffset = src.LastPosBlockOffset
	s.SingletonDocID = src.SingletonDocID
}

// String returns a human-readable representation for debugging.
func (s *IntBlockTermState) String() string {
	return fmt.Sprintf(
		"IntBlockTermState{docFreq=%d totalTermFreq=%d docStartFP=%d posStartFP=%d payStartFP=%d lastPosBlockOffset=%d singletonDocID=%d}",
		s.DocFreq, s.TotalTermFreq,
		s.DocStartFP, s.PosStartFP, s.PayStartFP,
		s.LastPosBlockOffset, s.SingletonDocID,
	)
}
