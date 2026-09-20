// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// STBlockWriter writes terms blocks with the Shared Terms format.
//
// As defined in STUniformSplitTermsWriter, all the fields terms are shared in
// the same dictionary. Each block line contains a term and all the fields
// TermStates for this term.
//
// Mirrors org.apache.lucene.codecs.uniformsplit.sharedterms.STBlockWriter from
// Apache Lucene 10.5.0, which extends
// org.apache.lucene.codecs.uniformsplit.BlockWriter.
type STBlockWriter struct {
	*uniformsplit.BlockWriter

	// FieldsInBlock mirrors the protected final field
	// STBlockWriter.fieldsInBlock (STBlockWriter.java:41), a
	// HashSet<FieldMetadata>. FieldMetadata overrides neither equals nor
	// hashCode, so the Java set is keyed by identity; a Go map keyed by the
	// pointer is that same set.
	FieldsInBlock map[*uniformsplit.FieldMetadata]struct{}

	// stBlockLineWriter is the STBlockLine.Serializer that
	// CreateBlockLineSerializer installed in BlockWriter.BlockLineWriter. It
	// renders the Java downcast `(STBlockLine.Serializer) blockLineWriter`
	// (STBlockWriter.java:93); see STBlockReader.stBlockLineReader.
	stBlockLineWriter *STBlockLineSerializer
}

// NewSTBlockWriter mirrors the public STBlockWriter(IndexOutput, int, int,
// BlockEncoder) constructor (STBlockWriter.java:43).
func NewSTBlockWriter(
	blockOutput store.IndexOutput,
	targetNumBlockLines int,
	deltaNumLines int,
	blockEncoder uniformsplit.BlockEncoder,
) *STBlockWriter {
	w := &STBlockWriter{
		BlockWriter:   uniformsplit.NewBlockWriter(blockOutput, targetNumBlockLines, deltaNumLines, blockEncoder),
		FieldsInBlock: make(map[*uniformsplit.FieldMetadata]struct{}),
	}
	// Java's `this` inside BlockWriter's bodies is the STBlockWriter being
	// constructed; see uniformsplit.BlockWriterOverrides. Java's BlockWriter
	// constructor resolves createBlockLineSerializer() virtually on that
	// `this` (BlockWriter.java:91); Go has no subclass instance while the
	// base constructor runs, so the call is re-run here and the
	// BlockLine.Serializer the base allocated — which carries no state yet —
	// is replaced by the STBlockLine.Serializer Java would have installed.
	w.Overrides = w
	w.BlockLineWriter = w.CreateBlockLineSerializer()
	return w
}

// AddLine adds a new BlockLine term for the current field.
//
// This method determines whether the new term is part of the current block, or
// if it is part of the next block. In the latter case, a new block is started
// (including one or more of the lastly added lines), the current block is
// written to the block file, and the current block key is added to the
// uniformsplit.IndexDictionaryBuilder.
//
// term is the block line term. The util.BytesRef instance is used directly,
// the caller is responsible to make a deep copy if needed. This is required
// because we keep a list of block lines until we decide to write the current
// block, and each line must have a different term instance. termStates holds
// the block line details for all fields in the line. dictionaryBuilder is the
// builder to which the block keys are added.
//
// Mirrors STBlockWriter.addLine(BytesRef, List<FieldMetadataTermState>,
// IndexDictionary.Builder) (STBlockWriter.java:69). Java overloads
// BlockWriter.addLine; Go has no overloading, so this declaration shadows the
// inherited one, exactly as the Java overload hides nothing and is selected by
// its argument types.
func (w *STBlockWriter) AddLine(
	term *util.BytesRef,
	termStates []*FieldMetadataTermState,
	dictionaryBuilder uniformsplit.IndexDictionaryBuilder,
) error {
	if len(termStates) == 0 {
		return nil
	}
	mdpLength, err := uniformsplit.ComputeMdpLength(w.LastTerm, term)
	if err != nil {
		return err
	}
	line := NewSTBlockLine(uniformsplit.NewTermBytes(mdpLength, term), termStates)
	w.BlockLines = append(w.BlockLines, line.BlockLine)
	w.LastTerm = term
	if len(w.BlockLines) >= w.TargetNumBlockLines+w.DeltaNumLines {
		return w.SplitAndWriteBlock(dictionaryBuilder)
	}
	return nil
}

// FinishLastBlock mirrors STBlockWriter.finishLastBlock
// (STBlockWriter.java:84), whose body is `super.finishLastBlock(...)` and
// whose only purpose, per its Lucene comment, is to "make this method
// accessible to package". BlockWriter.FinishLastBlock is already exported in
// Go, so the override adds no behaviour; it is declared to keep the Java
// member set complete.
func (w *STBlockWriter) FinishLastBlock(dictionaryBuilder uniformsplit.IndexDictionaryBuilder) error {
	return w.BlockWriter.FinishLastBlock(dictionaryBuilder)
}

// CreateBlockLineSerializer mirrors
// STBlockWriter.createBlockLineSerializer (STBlockWriter.java:90), whose body
// is `return new STBlockLine.Serializer()`. The Go return type is the base
// uniformsplit.BlockLineSerializer that BlockWriter.BlockLineWriter holds; the
// subclass view is kept in stBlockLineWriter for the Java downcast.
func (w *STBlockWriter) CreateBlockLineSerializer() *uniformsplit.BlockLineSerializer {
	w.stBlockLineWriter = NewSTBlockLineSerializer()
	return w.stBlockLineWriter.BlockLineSerializer
}

// WriteBlockLine mirrors STBlockWriter.writeBlockLine(boolean, BlockLine,
// BlockLine) (STBlockWriter.java:95).
func (w *STBlockWriter) WriteBlockLine(
	isIncrementalEncodingSeed bool,
	line *uniformsplit.BlockLine,
	previousLine *uniformsplit.BlockLine,
) error {
	termStateRelativeOffset, err := stBlockWriterToIntExact(w.TermStatesWriteBuffer.Size())
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
	// Renders Java's `(STBlockLine) line` downcast; see
	// uniformsplit.BlockLine.Derived.
	stLine := line.Derived.(*STBlockLine)
	if err := w.stBlockLineWriter.WriteLineTermStates(
		w.TermStatesWriteBuffer, stLine, w.TermStateSerializer); err != nil {
		return err
	}
	stLine.CollectFields(w.FieldsInBlock)
	return nil
}

// UpdateFieldMetadata mirrors STBlockWriter.updateFieldMetadata(long)
// (STBlockWriter.java:108).
func (w *STBlockWriter) UpdateFieldMetadata(blockStartFP int64) {
	// assert !fieldsInBlock.isEmpty();
	for fieldMetadata := range w.FieldsInBlock {
		if fieldMetadata.GetFirstBlockStartFP() == -1 {
			fieldMetadata.SetFirstBlockStartFP(blockStartFP)
		}
		fieldMetadata.SetLastBlockStartFP(blockStartFP)
	}
	clear(w.FieldsInBlock)
}

// stBlockWriterToIntExact renders java.lang.Math.toIntExact, which
// STBlockWriter.writeBlockLine applies to the write buffer size
// (STBlockWriter.java:99). Java throws ArithmeticException on overflow;
// Gocene reports it as an error.
func stBlockWriterToIntExact(value int64) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, fmt.Errorf("integer overflow: %d does not fit in an int", value)
	}
	return int32(value), nil
}

var _ uniformsplit.BlockWriterOverrides = (*STBlockWriter)(nil)
