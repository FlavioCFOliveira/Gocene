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

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// Format constants, ported from
// org.apache.lucene.codecs.blockterms.VariableGapTermsIndexWriter
// (Apache Lucene 10.5.0).
const (
	// VariableGapTermsIndexExtension mirrors
	// VariableGapTermsIndexWriter.TERMS_INDEX_EXTENSION.
	VariableGapTermsIndexExtension = "tiv"
	// VariableGapTermsMetaExtension mirrors
	// VariableGapTermsIndexWriter.TERMS_META_EXTENSION.
	VariableGapTermsMetaExtension = "tmv"

	// VariableGapMetaCodecName mirrors
	// VariableGapTermsIndexWriter.META_CODEC_NAME.
	VariableGapMetaCodecName = "VariableGapTermsMeta"
	// VariableGapCodecName mirrors VariableGapTermsIndexWriter.CODEC_NAME.
	VariableGapCodecName = "VariableGapTermsIndex"
	// VariableGapVersionStart mirrors
	// VariableGapTermsIndexWriter.VERSION_START.
	VariableGapVersionStart = int32(4)
	// VariableGapVersionCurrent mirrors
	// VariableGapTermsIndexWriter.VERSION_CURRENT.
	VariableGapVersionCurrent = VariableGapVersionStart
)

// ErrUnsupportedOperation reports a call to an operation the receiver does not
// implement. It is the Go counterpart of Java's UnsupportedOperationException,
// raised by VariableGapTermsIndexReader's ordinal operations because this
// reader does not support ords.
var ErrUnsupportedOperation = errors.New("blockterms: unsupported operation")

// VariableGapTermsIndexReader reads the FST-backed terms index written by
// VariableGapTermsIndexWriter.
//
// Port of org.apache.lucene.codecs.blockterms.VariableGapTermsIndexReader
// (Apache Lucene 10.5.0).
type VariableGapTermsIndexReader struct {
	fields map[string]*fst.FST[int64]
}

// compile-time check that VariableGapTermsIndexReader implements
// TermsIndexReader.
var _ TermsIndexReader = (*VariableGapTermsIndexReader)(nil)

// NewVariableGapTermsIndexReader opens the .tmv meta file and the .tiv index
// file of the segment described by state and loads one FST per field.
//
// Port of the VariableGapTermsIndexReader(SegmentReadState) constructor.
func NewVariableGapTermsIndexReader(state *spi.SegmentReadState) (*VariableGapTermsIndexReader, error) {
	fstOutputs := fst.PositiveIntOutputs()

	metaFileName := codecs.GetSegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, VariableGapTermsMetaExtension)
	indexFileName := codecs.GetSegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, VariableGapTermsIndexExtension)

	// Java uses Directory.openChecksumInput; Gocene's Directory has no such
	// method, so the raw input is opened and wrapped, which is what the rest
	// of the port does.
	metaRaw, err := state.Directory.OpenInput(metaFileName, store.IOContextRead)
	if err != nil {
		return nil, err
	}
	defer metaRaw.Close()
	metaIn := store.NewChecksumIndexInput(metaRaw)

	indexRaw, err := state.Directory.OpenInput(indexFileName, store.IOContextRead)
	if err != nil {
		return nil, err
	}
	defer indexRaw.Close()
	indexIn := store.NewChecksumIndexInput(indexRaw)

	r := &VariableGapTermsIndexReader{
		fields: make(map[string]*fst.FST[int64]),
	}

	priorErr := r.readIndex(metaIn, indexIn, fstOutputs, state)

	// Java runs checkFooter on both inputs in a finally block, passing the
	// prior exception so that a checksum failure is reported as its
	// suppressed cause. Gocene's CheckFooter takes no prior error, so the
	// prior error is kept and returned in preference to a footer error.
	_, metaFooterErr := store.CheckFooter(metaIn)
	_, indexFooterErr := store.CheckFooter(indexIn)

	if priorErr != nil {
		return nil, priorErr
	}
	if metaFooterErr != nil {
		return nil, metaFooterErr
	}
	if indexFooterErr != nil {
		return nil, indexFooterErr
	}

	return r, nil
}

// readIndex verifies both headers and reads the per-field directory, loading
// one FST per field. It is the body of the Java constructor's inner try block.
func (r *VariableGapTermsIndexReader) readIndex(
	metaIn, indexIn *store.ChecksumIndexInput,
	fstOutputs fst.Outputs[int64],
	state *spi.SegmentReadState,
) error {

	if _, err := codecs.CheckIndexHeader(
		metaIn,
		VariableGapMetaCodecName,
		VariableGapVersionStart,
		VariableGapVersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix); err != nil {
		return err
	}

	if _, err := codecs.CheckIndexHeader(
		indexIn,
		VariableGapCodecName,
		VariableGapVersionStart,
		VariableGapVersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix); err != nil {
		return err
	}

	// Read directory
	for {
		field, err := metaIn.ReadInt()
		if err != nil {
			return err
		}
		if field == -1 {
			break
		}

		indexStart, err := store.ReadVLong(metaIn)
		if err != nil {
			return err
		}
		fieldInfo := state.FieldInfos.FieldInfoByNumber(int(field))
		if fieldInfo == nil {
			return fmt.Errorf("blockterms: invalid field number: %d", field)
		}
		if fp := indexIn.GetFilePointer(); fp != indexStart {
			return fmt.Errorf(
				"blockterms: gap in FST, expected position %d, got %d", fp, indexStart)
		}

		metadata, err := fst.ReadMetadata[int64](metaIn, fstOutputs)
		if err != nil {
			return err
		}
		f, err := fst.NewFSTFromDataInput(metadata, indexIn)
		if err != nil {
			return err
		}
		if _, dup := r.fields[fieldInfo.Name()]; dup {
			return fmt.Errorf("blockterms: duplicate field: %s", fieldInfo.Name())
		}
		r.fields[fieldInfo.Name()] = f
	}

	return nil
}

// variableGapIndexEnum is the Go port of
// VariableGapTermsIndexReader.IndexEnum.
type variableGapIndexEnum struct {
	fstEnum *fst.BytesRefFSTEnum[int64]
	current *fst.BytesRefInputOutput[int64]
}

// compile-time check that variableGapIndexEnum implements TermsIndexEnum.
var _ TermsIndexEnum = (*variableGapIndexEnum)(nil)

// Term returns the current indexed term, or nil before the first positioning
// call and after the enumeration is exhausted.
//
// Port of IndexEnum.term().
func (e *variableGapIndexEnum) Term() *util.BytesRef {
	if e.current == nil {
		return nil
	}
	return e.current.Input
}

// Seek positions on the largest indexed term that is <= target and returns
// its file pointer into the main terms dictionary.
//
// Port of IndexEnum.seek(BytesRef).
func (e *variableGapIndexEnum) Seek(target *util.BytesRef) (int64, error) {
	current, err := e.fstEnum.SeekFloor(target)
	if err != nil {
		return 0, err
	}
	e.current = current
	if current == nil {
		// Java dereferences current.output here and would throw a
		// NullPointerException; Gocene reports the same condition as an error
		// rather than panicking.
		return 0, fmt.Errorf("blockterms: no indexed term <= target")
	}
	return current.Output, nil
}

// Next advances to the next indexed term, returning -1 at end.
//
// Port of IndexEnum.next().
func (e *variableGapIndexEnum) Next() (int64, error) {
	current, err := e.fstEnum.Next()
	if err != nil {
		return 0, err
	}
	e.current = current
	if current == nil {
		return -1, nil
	}
	return current.Output, nil
}

// Ord is not supported by this reader.
//
// Port of IndexEnum.ord(), which throws UnsupportedOperationException. Ord
// carries no error in the TermsIndexEnum contract (Java's ord() declares no
// checked exception), so the unsupported call panics, mirroring the unchecked
// Java exception.
func (e *variableGapIndexEnum) Ord() int64 {
	panic(ErrUnsupportedOperation)
}

// SeekOrd is not supported by this reader.
//
// Port of IndexEnum.seek(long), which throws UnsupportedOperationException.
func (e *variableGapIndexEnum) SeekOrd(ord int64) (int64, error) {
	return 0, fmt.Errorf("%w: VariableGapTermsIndexReader does not support ords", ErrUnsupportedOperation)
}

// SupportsOrd reports that this reader does not implement ordinal operations.
//
// Port of VariableGapTermsIndexReader.supportsOrd().
func (r *VariableGapTermsIndexReader) SupportsOrd() bool {
	return false
}

// GetFieldEnum returns an enumerator over fieldInfo's indexed terms, or nil
// when the field has no terms index.
//
// Port of VariableGapTermsIndexReader.getFieldEnum(FieldInfo).
func (r *VariableGapTermsIndexReader) GetFieldEnum(fieldInfo *schema.FieldInfo) TermsIndexEnum {
	fieldData, ok := r.fields[fieldInfo.Name()]
	if !ok {
		return nil
	}
	fstEnum, err := fst.NewBytesRefFSTEnum[int64](fieldData)
	if err != nil {
		// Java's BytesRefFSTEnum constructor cannot fail; Gocene's validates
		// the FST input type. A field index built by
		// VariableGapTermsIndexWriter is always BYTE1, so a failure here means
		// the loaded FST is not the one this format writes.
		return nil
	}
	return &variableGapIndexEnum{fstEnum: fstEnum}
}

// Close releases this reader. Both inputs are closed by the constructor, so
// there is nothing left to release.
//
// Port of VariableGapTermsIndexReader.close().
func (r *VariableGapTermsIndexReader) Close() error { return nil }

// String mirrors VariableGapTermsIndexReader.toString().
func (r *VariableGapTermsIndexReader) String() string {
	return fmt.Sprintf("VariableGapTermsIndexReader(fields=%d)", len(r.fields))
}
