// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// SimpleTextPostingsFormat is a plain-text PostingsFormat for debugging.
//
// All postings data is written to a single human-readable file with the
// ".pst" extension. Intended for curiosity and debugging only; do not use
// in production.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextPostingsFormat
// (Lucene 10.4.0).
type SimpleTextPostingsFormat struct{}

// NewSimpleTextPostingsFormat constructs the format.
func NewSimpleTextPostingsFormat() *SimpleTextPostingsFormat {
	return &SimpleTextPostingsFormat{}
}

// Name returns the codec name.
func (f *SimpleTextPostingsFormat) Name() string { return "SimpleText" }

// FieldsConsumer returns a SimpleTextFieldsWriter that writes the .pst file.
//
// Port of SimpleTextPostingsFormat.fieldsConsumer(SegmentWriteState).
func (f *SimpleTextPostingsFormat) FieldsConsumer(state *codecs.SegmentWriteState) (codecs.FieldsConsumer, error) {
	return NewSimpleTextFieldsWriter(state)
}

// FieldsProducer returns a SimpleTextFieldsReader that reads the .pst file.
//
// Port of SimpleTextPostingsFormat.fieldsProducer(SegmentReadState).
func (f *SimpleTextPostingsFormat) FieldsProducer(state *codecs.SegmentReadState) (codecs.FieldsProducer, error) {
	return NewSimpleTextFieldsReader(state)
}

// compile-time assertion.
var _ codecs.PostingsFormat = (*SimpleTextPostingsFormat)(nil)
