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

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// FixedGapTermsIndexWriter selects every Nth term as an index term and holds
// the term bytes (mostly) fully expanded in memory. This terms index supports
// seeking by ord. See [VariableGapTermsIndexWriter] for a more memory
// efficient terms index that does not support seeking by ord.
//
// Port of org.apache.lucene.codecs.blockterms.FixedGapTermsIndexWriter
// (Apache Lucene 10.5.0). The format constants are declared with the reader
// (FixedGapTermsIndexExtension, FixedGapCodecName, FixedGapVersionStart,
// FixedGapVersionCurrent, FixedGapBlocksize, DefaultTermIndexInterval).
//
// @lucene.experimental
type FixedGapTermsIndexWriter struct {
	// out mirrors the protected field IndexOutput out.
	out store.IndexOutput

	termIndexInterval int

	fields []*simpleFieldWriter
}

var _ TermsIndexWriterBase = (*FixedGapTermsIndexWriter)(nil)

// NewFixedGapTermsIndexWriter renders the constructor
// FixedGapTermsIndexWriter(SegmentWriteState), which uses
// DEFAULT_TERM_INDEX_INTERVAL.
func NewFixedGapTermsIndexWriter(state *index.SegmentWriteState) (*FixedGapTermsIndexWriter, error) {
	return NewFixedGapTermsIndexWriterWithTermIndexInterval(state, DefaultTermIndexInterval)
}

// NewFixedGapTermsIndexWriterWithTermIndexInterval renders the constructor
// FixedGapTermsIndexWriter(SegmentWriteState, int termIndexInterval). The
// IllegalArgumentException for a non-positive interval is returned as an
// error.
func NewFixedGapTermsIndexWriterWithTermIndexInterval(state *index.SegmentWriteState, termIndexInterval int) (*FixedGapTermsIndexWriter, error) {
	if termIndexInterval <= 0 {
		return nil, fmt.Errorf("invalid termIndexInterval: %d", termIndexInterval)
	}
	w := &FixedGapTermsIndexWriter{termIndexInterval: termIndexInterval}
	indexFileName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, FixedGapTermsIndexExtension)
	rawOut, err := state.Directory.CreateOutput(indexFileName, state.Context)
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
	if err := codecs.WriteIndexHeader(
		w.out, FixedGapCodecName, FixedGapVersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, err
	}
	if err := w.out.WriteVInt(int32(termIndexInterval)); err != nil {
		return nil, err
	}
	if err := w.out.WriteVInt(int32(packed.VersionCurrent)); err != nil {
		return nil, err
	}
	if err := w.out.WriteVInt(FixedGapBlocksize); err != nil {
		return nil, err
	}
	success = true
	return w, nil
}

// AddField mirrors FixedGapTermsIndexWriter.addField(FieldInfo, long).
func (w *FixedGapTermsIndexWriter) AddField(field *index.FieldInfo, termsFilePointer int64) (FieldWriter, error) {
	writer, err := newSimpleFieldWriter(w, field, termsFilePointer)
	if err != nil {
		return nil, err
	}
	w.fields = append(w.fields, writer)
	return writer, nil
}

// IndexedTermPrefixLength mirrors the protected
// FixedGapTermsIndexWriter.indexedTermPrefixLength(BytesRef, BytesRef).
//
// NOTE: if your codec does not sort in unicode code point order, you must
// override this method, to simply return indexedTerm.length.
func (w *FixedGapTermsIndexWriter) IndexedTermPrefixLength(priorTerm, indexedTerm *util.BytesRef) (int, error) {
	// As long as codec sorts terms in unicode codepoint
	// order, we can safely strip off the non-distinguishing
	// suffix to save RAM in the loaded terms index.
	return util.SortKeyLength(priorTerm, indexedTerm)
}

// simpleFieldWriter is the Go port of the private inner class
// FixedGapTermsIndexWriter.SimpleFieldWriter.
type simpleFieldWriter struct {
	owner              *FixedGapTermsIndexWriter
	fieldInfo          *index.FieldInfo
	numIndexTerms      int
	indexStart         int64
	termsStart         int64
	packedIndexStart   int64
	packedOffsetsStart int64
	numTerms           int64

	offsetsBuffer *store.ByteBuffersDataOutput
	termOffsets   *packed.MonotonicBlockPackedWriter
	currentOffset int64

	addressBuffer *store.ByteBuffersDataOutput
	termAddresses *packed.MonotonicBlockPackedWriter

	lastTerm *util.BytesRefBuilder
}

func newSimpleFieldWriter(owner *FixedGapTermsIndexWriter, fieldInfo *index.FieldInfo, termsFilePointer int64) (*simpleFieldWriter, error) {
	f := &simpleFieldWriter{
		owner:         owner,
		fieldInfo:     fieldInfo,
		offsetsBuffer: store.NewByteBuffersDataOutput(),
		addressBuffer: store.NewByteBuffersDataOutput(),
		lastTerm:      util.NewBytesRefBuilder(),
	}
	var err error
	if f.termOffsets, err = packed.NewMonotonicBlockPackedWriter(f.offsetsBuffer, FixedGapBlocksize); err != nil {
		return nil, err
	}
	if f.termAddresses, err = packed.NewMonotonicBlockPackedWriter(f.addressBuffer, FixedGapBlocksize); err != nil {
		return nil, err
	}
	f.indexStart = owner.out.GetFilePointer()
	f.termsStart = termsFilePointer
	// we write terms+1 offsets, term n's length is n+1 - n
	if err := f.termOffsets.Add(0); err != nil {
		return nil, err
	}
	return f, nil
}

// CheckIndexTerm mirrors SimpleFieldWriter.checkIndexTerm(BytesRef, TermStats).
func (f *simpleFieldWriter) CheckIndexTerm(text *util.BytesRef, stats codecs.TermStats) (bool, error) {
	// First term is first indexed term:
	n := f.numTerms
	f.numTerms++
	if 0 == n%int64(f.owner.termIndexInterval) {
		return true, nil
	}
	if 0 == f.numTerms%int64(f.owner.termIndexInterval) {
		// save last term just before next index term so we
		// can compute wasted suffix
		f.lastTerm.CopyBytesRef(text)
	}
	return false, nil
}

// Add mirrors SimpleFieldWriter.add(BytesRef, TermStats, long).
func (f *simpleFieldWriter) Add(text *util.BytesRef, stats codecs.TermStats, termsFilePointer int64) error {
	var indexedTermLength int
	if f.numIndexTerms == 0 {
		// no previous term: no bytes to write
		indexedTermLength = 0
	} else {
		var err error
		if indexedTermLength, err = f.owner.IndexedTermPrefixLength(f.lastTerm.Get(), text); err != nil {
			return err
		}
	}

	// write only the min prefix that shows the diff
	// against prior term
	if err := f.owner.out.WriteBytes(text.Bytes, text.Offset, indexedTermLength); err != nil {
		return err
	}

	// save delta terms pointer
	if err := f.termAddresses.Add(termsFilePointer - f.termsStart); err != nil {
		return err
	}

	// save term length (in bytes)
	if util.AssertsEnabled() && !(indexedTermLength <= math.MaxInt16) {
		panic(util.NewAssertionError(nil))
	}
	f.currentOffset += int64(indexedTermLength)
	if err := f.termOffsets.Add(f.currentOffset); err != nil {
		return err
	}

	f.lastTerm.CopyBytesRef(text)
	f.numIndexTerms++
	return nil
}

// Finish mirrors SimpleFieldWriter.finish(long).
func (f *simpleFieldWriter) Finish(termsFilePointer int64) error {
	out := f.owner.out
	// write primary terms dict offsets
	f.packedIndexStart = out.GetFilePointer()

	// relative to our indexStart
	if err := f.termAddresses.Finish(); err != nil {
		return err
	}
	if err := f.addressBuffer.CopyTo(out); err != nil {
		return err
	}

	f.packedOffsetsStart = out.GetFilePointer()

	// write offsets into the byte[] terms
	if err := f.termOffsets.Finish(); err != nil {
		return err
	}
	if err := f.offsetsBuffer.CopyTo(out); err != nil {
		return err
	}

	// our referrer holds onto us, while other fields are
	// being written, so don't tie up this RAM:
	f.termOffsets, f.termAddresses = nil, nil
	f.addressBuffer = nil
	f.offsetsBuffer = nil
	return nil
}

// Close mirrors FixedGapTermsIndexWriter.close().
func (w *FixedGapTermsIndexWriter) Close() (err error) {
	if w.out == nil {
		return nil
	}
	success := false
	defer func() {
		if success {
			err = w.out.Close()
		} else {
			util.CloseAllWhileHandlingException(w.out)
		}
		w.out = nil
	}()
	dirStart := w.out.GetFilePointer()
	fieldCount := len(w.fields)

	nonNullFieldCount := 0
	for i := 0; i < fieldCount; i++ {
		if w.fields[i].numIndexTerms > 0 {
			nonNullFieldCount++
		}
	}

	if err := w.out.WriteVInt(int32(nonNullFieldCount)); err != nil {
		return err
	}
	for i := 0; i < fieldCount; i++ {
		field := w.fields[i]
		if field.numIndexTerms > 0 {
			if err := w.out.WriteVInt(int32(field.fieldInfo.Number())); err != nil {
				return err
			}
			if err := w.out.WriteVInt(int32(field.numIndexTerms)); err != nil {
				return err
			}
			if err := w.out.WriteVLong(field.termsStart); err != nil {
				return err
			}
			if err := w.out.WriteVLong(field.indexStart); err != nil {
				return err
			}
			if err := w.out.WriteVLong(field.packedIndexStart); err != nil {
				return err
			}
			if err := w.out.WriteVLong(field.packedOffsetsStart); err != nil {
				return err
			}
		}
	}
	if err := w.writeTrailer(dirStart); err != nil {
		return err
	}
	if err := codecs.WriteFooter(w.out); err != nil {
		return err
	}
	success = true
	return nil
}

func (w *FixedGapTermsIndexWriter) writeTrailer(dirStart int64) error {
	return w.out.WriteLong(dirStart)
}
