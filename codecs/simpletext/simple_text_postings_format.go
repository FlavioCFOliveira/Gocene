// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// SimpleTextPostingsFormat writes postings as text.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextPostingsFormat
// (Lucene 10.5.0): "For debugging, curiosity, transparency only!! Do not use
// this codec in production." All postings data goes into one human-readable
// text file (_N.pst).
type SimpleTextPostingsFormat struct{}

// NewSimpleTextPostingsFormat builds the format.
// Port of SimpleTextPostingsFormat() (line 37), which is super("SimpleText").
func NewSimpleTextPostingsFormat() *SimpleTextPostingsFormat {
	return &SimpleTextPostingsFormat{}
}

func (f *SimpleTextPostingsFormat) Name() string {
	return "SimpleTextPostingsFormat"
}

// FieldsConsumer returns the writer for this segment.
//
// Port of SimpleTextPostingsFormat.fieldsConsumer(SegmentWriteState)
// (line 42): {@code return new SimpleTextFieldsWriter(state);}.
func (f *SimpleTextPostingsFormat) FieldsConsumer(state *index.SegmentWriteState) (spi.FieldsConsumer, error) {
	w, err := NewSimpleTextFieldsWriter(state)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// FieldsProducer returns the reader for this segment.
//
// Port of SimpleTextPostingsFormat.fieldsProducer(SegmentReadState)
// (line 47): {@code return new SimpleTextFieldsReader(state);}.
func (f *SimpleTextPostingsFormat) FieldsProducer(state *index.SegmentReadState) (spi.FieldsProducer, error) {
	r, err := NewSimpleTextFieldsReader(state)
	if err != nil {
		return nil, err
	}
	return r, nil
}
