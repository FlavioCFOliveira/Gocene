// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// PostingsFormat handles encoding/decoding of postings (term -> document mappings).
// This is the Go port of Lucene's org.apache.lucene.codecs.PostingsFormat.
//
// Postings are stored in files like _X.pst and contain the mapping from
// terms to documents, frequencies, positions, and offsets.
type PostingsFormat interface {
	// Name returns the name of this format.
	Name() string

	// FieldsConsumer returns a consumer for writing postings.
	// The caller should close the returned consumer when done.
	FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error)

	// FieldsProducer returns a producer for reading postings.
	// The caller should close the returned producer when done.
	FieldsProducer(state *SegmentReadState) (FieldsProducer, error)
}

// BasePostingsFormat provides common functionality.
type BasePostingsFormat struct {
	name string
}

// NewBasePostingsFormat creates a new BasePostingsFormat.
func NewBasePostingsFormat(name string) *BasePostingsFormat {
	return &BasePostingsFormat{name: name}
}

// Name returns the format name.
func (f *BasePostingsFormat) Name() string {
	return f.name
}

// FieldsConsumer returns a fields consumer (must be implemented by subclasses).
func (f *BasePostingsFormat) FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error) {
	return nil, fmt.Errorf("FieldsConsumer not implemented")
}

// FieldsProducer returns a fields producer (must be implemented by subclasses).
func (f *BasePostingsFormat) FieldsProducer(state *SegmentReadState) (FieldsProducer, error) {
	return nil, fmt.Errorf("FieldsProducer not implemented")
}

// Lucene104PostingsFormatName is the codec name embedded in segment metadata.
// Mirrors org.apache.lucene.codecs.lucene104.Lucene104PostingsFormat.NAME.
const Lucene104PostingsFormatName = "Lucene104"

// Lucene104PostingsFormat is the production postings format for Lucene 10.4.
//
// It wires Lucene104PostingsWriter (PFOR-delta .doc/.pos/.pay + two-level
// skip) into Lucene103BlockTreeTermsWriter (FST-based term dictionary) on
// the write side, and the symmetric reader pair on the read side.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene104.Lucene104PostingsFormat from Apache
// Lucene 10.4.0.
type Lucene104PostingsFormat struct {
	*BasePostingsFormat

	minTermBlockSize int
	maxTermBlockSize int
}

// NewLucene104PostingsFormat creates a Lucene104PostingsFormat with default
// block-tree block sizes.
func NewLucene104PostingsFormat() *Lucene104PostingsFormat {
	return NewLucene104PostingsFormatWithBlockSizes(
		Lucene103DefaultMinBlockSize,
		Lucene103DefaultMaxBlockSize,
	)
}

// NewLucene104PostingsFormatWithBlockSizes creates a Lucene104PostingsFormat
// pinned to a specific term-block size pair.
func NewLucene104PostingsFormatWithBlockSizes(minBlock, maxBlock int) *Lucene104PostingsFormat {
	return &Lucene104PostingsFormat{
		BasePostingsFormat: NewBasePostingsFormat(Lucene104PostingsFormatName),
		minTermBlockSize:   minBlock,
		maxTermBlockSize:   maxBlock,
	}
}

// FieldsConsumer returns a FieldsConsumer that wires Lucene104PostingsWriter
// through Lucene103BlockTreeTermsWriter.
//
// Mirrors Lucene104PostingsFormat.fieldsConsumer(SegmentWriteState).
func (f *Lucene104PostingsFormat) FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error) {
	postingsWriter, err := NewLucene104PostingsWriter(state)
	if err != nil {
		return nil, fmt.Errorf("Lucene104PostingsFormat.FieldsConsumer: %w", err)
	}

	btw, err := NewLucene103BlockTreeTermsWriter(
		state,
		postingsWriter,
		f.minTermBlockSize,
		f.maxTermBlockSize,
	)
	if err != nil {
		_ = postingsWriter.Close()
		return nil, fmt.Errorf("Lucene104PostingsFormat.FieldsConsumer: block-tree writer: %w", err)
	}
	return btw, nil
}

// FieldsProducer returns a FieldsProducer that wires Lucene104PostingsReader
// through Lucene103BlockTreeTermsReader.
//
// Mirrors Lucene104PostingsFormat.fieldsProducer(SegmentReadState).
func (f *Lucene104PostingsFormat) FieldsProducer(state *SegmentReadState) (FieldsProducer, error) {
	postingsReader, err := NewLucene104PostingsReader(state)
	if err != nil {
		return nil, fmt.Errorf("Lucene104PostingsFormat.FieldsProducer: %w", err)
	}

	btr, err := NewLucene103BlockTreeTermsReader(postingsReader, state)
	if err != nil {
		_ = postingsReader.Close()
		return nil, fmt.Errorf("Lucene104PostingsFormat.FieldsProducer: block-tree reader: %w", err)
	}
	return btr, nil
}

// FieldsConsumer is a consumer for writing postings.
// This is the Go port of Lucene's org.apache.lucene.codecs.FieldsConsumer.
type FieldsConsumer interface {
	// Write writes a field's postings.
	Write(field string, terms index.Terms) error

	// Close releases resources.
	Close() error
}

// FieldsProducer is a producer for reading postings.
// This is the Go port of Lucene's org.apache.lucene.codecs.FieldsProducer.
type FieldsProducer interface {
	// Terms returns the terms for a field.
	Terms(field string) (index.Terms, error)

	// Close releases resources.
	Close() error
}

// SegmentWriteState holds the state for writing a segment.
type SegmentWriteState struct {
	// Directory is where the segment files are written.
	Directory store.Directory

	// SegmentInfo contains metadata about the segment.
	SegmentInfo *index.SegmentInfo

	// FieldInfos contains metadata about all fields.
	FieldInfos *index.FieldInfos

	// SegmentSuffix is an optional suffix for segment files.
	SegmentSuffix string
}

// SegmentReadState holds the state for reading a segment.
type SegmentReadState struct {
	// Directory is where the segment files are read from.
	Directory store.Directory

	// SegmentInfo contains metadata about the segment.
	SegmentInfo *index.SegmentInfo

	// FieldInfos contains metadata about all fields.
	FieldInfos *index.FieldInfos

	// SegmentSuffix is an optional suffix for segment files.
	SegmentSuffix string
}
