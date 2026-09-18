// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// BlockTermState holds all state required for PostingsReaderBase to produce a
// PostingsEnum without re-seeking the terms dict.
// Mirrors org.apache.lucene.codecs.BlockTermState from Apache Lucene 10.5.0.
type BlockTermState struct {
	index.OrdTermState

	// DocFreq is how many docs have this term.
	DocFreq int

	// TotalTermFreq is the total number of occurrences of this term.
	TotalTermFreq int64

	// TermBlockOrd is the term's ord in the current block.
	TermBlockOrd int

	// BlockFilePointer is the fp into the terms dict primary file (_X.tim) that holds this term.
	BlockFilePointer int64
}

// NewBlockTermState constructs a new BlockTermState.
//
// Mirrors the protected no-argument constructor of
// org.apache.lucene.codecs.BlockTermState, whose body is empty: every field
// starts at its zero value. The postings-format-specific sentinels
// (lastPosBlockOffset = -1, singletonDocID = -1) belong to IntBlockTermState's
// constructor, not to this one.
func NewBlockTermState() *BlockTermState {
	return &BlockTermState{}
}

// Base returns the BlockTermState view of the receiver.
//
// Apache Lucene 10.5.0 declares BlockTermState as a class and derives the
// codec-specific states (for example
// Lucene104PostingsFormat.IntBlockTermState) from it, so a subclass instance
// widens to BlockTermState implicitly and narrows back with a cast. Go has no
// subclassing: the codec states embed *BlockTermState and travel across the
// postings SPI as index.TermState values, where the narrowing cast becomes a
// type assertion. Base is the matching widening step — it yields the very
// BlockTermState the subclass embeds, and every subclass promotes it.
func (s *BlockTermState) Base() *BlockTermState { return s }

// BaseState returns the BlockTermState view of ts, which every term state
// exchanged across PostingsReaderBase and PostingsWriterBase carries. It is
// the Go rendering of Java's implicit widening of a BlockTermState subclass to
// BlockTermState, and it panics when ts is not backed by a BlockTermState,
// exactly as Java raises ClassCastException for the same mistake.
func BaseState(ts index.TermState) *BlockTermState {
	holder, ok := ts.(interface{ Base() *BlockTermState })
	if !ok {
		panic(fmt.Sprintf("codecs.BaseState: %T is not a BlockTermState", ts))
	}
	return holder.Base()
}

// CopyFrom resets this state from other. Mirrors BlockTermState.copyFrom.
func (s *BlockTermState) CopyFrom(other index.TermState) error {
	o, ok := other.(*BlockTermState)
	if !ok {
		return fmt.Errorf("BlockTermState.CopyFrom: incompatible source type %T", other)
	}

	s.OrdTermState.CopyFrom(other)
	s.DocFreq = o.DocFreq
	s.TotalTermFreq = o.TotalTermFreq
	s.TermBlockOrd = o.TermBlockOrd
	s.BlockFilePointer = o.BlockFilePointer

	return nil
}

// Clone returns a copy of this state.
//
// Mirrors org.apache.lucene.index.TermState.clone(), which BlockTermState
// inherits unchanged through OrdTermState: it delegates to Object.clone(),
// producing a field-by-field shallow copy of the runtime type (including the
// inherited OrdTermState.ord). BlockTermState declares no reference-typed
// field, so the shallow copy is also a complete copy.
func (s *BlockTermState) Clone() *BlockTermState {
	c := *s
	return &c
}

// String returns the debug representation of BlockTermState.toString() in
// Apache Lucene 10.5.0:
//
//	"docFreq=" + docFreq
//	    + " totalTermFreq=" + totalTermFreq
//	    + " termBlockOrd=" + termBlockOrd
//	    + " blockFP=" + blockFilePointer
func (s *BlockTermState) String() string {
	return fmt.Sprintf("docFreq=%d totalTermFreq=%d termBlockOrd=%d blockFP=%d",
		s.DocFreq, s.TotalTermFreq, s.TermBlockOrd, s.BlockFilePointer)
}
