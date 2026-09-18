package uniformsplit

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// MaxNumBlockLines is the upper limit of the block size (maximum number of terms per block).
const MaxNumBlockLines = 1000

// BlockHeader contains block metadata.
//
// Holds the number of lines in the block.
//
// Holds the base file pointers to apply delta base encoding to all the file pointers in the
// block.
//
// Holds the offset to the details region of the block (the term states).
//
// Holds the offset to the middle term of the block to divide the number of terms to scan by 2.
type BlockHeader struct {
	linesCount           int32
	baseDocsFP           int64
	basePositionsFP      int64
	basePayloadsFP       int64
	termStatesBaseOffset int32
	middleLineIndex      int32
	middleLineOffset     int32
}

// NewBlockHeader creates a new BlockHeader and initializes it.
func NewBlockHeader(linesCount int32, baseDocsFP, basePositionsFP, basePayloadsFP int64, termStatesBaseOffset, middleLineOffset int32) *BlockHeader {
	bh := &BlockHeader{}
	bh.Reset(linesCount, baseDocsFP, basePositionsFP, basePayloadsFP, termStatesBaseOffset, middleLineOffset)
	return bh
}

// Reset initializes the BlockHeader fields.
func (bh *BlockHeader) Reset(linesCount int32, baseDocsFP, basePositionsFP, basePayloadsFP int64, termStatesBaseOffset, middleLineOffset int32) {
	bh.baseDocsFP = baseDocsFP
	bh.basePositionsFP = basePositionsFP
	bh.basePayloadsFP = basePayloadsFP
	bh.linesCount = linesCount
	bh.middleLineIndex = linesCount >> 1
	bh.termStatesBaseOffset = termStatesBaseOffset
	bh.middleLineOffset = middleLineOffset
}

// LinesCount returns the number of lines in the block.
func (bh *BlockHeader) LinesCount() int32 {
	return bh.linesCount
}

// MiddleLineIndex returns the index of the middle line of the block.
func (bh *BlockHeader) MiddleLineIndex() int32 {
	return bh.middleLineIndex
}

// MiddleLineOffset returns the offset to the middle line of the block, relative to the block start.
func (bh *BlockHeader) MiddleLineOffset() int32 {
	return bh.middleLineOffset
}

// TermStatesBaseOffset returns the offset to the details region of the block (the term states), relative to the block start.
func (bh *BlockHeader) TermStatesBaseOffset() int32 {
	return bh.termStatesBaseOffset
}

// BaseDocsFP returns the file pointer to the docs of the first term with docs in the block.
func (bh *BlockHeader) BaseDocsFP() int64 {
	return bh.baseDocsFP
}

// BasePositionsFP returns the file pointer to the positions of the first term with positions in the block.
func (bh *BlockHeader) BasePositionsFP() int64 {
	return bh.basePositionsFP
}

// BasePayloadsFP returns the file pointer to the payloads of the first term with payloads in the block.
func (bh *BlockHeader) BasePayloadsFP() int64 {
	return bh.basePayloadsFP
}

// BlockHeaderSerializer handles reading and writing of BlockHeader.
type BlockHeaderSerializer struct{}

// Write writes the BlockHeader to the provided DataOutput.
func (s *BlockHeaderSerializer) Write(output store.DataOutput, bh *BlockHeader) error {
	if bh.linesCount <= 0 {
		return fmt.Errorf("block header is not initialized")
	}
	if err := output.WriteVInt(bh.linesCount); err != nil {
		return err
	}
	if err := output.WriteVLong(bh.baseDocsFP); err != nil {
		return err
	}
	if err := output.WriteVLong(bh.basePositionsFP); err != nil {
		return err
	}
	if err := output.WriteVLong(bh.basePayloadsFP); err != nil {
		return err
	}
	if err := output.WriteVInt(bh.termStatesBaseOffset); err != nil {
		return err
	}
	if err := output.WriteVInt(bh.middleLineOffset); err != nil {
		return err
	}
	return nil
}

// Read reads a BlockHeader from the provided DataInput.
func (s *BlockHeaderSerializer) Read(input store.DataInput, reuse *BlockHeader) (*BlockHeader, error) {
	linesCount, err := store.ReadVInt(input)
	if err != nil {
		return nil, err
	}
	if linesCount <= 0 || linesCount > MaxNumBlockLines {
		return nil, fmt.Errorf("illegal number of lines in block: %d", linesCount)
	}

	baseDocsFP, err := input.ReadVLong()
	if err != nil {
		return nil, err
	}
	basePositionsFP, err := input.ReadVLong()
	if err != nil {
		return nil, err
	}
	basePayloadsFP, err := input.ReadVLong()
	if err != nil {
		return nil, err
	}

	termStatesBaseOffset, err := store.ReadVInt(input)
	if err != nil {
		return nil, err
	}
	if termStatesBaseOffset < 0 {
		return nil, fmt.Errorf("illegal termStatesBaseOffset= %d", termStatesBaseOffset)
	}

	middleTermOffset, err := store.ReadVInt(input)
	if err != nil {
		return nil, err
	}
	if middleTermOffset < 0 {
		return nil, fmt.Errorf("illegal middleTermOffset= %d", middleTermOffset)
	}

	bh := reuse
	if bh == nil {
		bh = &BlockHeader{}
	}
	bh.Reset(linesCount, baseDocsFP, basePositionsFP, basePayloadsFP, termStatesBaseOffset, middleTermOffset)
	return bh, nil
}
