// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package lucene912 implements the Lucene 9.12 backward-compatibility codec.
package lucene912

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// File extension constants for the Lucene 9.12 postings format.
//
// Port of the static final fields in
// org.apache.lucene.backward_codecs.lucene912.Lucene912PostingsFormat.
const (
	// MetaExtension is the extension for the small metadata file (.psm).
	MetaExtension = "psm"

	// DocExtension is the extension for the frequencies and skip data file (.doc).
	DocExtension = "doc"

	// PosExtension is the extension for the positions file (.pos).
	PosExtension = "pos"

	// PayExtension is the extension for the payloads and offsets file (.pay).
	PayExtension = "pay"

	// Level1Factor is the number of packed blocks between level-1 skip entries.
	Level1Factor = 32

	// Level1NumDocs is the total number of docs covered by one level-1 skip entry:
	// Level1Factor * BlockSize = 32 * 128 = 4096.
	Level1NumDocs = Level1Factor * BlockSize

	termsCodec = "Lucene90PostingsWriterTerms"
	metaCodec  = "Lucene912PostingsWriterMeta"
	docCodec   = "Lucene912PostingsWriterDoc"
	posCodec   = "Lucene912PostingsWriterPos"
	payCodec   = "Lucene912PostingsWriterPay"

	versionStart   = 0
	versionCurrent = versionStart
)

// Write-path bit-mask constants; unused until PostingsWriter is ported.
//
//lint:ignore U1000 write-path constant; used by Lucene912PostingsWriter skip-level logic (PostingsWriter sprint).
const level1Mask = Level1NumDocs - 1

//lint:ignore U1000 write-path constant; used by Lucene912PostingsWriter block-boundary logic (PostingsWriter sprint).
const blockMask = BlockSize - 1

// ErrWriteNotSupported is returned by FieldsConsumer when a caller
// attempts to write a Lucene 9.12 segment; this is a read-only backward
// format.
var ErrWriteNotSupported = errors.New(
	"lucene912: postings format may not be used for writing; use the current postings format",
)

// IntBlockTermState holds all per-term state required for
// Lucene912PostingsReader to produce a PostingsEnum without re-seeking the
// terms dictionary.
//
// Port of
// org.apache.lucene.backward_codecs.lucene912.Lucene912PostingsFormat.IntBlockTermState.
type IntBlockTermState struct {
	*codecs.BlockTermState

	// DocStartFP is the file pointer to the start of the document-id
	// enumeration in the .doc file.
	DocStartFP int64

	// PosStartFP is the file pointer to the start of the positions
	// enumeration in the .pos file.
	PosStartFP int64

	// PayStartFP is the file pointer to the start of the payloads
	// enumeration in the .pay file.
	PayStartFP int64

	// LastPosBlockOffset is the file offset of the last position in the
	// last block, or -1 when totalTermFreq ≤ BlockSize.
	LastPosBlockOffset int64

	// SingletonDocID holds the single document ID when docFreq == 1;
	// otherwise -1.
	SingletonDocID int
}

// NewIntBlockTermState returns an IntBlockTermState with sentinel defaults.
func NewIntBlockTermState() *IntBlockTermState {
	return &IntBlockTermState{
		BlockTermState:     codecs.NewBlockTermState(),
		LastPosBlockOffset: -1,
		SingletonDocID:     -1,
	}
}

// Lucene912PostingsFormat is the read-only Go port of
// org.apache.lucene.backward_codecs.lucene912.Lucene912PostingsFormat.
//
// It implements codecs.PostingsFormat. Writing is not supported; calling
// FieldsConsumer returns ErrWriteNotSupported. FieldsProducer requires the
// full block-tree and skip-reader infrastructure that is deferred to a later
// sprint; it currently returns a not-yet-implemented error.
type Lucene912PostingsFormat struct {
	*codecs.BasePostingsFormat
}

// NewLucene912PostingsFormat returns a Lucene912PostingsFormat instance.
func NewLucene912PostingsFormat() *Lucene912PostingsFormat {
	return &Lucene912PostingsFormat{
		BasePostingsFormat: codecs.NewBasePostingsFormat("Lucene912"),
	}
}

// FieldsConsumer returns ErrWriteNotSupported; this is a read-only format.
func (f *Lucene912PostingsFormat) FieldsConsumer(_ *codecs.SegmentWriteState) (codecs.FieldsConsumer, error) {
	return nil, ErrWriteNotSupported
}

// FieldsProducer opens the postings files for the given segment.
//
// The full Lucene90BlockTreeTermsReader port for this backward codec has not
// been completed yet. This method returns an error indicating the format is
// not available for reading.
func (f *Lucene912PostingsFormat) FieldsProducer(_ *codecs.SegmentReadState) (codecs.FieldsProducer, error) {
	return nil, errors.New("lucene912: FieldsProducer not available (backward postings format not ported)")
}
