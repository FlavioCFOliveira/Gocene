// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.codecs.idversion.IDVersionTermState.
package idversion

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// IDVersionTermState holds the codec-specific postings state for a single term
// in the IDVersionPostingsFormat: the single document ID and the 64-bit version
// encoded in the position payload.
//
// In the IDVersion codec a term exists in at most one document, so docFreq is
// always 1 and totalTermFreq is unused.
//
// Mirrors org.apache.lucene.sandbox.codecs.idversion.IDVersionTermState
// (package-private in Java; exported in Go because the SegmentTermsEnum needs
// to pass it across sub-packages).
type IDVersionTermState struct {
	codecs.BlockTermState

	// IDVersion is the 64-bit version extracted from the position payload.
	IDVersion int64

	// DocID is the single document that holds this term.
	DocID int
}

// NewIDVersionTermState returns a zero-valued IDVersionTermState. The embedded
// BlockTermState is initialised with TotalTermFreq = -1 (codec does not track
// this).
func NewIDVersionTermState() *IDVersionTermState {
	return &IDVersionTermState{
		BlockTermState: codecs.BlockTermState{
			TotalTermFreq: -1,
		},
	}
}

// Clone returns a deep copy of the receiver.
func (s *IDVersionTermState) Clone() *IDVersionTermState {
	c := *s
	// CopyFrom the embedded BlockTermState to reset any pointer fields.
	c.BlockTermState = *s.BlockTermState.Clone()
	return &c
}

// CopyFrom copies all fields from src.
func (s *IDVersionTermState) CopyFrom(src *IDVersionTermState) {
	s.BlockTermState.CopyFrom(&src.BlockTermState)
	s.IDVersion = src.IDVersion
	s.DocID = src.DocID
}

// AsBlockTermState returns a pointer to the embedded BlockTermState.
// This is used when the codec writer/reader needs a *codecs.BlockTermState.
//
// NOTE: the caller must ensure the returned pointer is not used after the
// IDVersionTermState is garbage collected.
func (s *IDVersionTermState) AsBlockTermState() *codecs.BlockTermState {
	return &s.BlockTermState
}
