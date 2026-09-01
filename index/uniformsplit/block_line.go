package uniformsplit

import (
	"fmt"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockLine represents one term block line.
//
// It contains a term and its details as a BlockTermState.
//
// The line is written to the UniformSplitPostingsFormat.TERMS_BLOCKS_EXTENSION block
// file in two parts. The first part is the term followed by an offset to the details region. The
// second part is the term BlockTermState, written in the details region, after all the
// terms of the block.
//
// The separate details region allows fast scan of the terms without having to decode the details
// for each term. At read time, the Serializer.ReadLine only reads the term and
// its offset to the details. The corresponding BlockTermState is decoded on demand in the
// BlockReader.
type BlockLine struct {
	termBytes              *TermBytes
	termStateRelativeOffset int
	// termState is only used for writing.
	termState *BlockTermState
}

// NewBlockLine creates a BlockLine for writing.
func NewBlockLine(termBytes *TermBytes, termState *BlockTermState) *BlockLine {
	return &BlockLine{
		termBytes: termBytes,
		termState: termState,
	}
}

// NewBlockLineForReading creates a BlockLine for reading.
func NewBlockLineForReading(termBytes *TermBytes, termStateRelativeOffset int) *BlockLine {
	return &BlockLine{
		termBytes:              termBytes,
		termStateRelativeOffset: termStateRelativeOffset,
	}
}

// Reset resets this BlockLine to reuse it when reading.
func (bl *BlockLine) Reset(termBytes *TermBytes, termStateRelativeOffset int) *BlockLine {
	bl.termBytes = termBytes
	bl.termStateRelativeOffset = termStateRelativeOffset
	return bl
}

// TermBytes returns the term bytes.
func (bl *BlockLine) TermBytes() *TermBytes {
	return bl.termBytes
}

// TermStateRelativeOffset returns the offset of the BlockTermState bytes in the block,
// relatively to the term states base offset.
func (bl *BlockLine) TermStateRelativeOffset() int {
	return bl.termStateRelativeOffset
}

// Serializer reads/writes block lines with terms encoded incrementally inside a block. This class keeps a
// state of the previous term read to decode the next term.
type Serializer struct {
	currentTerm *util.BytesRef
}

// NewSerializer creates a new Serializer.
func NewSerializer() *Serializer {
	return &Serializer{
		currentTerm: util.NewBytesRefWithCapacity(64),
	}
}

// ReadLine reads the current line.
//
// isIncrementalEncodingSeed: Whether the term is a seed of the incremental encoding.
// true for the first and middle term, false for other terms.
// reuse: A BlockLine instance to reuse; or nil if none.
func (s *Serializer) ReadLine(
	blockInput store.DataInput, isIncrementalEncodingSeed bool, reuse *BlockLine) (*BlockLine, error) {

	termStateRelativeOffset, err := int(blockInput.ReadVInt())
	if err != nil {
		return nil, err
	}
	if termStateRelativeOffset < 0 {
		return nil, fmt.Errorf("illegal termStateRelativeOffset= %d", termStateRelativeOffset)
	}

	termBytes, err := s.readIncrementallyEncodedTerm(blockInput, isIncrementalEncodingSeed, reuse)
	if err != nil {
		return nil, err
	}

	if reuse == nil {
		return NewBlockLineForReading(termBytes, termStateRelativeOffset), nil
	}
	return reuse.Reset(termBytes, termStateRelativeOffset), nil
}

// WriteLine writes a line and its offset to the corresponding term state details in the details region.
//
// blockOutput: The output pointing to the block terms region.
// line: The block line to write.
// previousLine: The previous block line in the block; nil if this is the first line.
// termStateRelativeOffset: The offset to the corresponding term state details in the details region.
// isIncrementalEncodingSeed: Whether the term is a seed of the incremental encoding.
// true for the first and middle term, false for other terms.
func (s *Serializer) WriteLine(
	blockOutput store.DataOutput,
	line *BlockLine,
	previousLine *BlockLine,
	termStateRelativeOffset int,
	isIncrementalEncodingSeed bool) error {

	if err := blockOutput.WriteVInt(int32(termStateRelativeOffset)); err != nil {
		return err
	}

	var previousTermBytes *TermBytes
	if previousLine != nil {
		previousTermBytes = previousLine.TermBytes()
	}

	return s.writeIncrementallyEncodedTerm(
		line.TermBytes(),
		previousTermBytes,
		isIncrementalEncodingSeed,
		blockOutput)
}

// WriteLineTermState writes the term state details of a line in the details region.
//
// termStatesOutput: The output pointing to the details region.
// line: The block line.
// fieldInfo: The field info.
// encoder: The DeltaBaseTermStateSerializer.
func (s *Serializer) WriteLineTermState(
	termStatesOutput store.DataOutput,
	line *BlockLine,
	fieldInfo index.FieldInfo,
	encoder DeltaBaseTermStateSerializer) error {

	if line.termState == nil {
		return fmt.Errorf("line.termState is nil")
	}
	return encoder.WriteTermState(termStatesOutput, fieldInfo, line.termState)
}

func (s *Serializer) writeIncrementallyEncodedTerm(
	termBytes *TermBytes,
	previousTermBytes *TermBytes,
	isIncrementalEncodingSeed bool,
	blockOutput store.DataOutput) error {

	term := termBytes.Term()
	if isIncrementalEncodingSeed {
		// Mdp length is always 1 for an incremental encoding seed.
		if err := blockOutput.WriteVLong(int64(term.Length)); err != nil {
			return err
		}
		if err := blockOutput.WriteBytes(term.ValidBytes()); err != nil {
			return err
		}
		return nil
	}

	if term.Length == 0 {
		// Empty term.
		return blockOutput.WriteVLong(0)
	}

	if previousTermBytes == nil {
		return fmt.Errorf("previousTermBytes is nil for non-seed term")
	}

	numMdpBits := numBitsToEncode(previousTermBytes.Term().Length)

	// mdpAndSuffixLengths = (suffixLength << numMdpBits) | (mdpLength - 1)
	mdpAndSuffixLengths := (int64(termBytes.SuffixLength()) << numMdpBits) | int64(termBytes.MdpLength()-1)

	if mdpAndSuffixLengths == 0 {
		return fmt.Errorf("mdpAndSuffixLengths is 0")
	}

	if err := blockOutput.WriteVLong(mdpAndSuffixLengths); err != nil {
		return err
	}

	// Write suffix bytes.
	suffix := term.ValidBytes()[termBytes.SuffixOffset():]
	return blockOutput.WriteBytes(suffix)
}

func (s *Serializer) readIncrementallyEncodedTerm(
	blockInput store.DataInput, isIncrementalEncodingSeed bool, reuse *TermBytes) (*TermBytes, error) {

	var mdpLength int
	if isIncrementalEncodingSeed {
		length, err := blockInput.ReadVLong()
		if err != nil {
			return nil, err
		}
		if length == 0 {
			mdpLength = 0
		} else {
			mdpLength = 1
		}

		// readBytes(blockInput, currentTerm, 0, length)
		termBytes := make([]byte, length)
		if err := blockInput.ReadBytes(termBytes); err != nil {
			return nil, err
		}
		s.currentTerm.Copy(termBytes)
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
			mdpLength = int(mdpAndSuffixLengths & ((1 << numMdpBits) - 1)) + 1
			suffixLength := int(mdpAndSuffixLengths >> numMdpBits)

			// readBytes(blockInput, currentTerm, mdpLength - 1, suffixLength)
			suffix := make([]byte, suffixLength)
			if err := blockInput.ReadBytes(suffix); err != nil {
				return nil, err
			}

			// Update currentTerm: [prefix from previous] + [new suffix]
			// The prefix is the first mdpLength-1 bytes of the previous currentTerm.
			prefix := s.currentTerm.ValidBytes()[:mdpLength-1]

			// Append suffix to prefix
			newBytes := append(prefix, suffix...)
			s.currentTerm.Copy(newBytes)
		}
	}

	if reuse == nil {
		return NewTermBytes(mdpLength, s.currentTerm), nil
	}
	return reuse.Reset(mdpLength, s.currentTerm), nil
}

func numBitsToEncode(i int) int {
	if i == 0 {
		return 0
	}
	return bits.Len(uint(i))
}
