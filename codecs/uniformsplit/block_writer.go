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
// BlockWriterOverrides is the set of protected BlockWriter methods that Apache
// Lucene 10.5.0 subclasses of BlockWriter override and that BlockWriter's own
// bodies then invoke on `this` — the constructor calls
// createBlockLineSerializer() (BlockWriter.java:91), writeBlock calls
// writeBlockLine() and updateFieldMetadata() (BlockWriter.java:211, 243).
//
// Java resolves those calls virtually. Go embedding does not, so BlockWriter
// keeps a back-pointer to the most-derived instance in [BlockWriter.Overrides]
// and makes the calls through it. See [BlockReaderOverrides] for the same
// mechanism on the read side.
//
// Every member is declared `protected` by
// org.apache.lucene.codecs.uniformsplit.BlockWriter, so its exported Go
// spelling is the rendering of `protected`: reachable by subclasses that live
// in another package, exactly as
// org.apache.lucene.codecs.uniformsplit.sharedterms.STBlockWriter reaches them.
type BlockWriterOverrides interface {
	// CreateBlockLineSerializer mirrors
	// BlockWriter.createBlockLineSerializer (BlockWriter.java:106).
	CreateBlockLineSerializer() *BlockLineSerializer

	// UpdateFieldMetadata mirrors BlockWriter.updateFieldMetadata
	// (BlockWriter.java:247).
	UpdateFieldMetadata(blockStartFP int64)

	// WriteBlockLine mirrors BlockWriter.writeBlockLine
	// (BlockWriter.java:259).
	WriteBlockLine(isIncrementalEncodingSeed bool, line *BlockLine, previousLine *BlockLine) error
}

type BlockWriter struct {
	// Overrides is the back-pointer to the most-derived instance, through
	// which BlockWriter makes the calls Java resolves virtually. See
	// [BlockWriterOverrides]. NewBlockWriter sets it to the BlockWriter
	// itself; a subclass constructor overwrites it with the subclass.
	Overrides BlockWriterOverrides

	TargetNumBlockLines int
	DeltaNumLines       int
	BlockLines          []*BlockLine

	BlockOutput           store.IndexOutput
	BlockLinesWriteBuffer *store.ByteBuffersDataOutput
	TermStatesWriteBuffer *store.ByteBuffersDataOutput

	BlockHeaderWriter   *BlockHeaderSerializer
	BlockLineWriter     *BlockLineSerializer
	TermStateSerializer *DeltaBaseTermStateSerializer
	BlockEncoder        BlockEncoder
	BlockWriteBuffer    *store.ByteBuffersDataOutput

	FieldMetadata *FieldMetadata
	LastTerm      *util.BytesRef

	ReusableBlockHeader *BlockHeader
	ScratchBytesRef     *util.BytesRef
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
		BlockOutput:         blockOutput,
		TargetNumBlockLines: targetNumBlockLines,
		DeltaNumLines:       deltaNumLines,
		BlockEncoder:        blockEncoder,
	}
	w.BlockLines = make([]*BlockLine, 0, targetNumBlockLines)
	w.BlockHeaderWriter = w.CreateBlockHeaderSerializer()
	w.Overrides = w
	w.BlockLineWriter = w.Overrides.CreateBlockLineSerializer()
	w.TermStateSerializer = w.CreateDeltaBaseTermStateSerializer()

	w.BlockLinesWriteBuffer = store.NewByteBuffersDataOutput()
	w.TermStatesWriteBuffer = store.NewByteBuffersDataOutput()
	w.BlockWriteBuffer = store.NewByteBuffersDataOutput()

	w.ReusableBlockHeader = &BlockHeader{}
	w.ScratchBytesRef = util.NewBytesRefEmpty()
	return w
}

// CreateBlockHeaderSerializer mirrors BlockWriter.createBlockHeaderSerializer
// (BlockWriter.java:99).
func (w *BlockWriter) CreateBlockHeaderSerializer() *BlockHeaderSerializer {
	return &BlockHeaderSerializer{}
}

// CreateBlockLineSerializer mirrors BlockWriter.createBlockLineSerializer
// (BlockWriter.java:103).
func (w *BlockWriter) CreateBlockLineSerializer() *BlockLineSerializer {
	return NewBlockLineSerializer()
}

// CreateDeltaBaseTermStateSerializer mirrors
// BlockWriter.createDeltaBaseTermStateSerializer (BlockWriter.java:107).
func (w *BlockWriter) CreateDeltaBaseTermStateSerializer() *DeltaBaseTermStateSerializer {
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
	mdpLength, err := ComputeMdpLength(w.LastTerm, term)
	if err != nil {
		return err
	}
	w.BlockLines = append(w.BlockLines, NewBlockLineForWriting(NewTermBytes(mdpLength, term), blockTermState))
	w.LastTerm = term
	if len(w.BlockLines) >= w.TargetNumBlockLines+w.DeltaNumLines {
		return w.SplitAndWriteBlock(dictionaryBuilder)
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
	for len(w.BlockLines) != 0 {
		if err := w.SplitAndWriteBlock(dictionaryBuilder); err != nil {
			return err
		}
	}
	w.FieldMetadata = nil
	w.LastTerm = nil
	return nil
}

// SplitAndWriteBlock defines the new block start according to
// targetNumBlockLines and deltaNumLines. The new block is started (including
// one or more of the lastly added lines), the current block is written to the
// block file, and the current block key is added to the
// IndexDictionaryBuilder.
//
// Mirrors BlockWriter.splitAndWriteBlock (BlockWriter.java:157).
func (w *BlockWriter) SplitAndWriteBlock(dictionaryBuilder IndexDictionaryBuilder) error {
	// assert !blockLines.isEmpty();
	numLines := len(w.BlockLines)

	if numLines <= w.TargetNumBlockLines-w.DeltaNumLines {
		if err := w.WriteBlock(w.BlockLines, dictionaryBuilder); err != nil {
			return err
		}
		w.BlockLines = w.BlockLines[:0]
		return nil
	}
	deltaStart := numLines - w.DeltaNumLines*2
	// assert deltaStart >= 1 : "blockLines size: " + numLines;
	minMdpLength := math.MaxInt32
	minMdpEndIndex := 0

	for i := deltaStart; i < numLines; i++ {
		term := w.BlockLines[i].GetTermBytes()
		mdpLength := term.GetMdpLength()
		if mdpLength <= minMdpLength {
			minMdpLength = mdpLength
			minMdpEndIndex = i
		}
	}

	subList := w.BlockLines[0:minMdpEndIndex]
	if err := w.WriteBlock(subList, dictionaryBuilder); err != nil {
		return err
	}
	// Clear the written block lines to keep only the lines composing the next block.
	// Java calls ArrayList.subList().clear(), which shifts the remaining
	// elements down inside the same backing array; the append below does the
	// same shift on the same backing array.
	w.BlockLines = append(w.BlockLines[:0], w.BlockLines[minMdpEndIndex:]...)
	return nil
}

// WriteBlock writes a block and adds its block key to the dictionary builder.
//
// Mirrors BlockWriter.writeBlock (BlockWriter.java:188).
func (w *BlockWriter) WriteBlock(blockLines []*BlockLine, dictionaryBuilder IndexDictionaryBuilder) error {
	blockStartFP := w.BlockOutput.GetFilePointer()

	if err := w.AddBlockKey(blockLines, dictionaryBuilder); err != nil {
		return err
	}

	middle := len(blockLines) >> 1
	middleOffset := int32(-1)
	var previousLine *BlockLine
	for i, size := 0, len(blockLines); i < size; i++ {
		isIncrementalEncodingSeed := i == 0
		if i == middle {
			offset, err := blockWriterToIntExact(w.BlockLinesWriteBuffer.Size())
			if err != nil {
				return err
			}
			middleOffset = offset
			isIncrementalEncodingSeed = true
		}
		line := blockLines[i]
		if err := w.Overrides.WriteBlockLine(isIncrementalEncodingSeed, line, previousLine); err != nil {
			return err
		}
		previousLine = line
	}

	termStatesBaseOffset, err := blockWriterToIntExact(w.BlockLinesWriteBuffer.Size())
	if err != nil {
		return err
	}
	w.ReusableBlockHeader.Reset(
		int32(len(blockLines)),
		w.TermStateSerializer.GetBaseDocStartFP(),
		w.TermStateSerializer.GetBasePosStartFP(),
		w.TermStateSerializer.GetBasePayStartFP(),
		termStatesBaseOffset,
		middleOffset)
	if err := w.BlockHeaderWriter.Write(w.BlockWriteBuffer, w.ReusableBlockHeader); err != nil {
		return err
	}

	if err := w.BlockLinesWriteBuffer.CopyTo(w.BlockWriteBuffer); err != nil {
		return err
	}
	if err := w.TermStatesWriteBuffer.CopyTo(w.BlockWriteBuffer); err != nil {
		return err
	}

	if w.BlockEncoder == nil {
		numBytes, err := blockWriterToIntExact(w.BlockWriteBuffer.Size())
		if err != nil {
			return err
		}
		if err := w.BlockOutput.WriteVInt(numBytes); err != nil {
			return err
		}
		if err := w.BlockWriteBuffer.CopyTo(w.BlockOutput); err != nil {
			return err
		}
	} else {
		encodedBytes, err := w.BlockEncoder.Encode(w.BlockWriteBuffer.ToDataInput(), w.BlockWriteBuffer.Size())
		if err != nil {
			return err
		}
		numBytes, err := blockWriterToIntExact(encodedBytes.Size())
		if err != nil {
			return err
		}
		if err := w.BlockOutput.WriteVInt(numBytes); err != nil {
			return err
		}
		if err := encodedBytes.WriteTo(w.BlockOutput); err != nil {
			return err
		}
	}

	w.BlockLinesWriteBuffer.Reset()
	w.TermStatesWriteBuffer.Reset()
	w.BlockWriteBuffer.Reset()

	w.TermStateSerializer.ResetBaseStartFP()

	w.Overrides.UpdateFieldMetadata(blockStartFP)
	return nil
}

// UpdateFieldMetadata updates the field metadata after all lines were written
// for the block.
//
// Mirrors BlockWriter.updateFieldMetadata (BlockWriter.java:241).
func (w *BlockWriter) UpdateFieldMetadata(blockStartFP int64) {
	// assert fieldMetadata != null;
	if w.FieldMetadata.GetFirstBlockStartFP() == -1 {
		w.FieldMetadata.SetFirstBlockStartFP(blockStartFP)
	}
	w.FieldMetadata.SetLastBlockStartFP(blockStartFP)
}

// SetField sets the field metadata the following lines belong to. Mirrors the
// package-private BlockWriter.setField (BlockWriter.java:249).
func (w *BlockWriter) SetField(fieldMetadata *FieldMetadata) {
	w.FieldMetadata = fieldMetadata
}

// WriteBlockLine mirrors BlockWriter.writeBlockLine (BlockWriter.java:253).
func (w *BlockWriter) WriteBlockLine(isIncrementalEncodingSeed bool, line *BlockLine, previousLine *BlockLine) error {
	// assert fieldMetadata != null;
	termStateRelativeOffset, err := blockWriterToIntExact(w.TermStatesWriteBuffer.Size())
	if err != nil {
		return err
	}
	if err := w.BlockLineWriter.WriteLine(
		w.BlockLinesWriteBuffer,
		line,
		previousLine,
		termStateRelativeOffset,
		isIncrementalEncodingSeed); err != nil {
		return err
	}
	return w.BlockLineWriter.WriteLineTermState(
		w.TermStatesWriteBuffer, line, w.FieldMetadata.GetFieldInfo(), w.TermStateSerializer)
}

// AddBlockKey adds a new block key with its corresponding block file pointer to
// the IndexDictionaryBuilder. The block key is the MDP (see TermBytes) of the
// block first term.
//
// Mirrors BlockWriter.addBlockKey (BlockWriter.java:271).
func (w *BlockWriter) AddBlockKey(blockLines []*BlockLine, dictionaryBuilder IndexDictionaryBuilder) error {
	// assert !blockLines.isEmpty();
	// assert dictionaryBuilder != null;
	firstTerm := blockLines[0].GetTermBytes()
	// assert firstTerm.getTerm().offset == 0;
	// assert scratchBytesRef.offset == 0;
	w.ScratchBytesRef.Bytes = firstTerm.GetTerm().Bytes
	w.ScratchBytesRef.Length = firstTerm.GetMdpLength()
	return dictionaryBuilder.Add(w.ScratchBytesRef, w.BlockOutput.GetFilePointer())
}

// blockWriterToIntExact renders java.lang.Math.toIntExact: it narrows a long to
// an int and fails instead of truncating.
func blockWriterToIntExact(value int64) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("integer overflow: %d does not fit in an int", value)
	}
	return int32(value), nil
}
