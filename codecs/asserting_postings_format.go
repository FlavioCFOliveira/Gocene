// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// AssertingPostingsFormat is a wrapper around the default postings format
// that adds additional assertions during indexing and searching.
//
// This is the Go port of Lucene's org.apache.lucene.tests.codecs.asserting.AssertingPostingsFormat.
type AssertingPostingsFormat struct {
	*BasePostingsFormat
	in PostingsFormat
}

// NewAssertingPostingsFormat creates a new AssertingPostingsFormat.
func NewAssertingPostingsFormat() *AssertingPostingsFormat {
	// Use Lucene104 as the default format
	in := NewLucene104PostingsFormat()
	return &AssertingPostingsFormat{
		BasePostingsFormat: NewBasePostingsFormat("Asserting"),
		in:                 in,
	}
}

// FieldsConsumer returns a fields consumer that wraps the underlying format's
// consumer with additional assertions.
func (f *AssertingPostingsFormat) FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error) {
	in, err := f.in.FieldsConsumer(state)
	if err != nil {
		return nil, err
	}
	return &AssertingFieldsConsumer{
		in:        in,
		writeState: state,
	}, nil
}

// FieldsProducer returns a fields producer that wraps the underlying format's
// producer with additional assertions.
func (f *AssertingPostingsFormat) FieldsProducer(state *SegmentReadState) (FieldsProducer, error) {
	in, err := f.in.FieldsProducer(state)
	if err != nil {
		return nil, err
	}
	return &AssertingFieldsProducer{
		in: in,
	}, nil
}

type AssertingFieldsProducer struct {
	in FieldsProducer
}

func (p *AssertingFieldsProducer) Terms(field string) (index.Terms, error) {
	terms, err := p.in.Terms(field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	// Wrap in AssertingTerms from index package
	return index.NewAssertingTerms(terms), nil
}

func (p *AssertingFieldsProducer) Close() error {
	err := p.in.Close()
	// Double close check as per Lucene implementation
	_ = p.in.Close()
	return err
}

func (p *AssertingFieldsProducer) Size() int {
	return p.in.Size()
}

func (p *AssertingFieldsProducer) CheckIntegrity() error {
	return p.in.CheckIntegrity()
}

func (p *AssertingFieldsProducer) GetMergeInstance() (FieldsProducer, error) {
	in, err := p.in.GetMergeInstance()
	if err != nil {
		return nil, err
	}
	return &AssertingFieldsProducer{in: in}, nil
}

func (p *AssertingFieldsProducer) String() string {
	return fmt.Sprintf("AssertingFieldsProducer(%s)", p.in.String())
}

type AssertingFieldsConsumer struct {
	in         FieldsConsumer
	writeState *SegmentWriteState
	lastField  string
	lastTerm   *util.BytesRef
}

func (c *AssertingFieldsConsumer) Write(field string, terms spi.Terms) error {
	// Write using the underlying consumer
	if err := c.in.Write(field, terms); err != nil {
		return err
	}

	// Assert field order
	if c.lastField != "" && c.lastField >= field {
		panic(fmt.Sprintf("AssertingFieldsConsumer: fields are not in sorted order: %s >= %s", c.lastField, field))
	}
	c.lastField = field

	if terms == nil {
		return nil
	}

	// Assert term order and postings correctness
	te, err := terms.GetIterator()
	if err != nil {
		return err
	}

	var term *util.BytesRef
	var postings PostingsEnum

	fieldInfo := c.writeState.FieldInfos.FieldInfo(field)
	if fieldInfo == nil {
		panic(fmt.Sprintf("AssertingFieldsConsumer: field %s not found in FieldInfos", field))
	}

	hasFreqs := fieldInfo.IndexOptions.Subsumes(spi.IndexOptionsDocsAndFreqs)
	hasPositions := fieldInfo.IndexOptions.Subsumes(spi.IndexOptionsDocsAndFreqsAndPositions)
	hasOffsets := fieldInfo.IndexOptions.Subsumes(spi.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	hasPayloads := terms.HasPayloads()

	for {
		term, err = te.Next()
		if err != nil {
			return err
		}
		if term == nil {
			break
		}

		// Assert term order
		if c.lastTerm != nil && c.lastTerm.CompareTo(term) >= 0 {
			panic(fmt.Sprintf("AssertingFieldsConsumer: terms for field %s are not in sorted order: %s >= %s", field, c.lastTerm, term))
		}
		c.lastTerm = term

		// Request postings
		flags := 0
		if !hasPositions {
			if hasFreqs {
				flags |= spi.PostingsEnumFreqs
			}
		} else {
			flags |= spi.PostingsEnumPositions
			if hasPayloads {
				flags |= spi.PostingsEnumPayloads
			}
			if hasOffsets {
				flags |= spi.PostingsEnumOffsets
			}
		}

		postings, err = te.Postings(postings, flags)
		if err != nil {
			return err
		}
		if postings == nil {
			panic(fmt.Sprintf("AssertingFieldsConsumer: postings are nil for term %s", term))
		}

		lastDocID := -1
		for {
			docID, err := postings.NextDoc()
			if err != nil {
				return err
			}
			if docID == spi.PostingsEnumNoMoreDocs {
				break
			}
			if docID <= lastDocID {
				panic(fmt.Sprintf("AssertingFieldsConsumer: docIDs are not strictly increasing: %d <= %d", docID, lastDocID))
			}
			lastDocID = docID

			if hasFreqs {
				freq, err := postings.Freq()
				if err != nil {
					return err
				}
				if freq <= 0 {
					panic(fmt.Sprintf("AssertingFieldsConsumer: freq %d <= 0 for doc %d", freq, docID))
				}

				if hasPositions {
					lastPos := -1
					lastStartOffset := -1
					for i := 0; i < freq; i++ {
						pos, err := postings.NextPosition()
						if err != nil {
							return err
						}
						if pos < lastPos {
							panic(fmt.Sprintf("AssertingFieldsConsumer: positions are not non-decreasing: %d < %d", pos, lastPos))
						}
						if pos > 1000000000 { // Approx IndexWriter.MAX_POSITION
							panic(fmt.Sprintf("AssertingFieldsConsumer: position %d exceeds MAX_POSITION", pos))
						}
						lastPos = pos

						if hasOffsets {
							startOffset, err := postings.StartOffset()
							if err != nil {
								return err
							}
							endOffset, err := postings.EndOffset()
							if err != nil {
								return err
							}
							if endOffset < startOffset {
								panic(fmt.Sprintf("AssertingFieldsConsumer: endOffset %d < startOffset %d", endOffset, startOffset))
							}
							if startOffset < lastStartOffset {
								panic(fmt.Sprintf("AssertingFieldsConsumer: startOffset %d < lastStartOffset %d", startOffset, lastStartOffset))
							}
							lastStartOffset = startOffset
						}
					}
				}
			}
		}
	}

	return nil
}

func (c *AssertingFieldsConsumer) Close() error {
	err := c.in.Close()
	_ = c.in.Close()
	return err
}

func (c *AssertingFieldsConsumer) IsClosed() bool {
	return c.in.IsClosed()
}

func (c *AssertingFieldsConsumer) GetState() *SegmentWriteState {
	return c.writeState
}
