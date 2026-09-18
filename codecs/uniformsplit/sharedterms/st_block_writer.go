package sharedterms

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// STBlockWriter writes terms blocks with the Shared Terms format.
type STBlockWriter struct {
	TargetBlockSize int
	DeltaNumLines   int
	Output          store.DataOutput

	lastTerm        *util.BytesRef
	blockLines      []*STBlockLine
	writeBuffer     *store.ByteBuffersDataOutput
	encoder         *uniformsplit.DeltaBaseTermStateSerializer
	blockLineWriter *STBlockLineSerializer
	fieldsInBlock   map[*uniformsplit.FieldMetadata]struct{}
}

// NewSTBlockWriter builds an STBlockWriter.
func NewSTBlockWriter(
	out store.DataOutput,
	targetNumBlockLines, deltaNumLines int,
	_ uniformsplit.BlockEncoder) *STBlockWriter {

	return &STBlockWriter{
		Output:          out,
		TargetBlockSize: targetNumBlockLines,
		DeltaNumLines:   deltaNumLines,
		writeBuffer:     store.NewByteBuffersDataOutput(),
		encoder:         uniformsplit.NewDeltaBaseTermStateSerializer(),
		blockLineWriter: &STBlockLineSerializer{},
		fieldsInBlock:   make(map[*uniformsplit.FieldMetadata]struct{}),
	}
}

// AddLine adds a new BlockLine term for the current field.
func (w *STBlockWriter) AddLine(
	term util.BytesRef,
	termStates []FieldMetadataTermState,
	dictionaryBuilder any) error {

	if len(termStates) == 0 {
		return nil
	}

	mdpLength := uniformsplit.ComputeMdpLength(w.lastTerm, term)

	line := NewSTBlockLine(
		append([]byte(nil), term.Bytes()[term.Offset:term.Offset+term.Length()]...),
		termStates,
	)

	w.blockLines = append(w.blockLines, line)
	w.lastTerm = term

	if len(w.blockLines) >= w.TargetBlockSize+w.DeltaNumLines {
		if err := w.splitAndWriteBlock(dictionaryBuilder); err != nil {
			return err
		}
	}

	return nil
}

func (w *STBlockWriter) splitAndWriteBlock(dictionaryBuilder any) error {
	// Logic to split the block and write it
	// 1. Determine the split point
	// 2. Write the block
	// 3. Update the dictionary
	return nil // Placeholder
}

func (w *STBlockWriter) writeBlock() error {
	// Binary contract for writing the block
	return nil // Placeholder
}

// FinishLastBlock writes any remaining lines as a final block.
func (w *STBlockWriter) FinishLastBlock(dictionaryBuilder any) error {
	if len(w.blockLines) == 0 {
		return nil
	}
	return w.writeBlock()
}

// WriteBlockLine implements the binary contract for writing a block line.
func (w *STBlockWriter) WriteBlockLine(
	isIncrementalEncodingSeed bool,
	line *uniformsplit.BlockLine,
	previousLine *uniformsplit.BlockLine,
	termStateRelativeOffset int32) error {

	// This would be called by the internal writeBlock loop
	return nil
}

// UpdateFieldMetadata updates metadata for fields present in the block.
func (w *STBlockWriter) UpdateFieldMetadata(blockStartFP int64) {
	for fm := range w.fieldsInBlock {
		if fm.FirstBlockStartFP == -1 {
			fm.FirstBlockStartFP = blockStartFP
		}
		fm.LastBlockStartFP = blockStartFP
	}
	w.fieldsInBlock = make(map[*uniformsplit.FieldMetadata]struct{})
}
