// Package uniformsplit implements org.apache.lucene.codecs.uniformsplit:
// the UniformSplit postings format and its building blocks.
package uniformsplit

import (
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func ComputeMdpLength(prev, next *util.BytesRef) int {
	if prev == nil {
		return 0
	}
	// Simplified MDP length calculation
	return 1
}

// FSTDictionary is the FST-backed IndexDictionary.
type FSTDictionary struct {
	Field  string
	Blocks int
}

func NewFSTDictionary(field string, blocks int) *FSTDictionary {
	return &FSTDictionary{Field: field, Blocks: blocks}
}

func (d *FSTDictionary) GetField() string { return d.Field }
func (d *FSTDictionary) NumBlocks() int   { return d.Blocks }

var _ IndexDictionary = (*FSTDictionary)(nil)

// BlockWriter is the streaming-encoder counterpart.
type BlockWriter struct {
	Encoder         BlockEncoder
	Output          store.DataOutput
	TargetBlockSize int
	DeltaNumLines   int

	lastTerm    *util.BytesRef
	blockLines  []*BlockLine
	writeBuffer *store.ByteBuffersDataOutput
}

func NewBlockWriter(out store.DataOutput, target, delta int, enc BlockEncoder) *BlockWriter {
	return &BlockWriter{
		Output:          out,
		TargetBlockSize: target,
		DeltaNumLines:   delta,
		Encoder:         enc,
		writeBuffer:     store.NewByteBuffersDataOutput(),
	}
}

func (w *BlockWriter) addLine(term *util.BytesRef, state []byte) {
	w.blockLines = append(w.blockLines, NewBlockLine(term, state))
	w.lastTerm = term
}

// UniformSplitPostingsFormat is the codec wrapper.
type UniformSplitPostingsFormat struct {
	TargetBlockSize int
	VersionCurrent  int
}

func NewUniformSplitPostingsFormat(targetBlockSize int) *UniformSplitPostingsFormat {
	return &UniformSplitPostingsFormat{
		TargetBlockSize: targetBlockSize,
		VersionCurrent:  1,
	}
}

type UniformSplitTerms struct {
	Field    string
	Metadata *FieldMetadata
}

type UniformSplitTermsReader struct {
	Format *UniformSplitPostingsFormat
}

type UniformSplitTermsWriter struct {
	Format *UniformSplitPostingsFormat
}
