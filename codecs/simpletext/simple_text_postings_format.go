// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SimpleTextPostingsFormat writes postings as text.
type SimpleTextPostingsFormat struct{}

func NewSimpleTextPostingsFormat() *SimpleTextPostingsFormat {
	return &SimpleTextPostingsFormat{}
}

func (f *SimpleTextPostingsFormat) Name() string {
	return "SimpleTextPostingsFormat"
}

func (f *SimpleTextPostingsFormat) FieldsConsumer(state *index.SegmentWriteState) (spi.FieldsConsumer, error) {
	return &simpleTextFieldsConsumer{
		state: state,
	}, nil
}

func (f *SimpleTextPostingsFormat) FieldsProducer(state *index.SegmentReadState) (spi.FieldsProducer, error) {
	return &simpleTextFieldsProducer{
		state: state,
	}, nil
}

type simpleTextFieldsConsumer struct {
	state *index.SegmentWriteState
}

func (c *simpleTextFieldsConsumer) Write(field string, terms spi.Terms) error {
	fileName := stateFileName(c.state, field)
	out, err := c.state.Directory.CreateOutput(fileName, c.state.Context)
	if err != nil {
		return err
	}
	defer out.Close()

	it := terms.Iterator()
	for {
		term := it.Next()
		if term == nil {
			break
		}
		
		postings := it.Postings(nil, 0)
		docID := postings.NextDoc()
		
		// Write term and its postings in text
		Write(out, field+": "+string(term)+" ")
		
		for docID != spi.PostingsEnumNoMoreDocs {
			Write(out, strconv.Itoa(docID))
			Write(out, " ")
			docID = postings.NextDoc()
		}
		WriteNewline(out)
	}
	return nil
}

func (c *simpleTextFieldsConsumer) Close() error {
	return nil
}

type simpleTextFieldsProducer struct {
	state *index.SegmentReadState
}

func (p *simpleTextFieldsProducer) Terms(field string) (spi.Terms, error) {
	// Simplified reader
	return nil, fmt.Errorf("SimpleTextFieldsProducer not yet implemented")
}

func (p *simpleTextFieldsProducer) Close() error {
	return nil
}

func stateFileName(state *index.SegmentWriteState, field string) string {
	return state.SegmentInfo.Name() + "." + field + ".pst"
}
