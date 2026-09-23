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
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// VariableGapTermsIndexWriter selects index terms according to the provided
// pluggable [IndexTermSelector] and stores them in a prefix trie that is
// loaded entirely in RAM, stored as an FST. This terms index only supports
// unsigned byte term sort order (unicode codepoint order when the bytes are
// UTF8).
//
// Port of org.apache.lucene.codecs.blockterms.VariableGapTermsIndexWriter
// (Apache Lucene 10.5.0). The format constants are declared with the reader
// (VariableGapTermsIndexExtension, VariableGapTermsMetaExtension,
// VariableGapMetaCodecName, VariableGapCodecName, VariableGapVersionStart,
// VariableGapVersionCurrent).
//
// @lucene.experimental
type VariableGapTermsIndexWriter struct {
	// metaOut and out mirror the protected fields of the same names.
	metaOut store.IndexOutput
	out     store.IndexOutput

	fieldInfos *index.FieldInfos // unread

	policy IndexTermSelector
}

var _ TermsIndexWriterBase = (*VariableGapTermsIndexWriter)(nil)

// IndexTermSelector is the hook for selecting which terms should be placed in
// the terms index: NewField is called at the start of each new field, and
// IsIndexTerm for each term in that field.
//
// Port of the public abstract static class
// VariableGapTermsIndexWriter.IndexTermSelector.
//
// @lucene.experimental
type IndexTermSelector interface {
	// IsIndexTerm is called sequentially on every term being written,
	// returning true if this term should be indexed.
	IsIndexTerm(term *util.BytesRef, stats codecs.TermStats) bool
	// NewField is called when a new field is started.
	NewField(fieldInfo *index.FieldInfo)
}

// EveryNTermSelector applies the same policy as [FixedGapTermsIndexWriter].
//
// Port of VariableGapTermsIndexWriter.EveryNTermSelector.
type EveryNTermSelector struct {
	count    int
	interval int
}

var _ IndexTermSelector = (*EveryNTermSelector)(nil)

// NewEveryNTermSelector mirrors EveryNTermSelector(int interval).
func NewEveryNTermSelector(interval int) *EveryNTermSelector {
	// First term is first indexed term:
	return &EveryNTermSelector{interval: interval, count: interval}
}

// IsIndexTerm mirrors EveryNTermSelector.isIndexTerm(BytesRef, TermStats).
func (s *EveryNTermSelector) IsIndexTerm(term *util.BytesRef, stats codecs.TermStats) bool {
	if s.count >= s.interval {
		s.count = 1
		return true
	}
	s.count++
	return false
}

// NewField mirrors EveryNTermSelector.newField(FieldInfo).
func (s *EveryNTermSelector) NewField(fieldInfo *index.FieldInfo) {
	s.count = s.interval
}

// EveryNOrDocFreqTermSelector sets an index term when docFreq >=
// docFreqThresh, or every interval terms. This should reduce seek time to high
// docFreq terms.
//
// Port of VariableGapTermsIndexWriter.EveryNOrDocFreqTermSelector.
type EveryNOrDocFreqTermSelector struct {
	count         int
	docFreqThresh int
	interval      int
}

var _ IndexTermSelector = (*EveryNOrDocFreqTermSelector)(nil)

// NewEveryNOrDocFreqTermSelector mirrors
// EveryNOrDocFreqTermSelector(int docFreqThresh, int interval).
func NewEveryNOrDocFreqTermSelector(docFreqThresh, interval int) *EveryNOrDocFreqTermSelector {
	// First term is first indexed term:
	return &EveryNOrDocFreqTermSelector{interval: interval, docFreqThresh: docFreqThresh, count: interval}
}

// IsIndexTerm mirrors EveryNOrDocFreqTermSelector.isIndexTerm(BytesRef, TermStats).
func (s *EveryNOrDocFreqTermSelector) IsIndexTerm(term *util.BytesRef, stats codecs.TermStats) bool {
	if stats.DocFreq >= s.docFreqThresh || s.count >= s.interval {
		s.count = 1
		return true
	}
	s.count++
	return false
}

// NewField mirrors EveryNOrDocFreqTermSelector.newField(FieldInfo).
func (s *EveryNOrDocFreqTermSelector) NewField(fieldInfo *index.FieldInfo) {
	s.count = s.interval
}

// NewVariableGapTermsIndexWriter mirrors the constructor
// VariableGapTermsIndexWriter(SegmentWriteState, IndexTermSelector).
func NewVariableGapTermsIndexWriter(state *index.SegmentWriteState, policy IndexTermSelector) (*VariableGapTermsIndexWriter, error) {
	w := &VariableGapTermsIndexWriter{fieldInfos: state.FieldInfos, policy: policy}

	metaFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, VariableGapTermsMetaExtension)
	indexFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, VariableGapTermsIndexExtension)

	success := false
	defer func() {
		if !success {
			// IOUtils.closeWhileHandlingException(this)
			util.CloseAllWhileHandlingException(w.out, w.metaOut)
		}
	}()
	// Gocene's raw outputs do not compute the CRC that CodecUtil.writeFooter
	// needs; the checksum wrapper renders Java's IndexOutput.getChecksum().
	rawMetaOut, err := state.Directory.CreateOutput(metaFileName, state.Context)
	if err != nil {
		return nil, err
	}
	w.metaOut = store.NewChecksumIndexOutput(rawMetaOut)
	rawOut, err := state.Directory.CreateOutput(indexFileName, state.Context)
	if err != nil {
		return nil, err
	}
	w.out = store.NewChecksumIndexOutput(rawOut)
	if err := codecs.WriteIndexHeader(
		w.metaOut, VariableGapMetaCodecName, VariableGapVersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, err
	}
	if err := codecs.WriteIndexHeader(
		w.out, VariableGapCodecName, VariableGapVersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, err
	}
	success = true
	return w, nil
}

// AddField mirrors VariableGapTermsIndexWriter.addField(FieldInfo, long).
func (w *VariableGapTermsIndexWriter) AddField(field *index.FieldInfo, termsFilePointer int64) (FieldWriter, error) {
	w.policy.NewField(field)
	return newFSTFieldWriter(w, field, termsFilePointer)
}

// IndexedTermPrefixLength mirrors the protected
// VariableGapTermsIndexWriter.indexedTermPrefixLength(BytesRef, BytesRef).
//
// NOTE: if your codec does not sort in unicode code point order, you must
// override this method, to simply return indexedTerm.length.
func (w *VariableGapTermsIndexWriter) IndexedTermPrefixLength(priorTerm, indexedTerm *util.BytesRef) int {
	// As long as codec sorts terms in unicode codepoint
	// order, we can safely strip off the non-distinguishing
	// suffix to save RAM in the loaded terms index.
	idxTermOffset := indexedTerm.Offset
	priorTermOffset := priorTerm.Offset
	limit := min(priorTerm.Length, indexedTerm.Length)
	for byteIdx := 0; byteIdx < limit; byteIdx++ {
		if priorTerm.Bytes[priorTermOffset+byteIdx] != indexedTerm.Bytes[idxTermOffset+byteIdx] {
			return byteIdx + 1
		}
	}
	return min(1+priorTerm.Length, indexedTerm.Length)
}

// fstFieldWriter is the Go port of the private inner class
// VariableGapTermsIndexWriter.FSTFieldWriter.
type fstFieldWriter struct {
	owner                 *VariableGapTermsIndexWriter
	fstCompiler           *fst.FSTCompiler[int64]
	fstOutputs            fst.Outputs[int64]
	startTermsFilePointer int64

	fieldInfo *index.FieldInfo
	fst       *fst.FST[int64]

	lastTerm *util.BytesRefBuilder
	first    bool

	scratchIntsRef *util.IntsRefBuilder
}

func newFSTFieldWriter(owner *VariableGapTermsIndexWriter, fieldInfo *index.FieldInfo, termsFilePointer int64) (*fstFieldWriter, error) {
	f := &fstFieldWriter{
		owner:          owner,
		fieldInfo:      fieldInfo,
		fstOutputs:     fst.PositiveIntOutputs(),
		lastTerm:       util.NewBytesRefBuilder(),
		first:          true,
		scratchIntsRef: util.NewIntsRefBuilder(),
	}
	f.fstCompiler = fst.NewFSTCompilerBuilder[int64](fst.InputTypeByte1, f.fstOutputs).Build()

	// Always put empty string in
	if err := f.fstCompiler.Add(util.NewIntsRefEmpty(), termsFilePointer); err != nil {
		return nil, err
	}
	f.startTermsFilePointer = termsFilePointer
	return f, nil
}

// CheckIndexTerm mirrors FSTFieldWriter.checkIndexTerm(BytesRef, TermStats).
func (f *fstFieldWriter) CheckIndexTerm(text *util.BytesRef, stats codecs.TermStats) (bool, error) {
	// NOTE: we must force the first term per field to be
	// indexed, in case policy doesn't:
	if f.owner.policy.IsIndexTerm(text, stats) || f.first {
		f.first = false
		return true, nil
	}
	f.lastTerm.CopyBytesRef(text)
	return false, nil
}

// Add mirrors FSTFieldWriter.add(BytesRef, TermStats, long).
func (f *fstFieldWriter) Add(text *util.BytesRef, stats codecs.TermStats, termsFilePointer int64) error {
	if text.Length == 0 {
		// We already added empty string in ctor
		if util.AssertsEnabled() && !(termsFilePointer == f.startTermsFilePointer) {
			panic(util.NewAssertionError(nil))
		}
		return nil
	}
	lengthSave := text.Length
	text.Length = f.owner.IndexedTermPrefixLength(f.lastTerm.Get(), text)
	err := f.fstCompiler.Add(fst.ToIntsRef(text, f.scratchIntsRef), termsFilePointer)
	text.Length = lengthSave
	if err != nil {
		return err
	}
	f.lastTerm.CopyBytesRef(text)
	return nil
}

// Finish mirrors FSTFieldWriter.finish(long).
func (f *fstFieldWriter) Finish(termsFilePointer int64) error {
	metadata, err := f.fstCompiler.Compile()
	if err != nil {
		return err
	}
	if f.fst, err = fst.FromFSTReader(metadata, f.fstCompiler.GetFSTReader()); err != nil {
		return err
	}
	if f.fst != nil {
		if err := f.owner.metaOut.WriteInt(int32(f.fieldInfo.Number())); err != nil {
			return err
		}
		if err := f.owner.metaOut.WriteVLong(f.owner.out.GetFilePointer()); err != nil {
			return err
		}
		if err := f.fst.Save(f.owner.metaOut, f.owner.out); err != nil {
			return err
		}
	}
	return nil
}

// Close mirrors VariableGapTermsIndexWriter.close().
func (w *VariableGapTermsIndexWriter) Close() (err error) {
	defer func() {
		cerr := util.CloseAll(w.out, w.metaOut)
		if err == nil {
			err = cerr
		}
		w.out = nil
		w.metaOut = nil
	}()
	if w.metaOut != nil {
		if err := w.metaOut.WriteInt(-1); err != nil {
			return err
		}
		if err := codecs.WriteFooter(w.metaOut); err != nil {
			return err
		}
	}
	if w.out != nil {
		if err := codecs.WriteFooter(w.out); err != nil {
			return err
		}
	}
	return nil
}
