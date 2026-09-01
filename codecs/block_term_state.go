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

	// docFreq is how many docs have this term.
	docFreq int32

	// totalTermFreq is the total number of occurrences of this term.
	totalTermFreq int64

	// termBlockOrd is the term's ord in the current block.
	termBlockOrd int32

	// blockFilePointer is the fp into the terms dict primary file (_X.tim) that holds this term.
	blockFilePointer int64

	// docStartFP is the file pointer to the start of the doc list.
	docStartFP int64

	// posStartFP is the file pointer to the start of the positions list.
	posStartFP int64

	// payStartFP is the file pointer to the start of the payloads list.
	payStartFP int64

	// lastPosBlockOffset is the offset of the last position block.
	lastPosBlockOffset int64

	// singletonDocID is the docID if the term is a singleton.
	singletonDocID int32
}

// NewBlockTermState constructs a new BlockTermState.
func NewBlockTermState() *BlockTermState {
	return &BlockTermState{
		lastPosBlockOffset: -1,
		singletonDocID:     -1,
	}
}

// CopyFrom resets this state from other. Mirrors Lucene's CopyFrom mechanism.
func (s *BlockTermState) CopyFrom(other index.TermState) error {
	o, ok := other.(*BlockTermState)
	if !ok {
		return fmt.Errorf("BlockTermState.CopyFrom: incompatible source type %T", other)
	}

	s.OrdTermState.CopyFrom(other)
	s.docFreq = o.docFreq
	s.totalTermFreq = o.totalTermFreq
	s.termBlockOrd = o.termBlockOrd
	s.blockFilePointer = o.blockFilePointer
	s.docStartFP = o.docStartFP
	s.posStartFP = o.posStartFP
	s.payStartFP = o.payStartFP
	s.lastPosBlockOffset = o.lastPosBlockOffset
	s.singletonDocID = o.singletonDocID

	return nil
}

// String returns a debug representation matching Lucene's.
func (s *BlockTermState) String() string {
	return fmt.Sprintf("BlockTermState{ord=%d, docFreq=%d, totalTermFreq=%d, termBlockOrd=%d, blockFP=%d, docFP=%d, posFP=%d, payFP=%d, lastPosOff=%d, singletonID=%d}",
		s.Ord, s.docFreq, s.totalTermFreq, s.termBlockOrd, s.blockFilePointer, s.docStartFP, s.posStartFP, s.payStartFP, s.lastPosBlockOffset, s.singletonDocID)
}
