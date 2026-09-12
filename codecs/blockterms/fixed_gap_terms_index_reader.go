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

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Format constants, ported from
// org.apache.lucene.codecs.blockterms.FixedGapTermsIndexWriter
// (Apache Lucene 10.5.0).
const (
	// FixedGapTermsIndexExtension mirrors
	// FixedGapTermsIndexWriter.TERMS_INDEX_EXTENSION.
	FixedGapTermsIndexExtension = "tii"

	// FixedGapCodecName mirrors FixedGapTermsIndexWriter.CODEC_NAME.
	FixedGapCodecName = "FixedGapTermsIndex"
	// FixedGapVersionStart mirrors FixedGapTermsIndexWriter.VERSION_START.
	FixedGapVersionStart = int32(4)
	// FixedGapVersionCurrent mirrors FixedGapTermsIndexWriter.VERSION_CURRENT.
	FixedGapVersionCurrent = FixedGapVersionStart

	// FixedGapBlocksize mirrors FixedGapTermsIndexWriter.BLOCKSIZE.
	FixedGapBlocksize = 4096

	// DefaultTermIndexInterval mirrors
	// FixedGapTermsIndexWriter.DEFAULT_TERM_INDEX_INTERVAL.
	DefaultTermIndexInterval = 32
)

// fixedGapPagedBytesBits mirrors FixedGapTermsIndexReader.PAGED_BYTES_BITS.
const fixedGapPagedBytesBits = 15

// vIntIndexInput adapts a store.IndexInput to the input interface expected by
// packed.NewMonotonicBlockPackedReader.
//
// Java's DataInput declares readVInt()/readVLong() as base-class methods, so
// MonotonicBlockPackedReader.of() accepts a bare IndexInput. In Gocene those
// two reads are package-level functions over store.DataInput rather than
// methods on store.IndexInput, so the reader's input interface is satisfied
// by this thin delegating wrapper. It changes no bytes and no read order.
type vIntIndexInput struct {
	store.IndexInput
}

// ReadVInt delegates to store.ReadVInt over the wrapped input.
func (in vIntIndexInput) ReadVInt() (int32, error) { return store.ReadVInt(in.IndexInput) }

// ReadVLong delegates to store.ReadVLong over the wrapped input.
func (in vIntIndexInput) ReadVLong() (int64, error) { return store.ReadVLong(in.IndexInput) }

// FixedGapTermsIndexReader is a TermsIndexReader for simple every-Nth terms
// indexes.
//
// Port of org.apache.lucene.codecs.blockterms.FixedGapTermsIndexReader
// (Apache Lucene 10.5.0).
//
// See FixedGapTermsIndexWriter for the format this reader consumes.
type FixedGapTermsIndexReader struct {
	// NOTE: int64 is overkill here, but it is used in a number of places to
	// multiply out the actual ord, and int would overflow during those
	// multiplies. So, to avoid having to widen each multiply (error-prone),
	// int64 is used throughout, exactly as Lucene does.
	indexInterval int64

	packedIntsVersion int
	blocksize         int

	// termBytesReader holds the single logical byte slice shared by all fields.
	termBytesReader *util.Reader

	fields map[string]*fixedGapFieldIndexData
}

// compile-time check that FixedGapTermsIndexReader implements TermsIndexReader.
var _ TermsIndexReader = (*FixedGapTermsIndexReader)(nil)

// NewFixedGapTermsIndexReader opens the .tii terms index of the segment
// described by state and loads every field's index into memory.
//
// Port of the FixedGapTermsIndexReader(SegmentReadState) constructor.
func NewFixedGapTermsIndexReader(state *spi.SegmentReadState) (*FixedGapTermsIndexReader, error) {
	termBytes, err := util.NewPagedBytes(fixedGapPagedBytesBits)
	if err != nil {
		return nil, err
	}

	r := &FixedGapTermsIndexReader{
		fields: make(map[string]*fixedGapFieldIndexData),
	}

	fileName := codecs.GetSegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, FixedGapTermsIndexExtension)

	// Java passes state.context here; Gocene's SegmentReadState carries no
	// IOContext field, so the read context is used.
	in, err := state.Directory.OpenInput(fileName, store.IOContextRead)
	if err != nil {
		return nil, err
	}

	// Java's finally block freezes termBytes on both the success and the
	// failure path, and closes the input either way.
	err = r.readIndex(in, termBytes, state)
	closeErr := in.Close()

	reader, freezeErr := termBytes.Freeze(true)
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = freezeErr
	}
	if err != nil {
		return nil, err
	}
	r.termBytesReader = reader

	return r, nil
}

// readIndex reads the header, the global index parameters and the per-field
// directory of the .tii file. It is the body of the Java constructor's try
// block.
func (r *FixedGapTermsIndexReader) readIndex(
	in store.IndexInput, termBytes *store.PagedBytes, state *spi.SegmentReadState) error {

	if _, err := codecs.CheckIndexHeader(
		in,
		FixedGapCodecName,
		FixedGapVersionCurrent,
		FixedGapVersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix); err != nil {
		return err
	}

	if _, err := codecs.ChecksumEntireFile(in); err != nil {
		return err
	}

	indexInterval, err := store.ReadVInt(in)
	if err != nil {
		return err
	}
	if indexInterval < 1 {
		return fmt.Errorf("blockterms: invalid indexInterval: %d", indexInterval)
	}
	r.indexInterval = int64(indexInterval)

	packedIntsVersion, err := store.ReadVInt(in)
	if err != nil {
		return err
	}
	r.packedIntsVersion = int(packedIntsVersion)

	blocksize, err := store.ReadVInt(in)
	if err != nil {
		return err
	}
	r.blocksize = int(blocksize)

	if err := seekDir(in); err != nil {
		return err
	}

	// Read directory
	numFields, err := store.ReadVInt(in)
	if err != nil {
		return err
	}
	if numFields < 0 {
		return fmt.Errorf("blockterms: invalid numFields: %d", numFields)
	}

	for i := int32(0); i < numFields; i++ {
		field, err := store.ReadVInt(in)
		if err != nil {
			return err
		}
		// TODO (from Lucene): change this to a vLong if the writer is fixed to
		// support > 2B index terms.
		numIndexTermsInt, err := store.ReadVInt(in)
		if err != nil {
			return err
		}
		numIndexTerms := int64(numIndexTermsInt)
		if numIndexTerms < 0 {
			return fmt.Errorf("blockterms: invalid numIndexTerms: %d", numIndexTerms)
		}
		termsStart, err := store.ReadVLong(in)
		if err != nil {
			return err
		}
		indexStart, err := store.ReadVLong(in)
		if err != nil {
			return err
		}
		packedIndexStart, err := store.ReadVLong(in)
		if err != nil {
			return err
		}
		packedOffsetsStart, err := store.ReadVLong(in)
		if err != nil {
			return err
		}
		if packedIndexStart < indexStart {
			return fmt.Errorf(
				"blockterms: invalid packedIndexStart: %d indexStart: %d numIndexTerms: %d",
				packedIndexStart, indexStart, numIndexTerms)
		}

		fieldInfo := state.FieldInfos.FieldInfoByNumber(int(field))
		if fieldInfo == nil {
			return fmt.Errorf("blockterms: invalid field number: %d", field)
		}

		data, err := r.newFieldIndexData(
			in, termBytes, indexStart, termsStart, packedIndexStart, packedOffsetsStart, numIndexTerms)
		if err != nil {
			return err
		}
		if _, dup := r.fields[fieldInfo.Name()]; dup {
			return fmt.Errorf("blockterms: duplicate field: %s", fieldInfo.Name())
		}
		r.fields[fieldInfo.Name()] = data
	}

	return nil
}

// fixedGapIndexEnum is the Go port of FixedGapTermsIndexReader.IndexEnum.
type fixedGapIndexEnum struct {
	reader     *FixedGapTermsIndexReader
	fieldIndex *fixedGapFieldIndexData
	term       *util.BytesRef
	ord        int64
}

// compile-time check that fixedGapIndexEnum implements TermsIndexEnum.
var _ TermsIndexEnum = (*fixedGapIndexEnum)(nil)

// Term returns the current indexed term.
//
// Port of IndexEnum.term().
func (e *fixedGapIndexEnum) Term() *util.BytesRef {
	return e.term
}

// Seek binary-searches the indexed terms for the largest one that is <=
// target and returns its file pointer into the main terms dictionary.
//
// Port of IndexEnum.seek(BytesRef).
func (e *fixedGapIndexEnum) Seek(target *util.BytesRef) (int64, error) {
	var lo int64 // binary search
	hi := e.fieldIndex.numIndexTerms - 1

	for hi >= lo {
		mid := int64(uint64(lo+hi) >> 1)

		offset := e.fieldIndex.termOffsets.Get(mid)
		length := int(e.fieldIndex.termOffsets.Get(1+mid) - offset)
		if err := e.reader.termBytesReader.FillSlice(
			e.term, e.fieldIndex.termBytesStart+offset, length); err != nil {
			return 0, err
		}

		delta := util.BytesRefCompare(target, e.term)
		switch {
		case delta < 0:
			hi = mid - 1
		case delta > 0:
			lo = mid + 1
		default:
			e.ord = mid * e.reader.indexInterval
			return e.fieldIndex.termsStart + e.fieldIndex.termsDictOffsets.Get(mid), nil
		}
	}

	if hi < 0 {
		hi = 0
	}

	offset := e.fieldIndex.termOffsets.Get(hi)
	length := int(e.fieldIndex.termOffsets.Get(1+hi) - offset)
	if err := e.reader.termBytesReader.FillSlice(
		e.term, e.fieldIndex.termBytesStart+offset, length); err != nil {
		return 0, err
	}

	e.ord = hi * e.reader.indexInterval
	return e.fieldIndex.termsStart + e.fieldIndex.termsDictOffsets.Get(hi), nil
}

// Next advances to the next indexed term, returning -1 at end.
//
// Port of IndexEnum.next().
func (e *fixedGapIndexEnum) Next() (int64, error) {
	idx := 1 + (e.ord / e.reader.indexInterval)
	if idx >= e.fieldIndex.numIndexTerms {
		return -1, nil
	}
	e.ord += e.reader.indexInterval

	offset := e.fieldIndex.termOffsets.Get(idx)
	length := int(e.fieldIndex.termOffsets.Get(1+idx) - offset)
	if err := e.reader.termBytesReader.FillSlice(
		e.term, e.fieldIndex.termBytesStart+offset, length); err != nil {
		return 0, err
	}
	return e.fieldIndex.termsStart + e.fieldIndex.termsDictOffsets.Get(idx), nil
}

// Ord returns the ordinal of the current indexed term.
//
// Port of IndexEnum.ord().
func (e *fixedGapIndexEnum) Ord() int64 {
	return e.ord
}

// SeekOrd moves to the indexed term covering ord and returns its file pointer.
// The caller must ensure ord is in bounds.
//
// Port of IndexEnum.seek(long).
func (e *fixedGapIndexEnum) SeekOrd(ord int64) (int64, error) {
	idx := ord / e.reader.indexInterval
	// caller must ensure ord is in bounds
	if idx >= e.fieldIndex.numIndexTerms {
		return 0, fmt.Errorf(
			"blockterms: ord %d out of bounds: numIndexTerms=%d", ord, e.fieldIndex.numIndexTerms)
	}
	offset := e.fieldIndex.termOffsets.Get(idx)
	length := int(e.fieldIndex.termOffsets.Get(1+idx) - offset)
	if err := e.reader.termBytesReader.FillSlice(
		e.term, e.fieldIndex.termBytesStart+offset, length); err != nil {
		return 0, err
	}
	e.ord = idx * e.reader.indexInterval
	return e.fieldIndex.termsStart + e.fieldIndex.termsDictOffsets.Get(idx), nil
}

// SupportsOrd reports that this reader implements the ordinal operations.
//
// Port of FixedGapTermsIndexReader.supportsOrd().
func (r *FixedGapTermsIndexReader) SupportsOrd() bool {
	return true
}

// fixedGapFieldIndexData is the Go port of
// FixedGapTermsIndexReader.FieldIndexData.
type fixedGapFieldIndexData struct {
	// termBytesStart is where this field's terms begin in the packed byte
	// slice.
	termBytesStart int64

	// termOffsets holds offsets into the index termBytes.
	termOffsets *packed.MonotonicBlockPackedReader

	// termsDictOffsets holds index pointers into the main terms dict.
	termsDictOffsets *packed.MonotonicBlockPackedReader

	numIndexTerms int64
	termsStart    int64
}

// newFieldIndexData slurps one field's term bytes and both packed offset
// streams from disk.
//
// Port of the FieldIndexData(IndexInput, PagedBytes, long, long, long, long,
// long) constructor.
func (r *FixedGapTermsIndexReader) newFieldIndexData(
	in store.IndexInput,
	termBytes *store.PagedBytes,
	indexStart, termsStart, packedIndexStart, packedOffsetsStart, numIndexTerms int64,
) (*fixedGapFieldIndexData, error) {

	// packedOffsetsStart is part of the on-disk directory entry but is not
	// needed to read the stream, exactly as in Lucene: the two packed readers
	// are consumed back-to-back from packedIndexStart onwards.
	_ = packedOffsetsStart

	d := &fixedGapFieldIndexData{
		termsStart:     termsStart,
		termBytesStart: termBytes.GetPointer(),
		numIndexTerms:  numIndexTerms,
	}

	if numIndexTerms <= 0 {
		return nil, fmt.Errorf("blockterms: numIndexTerms=%d", numIndexTerms)
	}

	clone := in.Clone()
	defer clone.Close()
	if err := clone.SetPosition(indexStart); err != nil {
		return nil, err
	}

	// slurp in the images from disk:
	numTermBytes := packedIndexStart - indexStart
	if err := termBytes.Copy(clone, numTermBytes); err != nil {
		return nil, err
	}

	vClone := vIntIndexInput{IndexInput: clone}

	// records offsets into main terms dict file
	termsDictOffsets, err := packed.NewMonotonicBlockPackedReader(
		vClone, r.packedIntsVersion, r.blocksize, numIndexTerms)
	if err != nil {
		return nil, err
	}
	d.termsDictOffsets = termsDictOffsets

	// records offsets into byte[] term data
	termOffsets, err := packed.NewMonotonicBlockPackedReader(
		vClone, r.packedIntsVersion, r.blocksize, 1+numIndexTerms)
	if err != nil {
		return nil, err
	}
	d.termOffsets = termOffsets

	return d, nil
}

// String mirrors FieldIndexData.toString().
func (d *fixedGapFieldIndexData) String() string {
	return fmt.Sprintf("FixedGapTermIndex(indexterms=%d)", d.numIndexTerms)
}

// GetFieldEnum returns an enumerator over fieldInfo's indexed terms.
//
// Port of FixedGapTermsIndexReader.getFieldEnum(FieldInfo).
func (r *FixedGapTermsIndexReader) GetFieldEnum(fieldInfo *spi.FieldInfo) TermsIndexEnum {
	return &fixedGapIndexEnum{
		reader:     r,
		fieldIndex: r.fields[fieldInfo.Name()],
		term:       util.NewBytesRefEmpty(),
	}
}

// Close releases this reader. The .tii input is closed by the constructor, so
// there is nothing left to release.
//
// Port of FixedGapTermsIndexReader.close().
func (r *FixedGapTermsIndexReader) Close() error { return nil }

// String mirrors FixedGapTermsIndexReader.toString().
func (r *FixedGapTermsIndexReader) String() string {
	return fmt.Sprintf("FixedGapTermsIndexReader(fields=%d,interval=%d)", len(r.fields), r.indexInterval)
}
