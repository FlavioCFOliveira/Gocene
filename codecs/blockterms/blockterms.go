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

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
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

	filename := index.SegmentFileName(
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
	// Java counts the fields in an int (`final int numFields = in.readVInt()`,
	// BlockTermsReader.java:132) and indexes the loop with one too
	// (BlockTermsReader.java:136); readVInt yields exactly that 32-bit value,
	// so the loop variable is int32 and nothing read from .tib changes.
	for i := int32(0); i < numFields; i++ {
		fieldNum, err := store.ReadVInt(in)
		if err != nil {
			return nil, err
		}
		numTerms, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}
		termsStartPointer, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}

		// Java reads the field number into an int
		// (`final int field = in.readVInt()`, BlockTermsReader.java:137) and
		// hands it to fieldInfos.fieldInfo(int) (BlockTermsReader.java:141);
		// GetByNumber takes Go's int, so the 32-bit value is widened, never
		// truncated.
		fieldInfo := state.FieldInfos.GetByNumber(int(fieldNum))
		sumTotalTermFreq, err := in.ReadVLong()
		if err != nil {
			return nil, err
		}

		// when frequencies are omitted, sumDocFreq=totalTermFreq and we only write one value
		var sumDocFreq int64
		if fieldInfo.IndexOptions() == index.IndexOptionsDocs {
			sumDocFreq = sumTotalTermFreq
		} else {
			sumDocFreq, err = in.ReadVLong()
			if err != nil {
				return nil, err
			}
		}

		docCount32, err := store.ReadVInt(in)
		if err != nil {
			return nil, err
		}
		// Java holds docCount in an int (BlockTermsReader.java:146), which is
		// 32-bit and is exactly what readVInt yields; it is widened to Go's int
		// here because FieldReader.docCount and SegmentInfo.maxDoc() are both
		// Java ints too. Nothing about the bytes read from .tib changes.
		docCount := int(docCount32)

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
	footerLen := store.FooterLength()
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

// Field returns the name of the field this Terms instance represents.
//
// org.apache.lucene.index.Terms declares no field() accessor, so Apache Lucene
// 10.5.0 reads the name straight off FieldReader.fieldInfo
// (BlockTermsReader.java:228, `final FieldInfo fieldInfo`). Gocene's
// [spi.Terms] contract does declare one, so the accessor is spelled here over
// the same fieldInfo.
func (fr *fieldReader) Field() string { return fr.fieldInfo.Name() }

func (fr *fieldReader) Iterator() (index.TermsEnum, error) {
	return newSegmentTermsEnum(fr)
}

// GetIteratorWithSeek returns an iterator positioned at or after seekTerm.
//
// org.apache.lucene.index.Terms declares no such member, so there is nothing to
// override in BlockTermsReader.FieldReader (BlockTermsReader.java:226); the
// Gocene [spi.Terms] contract does declare it, and it is satisfied here by the
// two Lucene operations a Java caller would spell out itself —
// Terms.iterator() followed by TermsEnum.seekCeil(BytesRef).
func (fr *fieldReader) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	te, err := fr.Iterator()
	if err != nil {
		return nil, err
	}
	if seekTerm != nil {
		if _, err := te.SeekCeil(seekTerm); err != nil {
			return nil, err
		}
	}
	return te, nil
}

// GetPostingsReader returns the postings of termText, or nil when the term is
// absent.
//
// Like GetIteratorWithSeek this is a Gocene [spi.Terms] member with no
// counterpart on org.apache.lucene.index.Terms; it is satisfied by the Lucene
// sequence iterator() -> seekExact(BytesRef) -> postings(int).
func (fr *fieldReader) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	te, err := fr.Iterator()
	if err != nil {
		return nil, err
	}
	found, err := te.SeekExact(index.NewTerm(fr.fieldInfo.Name(), termText))
	if err != nil || !found {
		return nil, err
	}
	return te.Postings(flags)
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

// Intersect is the default org.apache.lucene.index.Terms#intersect(
// CompiledAutomaton, BytesRef) (Terms.java:64) that FieldReader inherits:
// iterator() wrapped in an AutomatonTermsEnum, rejecting any CompiledAutomaton
// that is not AUTOMATON_TYPE.NORMAL. Java expresses the non-null startTerm case
// as an anonymous subclass overriding nextSeekTerm; Gocene spells the same
// thing through AutomatonTermsEnum.SetInitialSeekTerm, exactly as the sibling
// term-vectors reader does.
func (fr *fieldReader) Intersect(compiled *automaton.CompiledAutomaton, startTerm *index.Term) (index.TermsEnum, error) {
	termsEnum, err := fr.Iterator()
	if err != nil {
		return nil, err
	}
	if compiled.Type != automaton.AutomatonTypeNormal {
		return nil, errors.New("please use CompiledAutomaton.getTermsEnum instead")
	}
	automatonTermsEnum := index.NewAutomatonTermsEnum(termsEnum, compiled)
	if startTerm != nil {
		automatonTermsEnum.SetInitialSeekTerm(startTerm)
	}
	return automatonTermsEnum, nil
}

// GetMin is the default org.apache.lucene.index.Terms#getMin()
// (Terms.java:143) that FieldReader inherits: `return iterator().next()`.
func (fr *fieldReader) GetMin() (*index.Term, error) {
	te, err := fr.Iterator()
	if err != nil {
		return nil, err
	}
	return te.Next()
}

// GetMax is the default org.apache.lucene.index.Terms#getMax()
// (Terms.java:153) that FieldReader inherits: seek-by-ord when size() is known,
// otherwise a digit-by-digit binary search over seekCeil. Java's
// `catch (UnsupportedOperationException)` around the seek-by-ord attempt cannot
// fire for this class — SegmentTermsEnum.seekExact(long)
// (BlockTermsReader.java:711) throws IllegalStateException, never
// UnsupportedOperationException — so a failure of the ord seek is propagated
// here rather than swallowed.
func (fr *fieldReader) GetMax() (*index.Term, error) {
	size := fr.Size()

	if size == 0 {
		// empty: only possible from a FilteredTermsEnum...
		return nil, nil
	} else if size >= 0 {
		// try to seek-by-ord
		te, err := fr.Iterator()
		if err != nil {
			return nil, err
		}
		ste, ok := te.(*segmentTermsEnum)
		if !ok {
			return nil, fmt.Errorf("blockterms: fieldReader.GetMax: unexpected TermsEnum %T", te)
		}
		if err := ste.SeekExactOrd(size - 1); err != nil {
			return nil, err
		}
		return ste.Term(), nil
	}

	// otherwise: binary search
	te, err := fr.Iterator()
	if err != nil {
		return nil, err
	}
	v, err := te.Next()
	if err != nil {
		return nil, err
	}
	if v == nil {
		// empty: only possible from a FilteredTermsEnum...
		return nil, nil
	}

	scratch := []byte{0}

	// Iterates over digits:
	for {
		low := 0
		high := 256

		// Binary search current digit to find the highest
		// digit before END:
		for low != high {
			mid := int(uint(low+high) >> 1)
			scratch[len(scratch)-1] = byte(mid)
			term, err := te.SeekCeil(index.NewTermFromBytes(fr.fieldInfo.Name(), scratch))
			if err != nil {
				return nil, err
			}
			if term == nil {
				// Scratch was too high
				if mid == 0 {
					scratch = scratch[:len(scratch)-1]
					return index.NewTermFromBytes(fr.fieldInfo.Name(), scratch), nil
				}
				high = mid
			} else {
				// Scratch was too low; there is at least one term
				// still after it:
				if low == mid {
					break
				}
				low = mid
			}
		}

		// Recurse to next digit:
		scratch = append(scratch, 0)
	}
}

// GetSumTotalTermFreq mirrors FieldReader.getSumTotalTermFreq()
// (BlockTermsReader.java:288), whose body is `return sumTotalTermFreq`. Java
// declares no checked exception on it; Gocene's [spi.Terms] carries the error
// return on every statistic, so a nil error is always reported here.
func (fr *fieldReader) GetSumTotalTermFreq() (int64, error) {
	return fr.sumTotalTermFreq, nil
}

func (fr *fieldReader) GetSumDocFreq() (int64, error) {
	return fr.sumDocFreq, nil
}

func (fr *fieldReader) GetDocCount() (int, error) {
	return fr.docCount, nil
}

type segmentTermsEnum struct {
	// TermsEnumBase renders `extends BaseTermsEnum`
	// (BlockTermsReader.java:303): it carries the lazily created
	// AttributeSource behind BaseTermsEnum.attributes().
	index.TermsEnumBase

	// fr is the enclosing FieldReader. Java spells SegmentTermsEnum as a
	// private inner class of FieldReader (BlockTermsReader.java:303), so every
	// bare `fieldInfo` in its bodies is FieldReader.this.fieldInfo; Go has no
	// implicit outer instance, so the reference is carried explicitly.
	fr *fieldReader

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
	// term mirrors `private final BytesRefBuilder term`
	// (BlockTermsReader.java:309).
	term *util.BytesRefBuilder
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
		fr:           fr,
		reader:       fr.reader,
		in:           in,
		state:        state,
		doOrd:        doOrd,
		indexEnum:    indexEnum,
		termSuffixes: make([]byte, 128),
		docFreqBytes: make([]byte, 64),
		term:         util.NewBytesRefBuilder(),
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
	if err := e.in.ReadBytes(e.termSuffixes, 0, int(lenSuf)); err != nil {
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
	if err := e.in.ReadBytes(e.docFreqBytes, 0, int(lenFreq)); err != nil {
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
	if err := e.in.ReadBytes(e.bytes, 0, int(lenMeta)); err != nil {
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

	// TODO: cutover to something better for these ints!  simple64?
	suffix, err := e.termSuffixesReader.ReadVInt()
	if err != nil {
		return nil, err
	}

	// term.setLength(termBlockPrefix + suffix);
	// term.grow(term.length());
	// termSuffixesReader.readBytes(term.bytes(), termBlockPrefix, suffix);
	//
	// (BlockTermsReader.java:643-645). Java holds suffix in an int, which is
	// what readVInt yields; it is widened to Go's int only to index the term
	// buffer, so nothing read from .tib changes.
	e.term.SetLength(e.termBlockPrefix + int(suffix))
	e.term.Grow(e.term.Length())
	if err := e.termSuffixesReader.ReadBytes(e.term.Bytes(), e.termBlockPrefix, int(suffix)); err != nil {
		return nil, err
	}
	e.state.TermBlockOrd++

	// NOTE: meaningless in the non-ord case
	e.state.Ord++

	return e.term.Get(), nil
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
	return index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), t), nil
}

// Term returns the current term. Port of SegmentTermsEnum.term()
// (BlockTermsReader.java), whose body is `return term.get()`; the field name
// comes from the enclosing FieldReader because Gocene's [spi.Term] carries it.
func (e *segmentTermsEnum) Term() *index.Term {
	if e.term == nil {
		return nil
	}
	return index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), e.term.Get())
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

// Impacts returns an ImpactsEnum for the current term. Port of
// SegmentTermsEnum.impacts(int) (BlockTermsReader.java:684):
//
//	decodeMetaData();
//	return postingsReader.impacts(fieldInfo, state, flags);
func (e *segmentTermsEnum) Impacts(flags int) (index.ImpactsEnum, error) {
	if err := e.decodeMetaData(); err != nil {
		return nil, err
	}
	return e.reader.postingsReader.Impacts(e.fr.fieldInfo, e.state, flags)
}

func (e *segmentTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

func (e *segmentTermsEnum) SeekCeil(target *index.Term) (*index.Term, error) {
	if e.indexEnum == nil {
		return nil, errors.New("terms index was not loaded")
	}

	targetRef := target.Bytes
	doSeek := true
	if e.indexIsCurrent {
		cmp := util.BytesRefCompare(e.term.Get(), targetRef)
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
		e.term.CopyBytesRef(e.indexEnum.Term())
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
		if e.term == nil || util.BytesRefCompare(e.term.Get(), targetRef) >= 0 {
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
	e.term.CopyBytesRef(e.indexEnum.Term())

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
