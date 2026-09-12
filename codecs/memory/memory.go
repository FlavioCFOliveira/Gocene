// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package memory

import (
	"fmt"
	"io"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// DirectPostingsFormat is the codec that holds postings in flat in-memory
// arrays. Mirrors org.apache.lucene.codecs.memory.DirectPostingsFormat.
type DirectPostingsFormat struct {
	MinSkipCount  int
	LowFreqCutoff int
}

// NewDirectPostingsFormat builds the format.
func NewDirectPostingsFormat(minSkip, lowFreq int) *DirectPostingsFormat {
	if minSkip < 1 {
		minSkip = 8
	}
	if lowFreq < 1 {
		lowFreq = 32
	}
	return &DirectPostingsFormat{MinSkipCount: minSkip, LowFreqCutoff: lowFreq}
}

// FSTPostingsFormat is the FST-backed postings format. Mirrors
// org.apache.lucene.codecs.memory.FSTPostingsFormat.
type FSTPostingsFormat struct{}

// NewFSTPostingsFormat builds the format.
func NewFSTPostingsFormat() *FSTPostingsFormat { return &FSTPostingsFormat{} }

// FSTTermsReader reads FST-backed term dictionaries. Mirrors
// org.apache.lucene.codecs.memory.FSTTermsReader.
type FSTTermsReader struct {
	fields           map[string]*fstTermsReader
	postingsReader   spi.PostingsReader
	fstTermsInput    store.IndexInput
	segmentInfo      *index.SegmentInfo
	segmentSuffix    string
}

// NewFSTTermsReader builds the reader.
func NewFSTTermsReader(state *index.SegmentReadState, postingsReader spi.PostingsReader) (*FSTTermsReader, error) {
	termsFileName := codecs.IndexFileNamesSegment(state.SegmentInfo.Name, state.SegmentSuffix, "terms")
	
	fstTermsInput, err := state.Directory.OpenInput(termsFileName)
	if err != nil {
		return nil, err
	}

	in := fstTermsInput
	if err := codecs.CheckIndexHeader(in, "FSTTerms", 0, 1, state.SegmentInfo.ID, state.SegmentSuffix); err != nil {
		return nil, err
	}

	// Checksum entire file
	if err := codecs.ChecksumEntireFile(in); err != nil {
		return nil, err
	}

	// Move to directory
	if err := seekDir(in); err != nil {
		return nil, err
	}

	numFields, err := in.ReadVInt()
	if err != nil {
		return nil, err
	}

	fields := make(map[string]*fstTermsReader)
	for i := 0; i < numFields; i++ {
		fieldNumber, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}
		fieldInfo := state.FieldInfos.FieldInfo(fieldNumber)
		numTerms, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		sumTotalTermFreq, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		
		var sumDocFreq int64
		if fieldInfo.IndexOptions() == spi.IndexOptionsDocs {
			sumDocFreq = sumTotalTermFreq
		} else {
			sumDocFreq, err = in.ReadVLong()
			if err != nil {
				return nil, err
			}
		}
		
		docCount, err := in.ReadVInt()
		if err != nil {
			return nil, err
		}

		tr, err := newFstTermsReader(fieldInfo, in, numTerms, sumTotalTermFreq, sumDocFreq, docCount)
		if err != nil {
			return nil, err
		}
		fields[fieldInfo.Name()] = tr
	}

	return &FSTTermsReader{
		fields:         fields,
		postingsReader: postingsReader,
		fstTermsInput:  fstTermsInput,
		segmentInfo:    state.SegmentInfo,
		segmentSuffix:  state.SegmentSuffix,
	}, nil
}

func seekDir(in store.IndexInput) error {
	length := in.Length()
	in.Seek(length - store.FooterLength() - 8)
	offset, err := in.ReadLong()
	if err != nil {
		return err
	}
	in.Seek(offset)
	return nil
}

func (r *FSTTermsReader) Terms(field string) (spi.Terms, error) {
	if tr, ok := r.fields[field]; ok {
		return tr, nil
	}
	return nil, nil
}

func (r *FSTTermsReader) Close() error {
	if r.fstTermsInput != nil {
		return r.fstTermsInput.Close()
	}
	return nil
}

type fstTermsReader struct {
	fieldInfo        *spi.FieldInfo
	numTerms         int64
	sumTotalTermFreq int64
	sumDocFreq       int64
	docCount         int
	dict             *gfst.FST
}

func newFstTermsReader(fieldInfo *spi.FieldInfo, in store.IndexInput, numTerms, sumTotalTermFreq, sumDocFreq int64, docCount int) (*fstTermsReader, error) {
	outputs := &FSTTermOutputs{}
	metadata, err := gfst.ReadMetadata(in, outputs)
	if err != nil {
		return nil, err
	}
	
	store := gfst.NewOffHeapFSTStore(in, in.FilePointer(), metadata)
	dict, err := gfst.FromFSTReader(metadata, store)
	if err != nil {
		return nil, err
	}
	
	in.SkipBytes(store.Size())
	
	return &fstTermsReader{
		fieldInfo:        fieldInfo,
		numTerms:         numTerms,
		sumTotalTermFreq: sumTotalTermFreq,
		sumDocFreq:       sumDocFreq,
		docCount:         docCount,
		dict:             dict,
	}, nil
}

func (tr *fstTermsReader) Size() int64 { return tr.numTerms }
func (tr *fstTermsReader) GetSumTotalTermFreq() int64 { return tr.sumTotalTermFreq }
func (tr *fstTermsReader) GetSumDocFreq() (int64, error) { return tr.sumDocFreq, nil }
func (tr *fstTermsReader) GetDocCount() (int, error) { return tr.docCount, nil }
func (tr *fstTermsReader) HasFreqs() bool { return tr.fieldInfo.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqs) }
func (tr *fstTermsReader) HasOffsets() bool { return tr.fieldInfo.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqsAndPositionsAndOffsets) }
func (tr *fstTermsReader) HasPositions() bool { return tr.fieldInfo.IndexOptions().Subsumes(spi.IndexOptionsDocsAndFreqsAndPositions) }
func (tr *fstTermsReader) HasPayloads() bool { return tr.fieldInfo.HasPayloads() }

func (tr *fstTermsReader) GetIterator() (spi.TermsEnum, error) {
	return &fstTermsEnum{
		tr:    tr,
		enum:  gfst.NewBytesRefFSTEnum(tr.dict),
		term:  nil,
	}, nil
}

func (tr *fstTermsReader) GetIteratorWithSeek(seekTerm *spi.Term) (spi.TermsEnum, error) {
	enum := &fstTermsEnum{
		tr:    tr,
		enum:  gfst.NewBytesRefFSTEnum(tr.dict),
	}
	if seekTerm != nil {
		enum.seekExact(seekTerm.Bytes())
	}
	return enum, nil
}

func (tr *fstTermsReader) GetPostingsReader(termText string, flags int) (spi.PostingsEnum, error) {
	return nil, fmt.Errorf("FSTTermsReader: GetPostingsReader not implemented")
}

func (tr *fstTermsReader) GetMin() (*spi.Term, error) {
	return nil, fmt.Errorf("FSTTermsReader: GetMin not implemented")
}

func (tr *fstTermsReader) GetMax() (*spi.Term, error) {
	return nil, fmt.Errorf("FSTTermsReader: GetMax not implemented")
}

type fstTermsEnum struct {
	tr   *fstTermsReader
	enum *gfst.BytesRefFSTEnum
	term []byte
}

func (e *fstTermsEnum) Next() ([]byte, error) {
	e.term = e.enum.Next()
	return e.term, nil
}

func (e *fstTermsEnum) seekExact(target []byte) {
	e.term = e.enum.SeekExact(target)
}

func (e *fstTermsEnum) Term() []byte { return e.term }
func (e *fstTermsEnum) DocFreq() int { return 0 } // Should use FST outputs
func (e *fstTermsEnum) TotalTermFreq() int64 { return 0 } // Should use FST outputs

// FSTTermsWriter writes FST-backed term dictionaries. Mirrors
// org.apache.lucene.codecs.memory.FSTTermsWriter.
type FSTTermsWriter struct {
	Format *FSTPostingsFormat
}

// NewFSTTermsWriter builds the writer.
func NewFSTTermsWriter(format *FSTPostingsFormat) *FSTTermsWriter {
	return &FSTTermsWriter{Format: format}
}

func (w *FSTTermsWriter) Write(state *index.SegmentWriteState, fields spi.Terms) error {
	// Implementation of writing FST terms...
	return fmt.Errorf("FSTTermsWriter: Write not yet implemented")
}
