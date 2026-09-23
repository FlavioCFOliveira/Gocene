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
	"fmt"
	"math"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TODO: currently we encode all terms between two indexed
// terms as a block; but, we could decouple the two, ie
// allow several blocks in between two indexed terms

// BlockTermsWriter writes the terms dict, block-encoding (column stride) each
// term's metadata for each set of terms between two index terms.
//
// Port of org.apache.lucene.codecs.blockterms.BlockTermsWriter (Apache Lucene
// 10.5.0), which extends FieldsConsumer. The format constants are declared
// with the reader (TermsExtension, CodecName, VersionStart, VersionCurrent).
//
// @lucene.experimental
type BlockTermsWriter struct {
	// FieldsConsumerBase carries the concrete FieldsConsumer.merge that Java
	// inherits from the superclass.
	*codecs.FieldsConsumerBase

	// out mirrors the protected field IndexOutput out.
	out              store.IndexOutput
	postingsWriter   codecs.PostingsWriterBase
	fieldInfos       *index.FieldInfos
	currentField     *index.FieldInfo
	termsIndexWriter TermsIndexWriterBase
	maxDoc           int

	fields []*blockTermsFieldMetaData
}

var _ spi.FieldsConsumer = (*BlockTermsWriter)(nil)

// blockTermsFieldMetaData is the Go port of the private record
// BlockTermsWriter.FieldMetaData.
type blockTermsFieldMetaData struct {
	fieldInfo         *index.FieldInfo
	numTerms          int64
	termsStartPointer int64
	sumTotalTermFreq  int64
	sumDocFreq        int64
	docCount          int
}

func newBlockTermsFieldMetaData(fieldInfo *index.FieldInfo, numTerms, termsStartPointer, sumTotalTermFreq,
	sumDocFreq int64, docCount int) *blockTermsFieldMetaData {
	if util.AssertsEnabled() && !(numTerms > 0) {
		panic(util.NewAssertionError(nil))
	}
	return &blockTermsFieldMetaData{
		fieldInfo:         fieldInfo,
		numTerms:          numTerms,
		termsStartPointer: termsStartPointer,
		sumTotalTermFreq:  sumTotalTermFreq,
		sumDocFreq:        sumDocFreq,
		docCount:          docCount,
	}
}

// NewBlockTermsWriter mirrors the constructor
// BlockTermsWriter(TermsIndexWriterBase, SegmentWriteState, PostingsWriterBase).
func NewBlockTermsWriter(termsIndexWriter TermsIndexWriterBase, state *index.SegmentWriteState,
	postingsWriter codecs.PostingsWriterBase) (*BlockTermsWriter, error) {
	termsFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, TermsExtension)
	w := &BlockTermsWriter{
		termsIndexWriter: termsIndexWriter,
		maxDoc:           state.SegmentInfo.MaxDoc(),
	}
	w.FieldsConsumerBase = codecs.NewFieldsConsumerBase(w)
	rawOut, err := state.Directory.CreateOutput(termsFileName, state.Context)
	if err != nil {
		return nil, err
	}
	// Gocene's raw outputs do not compute the CRC that CodecUtil.writeFooter
	// needs; the checksum wrapper renders Java's IndexOutput.getChecksum().
	w.out = store.NewChecksumIndexOutput(rawOut)
	success := false
	defer func() {
		if !success {
			util.CloseAllWhileHandlingException(w.out)
		}
	}()
	w.fieldInfos = state.FieldInfos
	if err := codecs.WriteIndexHeader(
		w.out, CodecName, VersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, err
	}
	w.currentField = nil
	w.postingsWriter = postingsWriter

	if err := postingsWriter.Init(w.out, state); err != nil { // have consumer write its format/header
		return nil, err
	}
	success = true
	return w, nil
}

// Write mirrors BlockTermsWriter.write(Fields, NormsProducer). Java's
// for (String field : fields) walks Fields.iterator(); Gocene's spi.Fields
// exposes the same walk through a FieldIterator.
func (w *BlockTermsWriter) Write(fields spi.Fields, norms spi.NormsProducer) error {
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
			break
		}

		terms, err := fields.Terms(field)
		if err != nil {
			return err
		}
		if terms == nil {
			continue
		}

		termsEnum, err := terms.Iterator()
		if err != nil {
			return err
		}

		termsWriter, err := w.addField(w.fieldInfos.FieldInfo(field))
		if err != nil {
			return err
		}

		for {
			term, err := termsEnum.Next()
			if err != nil {
				return err
			}
			if term == nil {
				break
			}

			if err := termsWriter.write(term.BytesValue(), termsEnum, norms); err != nil {
				return err
			}
		}

		if err := termsWriter.finish(); err != nil {
			return err
		}
	}
	return nil
}

func (w *BlockTermsWriter) addField(field *index.FieldInfo) (*blockTermsTermsWriter, error) {
	if util.AssertsEnabled() && !(w.currentField == nil || strings.Compare(w.currentField.Name(), field.Name()) < 0) {
		panic(util.NewAssertionError(nil))
	}
	w.currentField = field
	fieldIndexWriter, err := w.termsIndexWriter.AddField(field, w.out.GetFilePointer())
	if err != nil {
		return nil, err
	}
	return newBlockTermsTermsWriter(w, fieldIndexWriter, field, w.postingsWriter)
}

// Close mirrors BlockTermsWriter.close().
func (w *BlockTermsWriter) Close() (err error) {
	if w.out == nil {
		return nil
	}
	defer func() {
		cerr := util.CloseAll(w.out, w.postingsWriter, w.termsIndexWriter)
		if err == nil {
			err = cerr
		}
		w.out = nil
	}()
	dirStart := w.out.GetFilePointer()

	if err := w.out.WriteVInt(int32(len(w.fields))); err != nil {
		return err
	}
	for _, field := range w.fields {
		if err := w.out.WriteVInt(int32(field.fieldInfo.Number())); err != nil {
			return err
		}
		if err := w.out.WriteVLong(field.numTerms); err != nil {
			return err
		}
		if err := w.out.WriteVLong(field.termsStartPointer); err != nil {
			return err
		}
		if field.fieldInfo.GetIndexOptions() != index.IndexOptionsDocs {
			if err := w.out.WriteVLong(field.sumTotalTermFreq); err != nil {
				return err
			}
		}
		if err := w.out.WriteVLong(field.sumDocFreq); err != nil {
			return err
		}
		if err := w.out.WriteVInt(int32(field.docCount)); err != nil {
			return err
		}
	}
	if err := w.writeTrailer(dirStart); err != nil {
		return err
	}
	return codecs.WriteFooter(w.out)
}

func (w *BlockTermsWriter) writeTrailer(dirStart int64) error {
	return w.out.WriteLong(dirStart)
}

// blockTermsTermEntry is the Go port of the private static class
// BlockTermsWriter.TermEntry.
type blockTermsTermEntry struct {
	term  *util.BytesRefBuilder
	state index.TermState
}

func newBlockTermsTermEntry() *blockTermsTermEntry {
	return &blockTermsTermEntry{term: util.NewBytesRefBuilder()}
}

// blockTermsTermsWriter is the Go port of the inner class
// BlockTermsWriter.TermsWriter.
type blockTermsTermsWriter struct {
	owner             *BlockTermsWriter
	fieldInfo         *index.FieldInfo
	postingsWriter    codecs.PostingsWriterBase
	enumFlags         int
	termsStartPointer int64
	numTerms          int64
	fieldIndexWriter  FieldWriter
	docsSeen          *util.FixedBitSet
	sumTotalTermFreq  int64
	sumDocFreq        int64
	docCount          int

	pendingTerms []*blockTermsTermEntry
	pendingCount int

	lastPrevTerm *util.BytesRefBuilder
	bytesWriter  *store.ByteBuffersDataOutput
}

func newBlockTermsTermsWriter(owner *BlockTermsWriter, fieldIndexWriter FieldWriter, fieldInfo *index.FieldInfo,
	postingsWriter codecs.PostingsWriterBase) (*blockTermsTermsWriter, error) {
	docsSeen, err := util.NewFixedBitSet(owner.maxDoc)
	if err != nil {
		return nil, err
	}
	tw := &blockTermsTermsWriter{
		owner:            owner,
		fieldInfo:        fieldInfo,
		fieldIndexWriter: fieldIndexWriter,
		docsSeen:         docsSeen,
		pendingTerms:     make([]*blockTermsTermEntry, 32),
		lastPrevTerm:     util.NewBytesRefBuilder(),
		bytesWriter:      store.NewByteBuffersDataOutput(),
	}
	for i := range tw.pendingTerms {
		tw.pendingTerms[i] = newBlockTermsTermEntry()
	}
	tw.termsStartPointer = owner.out.GetFilePointer()
	tw.postingsWriter = postingsWriter
	// Gocene's PostingsWriterBase.SetField returns the enum flags that the
	// Java PushPostingsWriterBase keeps in a field for writeTerm.
	if tw.enumFlags, err = postingsWriter.SetField(fieldInfo); err != nil {
		return nil, err
	}
	return tw, nil
}

// writeTerm renders PostingsWriterBase.writeTerm(BytesRef, TermsEnum,
// FixedBitSet, NormsProducer) as implemented by PushPostingsWriterBase:
// Gocene's codecs.PostingsWriterBase declares no writeTerm member, so its body
// is driven through codecs.WriteTerm, as the other Gocene terms dictionary
// writers do. It returns nil when the term has no docs.
func (tw *blockTermsTermsWriter) writeTerm(termsEnum spi.TermsEnum, norms spi.NormsProducer) (index.TermState, error) {
	pusher, ok := tw.postingsWriter.(codecs.PushPostingsWriterBase)
	if !ok {
		return nil, fmt.Errorf("BlockTermsWriter: postingsWriter %T does not implement PushPostingsWriterBase", tw.postingsWriter)
	}
	var normValues index.NumericDocValues
	if tw.fieldInfo.HasNorms() {
		var err error
		if normValues, err = norms.GetNorms(tw.fieldInfo); err != nil {
			return nil, err
		}
	}
	if err := tw.postingsWriter.StartTerm(normValues); err != nil {
		return nil, err
	}
	postingsEnum, err := termsEnum.Postings(tw.enumFlags)
	if err != nil {
		return nil, err
	}
	writeFreqs := tw.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqs
	writePositions := tw.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
	writeOffsets := tw.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	writePayloads := tw.fieldInfo.HasPayloads()
	docFreq, totalTermFreq, err := codecs.WriteTerm(
		pusher, postingsEnum, writeFreqs, writePositions, writeOffsets, writePayloads, tw.docsSeen)
	if err != nil {
		return nil, err
	}
	if docFreq == 0 {
		return nil, nil
	}
	state := tw.postingsWriter.NewTermState()
	base := codecs.BaseState(state)
	base.DocFreq = docFreq
	// PushPostingsWriterBase.writeTerm: state.totalTermFreq = writeFreqs ? totalTermFreq : -1
	if writeFreqs {
		base.TotalTermFreq = totalTermFreq
	} else {
		base.TotalTermFreq = -1
	}
	if err := tw.postingsWriter.FinishTerm(state); err != nil {
		return nil, err
	}
	return state, nil
}

// write mirrors TermsWriter.write(BytesRef, TermsEnum, NormsProducer).
func (tw *blockTermsTermsWriter) write(text *util.BytesRef, termsEnum spi.TermsEnum, norms spi.NormsProducer) error {
	state, err := tw.writeTerm(termsEnum, norms)
	if err != nil {
		return err
	}
	if state == nil {
		// No docs for this term:
		return nil
	}
	base := codecs.BaseState(state)
	tw.sumDocFreq += int64(base.DocFreq)
	tw.sumTotalTermFreq += base.TotalTermFreq

	if util.AssertsEnabled() && !(base.DocFreq > 0) {
		panic(util.NewAssertionError(nil))
	}

	stats := codecs.NewTermStats(base.DocFreq, base.TotalTermFreq)
	isIndexTerm, err := tw.fieldIndexWriter.CheckIndexTerm(text, stats)
	if err != nil {
		return err
	}

	if isIndexTerm {
		if tw.pendingCount > 0 {
			// Instead of writing each term, live, we gather terms
			// in RAM in a pending buffer, and then write the
			// entire block in between index terms:
			if err := tw.flushBlock(); err != nil {
				return err
			}
		}
		if err := tw.fieldIndexWriter.Add(text, stats, tw.owner.out.GetFilePointer()); err != nil {
			return err
		}
	}

	if tw.pendingCount+1 > len(tw.pendingTerms) {
		grown := make([]*blockTermsTermEntry, util.Oversize(tw.pendingCount+1, 8))
		copy(grown, tw.pendingTerms)
		for i := len(tw.pendingTerms); i < len(grown); i++ {
			grown[i] = newBlockTermsTermEntry()
		}
		tw.pendingTerms = grown
	}
	te := tw.pendingTerms[tw.pendingCount]
	te.term.CopyBytesRef(text)
	te.state = state

	tw.pendingCount++
	tw.numTerms++
	return nil
}

// finish mirrors TermsWriter.finish(): it finishes all terms in this field.
func (tw *blockTermsTermsWriter) finish() error {
	if tw.pendingCount > 0 {
		if err := tw.flushBlock(); err != nil {
			return err
		}
	}
	// EOF marker:
	if err := tw.owner.out.WriteVInt(0); err != nil {
		return err
	}

	if err := tw.fieldIndexWriter.Finish(tw.owner.out.GetFilePointer()); err != nil {
		return err
	}
	if tw.numTerms > 0 {
		sumTotalTermFreq := int64(-1)
		if tw.fieldInfo.GetIndexOptions().Subsumes(index.IndexOptionsDocsAndFreqs) {
			sumTotalTermFreq = tw.sumTotalTermFreq
		}
		tw.owner.fields = append(tw.owner.fields, newBlockTermsFieldMetaData(
			tw.fieldInfo,
			tw.numTerms,
			tw.termsStartPointer,
			sumTotalTermFreq,
			tw.sumDocFreq,
			tw.docsSeen.Cardinality()))
	}
	return nil
}

func (tw *blockTermsTermsWriter) sharedPrefix(term1, term2 *util.BytesRef) int {
	if util.AssertsEnabled() && !(term1.Offset == 0) {
		panic(util.NewAssertionError(nil))
	}
	if util.AssertsEnabled() && !(term2.Offset == 0) {
		panic(util.NewAssertionError(nil))
	}
	pos1 := 0
	pos1End := pos1 + min(term1.Length, term2.Length)
	pos2 := 0
	for pos1 < pos1End {
		if term1.Bytes[pos1] != term2.Bytes[pos2] {
			return pos1
		}
		pos1++
		pos2++
	}
	return pos1
}

func toIntExact(v int64) (int32, error) {
	if v > math.MaxInt32 || v < math.MinInt32 {
		return 0, fmt.Errorf("integer overflow")
	}
	return int32(v), nil
}

// copyBytesWriterTo writes the size of the buffered bytes, copies them to out
// and resets the buffer.
func (tw *blockTermsTermsWriter) copyBytesWriterTo(out store.IndexOutput) error {
	size, err := toIntExact(tw.bytesWriter.Size())
	if err != nil {
		return err
	}
	if err := out.WriteVInt(size); err != nil {
		return err
	}
	if err := tw.bytesWriter.CopyTo(out); err != nil {
		return err
	}
	tw.bytesWriter.Reset()
	return nil
}

// flushBlock mirrors TermsWriter.flushBlock().
func (tw *blockTermsTermsWriter) flushBlock() error {
	out := tw.owner.out
	// First pass: compute common prefix for all terms
	// in the block, against term before first term in
	// this block:
	commonPrefix := tw.sharedPrefix(tw.lastPrevTerm.Get(), tw.pendingTerms[0].term.Get())
	for termCount := 1; termCount < tw.pendingCount; termCount++ {
		commonPrefix = min(commonPrefix, tw.sharedPrefix(tw.lastPrevTerm.Get(), tw.pendingTerms[termCount].term.Get()))
	}

	if err := out.WriteVInt(int32(tw.pendingCount)); err != nil {
		return err
	}
	if err := out.WriteVInt(int32(commonPrefix)); err != nil {
		return err
	}

	// 2nd pass: write suffixes, as separate byte[] blob
	for termCount := 0; termCount < tw.pendingCount; termCount++ {
		suffix := tw.pendingTerms[termCount].term.Length() - commonPrefix
		// TODO: cutover to better intblock codec, instead
		// of interleaving here:
		if err := tw.bytesWriter.WriteVInt(int32(suffix)); err != nil {
			return err
		}
		if err := tw.bytesWriter.WriteBytes(tw.pendingTerms[termCount].term.Bytes(), commonPrefix, suffix); err != nil {
			return err
		}
	}
	if err := tw.copyBytesWriterTo(out); err != nil {
		return err
	}

	// 3rd pass: write the freqs as byte[] blob
	// TODO: cutover to better intblock codec.  simple64?
	// write prefix, suffix first:
	for termCount := 0; termCount < tw.pendingCount; termCount++ {
		state := tw.pendingTerms[termCount].state
		if util.AssertsEnabled() && !(state != nil) {
			panic(util.NewAssertionError(nil))
		}
		base := codecs.BaseState(state)
		if err := tw.bytesWriter.WriteVInt(int32(base.DocFreq)); err != nil {
			return err
		}
		if tw.fieldInfo.GetIndexOptions() != index.IndexOptionsDocs {
			if err := tw.bytesWriter.WriteVLong(base.TotalTermFreq - int64(base.DocFreq)); err != nil {
				return err
			}
		}
	}
	if err := tw.copyBytesWriterTo(out); err != nil {
		return err
	}

	// 4th pass: write the metadata
	absolute := true
	for termCount := 0; termCount < tw.pendingCount; termCount++ {
		state := tw.pendingTerms[termCount].state
		if err := tw.postingsWriter.EncodeTerm(tw.bytesWriter, tw.fieldInfo, state, absolute); err != nil {
			return err
		}
		absolute = false
	}
	if err := tw.copyBytesWriterTo(out); err != nil {
		return err
	}

	tw.lastPrevTerm.CopyBuilder(tw.pendingTerms[tw.pendingCount-1].term)
	tw.pendingCount = 0
	return nil
}
