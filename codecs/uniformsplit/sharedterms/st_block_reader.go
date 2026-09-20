// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// STBlockReader reads terms blocks with the Shared Terms format.
//
// See STBlockWriter.
//
// Mirrors org.apache.lucene.codecs.uniformsplit.sharedterms.STBlockReader from
// Apache Lucene 10.5.0, which extends
// org.apache.lucene.codecs.uniformsplit.BlockReader.
type STBlockReader struct {
	*uniformsplit.BlockReader

	// FieldInfos mirrors the protected final field STBlockReader.fieldInfos
	// (STBlockReader.java:38).
	FieldInfos *index.FieldInfos

	// stBlockLineReader is the STBlockLine.Serializer that
	// CreateBlockLineSerializer installed in BlockReader.BlockLineReader. It
	// renders the Java downcast `(STBlockLine.Serializer) blockLineReader`
	// (STBlockReader.java:139): Java stores one object in the protected
	// BlockLine.Serializer field and casts it back, Go embedding makes the
	// base a distinct object, so the subclass view is kept here. Both views
	// share the single BlockLineSerializer that holds the incremental
	// decoding state.
	stBlockLineReader *STBlockLineSerializer
}

// NewSTBlockReader mirrors the public STBlockReader(IndexDictionary.
// BrowserSupplier, IndexInput, PostingsReaderBase, FieldMetadata,
// BlockDecoder, FieldInfos) constructor (STBlockReader.java:40).
func NewSTBlockReader(
	dictionaryBrowserSupplier uniformsplit.IndexDictionaryBrowserSupplier,
	blockInput store.IndexInput,
	postingsReader codecs.PostingsReaderBase,
	fieldMetadata *uniformsplit.FieldMetadata,
	blockDecoder uniformsplit.BlockDecoder,
	fieldInfos *index.FieldInfos,
) (*STBlockReader, error) {
	blockReader, err := uniformsplit.NewBlockReader(
		dictionaryBrowserSupplier, blockInput, postingsReader, fieldMetadata, blockDecoder)
	if err != nil {
		return nil, err
	}
	r := &STBlockReader{BlockReader: blockReader, FieldInfos: fieldInfos}
	// Java's `this` inside BlockReader's bodies is the STBlockReader being
	// constructed; see uniformsplit.BlockReaderOverrides.
	r.Overrides = r
	return r, nil
}

// Next advances to the next term that occurs for the searched field.
//
// Mirrors STBlockReader.next (STBlockReader.java:52). Java's
// org.apache.lucene.index.TermsEnum returns a bare BytesRef; Gocene's
// spi.TermsEnum returns a *spi.Term, as BlockReader.Next already does.
func (r *STBlockReader) Next() (*spi.Term, error) {
	for {
		next, err := r.BlockReader.Next()
		if err != nil {
			return nil, err
		}
		if next == nil {
			// No more terms.
			return nil, nil
		}
		// Check if the term occurs for the searched field.
		occurs, err := r.termOccursInField()
		if err != nil {
			return nil, err
		}
		if occurs {
			// The term occurs for the searched field.
			return next, nil
		}
	}
}

// termOccursInField mirrors the private STBlockReader.termOccursInField
// (STBlockReader.java:66).
func (r *STBlockReader) termOccursInField() (bool, error) {
	if _, err := r.Overrides.ReadTermStateIfNotRead(); err != nil {
		return false, err
	}
	return r.CurrentTermState != nil, nil
}

// NextTerm moves to the next term line and reads it, whichever are the
// corresponding fields. The term details are not read yet. They will be read
// only when needed with ReadTermStateIfNotRead.
//
// Returns the read term bytes.
//
// Mirrors STBlockReader.nextTerm (STBlockReader.java:78).
func (r *STBlockReader) NextTerm() (*util.BytesRef, error) {
	nextTerm, err := r.BlockReader.NextTerm()
	if err != nil {
		return nil, err
	}
	// Java calls super.isBeyondLastTerm here, not the override below.
	if nextTerm != nil && r.BlockReader.IsBeyondLastTerm(nextTerm, r.BlockStartFP) {
		return nil, nil
	}
	return nextTerm, nil
}

// SeekCeilBytes mirrors STBlockReader.seekCeil(BytesRef)
// (STBlockReader.java:87).
func (r *STBlockReader) SeekCeilBytes(searchedTerm *util.BytesRef) (spi.SeekStatus, error) {
	seekStatus, err := r.seekCeilIgnoreField(searchedTerm)
	if err != nil {
		return spi.SeekStatusEnd, err
	}
	if seekStatus != spi.SeekStatusEnd {
		occurs, err := r.termOccursInField()
		if err != nil {
			return spi.SeekStatusEnd, err
		}
		if !occurs {
			// The term does not occur for the field.
			// We have to move the iterator to the next valid term for the
			// field.
			nextTerm, err := r.Next()
			if err != nil {
				return spi.SeekStatusEnd, err
			}
			if nextTerm == nil {
				seekStatus = spi.SeekStatusEnd
			} else {
				seekStatus = spi.SeekStatusNotFound
			}
		}
	}
	return seekStatus, nil
}

// seekCeilIgnoreField mirrors the package-private
// STBlockReader.seekCeilIgnoreField (STBlockReader.java:100), which Lucene
// marks "Visible for testing".
func (r *STBlockReader) seekCeilIgnoreField(searchedTerm *util.BytesRef) (spi.SeekStatus, error) {
	return r.BlockReader.SeekCeilBytes(searchedTerm)
}

// SeekExactBytes mirrors STBlockReader.seekExact(BytesRef)
// (STBlockReader.java:105).
func (r *STBlockReader) SeekExactBytes(searchedTerm *util.BytesRef) (bool, error) {
	found, err := r.BlockReader.SeekExactBytes(searchedTerm)
	if err != nil {
		return false, err
	}
	if found {
		return r.termOccursInField()
	}
	return false, nil
}

// IsBeyondLastTerm mirrors STBlockReader.isBeyondLastTerm(BytesRef, long)
// (STBlockReader.java:113).
func (r *STBlockReader) IsBeyondLastTerm(searchedTerm *util.BytesRef, blockStartFP int64) bool {
	return blockStartFP > r.FieldMetadata.GetLastBlockStartFP() ||
		r.BlockReader.IsBeyondLastTerm(searchedTerm, blockStartFP)
}

// CreateBlockLineSerializer mirrors
// STBlockReader.createBlockLineSerializer (STBlockReader.java:119), whose body
// is `return new STBlockLine.Serializer()`. The Go return type is the base
// uniformsplit.BlockLineSerializer that BlockReader.BlockLineReader holds; the
// subclass view is kept in stBlockLineReader for the Java downcast.
func (r *STBlockReader) CreateBlockLineSerializer() *uniformsplit.BlockLineSerializer {
	r.stBlockLineReader = NewSTBlockLineSerializer()
	return r.stBlockLineReader.BlockLineSerializer
}

// ReadTermState reads the BlockTermState on the current line for this reader's
// field.
//
// Returns the BlockTermState; or nil if the term does not occur for the field.
//
// Mirrors STBlockReader.readTermState (STBlockReader.java:129).
func (r *STBlockReader) ReadTermState() (index.TermState, error) {
	r.TermStatesReadBuffer.SetPosition(
		r.BlockFirstLineStart +
			int(r.BlockHeader.TermStatesBaseOffset()) +
			int(r.BlockLine.GetTermStateRelativeOffset()))
	termState, err := r.stBlockLineReader.ReadTermStateForField(
		r.FieldMetadata.GetFieldInfo().Number(),
		r.TermStatesReadBuffer,
		r.TermStateSerializer,
		r.BlockHeader,
		r.FieldInfos,
		r.ScratchTermState)
	if err != nil {
		return nil, err
	}
	r.CurrentTermState = termState
	return termState, nil
}

var (
	_ spi.TermsEnum                     = (*STBlockReader)(nil)
	_ uniformsplit.BlockReaderOverrides = (*STBlockReader)(nil)
)
