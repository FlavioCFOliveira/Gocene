// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"fmt"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockLine is one term block line.
//
// Contains a term and its details as a BlockTermState.
//
// The line is written to the block file in two parts. The first part is the term
// followed by an offset to the details region. The second part is the term
// BlockTermState, written in the details region, after all the terms of the
// block.
//
// The separate details region allows fast scan of the terms without having to
// decode the details for each term. At read time, BlockLineSerializer.ReadLine
// only reads the term and its offset to the details. The corresponding
// BlockTermState is decoded on demand in the BlockReader (see
// BlockReader.readTermStateIfNotRead).
//
// Mirrors org.apache.lucene.codecs.uniformsplit.BlockLine from Apache Lucene
// 10.5.0.
type BlockLine struct {
	// Derived is the back-pointer to the most-derived instance when this
	// BlockLine is the base of a subclass instance — in Apache Lucene 10.5.0
	// the only subclass is
	// org.apache.lucene.codecs.uniformsplit.sharedterms.STBlockLine. Java
	// keeps a single object and recovers the subclass with a downcast,
	// `(STBlockLine) line` (STBlockWriter.java:92, 94), from the
	// List<BlockLine> that BlockWriter.blockLines holds
	// (BlockWriter.java:57). Go embedding makes the base a distinct object,
	// so the subclass constructor records itself here for the same recovery.
	// It is nil for a plain BlockLine.
	Derived any

	termBytes               *TermBytes
	termStateRelativeOffset int32

	// termState is only used for writing.
	termState index.TermState
}

// NewBlockLineForWriting constructs a BlockLine for writing. Mirrors the
// BlockLine(TermBytes, BlockTermState) constructor (BlockLine.java:184), which
// delegates with termStateRelativeOffset = -1.
func NewBlockLineForWriting(termBytes *TermBytes, termState index.TermState) *BlockLine {
	return newBlockLine(termBytes, -1, termState)
}

// NewBlockLine constructs a BlockLine for reading. Mirrors the
// BlockLine(TermBytes, int) constructor (BlockLine.java:189), which delegates
// with a null term state.
func NewBlockLine(termBytes *TermBytes, termStateRelativeOffset int32) *BlockLine {
	return newBlockLine(termBytes, termStateRelativeOffset, nil)
}

// newBlockLine mirrors the private BlockLine(TermBytes, int, BlockTermState)
// constructor (BlockLine.java:193).
func newBlockLine(termBytes *TermBytes, termStateRelativeOffset int32, termState index.TermState) *BlockLine {
	bl := &BlockLine{}
	bl.Reset(termBytes, termStateRelativeOffset)
	bl.termState = termState
	return bl
}

// Reset resets this BlockLine to reuse it when reading. Mirrors BlockLine.reset
// (BlockLine.java:199), which ends in `return this`.
func (bl *BlockLine) Reset(termBytes *TermBytes, termStateRelativeOffset int32) *BlockLine {
	// assert termState == null;
	bl.termBytes = termBytes
	bl.termStateRelativeOffset = termStateRelativeOffset
	return bl
}

// GetTermBytes returns the term of this line.
func (bl *BlockLine) GetTermBytes() *TermBytes {
	return bl.termBytes
}

// GetTermStateRelativeOffset returns the offset of the TermState bytes in the
// block, relatively to the term states base offset.
func (bl *BlockLine) GetTermStateRelativeOffset() int32 {
	return bl.termStateRelativeOffset
}

// BlockLineSerializer reads/writes block lines with terms encoded incrementally
// inside a block. This type keeps a state of the previous term read to decode
// the next term.
//
// Mirrors the nested class
// org.apache.lucene.codecs.uniformsplit.BlockLine.Serializer
// (BlockLine.java:227).
type BlockLineSerializer struct {
	currentTerm *util.BytesRef
}

// NewBlockLineSerializer constructs a BlockLineSerializer. Mirrors the
// Serializer() constructor (BlockLine.java:234), which allocates
// `new BytesRef(64)`.
func NewBlockLineSerializer() *BlockLineSerializer {
	return &BlockLineSerializer{
		currentTerm: util.NewBytesRefWithCapacity(64),
	}
}

// ReadLine reads the current line.
//
// isIncrementalEncodingSeed tells whether the term is a seed of the incremental
// encoding: true for the first and middle term, false for other terms. reuse is
// a BlockLine instance to reuse; or nil if none.
//
// Mirrors Serializer.readLine (BlockLine.java:245).
func (s *BlockLineSerializer) ReadLine(blockInput store.DataInput, isIncrementalEncodingSeed bool, reuse *BlockLine) (*BlockLine, error) {
	termStateRelativeOffset, err := blockInput.ReadVInt()
	if err != nil {
		return nil, err
	}
	if termStateRelativeOffset < 0 {
		return nil, index.NewCorruptIndexException(
			fmt.Sprintf("Illegal termStateRelativeOffset= %d", termStateRelativeOffset),
			fmt.Sprint(blockInput))
	}
	if reuse == nil {
		termBytes, err := s.readIncrementallyEncodedTerm(blockInput, isIncrementalEncodingSeed, nil)
		if err != nil {
			return nil, err
		}
		return NewBlockLine(termBytes, termStateRelativeOffset), nil
	}
	termBytes, err := s.readIncrementallyEncodedTerm(blockInput, isIncrementalEncodingSeed, reuse.termBytes)
	if err != nil {
		return nil, err
	}
	return reuse.Reset(termBytes, termStateRelativeOffset), nil
}

// WriteLine writes a line and its offset to the corresponding term state details
// in the details region.
//
// blockOutput is the output pointing to the block terms region.
// termStateRelativeOffset is the offset to the corresponding term state details
// in the details region. isIncrementalEncodingSeed tells whether the term is a
// seed of the incremental encoding: true for the first and middle term, false
// for other terms.
//
// Mirrors Serializer.writeLine (BlockLine.java:271).
func (s *BlockLineSerializer) WriteLine(
	blockOutput store.DataOutput,
	line *BlockLine,
	previousLine *BlockLine,
	termStateRelativeOffset int32,
	isIncrementalEncodingSeed bool,
) error {
	if err := blockOutput.WriteVInt(termStateRelativeOffset); err != nil {
		return err
	}
	var previousTermBytes *TermBytes
	if previousLine != nil {
		previousTermBytes = previousLine.GetTermBytes()
	}
	return s.writeIncrementallyEncodedTerm(line.GetTermBytes(), previousTermBytes, isIncrementalEncodingSeed, blockOutput)
}

// WriteLineTermState writes the term state details of a line in the details
// region. termStatesOutput is the output pointing to the details region.
//
// Mirrors Serializer.writeLineTermState (BlockLine.java:291).
func (s *BlockLineSerializer) WriteLineTermState(
	termStatesOutput store.DataOutput,
	line *BlockLine,
	fieldInfo *index.FieldInfo,
	encoder *DeltaBaseTermStateSerializer,
) error {
	// assert line.termState != null;
	return encoder.WriteTermState(termStatesOutput, fieldInfo, line.termState)
}

// writeIncrementallyEncodedTerm mirrors
// Serializer.writeIncrementallyEncodedTerm (BlockLine.java:301).
func (s *BlockLineSerializer) writeIncrementallyEncodedTerm(
	termBytes *TermBytes,
	previousTermBytes *TermBytes,
	isIncrementalEncodingSeed bool,
	blockOutput store.DataOutput,
) error {
	term := termBytes.GetTerm()
	// assert term.offset == 0;
	if isIncrementalEncodingSeed {
		// Mdp length is always 1 for an incremental encoding seed.
		if err := blockOutput.WriteVLong(int64(term.Length)); err != nil {
			return err
		}
		return blockOutput.WriteBytes(term.Bytes, 0, term.Length)
	}
	if term.Length == 0 {
		// Empty term.
		return blockOutput.WriteVLong(0)
	}

	// For other lines we store:
	// - Mdp length.
	// - Suffix length.
	// - Suffix bytes.
	// Instead of writing mdp length and suffix length with 2 VInt, we can compress the storage
	// by merging them in a single VLong. The idea is to leverage the information we have about
	// the previous line. We know the previous line term length. And we know that
	// new line mdp length <= (previous line term length + 1)
	// So if numMdpBits = numBitsToEncode(previous line term length),
	// then we know we can encode (new line mdp length - 1) in numMdpBits.
	// Hence we encode (new line mdp length - 1) in the rightmost numMdpBits of the VLong.
	// And we encode new line suffix length in the remaining left bits of the VLong.
	// Most of the time both values will be encoded in a single byte.

	// assert previousTermBytes != null;
	// assert termBytes.getMdpLength() >= 1;

	numMdpBits := numBitsToEncode(previousTermBytes.GetTerm().Length)
	// assert numBitsToEncode(termBytes.getMdpLength() - 1) <= numMdpBits;

	mdpAndSuffixLengths := (int64(termBytes.GetSuffixLength()) << numMdpBits) | int64(termBytes.GetMdpLength()-1)
	// assert mdpAndSuffixLengths != 0;
	if err := blockOutput.WriteVLong(mdpAndSuffixLengths); err != nil {
		return err
	}
	return blockOutput.WriteBytes(term.Bytes, termBytes.GetSuffixOffset(), termBytes.GetSuffixLength())
}

// readIncrementallyEncodedTerm mirrors
// Serializer.readIncrementallyEncodedTerm (BlockLine.java:348).
func (s *BlockLineSerializer) readIncrementallyEncodedTerm(
	blockInput store.DataInput,
	isIncrementalEncodingSeed bool,
	reuse *TermBytes,
) (*TermBytes, error) {
	// assert currentTerm.offset == 0;
	var mdpLength int
	if isIncrementalEncodingSeed {
		l, err := blockInput.ReadVLong()
		if err != nil {
			return nil, err
		}
		length := int(l)
		if length == 0 {
			mdpLength = 0
		} else {
			mdpLength = 1
		}
		if err := s.readBytes(blockInput, s.currentTerm, 0, length); err != nil {
			return nil, err
		}
	} else {
		mdpAndSuffixLengths, err := blockInput.ReadVLong()
		if err != nil {
			return nil, err
		}
		if mdpAndSuffixLengths == 0 {
			// Empty term.
			mdpLength = 0
			s.currentTerm.Length = 0
		} else {
			numMdpBits := numBitsToEncode(s.currentTerm.Length)
			// Get rightmost numMdpBits.
			mdpLength = int(mdpAndSuffixLengths&((1<<numMdpBits)-1)) + 1
			// Get remaining left bits. Java's >>> on a long is an unsigned
			// shift, rendered here by shifting the unsigned view.
			suffixLength := int(uint64(mdpAndSuffixLengths) >> numMdpBits)
			// assert mdpLength >= 1;
			// assert suffixLength >= 1;
			if err := s.readBytes(blockInput, s.currentTerm, mdpLength-1, suffixLength); err != nil {
				return nil, err
			}
		}
	}
	if reuse == nil {
		return NewTermBytes(mdpLength, s.currentTerm), nil
	}
	return reuse.Reset(mdpLength, s.currentTerm), nil
}

// readBytes reads length bytes from the given DataInput and stores them at
// offset in bytes.Bytes.
//
// Mirrors Serializer.readBytes (BlockLine.java:383). Java's
// ArrayUtil.grow(byte[], int) is util.GrowByte here.
func (s *BlockLineSerializer) readBytes(input store.DataInput, bytes *util.BytesRef, offset, length int) error {
	// assert bytes.offset == 0;
	bytes.Length = offset + length
	bytes.Bytes = util.GrowByte(bytes.Bytes, bytes.Length)
	return input.ReadBytes(bytes.Bytes, offset, length)
}

// numBitsToEncode gets the number of bits required to encode the value of the
// provided int. Returns 0 for int value 0. Equivalent to (log2(i) + 1).
//
// Mirrors the static Serializer.numBitsToEncode (BlockLine.java:400).
func numBitsToEncode(i int) int {
	return 32 - bits.LeadingZeros32(uint32(i))
}

// blockLineBaseRAMUsage renders the private static
// BASE_RAM_USAGE = RamUsageEstimator.shallowSizeOfInstance(BlockLine.class)
// (BlockLine.java:50).
var blockLineBaseRAMUsage = util.ShallowSizeOf(BlockLine{})

// RamBytesUsed mirrors BlockLine.ramBytesUsed (BlockLine.java:94).
func (bl *BlockLine) RamBytesUsed() int64 {
	return blockLineBaseRAMUsage + bl.termBytes.RamBytesUsed() + RamBytesUsedByTermState(bl.termState)
}

// blockLineSerializerBaseRAMUsage renders the private static
// BASE_RAM_USAGE = RamUsageEstimator.shallowSizeOfInstance(Serializer.class)
// of the nested class (BlockLine.java:105).
var blockLineSerializerBaseRAMUsage = util.ShallowSizeOf(BlockLineSerializer{})

// RamBytesUsed mirrors BlockLine.Serializer.ramBytesUsed
// (BlockLine.java:267).
func (s *BlockLineSerializer) RamBytesUsed() int64 {
	return blockLineSerializerBaseRAMUsage + RamBytesUsedByBytesRef(s.currentTerm)
}

// BlockLine and BlockLine.Serializer implement Accountable (BlockLine.java:48
// and BlockLine.java:103).
var (
	_ util.Accountable = (*BlockLine)(nil)
	_ util.Accountable = (*BlockLineSerializer)(nil)
)
