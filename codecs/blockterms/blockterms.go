// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package blockterms

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Format constants, ported from
// org.apache.lucene.codecs.blockterms.BlockTermsWriter (Apache Lucene 10.5.0).
const (
	// TermsExtension mirrors BlockTermsWriter.TERMS_EXTENSION.
	TermsExtension = "tib"

	// CodecName mirrors BlockTermsWriter.CODEC_NAME.
	CodecName = "BlockTermsWriter"
	// VersionStart mirrors BlockTermsWriter.VERSION_START.
	VersionStart = int32(4)
	// VersionCurrent mirrors BlockTermsWriter.VERSION_CURRENT.
	VersionCurrent = int32(4)
)

// BlockTermsReader handles a terms dict, but decouples all details of doc/freqs/positions reading to an instance of
// PostingsReaderBase. This class is reusable for codecs that use a different format for
// docs/freqs/positions.
//
// Mirrors org.apache.lucene.codecs.blockterms.BlockTermsReader.
type BlockTermsReader struct {
	in             store.IndexInput
	postingsReader codecs.PostingsReaderBase
	fields         map[string]*fieldReader
	indexReader    TermsIndexReader
}

// NewBlockTermsReader builds a BlockTermsReader.
func NewBlockTermsReader(indexReader TermsIndexReader, postingsReader codecs.PostingsReaderBase, state *index.SegmentReadState) (*BlockTermsReader, error) {
	postingsReader = postingsReader

	filename := index.GetSegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, TermsExtension)
	in, err := state.Directory.OpenInput(filename, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, err
	}

	success := false
	defer func() {
		if !success {
			in.Close()
		}
	}()

	version, err := codecs.CheckIndexHeader(
		in,
		CodecName,
		VersionStart,
		VersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix)
	if err != nil {
		return nil, err
	}
	_ = version

	// Have PostingsReader init itself
	if err := postingsReader.Init(in, state); err != nil {
		return nil, err
	}

	// Verify proper structure of the checksum footer
	if _, err := codecs.RetrieveChecksum(in); err != nil {
		return nil, err
	}

	// Read per-field details
	if err := seekDir(in); err != nil {
		return nil, err
	}

	numFields, err := store.ReadVInt(in)
	if err != nil {
		return nil, err
	}
	if numFields < 0 {
		return nil, fmt.Errorf("invalid number of fields: %d", numFields)
	}

	r := &BlockTermsReader{
		in:             in,
		postingsReader: postingsReader,
		indexReader:    indexReader,
	}

	fields := make(map[string]*fieldReader)
	for i := 0; i < numFields; i++ {
		fieldNum, err := store.ReadVInt(in)
		if err != nil {
			return nil, err
		}
		numTerms, err := store.ReadVLong(in)
		if err != nil {
			return nil, err
		}
		termsStartPointer, err := store.ReadVLong(in)
		if err != nil {
			return nil, err
		}

		fieldInfo := state.FieldInfos.GetByNumber(fieldNum)
		sumTotalTermFreq, err := store.ReadVLong(in)
		if err != nil {
			return nil, err
		}

		// when frequencies are omitted, sumDocFreq=totalTermFreq and we only write one value
		var sumDocFreq int64
		if fieldInfo.IndexOptions() == index.IndexOptionsDocs {
			sumDocFreq = sumTotalTermFreq
		} else {
			sumDocFreq, err = store.ReadVLong(in)
			if err != nil {
				return nil, err
			}
		}

		docCount, err := store.ReadVInt(in)
		if err != nil {
			return nil, err
		}

		if docCount < 0 || docCount > state.SegmentInfo.MaxDoc() {
			return nil, fmt.Errorf("invalid docCount: %d maxDoc: %d", docCount, state.SegmentInfo.MaxDoc())
		}
		if sumDocFreq < int64(docCount) {
			return nil, fmt.Errorf("invalid sumDocFreq: %d docCount: %d", sumDocFreq, docCount)
		}
		if sumTotalTermFreq < sumDocFreq {
			return nil, fmt.Errorf("invalid sumTotalTermFreq: %d sumDocFreq: %d", sumTotalTermFreq, sumDocFreq)
		}

		if _, dup := fields[fieldInfo.Name()]; dup {
			return nil, fmt.Errorf("duplicate fields: %s", fieldInfo.Name())
		}
		fields[fieldInfo.Name()] = &fieldReader{
			reader:            r,
			fieldInfo:         fieldInfo,
			numTerms:          numTerms,
			termsStartPointer: termsStartPointer,
			sumTotalTermFreq:  sumTotalTermFreq,
			sumDocFreq:        sumDocFreq,
			docCount:          docCount,
		}
	}

	r.fields = fields
	success = true
	return r, nil
}

func seekDir(input store.IndexInput) error {
	footerLen := codecs.FooterLength()
	if input.Length() < int64(footerLen+8) {
		return errors.New("file too short to contain directory offset")
	}
	if err := input.SetPosition(input.Length() - int64(footerLen) - 8); err != nil {
		return err
	}
	dirOffset, err := input.ReadLong()
	if err != nil {
		return err
	}
	return input.SetPosition(dirOffset)
}

func (r *BlockTermsReader) Close() error {
	var firstErr error
	if r.indexReader != nil {
		if closer, ok := r.indexReader.(io.Closer); ok {
			if err := closer.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	if err := r.in.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := r.postingsReader.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (r *BlockTermsReader) Terms(field string) (index.Terms, error) {
	fr, ok := r.fields[field]
	if !ok {
		return nil, nil
	}
	return fr, nil
}

func (r *BlockTermsReader) Size() int {
	return len(r.fields)
}

type fieldReader struct {
	reader            *BlockTermsReader
	fieldInfo         *index.FieldInfo
	numTerms          int64
	termsStartPointer int64
	sumTotalTermFreq  int64
	sumDocFreq        int64
	docCount          int
}

func (fr *fieldReader) Iterator() (index.TermsEnum, error) {
	return newSegmentTermsEnum(fr)
}

func (fr *fieldReader) HasFreqs() bool {
	return fr.fieldInfo.IndexOptions().Subsumes(index.IndexOptionsDocsAndFreqs)
}

func (fr *fieldReader) HasOffsets() bool {
	return fr.fieldInfo.IndexOptions().Subsumes(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
}

func (fr *fieldReader) HasPositions() bool {
	return fr.fieldInfo.IndexOptions().Subsumes(index.IndexOptionsDocsAndFreqsAndPositions)
}

func (fr *fieldReader) HasPayloads() bool {
	return fr.fieldInfo.HasPayloads()
}

func (fr *fieldReader) Size() int64 {
	return fr.numTerms
}

func (fr *fieldReader) GetSumTotalTermFreq() int64 {
	return fr.sumTotalTermFreq
}

func (fr *fieldReader) GetSumDocFreq() (int64, error) {
	return fr.sumDocFreq, nil
}

func (fr *fieldReader) GetDocCount() (int, error) {
	return fr.docCount, nil
}

type segmentTermsEnum struct {
	reader             *BlockTermsReader
	in                 store.IndexInput
	state              *codecs.BlockTermState
	doOrd              bool
	indexEnum          TermsIndexEnum
	indexIsCurrent     bool
	didIndexNext       bool
	nextIndexTerm      *util.BytesRef
	seekPending        bool
	termSuffixes       []byte
	termSuffixesReader *store.ByteArrayDataInput
	termBlockPrefix    int
	blockTermCount     int
	docFreqBytes       []byte
	freqReader         *store.ByteArrayDataInput
	metaDataUpto       int
	bytes              []byte
	bytesReader        *store.ByteArrayDataInput
	term               *util.BytesRef
}

func newSegmentTermsEnum(fr *fieldReader) (*segmentTermsEnum, error) {
	in := fr.reader.in.Clone()
	in.SetPosition(fr.termsStartPointer)

	indexEnum := fr.reader.indexReader.GetFieldEnum(fr.fieldInfo)
	doOrd := fr.reader.indexReader.SupportsOrd()

	state := fr.reader.postingsReader.NewTermState()
	state.TotalTermFreq = -1
	state.Ord = -1

	return &segmentTermsEnum{
		reader:       fr.reader,
		in:           in,
		state:        state,
		doOrd:        doOrd,
		indexEnum:    indexEnum,
		termSuffixes: make([]byte, 128),
		docFreqBytes: make([]byte, 64),
		term:         util.NewBytesRefEmpty(),
	}, nil
}

func (e *segmentTermsEnum) nextBlock() (bool, error) {
	e.state.BlockFilePointer = e.in.GetFilePointer()
	blockTermCount, err := store.ReadVInt(e.in)
	if err != nil {
		return false, err
	}
	e.blockTermCount = int(blockTermCount)
	if e.blockTermCount == 0 {
		return false, nil
	}

	termBlockPrefix, err := store.ReadVInt(e.in)
	if err != nil {
		return false, err
	}
	e.termBlockPrefix = int(termBlockPrefix)

	lenSuf, err := store.ReadVInt(e.in)
	if err != nil {
		return false, err
	}
	if len(e.termSuffixes) < int(lenSuf) {
		e.termSuffixes = make([]byte, int(lenSuf))
	}
	if _, err := e.in.ReadBytes(e.termSuffixes[:int(lenSuf)]); err != nil {
		return false, err
	}
	e.termSuffixesReader = store.NewByteArrayDataInput(e.termSuffixes[:int(lenSuf)])

	lenFreq, err := store.ReadVInt(e.in)
	if err != nil {
		return false, err
	}
	if len(e.docFreqBytes) < int(lenFreq) {
		e.docFreqBytes = make([]byte, int(lenFreq))
	}
	if _, err := e.in.ReadBytes(e.docFreqBytes[:int(lenFreq)]); err != nil {
		return false, err
	}
	e.freqReader = store.NewByteArrayDataInput(e.docFreqBytes[:int(lenFreq)])

	lenMeta, err := store.ReadVInt(e.in)
	if err != nil {
		return false, err
	}
	if e.bytes == nil || len(e.bytes) < int(lenMeta) {
		e.bytes = make([]byte, int(lenMeta))
	}
	if _, err := e.in.ReadBytes(e.bytes[:int(lenMeta)]); err != nil {
		return false, err
	}
	e.bytesReader = store.NewByteArrayDataInput(e.bytes[:int(lenMeta)])

	e.metaDataUpto = 0
	e.state.TermBlockOrd = 0
	e.indexIsCurrent = false

	return true, nil
}

func (e *segmentTermsEnum) decodeMetaData() error {
	if e.seekPending {
		return nil
	}

	limit := e.state.TermBlockOrd
	absolute := e.metaDataUpto == 0
	for e.metaDataUpto < limit {
		docFreq, err := e.freqReader.ReadVInt()
		if err != nil {
			return err
		}
		e.state.DocFreq = int(docFreq)

		if e.reader.fields[e.term.String()].fieldInfo.IndexOptions() == index.IndexOptionsDocs {
			e.state.TotalTermFreq = int64(docFreq)
		} else {
			tf, err := e.freqReader.ReadVLong()
			if err != nil {
				return err
			}
			e.state.TotalTermFreq = int64(docFreq) + tf
		}

		if err := e.reader.postingsReader.DecodeTerm(e.bytesReader, e.reader.fields[e.term.String()].fieldInfo, e.state, absolute); err != nil {
			return err
		}
		e.metaDataUpto++
		absolute = false
	}
	return nil
}

func (e *segmentTermsEnum) _next() (*util.BytesRef, error) {
	if e.state.TermBlockOrd == e.blockTermCount {
		ok, err := e.nextBlock()
		if err != nil {
			return nil, err
		}
		if !ok {
			e.indexIsCurrent = false
			return nil, nil
		}
	}

	suffixLen, err := e.termSuffixesReader.ReadVInt()
	if err != nil {
		return nil, err
	}

	e.state.TermBlockOrd++
	e.state.Ord++

	return e.term, nil
}

func (e *segmentTermsEnum) Next() (*index.Term, error) {
	if e.seekPending {
		e.in.SetPosition(e.state.BlockFilePointer)
		pendingSeekCount := e.state.TermBlockOrd
		ok, err := e.nextBlock()
		if err != nil {
			return nil, err
		}
		if !ok && pendingSeekCount == 0 {
			return nil, nil
		}

		savOrd := e.state.Ord
		for e.state.TermBlockOrd < pendingSeekCount {
			if _, err := e._next(); err != nil {
				return nil, err
			}
		}
		e.seekPending = false
		e.state.Ord = savOrd
	}
	t, err := e._next()
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, nil
	}
	return index.NewTerm(t), nil
}

func (e *segmentTermsEnum) Term() *index.Term {
	if e.term == nil {
		return nil
	}
	return index.NewTerm(e.term)
}

func (e *segmentTermsEnum) DocFreq() (int, error) {
	if err := e.decodeMetaData(); err != nil {
		return 0, err
	}
	return e.state.DocFreq, nil
}

func (e *segmentTermsEnum) TotalTermFreq() (int64, error) {
	if err := e.decodeMetaData(); err != nil {
		return 0, err
	}
	return e.state.TotalTermFreq, nil
}

func (e *segmentTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if err := e.decodeMetaData(); err != nil {
		return nil, err
	}
	return e.reader.postingsReader.Postings(e.reader.fields[e.term.String()].fieldInfo, e.state, nil, flags)
}

func (e *segmentTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

func (e *segmentTermsEnum) SeekCeil(target *index.Term) (*index.Term, error) {
	if e.indexEnum == nil {
		return nil, errors.New("terms index was not loaded")
	}

	targetRef := target.Bytes()
	doSeek := true
	if e.indexIsCurrent {
		cmp := util.BytesRefCompare(e.term, targetRef)
		if cmp == 0 {
			return e.Term(), nil
		} else if cmp < 0 {
			if !e.didIndexNext {
				fp, err := e.indexEnum.Next()
				if err != nil {
					return nil, err
				}
				if fp == -1 {
					e.nextIndexTerm = nil
				} else {
					e.nextIndexTerm = e.indexEnum.Term()
				}
				e.didIndexNext = true
			}
			if e.nextIndexTerm == nil || util.BytesRefCompare(targetRef, e.nextIndexTerm) < 0 {
				doSeek = false
			}
		}
	}

	if doSeek {
		fp, err := e.indexEnum.Seek(targetRef)
		if err != nil {
			return nil, err
		}
		e.in.SetPosition(fp)
		ok, err := e.nextBlock()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		e.indexIsCurrent = true
		e.didIndexNext = false
		if e.doOrd {
			e.state.Ord = e.indexEnum.Ord() - 1
		}
		e.term.Copy(e.indexEnum.Term())
	} else {
		if e.state.TermBlockOrd == e.blockTermCount {
			if ok, err := e.nextBlock(); !ok || err != nil {
				e.indexIsCurrent = false
				if err != nil {
					return nil, err
				}
				return nil, nil
			}
		}
	}

	e.seekPending = false

	for {
		if e.term == nil || util.BytesRefCompare(e.term, targetRef) >= 0 {
			return e.Term(), nil
		}
		t, err := e._next()
		if err != nil {
			return nil, err
		}
		if t == nil {
			return nil, nil
		}
	}
}

func (e *segmentTermsEnum) SeekExact(target *index.Term) (bool, error) {
	if target == nil {
		return false, nil
	}
	res, err := e.SeekCeil(target)
	if err != nil {
		return false, err
	}
	if res != nil && res.Equals(target) {
		return true, nil
	}
	return false, nil
}

func (e *segmentTermsEnum) SeekExactOrd(ord int64) error {
	if e.indexEnum == nil {
		return errors.New("terms index was not loaded")
	}
	fp, err := e.indexEnum.SeekOrd(ord)
	if err != nil {
		return err
	}
	e.in.SetPosition(fp)
	ok, err := e.nextBlock()
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("block not found")
	}
	e.indexIsCurrent = true
	e.didIndexNext = false
	e.seekPending = false
	e.state.Ord = e.indexEnum.Ord() - 1
	e.term.Copy(e.indexEnum.Term())

	left := int(ord - e.state.Ord)
	for left > 0 {
		if _, err := e._next(); err != nil {
			return err
		}
		left--
	}
	return nil
}

func (e *segmentTermsEnum) Ord() int64 {
	if !e.doOrd {
		return -1
	}
	return e.state.Ord
}

func (e *segmentTermsEnum) TermState() (index.TermState, error) {
	if err := e.decodeMetaData(); err != nil {
		return nil, err
	}
	return e.state.Clone(), nil
}
