// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"fmt"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

var DefaultBlockHeaderSerializer = &BlockHeaderSerializer{}

// BlockLineSerializer reads and writes BlockLine with incremental term encoding.
type BlockLineSerializer struct {
	currentTerm *util.BytesRef
}

func NewBlockLineSerializer() *BlockLineSerializer {
	return &BlockLineSerializer{
		currentTerm: util.NewBytesRef(64),
	}
}

func (s *BlockLineSerializer) ReadLine(in store.DataInput, isIncrementalEncodingSeed bool, reuse *BlockLine) (*BlockLine, error) {
	termStateRelativeOffset, err := in.ReadVInt()
	if err != nil {
		return nil, err
	}
	if termStateRelativeOffset < 0 {
		return nil, fmt.Errorf("illegal termStateRelativeOffset= %d", termStateRelativeOffset)
	}

	term, err := s.readIncrementallyEncodedTerm(in, isIncrementalEncodingSeed)
	if err != nil {
		return nil, err
	}

	bl := reuse
	if bl == nil {
		bl = &BlockLine{}
	}
	return bl.Reset(term, termStateRelativeOffset), nil
}

func (s *BlockLineSerializer) WriteLine(out store.DataOutput, line *BlockLine, previousLine *BlockLine, termStateRelativeOffset int32, isIncrementalEncodingSeed bool) error {
	if err := out.WriteVInt(termStateRelativeOffset); err != nil {
		return err
	}

	var prevTermBytes *TermBytes
	if previousLine != nil {
		prevTermBytes = previousLine.term
	}

	if err := s.writeIncrementallyEncodedTerm(out, line.term, prevTermBytes, isIncrementalEncodingSeed); err != nil {
		return err
	}
	return nil
}

func (s *BlockLineSerializer) writeIncrementallyEncodedTerm(out store.DataOutput, termBytes *TermBytes, prevTermBytes *TermBytes, isIncrementalEncodingSeed bool) error {
	term := termBytes.GetTerm()
	if isIncrementalEncodingSeed {
		if err := out.WriteVLong(int64(term.Length())); err != nil {
			return err
		}
		if err := out.WriteBytes(term.Bytes(), term.Offset, term.Length()); err != nil {
			return err
		}
		return nil
	}

	if term.Length() == 0 {
		return out.WriteVLong(0)
	}

	if prevTermBytes == nil {
		return fmt.Errorf("previousTermBytes must not be nil for non-seed incremental encoding")
	}

	numMdpBits := numBitsToEncode(prevTermBytes.GetTerm().Length())
	mdpLength := termBytes.GetMdpLength()
	if mdpLength < 1 {
		return fmt.Errorf("mdpLength must be >= 1 for non-seed terms")
	}

	mdpAndSuffixLengths := (int64(termBytes.GetSuffixLength()) << numMdpBits) | int64(mdpLength-1)
	if mdpAndSuffixLengths == 0 {
		return fmt.Errorf("mdpAndSuffixLengths cannot be 0 for non-empty terms")
	}

	if err := out.WriteVLong(mdpAndSuffixLengths); err != nil {
		return err
	}

	suffixLen := termBytes.GetSuffixLength()
	suffixOff := termBytes.GetSuffixOffset()
	if err := out.WriteBytes(term.Bytes(), term.Offset+suffixOff, suffixLen); err != nil {
		return err
	}

	return nil
}

func (s *BlockLineSerializer) readIncrementallyEncodedTerm(in store.DataInput, isIncrementalEncodingSeed bool) (*TermBytes, error) {
	var mdpLength int32
	if isIncrementalEncodingSeed {
		length, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		if length == 0 {
			mdpLength = 0
		} else {
			mdpLength = 1
		}

		// Read bytes into s.currentTerm
		if err := s.readBytes(in, int(length), 0); err != nil {
			return nil, err
		}
	} else {
		mdpAndSuffixLengths, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}

		if mdpAndSuffixLengths == 0 {
			mdpLength = 0
			s.currentTerm.SetLength(0)
		} else {
			numMdpBits := numBitsToEncode(s.currentTerm.Length())
			mdpLength = int32(mdpAndSuffixLengths&((1<<numMdpBits)-1)) + 1
			suffixLength := int(mdpAndSuffixLengths >> numMdpBits)

			if err := s.readBytes(in, suffixLength, int(mdpLength-1)); err != nil {
				return nil, err
			}
		}
	}

	return NewTermBytes(mdpLength, s.currentTerm), nil
}

func (s *BlockLineSerializer) readBytes(in store.DataInput, length, offset int) error {
	if length == 0 {
		return nil
	}
	// Ensure currentTerm has enough capacity
	if s.currentTerm.Capacity() < offset+length {
		s.currentTerm = util.NewBytesRef(offset + length)
	}

	// Read bytes directly into the buffer
	if err := in.ReadBytes(s.currentTerm.Bytes(), offset, length); err != nil {
		return err
	}
	s.currentTerm.SetLength(offset + length)
	return nil
}

func numBitsToEncode(i int) int {
	if i == 0 {
		return 0
	}
	return 32 - bits.LeadingZeros32(uint32(i))
}

// DeltaBaseTermStateSerializer encodes each file pointer as a delta relative to a base file pointer.
type DeltaBaseTermStateSerializer struct {
	baseDocStartFP int64
	basePosStartFP int64
	basePayStartFP int64
}

func NewDeltaBaseTermStateSerializer() *DeltaBaseTermStateSerializer {
	return &DeltaBaseTermStateSerializer{}
}

func (s *DeltaBaseTermStateSerializer) ResetBaseStartFP() {
	s.baseDocStartFP = 0
	s.basePosStartFP = 0
	s.basePayStartFP = 0
}

func (s *DeltaBaseTermStateSerializer) GetBaseDocStartFP() int64 { return s.baseDocStartFP }
func (s *DeltaBaseTermStateSerializer) GetBasePosStartFP() int64 { return s.basePosStartFP }
func (s *DeltaBaseTermStateSerializer) GetBasePayStartFP() int64 { return s.basePayStartFP }

// WriteTermState writes a BlockTermState to the provided output.
//
// Simpler variant of Lucene104PostingsWriter.encodeTerm(DataOutput, FieldInfo,
// BlockTermState, boolean). Mirrors
// DeltaBaseTermStateSerializer.writeTermState(DataOutput, FieldInfo,
// BlockTermState).
func (s *DeltaBaseTermStateSerializer) WriteTermState(out store.DataOutput, fieldInfo *index.FieldInfo, termState index.TermState) error {
	opts := fieldInfo.IndexOptions()
	hasFreqs := opts != index.IndexOptionsDocs
	hasPositions := opts >= index.IndexOptionsDocsAndFreqsAndPositions
	hasOffsets := opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	hasPayloads := fieldInfo.HasPayloads()

	// Mirrors "IntBlockTermState intTermState = (IntBlockTermState) termState".
	ts, ok := termState.(*codecs.IntBlockTermState)
	if !ok {
		return fmt.Errorf("DeltaBaseTermStateSerializer.WriteTermState: term state is %T, want *codecs.IntBlockTermState", termState)
	}

	if err := out.WriteVInt(int32(ts.DocFreq)); err != nil {
		return err
	}
	if hasFreqs {
		if err := out.WriteVLong(ts.TotalTermFreq - int64(ts.DocFreq)); err != nil {
			return err
		}
	}

	if ts.SingletonDocID != -1 {
		if err := out.WriteVInt(int32(ts.SingletonDocID)); err != nil {
			return err
		}
	} else {
		if s.baseDocStartFP == 0 {
			s.baseDocStartFP = ts.DocStartFP
		}
		if err := out.WriteVLong(ts.DocStartFP - s.baseDocStartFP); err != nil {
			return err
		}
	}

	if hasPositions {
		if s.basePosStartFP == 0 {
			s.basePosStartFP = ts.PosStartFP
		}
		if err := out.WriteVLong(ts.PosStartFP - s.basePosStartFP); err != nil {
			return err
		}
		if hasPayloads || hasOffsets {
			if s.basePayStartFP == 0 {
				s.basePayStartFP = ts.PayStartFP
			}
			if err := out.WriteVLong(ts.PayStartFP - s.basePayStartFP); err != nil {
				return err
			}
		}
		if ts.LastPosBlockOffset != -1 {
			if err := out.WriteVLong(ts.LastPosBlockOffset); err != nil {
				return err
			}
		}
	}
	return nil
}

// ReadTermState reads a BlockTermState from the provided input.
//
// Simpler variant of Lucene104PostingsReader.decodeTerm(DataInput, FieldInfo,
// BlockTermState, boolean). reuse is a BlockTermState to reuse, or nil to
// create a new one. Mirrors DeltaBaseTermStateSerializer.readTermState.
func (s *DeltaBaseTermStateSerializer) ReadTermState(
	baseDocStartFP, basePosStartFP, basePayStartFP int64,
	in store.DataInput,
	fieldInfo *index.FieldInfo,
	reuse index.TermState,
) (index.TermState, error) {
	opts := fieldInfo.IndexOptions()
	hasFreqs := opts != index.IndexOptionsDocs
	hasPositions := opts >= index.IndexOptionsDocsAndFreqsAndPositions

	// Mirrors "reuse != null ? reset((IntBlockTermState) reuse) : new IntBlockTermState()".
	var ts *codecs.IntBlockTermState
	if reuse == nil {
		ts = codecs.NewIntBlockTermState()
	} else {
		r, ok := reuse.(*codecs.IntBlockTermState)
		if !ok {
			return nil, fmt.Errorf("DeltaBaseTermStateSerializer.ReadTermState: reuse is %T, want *codecs.IntBlockTermState", reuse)
		}
		ts = s.reset(r)
	}

	docFreq, err := in.ReadVInt()
	if err != nil {
		return nil, err
	}
	ts.DocFreq = int(docFreq)

	if hasFreqs {
		deltaTF, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		ts.TotalTermFreq = int64(docFreq) + deltaTF
	} else {
		ts.TotalTermFreq = int64(docFreq)
	}

	if ts.DocFreq == 1 {
		singletonID, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		ts.SingletonDocID = int(singletonID)
	} else {
		deltaDocFP, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		ts.DocStartFP = baseDocStartFP + deltaDocFP
	}

	if hasPositions {
		deltaPosFP, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		ts.PosStartFP = basePosStartFP + deltaPosFP

		hasOffsets := opts >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
		if hasOffsets || fieldInfo.HasPayloads() {
			deltaPayFP, err := in.ReadVLong()
			if err != nil {
				return nil, err
			}
			ts.PayStartFP = basePayStartFP + deltaPayFP
		}

		// Lucene104PostingsFormat.BLOCK_SIZE, which is ForUtil.BLOCK_SIZE.
		if ts.TotalTermFreq > int64(codecs.ForUtilBlockSize) {
			lastPosOff, err := in.ReadVLong()
			if err != nil {
				return nil, err
			}
			ts.LastPosBlockOffset = lastPosOff
		}
	}

	return ts, nil
}

// reset clears every field of termState back to its constructor value.
//
// Mirrors DeltaBaseTermStateSerializer.reset(IntBlockTermState).
func (s *DeltaBaseTermStateSerializer) reset(termState *codecs.IntBlockTermState) *codecs.IntBlockTermState {
	// OrdTermState.
	termState.Ord = 0

	// BlockTermState.
	termState.DocFreq = 0
	termState.TotalTermFreq = 0
	termState.TermBlockOrd = 0
	termState.BlockFilePointer = 0

	// IntBlockTermState.
	termState.DocStartFP = 0
	termState.PosStartFP = 0
	termState.PayStartFP = 0
	termState.LastPosBlockOffset = -1
	termState.SingletonDocID = -1

	return termState
}
