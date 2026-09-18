// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
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
		in:         in,
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

func (p *AssertingFieldsProducer) Iterator() (index.FieldIterator, error) {
	iterator, err := p.in.Iterator()
	if err != nil {
		return nil, err
	}
	// assert iterator != null;
	if iterator == nil {
		panic("AssertingFieldsProducer: in.iterator() returned null")
	}
	return iterator, nil
}

func (p *AssertingFieldsProducer) Terms(field string) (index.Terms, error) {
	terms, err := p.in.Terms(field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	// terms == null ? null : new AssertingLeafReader.AssertingTerms(terms)
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

func (p *AssertingFieldsProducer) GetMergeInstance() FieldsProducer {
	// new AssertingFieldsProducer(in.getMergeInstance())
	return &AssertingFieldsProducer{in: p.in.GetMergeInstance()}
}

func (p *AssertingFieldsProducer) String() string {
	// getClass().getSimpleName() + "(" + in.toString() + ")"
	return fmt.Sprintf("AssertingFieldsProducer(%v)", p.in)
}

type AssertingFieldsConsumer struct {
	in         FieldsConsumer
	writeState *SegmentWriteState
}

// Write forwards to the delegate and then re-walks every field, term and
// posting to assert the ordering and statistics contract.
//
// Mirrors AssertingPostingsFormat.AssertingFieldsConsumer.write(Fields,
// NormsProducer) (AssertingPostingsFormat.java:106-231).
func (c *AssertingFieldsConsumer) Write(fields index.Fields, norms NormsProducer) error {
	// in.write(fields, norms);
	if err := c.in.Write(fields, norms); err != nil {
		return err
	}

	// TODO: more asserts?  can we somehow run a
	// "limited" CheckIndex here???  Or ... can we improve
	// AssertingFieldsProducer and us it also to wrap the
	// incoming Fields here?

	lastField := ""
	if fields == nil {
		return nil
	}
	it, err := fields.Iterator()
	if err != nil {
		return err
	}
	for {
		field, err := it.Next()
		if err != nil {
			return err
		}
		if field == "" {
			return nil
		}

		// FieldInfo fieldInfo = writeState.fieldInfos.fieldInfo(field);
		// assert fieldInfo != null;
		// assert lastField == null || lastField.compareTo(field) < 0;
		fieldInfo := c.writeState.FieldInfos.FieldInfo(field)
		if fieldInfo == nil {
			panic(fmt.Sprintf("AssertingFieldsConsumer: field %s not found in FieldInfos", field))
		}
		if lastField != "" && lastField >= field {
			panic(fmt.Sprintf("AssertingFieldsConsumer: fields are not in sorted order: %s >= %s", lastField, field))
		}
		lastField = field

		terms, err := fields.Terms(field)
		if err != nil {
			return err
		}
		if terms == nil {
			continue
		}
		if err := c.assertField(field, fieldInfo, terms); err != nil {
			return err
		}
	}
}

// assertField carries the per-field body of the Java write loop.
func (c *AssertingFieldsConsumer) assertField(field string, fieldInfo *spi.FieldInfo, terms spi.Terms) error {
	te, err := terms.Iterator()
	if err != nil {
		return err
	}
	var lastTerm *util.BytesRefBuilder
	var postings index.PostingsEnum

	hasFreqs := fieldInfo.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqs)
	hasPositions := fieldInfo.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqsAndPositions)
	hasOffsets := fieldInfo.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	hasPayloads := terms.HasPayloads()

	// assert hasPositions == terms.hasPositions();
	// assert hasOffsets == terms.hasOffsets();
	if hasPositions != terms.HasPositions() {
		panic(fmt.Sprintf("AssertingFieldsConsumer: hasPositions (%v) != terms.hasPositions() (%v) for field %s", hasPositions, terms.HasPositions(), field))
	}
	if hasOffsets != terms.HasOffsets() {
		panic(fmt.Sprintf("AssertingFieldsConsumer: hasOffsets (%v) != terms.hasOffsets() (%v) for field %s", hasOffsets, terms.HasOffsets(), field))
	}

	for {
		term, err := te.Next()
		if err != nil {
			return err
		}
		if term == nil {
			break
		}

		// assert lastTerm == null || lastTerm.get().compareTo(term) < 0;
		if lastTerm != nil && util.BytesRefCompare(lastTerm.Get(), term.Bytes) >= 0 {
			panic(fmt.Sprintf("AssertingFieldsConsumer: terms for field %s are not in sorted order: %s >= %s", field, lastTerm.Get(), term.Bytes))
		}
		if lastTerm == nil {
			lastTerm = util.NewBytesRefBuilder()
			lastTerm.Append(term.Bytes)
		} else {
			lastTerm.CopyBytesRef(term.Bytes)
		}

		flags := 0
		if !hasPositions {
			if hasFreqs {
				flags |= spi.PostingsFlagFreqs
			}
		} else {
			flags = spi.PostingsFlagPositions
			if hasPayloads {
				flags |= spi.PostingsFlagPayloads
			}
			if hasOffsets {
				flags |= spi.PostingsFlagOffsets
			}
		}

		// postingsEnum = termsEnum.postings(postingsEnum, flags); the spi
		// TermsEnum has no reuse parameter.
		postings, err = te.Postings(flags)
		if err != nil {
			return err
		}
		if postings == nil {
			panic(fmt.Sprintf("AssertingFieldsConsumer: postings are nil for term %s (hasPositions=%v)", term.Bytes, hasPositions))
		}

		lastDocID := -1
		for {
			docID, err := postings.NextDoc()
			if err != nil {
				return err
			}
			if docID == spi.NO_MORE_DOCS {
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
						// assert pos <= IndexWriter.MAX_POSITION
						if pos > index.MaxPosition {
							panic(fmt.Sprintf("AssertingFieldsConsumer: pos=%d is > IndexWriter.MAX_POSITION=%d", pos, index.MaxPosition))
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

func (c *AssertingFieldsConsumer) GetState() *SegmentWriteState {
	return c.writeState
}
