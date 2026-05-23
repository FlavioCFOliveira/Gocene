// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// docValuesExtension is the file extension used for SimpleText doc-values files.
const docValuesExtension = "dat"

// SimpleTextDocValuesFormat is a plain-text DocValuesFormat for debugging.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextDocValuesFormat
// (Lucene 10.4.0).
//
// The on-disk layout is described in detail in the class-level Javadoc of the
// Java original (fixed-width text blocks, one section per field). Writer and
// reader implementations are provided by SimpleTextDocValuesWriter (task 3204)
// and SimpleTextDocValuesReader (task 3205).
type SimpleTextDocValuesFormat struct{}

// Name returns the codec name.
func (f *SimpleTextDocValuesFormat) Name() string { return "SimpleText" }

// NewSimpleTextDocValuesFormat constructs the format.
func NewSimpleTextDocValuesFormat() *SimpleTextDocValuesFormat {
	return &SimpleTextDocValuesFormat{}
}

// FieldsConsumer returns a DocValuesConsumer that writes the plain-text
// doc-values file (.dat).
//
// Port of SimpleTextDocValuesFormat.fieldsConsumer(SegmentWriteState).
func (f *SimpleTextDocValuesFormat) FieldsConsumer(state *codecs.SegmentWriteState) (codecs.DocValuesConsumer, error) {
	return NewSimpleTextDocValuesWriter(state, docValuesExtension)
}

// FieldsProducer returns a DocValuesProducer that reads the plain-text
// doc-values file (.dat).
//
// Port of SimpleTextDocValuesFormat.fieldsProducer(SegmentReadState).
func (f *SimpleTextDocValuesFormat) FieldsProducer(state *codecs.SegmentReadState) (codecs.DocValuesProducer, error) {
	return NewSimpleTextDocValuesReader(state, docValuesExtension)
}

// compile-time assertion.
var _ codecs.DocValuesFormat = (*SimpleTextDocValuesFormat)(nil)

// ---------------------------------------------------------------------------
// Placeholder constructors — implemented in tasks 3204 and 3205.
// These return descriptive errors rather than panicking so that integration
// code can handle missing functionality gracefully until the tasks land.
// ---------------------------------------------------------------------------

// ErrDocValuesWriterNotImplemented is returned until task 3204 lands.
var ErrDocValuesWriterNotImplemented = errors.New(
	"SimpleTextDocValuesWriter: not yet implemented (task 3204)")

// ErrDocValuesReaderNotImplemented is returned until task 3205 lands.
var ErrDocValuesReaderNotImplemented = errors.New(
	"SimpleTextDocValuesReader: not yet implemented (task 3205)")

// NewSimpleTextDocValuesWriter is the constructor called by FieldsConsumer.
// Implemented in simple_text_doc_values_writer.go (task 3204).
func NewSimpleTextDocValuesWriter(_ *codecs.SegmentWriteState, _ string) (codecs.DocValuesConsumer, error) {
	return nil, ErrDocValuesWriterNotImplemented
}

// NewSimpleTextDocValuesReader is the constructor called by FieldsProducer.
// Implemented in simple_text_doc_values_reader.go (task 3205).
func NewSimpleTextDocValuesReader(_ *codecs.SegmentReadState, _ string) (codecs.DocValuesProducer, error) {
	return nil, ErrDocValuesReaderNotImplemented
}
