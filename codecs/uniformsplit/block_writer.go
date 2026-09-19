// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockWriter writes blocks in the block file.
//
// According to the Uniform Split technique, the writing combines three steps
// per block, and it is repeated for all the field blocks:
//
//  1. Select the term with the shortest minimal distinguishing prefix (MDP, see
//     TermBytes) in the neighborhood of the targetNumBlockLines target block
//     size (+- deltaNumLines delta size);
//  2. The selected term becomes the first term of the next block, and its MDP
//     is the next block key;
//  3. The current block is written to the block file. And its block key is
//     added to the IndexDictionaryBuilder.
//
// This stateful BlockWriter is called repeatedly to AddLine all the BlockLine
// terms of a field. Then FinishLastBlock is called. And then this BlockWriter
// can be reused to add the terms of another field.
//
// Mirrors org.apache.lucene.codecs.uniformsplit.BlockWriter from Apache Lucene
// 10.5.0.
type BlockWriter struct {
	targetNumBlockLines int
	deltaNumLines       int
	blockLines          []*BlockLine

	blockOutput           store.IndexOutput
	blockLinesWriteBuffer *store.ByteBuffersDataOutput
	termStatesWriteBuffer *store.ByteBuffersDataOutput

	blockHeaderWriter   *BlockHeaderSerializer
	blockLineWriter     *BlockLineSerializer
	termStateSerializer *DeltaBaseTermStateSerializer
	blockEncoder        BlockEncoder
	blockWriteBuffer    *store.ByteBuffersDataOutput

	fieldMetadata *FieldMetadata
	lastTerm      *util.BytesRef

	reusableBlockHeader *BlockHeader
	scratchBytesRef     *util.BytesRef
}

// NewBlockWriter constructs a BlockWriter.
//
// Mirrors the protected BlockWriter constructor (BlockWriter.java:73). Java's
// ByteBuffersDataOutput.newResettableInstance() only adds buffer recycling to
// the same output; Gocene's store package exposes no recycling constructor, so
// the plain instance is used and the written bytes are unchanged.
func NewBlockWriter(
	blockOutput store.IndexOutput,
	targetNumBlockLines int,
	deltaNumLines int,
	blockEncoder BlockEncoder,
) *BlockWriter {
	// assert blockOutput != null;
	// assert targetNumBlockLines > 0;
	// assert deltaNumLines >= 0;
	// assert deltaNumLines < targetNumBlockLines;
	w := &BlockWriter{
		blockOutput:         blockOutput,
		targetNumBlockLines: targetNumBlockLines,
		deltaNumLines:       deltaNumLines,
		blockEncoder:        blockEncoder,
	}
	w.blockLines = make([]*BlockLine, 0, targetNumBlockLines)
	w.blockHeaderWriter = w.createBlockHeaderSerializer()
	w.blockLineWriter = w.createBlockLineSerializer()
	w.termStateSerializer = w.createDeltaBaseTermStateSerializer()

	w.blockLinesWriteBuffer = store.NewByteBuffersDataOutput()
	w.termStatesWriteBuffer = store.NewByteBuffersDataOutput()
	w.blockWriteBuffer = store.NewByteBuffersDataOutput()

	w.reusableBlockHeader = &BlockHeader{}
	w.scratchBytesRef = util.NewBytesRefEmpty()
	return w
}

// createBlockHeaderSerializer mirrors BlockWriter.createBlockHeaderSerializer
// (BlockWriter.java:99).
func (w *BlockWriter) createBlockHeaderSerializer() *BlockHeaderSerializer {
	return &BlockHeaderSerializer{}
}

// createBlockLineSerializer mirrors BlockWriter.createBlockLineSerializer
// (BlockWriter.java:103).
func (w *BlockWriter) createBlockLineSerializer() *BlockLineSerializer {
	return NewBlockLineSerializer()
}

// createDeltaBaseTermStateSerializer mirrors
// BlockWriter.createDeltaBaseTermStateSerializer (BlockWriter.java:107).
func (w *BlockWriter) createDeltaBaseTermStateSerializer() *DeltaBaseTermStateSerializer {
	return NewDeltaBaseTermStateSerializer()
}

// AddLine adds a new BlockLine term for the current field.
//
// This method determines whether the new term is part of the current block, or
// if it is part of the next block. In the latter case, a new block is started
// (including one or more of the lastly added lines), the current block is
// written to the block file, and the current block key is added to the
// IndexDictionaryBuilder.
//
// term is the block line term. The BytesRef instance is used directly, the
// caller is responsible to make a deep copy if needed. This is required because
// we keep a list of block lines until we decide to write the current block, and
// each line must have a different term instance. blockTermState holds the block
// line details. dictionaryBuilder is the builder to which the block keys are
// added.
//
// Mirrors BlockWriter.addLine (BlockWriter.java:126).
func (w *BlockWriter) AddLine(term *util.BytesRef, blockTermState index.TermState, dictionaryBuilder IndexDictionaryBuilder) error {
	// assert term != null;
	// assert blockTermState != null;
	mdpLength, err := ComputeMdpLength(w.lastTerm, term)
	if err != nil {
		return err
	}
	w.blockLines = append(w.blockLines, NewBlockLineForWriting(NewTermBytes(mdpLength, term), blockTermState))
	w.lastTerm = term
	if len(w.blockLines) >= w.targetNumBlockLines+w.deltaNumLines {
		return w.splitAndWriteBlock(dictionaryBuilder)
	}
	return nil
}

// FinishLastBlock is called when there is no more term for the field. It writes
// the remaining lines added with AddLine as the last block of the field and
// resets this BlockWriter state. Then this BlockWriter can be used for another
// field.
//
// Mirrors BlockWriter.finishLastBlock (BlockWriter.java:143).
func (w *BlockWriter) FinishLastBlock(dictionaryBuilder IndexDictionaryBuilder) error {
	for len(w.blockLines) != 0 {
		if err := w.splitAndWriteBlock(dictionaryBuilder); err != nil {
			return err
		}
	}
	w.fieldMetadata = nil
	w.lastTerm = nil
	return nil
}

// splitAndWriteBlock defines the new block start according to
// targetNumBlockLines and deltaNumLines. The new block is started (including
// one or more of the lastly added lines), the current block is written to the
// block file, and the current block key is added to the
// IndexDictionaryBuilder.
//
// Mirrors BlockWriter.splitAndWriteBlock (BlockWriter.java:157).
func (w *BlockWriter) splitAndWriteBlock(dictionaryBuilder IndexDictionaryBuilder) error {
	// assert !blockLines.isEmpty();
	numLines := len(w.blockLines)

	if numLines <= w.targetNumBlockLines-w.deltaNumLines {
		if err := w.writeBlock(w.blockLines, dictionaryBuilder); err != nil {
			return err
		}
		w.blockLines = w.blockLines[:0]
		return nil
	}
	deltaStart := numLines - w.deltaNumLines*2
	// assert deltaStart >= 1 : "blockLines size: " + numLines;
	minMdpLength := math.MaxInt32
	minMdpEndIndex := 0

	for i := deltaStart; i < numLines; i++ {
		term := w.blockLines[i].GetTermBytes()
		mdpLength := term.GetMdpLength()
		if mdpLength <= minMdpLength {
			minMdpLength = mdpLength
			minMdpEndIndex = i
		}
	}

	subList := w.blockLines[0:minMdpEndIndex]
	if err := w.writeBlock(subList, dictionaryBuilder); err != nil {
		return err
	}
	// Clear the written block lines to keep only the lines composing the next block.
	// Java calls ArrayList.subList().clear(), which shifts the remaining
	// elements down inside the same backing array; the append below does the
	// same shift on the same backing array.
	w.blockLines = append(w.blockLines[:0], w.blockLines[minMdpEndIndex:]...)
	return nil
}

// writeBlock writes a block and adds its block key to the dictionary builder.
//
// Mirrors BlockWriter.writeBlock (BlockWriter.java:188).
func (w *BlockWriter) writeBlock(blockLines []*BlockLine, dictionaryBuilder IndexDictionaryBuilder) error {
	blockStartFP := w.blockOutput.GetFilePointer()

	if err := w.addBlockKey(blockLines, dictionaryBuilder); err != nil {
		return err
	}

	middle := len(blockLines) >> 1
	middleOffset := int32(-1)
	var previousLine *BlockLine
	for i, size := 0, len(blockLines); i < size; i++ {
		isIncrementalEncodingSeed := i == 0
		if i == middle {
			offset, err := blockWriterToIntExact(w.blockLinesWriteBuffer.Size())
			if err != nil {
				return err
			}
			middleOffset = offset
			isIncrementalEncodingSeed = true
		}
		line := blockLines[i]
		if err := w.writeBlockLine(isIncrementalEncodingSeed, line, previousLine); err != nil {
			return err
		}
		previousLine = line
	}

	termStatesBaseOffset, err := blockWriterToIntExact(w.blockLinesWriteBuffer.Size())
	if err != nil {
		return err
	}
	w.reusableBlockHeader.Reset(
		int32(len(blockLines)),
		w.termStateSerializer.GetBaseDocStartFP(),
		w.termStateSerializer.GetBasePosStartFP(),
		w.termStateSerializer.GetBasePayStartFP(),
		termStatesBaseOffset,
		middleOffset)
	if err := w.blockHeaderWriter.Write(w.blockWriteBuffer, w.reusableBlockHeader); err != nil {
		return err
	}

	if err := w.blockLinesWriteBuffer.CopyTo(w.blockWriteBuffer); err != nil {
		return err
	}
	if err := w.termStatesWriteBuffer.CopyTo(w.blockWriteBuffer); err != nil {
		return err
	}

	if w.blockEncoder == nil {
		numBytes, err := blockWriterToIntExact(w.blockWriteBuffer.Size())
		if err != nil {
			return err
		}
		if err := w.blockOutput.WriteVInt(numBytes); err != nil {
			return err
		}
		if err := w.blockWriteBuffer.CopyTo(w.blockOutput); err != nil {
			return err
		}
	} else {
		encodedBytes, err := w.blockEncoder.Encode(w.blockWriteBuffer.ToDataInput(), w.blockWriteBuffer.Size())
		if err != nil {
			return err
		}
		numBytes, err := blockWriterToIntExact(encodedBytes.Size())
		if err != nil {
			return err
		}
		if err := w.blockOutput.WriteVInt(numBytes); err != nil {
			return err
		}
		if err := encodedBytes.WriteTo(w.blockOutput); err != nil {
			return err
		}
	}

	w.blockLinesWriteBuffer.Reset()
	w.termStatesWriteBuffer.Reset()
	w.blockWriteBuffer.Reset()

	w.termStateSerializer.ResetBaseStartFP()

	w.updateFieldMetadata(blockStartFP)
	return nil
}

// updateFieldMetadata updates the field metadata after all lines were written
// for the block.
//
// Mirrors BlockWriter.updateFieldMetadata (BlockWriter.java:241).
func (w *BlockWriter) updateFieldMetadata(blockStartFP int64) {
	// assert fieldMetadata != null;
	if w.fieldMetadata.GetFirstBlockStartFP() == -1 {
		w.fieldMetadata.SetFirstBlockStartFP(blockStartFP)
	}
	w.fieldMetadata.SetLastBlockStartFP(blockStartFP)
}

// SetField sets the field metadata the following lines belong to. Mirrors the
// package-private BlockWriter.setField (BlockWriter.java:249).
func (w *BlockWriter) SetField(fieldMetadata *FieldMetadata) {
	w.fieldMetadata = fieldMetadata
}

// writeBlockLine mirrors BlockWriter.writeBlockLine (BlockWriter.java:253).
func (w *BlockWriter) writeBlockLine(isIncrementalEncodingSeed bool, line *BlockLine, previousLine *BlockLine) error {
	// assert fieldMetadata != null;
	termStateRelativeOffset, err := blockWriterToIntExact(w.termStatesWriteBuffer.Size())
	if err != nil {
		return err
	}
	if err := w.blockLineWriter.WriteLine(
		w.blockLinesWriteBuffer,
		line,
		previousLine,
		termStateRelativeOffset,
		isIncrementalEncodingSeed); err != nil {
		return err
	}
	return w.blockLineWriter.WriteLineTermState(
		w.termStatesWriteBuffer, line, w.fieldMetadata.GetFieldInfo(), w.termStateSerializer)
}

// addBlockKey adds a new block key with its corresponding block file pointer to
// the IndexDictionaryBuilder. The block key is the MDP (see TermBytes) of the
// block first term.
//
// Mirrors BlockWriter.addBlockKey (BlockWriter.java:271).
func (w *BlockWriter) addBlockKey(blockLines []*BlockLine, dictionaryBuilder IndexDictionaryBuilder) error {
	// assert !blockLines.isEmpty();
	// assert dictionaryBuilder != null;
	firstTerm := blockLines[0].GetTermBytes()
	// assert firstTerm.getTerm().offset == 0;
	// assert scratchBytesRef.offset == 0;
	w.scratchBytesRef.Bytes = firstTerm.GetTerm().Bytes
	w.scratchBytesRef.Length = firstTerm.GetMdpLength()
	return dictionaryBuilder.Add(w.scratchBytesRef, w.blockOutput.GetFilePointer())
}

// blockWriterToIntExact renders java.lang.Math.toIntExact: it narrows a long to
// an int and fails instead of truncating.
func blockWriterToIntExact(value int64) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("integer overflow: %d does not fit in an int", value)
	}
	return int32(value), nil
}
