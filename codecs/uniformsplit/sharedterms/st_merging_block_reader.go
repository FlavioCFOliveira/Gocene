// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// errSTMergingBlockReaderUnsupported renders the UnsupportedOperationException
// thrown by the seek methods and by readTermStateIfNotRead of
// STMergingBlockReader (STMergingBlockReader.java:56-90).
var errSTMergingBlockReaderUnsupported = errors.New("STMergingBlockReader: unsupported operation")

// STMergingBlockReader is the spi.TermsEnum used when merging segments, to
// enumerate the terms of an input segment and get all the fields TermStates of
// each term.
//
// It only supports calls to Next and no seek method.
//
// Mirrors
// org.apache.lucene.codecs.uniformsplit.sharedterms.STMergingBlockReader from
// Apache Lucene 10.5.0, which extends STBlockReader.
type STMergingBlockReader struct {
	*STBlockReader
}

// NewSTMergingBlockReader mirrors the public
// STMergingBlockReader(IndexDictionary.BrowserSupplier, IndexInput,
// PostingsReaderBase, FieldMetadata, BlockDecoder, FieldInfos) constructor
// (STMergingBlockReader.java:41).
func NewSTMergingBlockReader(
	dictionaryBrowserSupplier uniformsplit.IndexDictionaryBrowserSupplier,
	blockInput store.IndexInput,
	postingsReader codecs.PostingsReaderBase,
	fieldMetadata *uniformsplit.FieldMetadata,
	blockDecoder uniformsplit.BlockDecoder,
	fieldInfos *index.FieldInfos,
) (*STMergingBlockReader, error) {
	stBlockReader, err := NewSTBlockReader(
		dictionaryBrowserSupplier, blockInput, postingsReader, fieldMetadata, blockDecoder, fieldInfos)
	if err != nil {
		return nil, err
	}
	r := &STMergingBlockReader{STBlockReader: stBlockReader}
	// Java's `this` inside BlockReader's and STBlockReader's bodies is the
	// STMergingBlockReader being constructed; see
	// uniformsplit.BlockReaderOverrides.
	r.Overrides = r
	return r, nil
}

// SeekCeilBytes mirrors STMergingBlockReader.seekCeil(BytesRef)
// (STMergingBlockReader.java:56), whose body is `throw new
// UnsupportedOperationException()`.
func (r *STMergingBlockReader) SeekCeilBytes(_ *util.BytesRef) (spi.SeekStatus, error) {
	return spi.SeekStatusEnd, errSTMergingBlockReaderUnsupported
}

// SeekExactBytes mirrors STMergingBlockReader.seekExact(BytesRef)
// (STMergingBlockReader.java:61), whose body is `throw new
// UnsupportedOperationException()`.
func (r *STMergingBlockReader) SeekExactBytes(_ *util.BytesRef) (bool, error) {
	return false, errSTMergingBlockReaderUnsupported
}

// SeekExactWithState mirrors STMergingBlockReader.seekExact(BytesRef,
// TermState) (STMergingBlockReader.java:66), whose body is `throw new
// UnsupportedOperationException()`.
func (r *STMergingBlockReader) SeekExactWithState(_ *spi.Term, _ index.TermState) error {
	return errSTMergingBlockReaderUnsupported
}

// SeekExactOrd mirrors STMergingBlockReader.seekExact(long)
// (STMergingBlockReader.java:71), whose body is `throw new
// UnsupportedOperationException()`.
func (r *STMergingBlockReader) SeekExactOrd(_ int64) error {
	return errSTMergingBlockReaderUnsupported
}

// ReadTermStateIfNotRead mirrors
// STMergingBlockReader.readTermStateIfNotRead
// (STMergingBlockReader.java:76), whose body is `throw new
// UnsupportedOperationException()`.
func (r *STMergingBlockReader) ReadTermStateIfNotRead() (index.TermState, error) {
	return nil, errSTMergingBlockReaderUnsupported
}

// Next mirrors STMergingBlockReader.next (STMergingBlockReader.java:81), whose
// body is `return nextTerm()`.
//
// PORT NOTE: the Java method returns a bare BytesRef, while the Gocene
// spi.TermsEnum contract returns a *spi.Term, which carries a field name. This
// enumerator has no single field — that is the point of the shared-terms
// format — and its FieldMetadata is the union built by
// UnionFieldMetadataBuilder, whose FieldInfo is null (FieldMetadata.java:61).
// The returned Term therefore carries the empty field name; its bytes are the
// bytes Java returns.
func (r *STMergingBlockReader) Next() (*spi.Term, error) {
	termBytes, err := r.Overrides.NextTerm()
	if err != nil {
		return nil, err
	}
	if termBytes == nil {
		return nil, nil
	}
	return spi.NewTermFromBytesRef("", termBytes), nil
}

// PostingsForField creates a new spi.PostingsEnum for the provided field and
// BlockTermState.
//
// reuse is a previous spi.PostingsEnum to reuse, or nil to create a new one.
// flags holds the postings flags.
//
// Mirrors STMergingBlockReader.postings(String, BlockTermState, PostingsEnum,
// int) (STMergingBlockReader.java:92). Java overloads TermsEnum.postings; Go
// has no overloading, so this one carries the field in its name and the
// inherited Postings(int) keeps the spi.TermsEnum spelling.
func (r *STMergingBlockReader) PostingsForField(
	fieldName string,
	termState index.TermState,
	reuse index.PostingsEnum,
	flags int,
) (index.PostingsEnum, error) {
	return r.PostingsReader.Postings(r.FieldInfos.FieldInfo(fieldName), termState, reuse, flags)
}

// ReadFieldTermStatesMap reads all the fields TermStates of the current term
// and puts them in the provided map. Clears the map first, before putting
// TermStates.
//
// Mirrors STMergingBlockReader.readFieldTermStatesMap(Map<String,
// BlockTermState>) (STMergingBlockReader.java:102).
func (r *STMergingBlockReader) ReadFieldTermStatesMap(fieldTermStatesMap map[string]index.TermState) error {
	if r.TermBytes() != nil {
		r.TermStatesReadBuffer.SetPosition(
			r.BlockFirstLineStart +
				int(r.BlockHeader.TermStatesBaseOffset()) +
				int(r.BlockLine.GetTermStateRelativeOffset()))
		return r.stBlockLineReader.ReadFieldTermStatesMap(
			r.TermStatesReadBuffer,
			r.TermStateSerializer,
			r.BlockHeader,
			r.FieldInfos,
			fieldTermStatesMap)
	}
	return nil
}

var (
	_ spi.TermsEnum                     = (*STMergingBlockReader)(nil)
	_ uniformsplit.BlockReaderOverrides = (*STMergingBlockReader)(nil)
)
