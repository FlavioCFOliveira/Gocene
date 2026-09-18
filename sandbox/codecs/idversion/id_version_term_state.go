// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.codecs.idversion.IDVersionTermState.
package idversion

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
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
//
// Mirrors IDVersionTermState.clone(), which allocates a fresh instance and
// delegates to copyFrom. CopyFrom only rejects a source of another type, and
// the source here is the receiver, so the error branch is unreachable.
func (s *IDVersionTermState) Clone() *IDVersionTermState {
	other := NewIDVersionTermState()
	if err := other.CopyFrom(s); err != nil {
		panic(err)
	}
	return other
}

// CopyFrom resets this state from other.
//
// Mirrors IDVersionTermState.copyFrom(TermState): the BlockTermState part
// first, then the two IDVersion-specific fields. The Java cast to
// IDVersionTermState is rendered as a type assertion.
func (s *IDVersionTermState) CopyFrom(other index.TermState) error {
	o, ok := other.(*IDVersionTermState)
	if !ok {
		return fmt.Errorf("IDVersionTermState.CopyFrom: incompatible source type %T", other)
	}
	if err := s.BlockTermState.CopyFrom(&o.BlockTermState); err != nil {
		return err
	}
	s.IDVersion = o.IDVersion
	s.DocID = o.DocID
	return nil
}
