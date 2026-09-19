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
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// STIntersectBlockReader is the "intersect" spi.TermsEnum response to
// STUniformSplitTerms.Intersect, intersecting the terms with an automaton.
//
// Mirrors
// org.apache.lucene.codecs.uniformsplit.sharedterms.STIntersectBlockReader
// from Apache Lucene 10.5.0, which extends
// org.apache.lucene.codecs.uniformsplit.IntersectBlockReader.
type STIntersectBlockReader struct {
	*uniformsplit.IntersectBlockReader

	// FieldInfos mirrors the protected final field
	// STIntersectBlockReader.fieldInfos (STIntersectBlockReader.java:40).
	FieldInfos *index.FieldInfos

	// stBlockLineReader is the STBlockLine.Serializer that
	// CreateBlockLineSerializer installed in BlockReader.BlockLineReader. It
	// renders the Java downcast `(STBlockLine.Serializer) blockLineReader`
	// (STIntersectBlockReader.java:106); see STBlockReader.stBlockLineReader.
	stBlockLineReader *STBlockLineSerializer
}

// NewSTIntersectBlockReader mirrors the public
// STIntersectBlockReader(CompiledAutomaton, BytesRef,
// IndexDictionary.BrowserSupplier, IndexInput, PostingsReaderBase,
// FieldMetadata, BlockDecoder, FieldInfos) constructor
// (STIntersectBlockReader.java:42).
func NewSTIntersectBlockReader(
	compiled *automaton.CompiledAutomaton,
	startTerm *util.BytesRef,
	dictionaryBrowserSupplier uniformsplit.IndexDictionaryBrowserSupplier,
	blockInput store.IndexInput,
	postingsReader codecs.PostingsReaderBase,
	fieldMetadata *uniformsplit.FieldMetadata,
	blockDecoder uniformsplit.BlockDecoder,
	fieldInfos *index.FieldInfos,
) (*STIntersectBlockReader, error) {
	intersectBlockReader, err := uniformsplit.NewIntersectBlockReader(
		compiled, startTerm, dictionaryBrowserSupplier, blockInput, postingsReader, fieldMetadata, blockDecoder)
	if err != nil {
		return nil, err
	}
	r := &STIntersectBlockReader{IntersectBlockReader: intersectBlockReader, FieldInfos: fieldInfos}
	// Java's `this` inside BlockReader's bodies is the
	// STIntersectBlockReader being constructed; see
	// uniformsplit.BlockReaderOverrides.
	r.Overrides = r
	return r, nil
}

// ---------------------------------------------
// The methods below are duplicate from STBlockReader.
//
// This class inherits code from both IntersectBlockReader and STBlockReader.
// We choose to extend IntersectBlockReader because this is the one that
// runs the next(), reads the block lines and keeps the reader state.
// But we still need the STBlockReader logic to skip terms that do not occur
// in this TermsEnum field.
// So we end up having a couple of methods directly duplicate from STBlockReader.
// We tried various different approaches to avoid duplicating the code, but
// actually this becomes difficult to read and to understand. This is simpler
// to duplicate and explain it here.
// ---------------------------------------------

// Next advances to the next term that occurs for the searched field.
//
// Mirrors STIntersectBlockReader.next (STIntersectBlockReader.java:76).
func (r *STIntersectBlockReader) Next() (*spi.Term, error) {
	for {
		next, err := r.IntersectBlockReader.Next()
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

// termOccursInField mirrors the private
// STIntersectBlockReader.termOccursInField (STIntersectBlockReader.java:90).
func (r *STIntersectBlockReader) termOccursInField() (bool, error) {
	if _, err := r.Overrides.ReadTermStateIfNotRead(); err != nil {
		return false, err
	}
	return r.CurrentTermState != nil, nil
}

// CreateBlockLineSerializer mirrors
// STIntersectBlockReader.createBlockLineSerializer
// (STIntersectBlockReader.java:96), whose body is `return new
// STBlockLine.Serializer()`.
func (r *STIntersectBlockReader) CreateBlockLineSerializer() *uniformsplit.BlockLineSerializer {
	r.stBlockLineReader = NewSTBlockLineSerializer()
	return r.stBlockLineReader.BlockLineSerializer
}

// ReadTermState reads the BlockTermState on the current line for the specific
// field corresponding to this reader. Returns nil if the term does not occur
// for the field.
//
// Mirrors STIntersectBlockReader.readTermState
// (STIntersectBlockReader.java:105). Unlike STBlockReader.ReadTermState, Java
// does not assign the termState field here.
func (r *STIntersectBlockReader) ReadTermState() (index.TermState, error) {
	r.TermStatesReadBuffer.SetPosition(
		r.BlockFirstLineStart +
			int(r.BlockHeader.TermStatesBaseOffset()) +
			int(r.BlockLine.GetTermStateRelativeOffset()))
	return r.stBlockLineReader.ReadTermStateForField(
		r.FieldMetadata.GetFieldInfo().Number(),
		r.TermStatesReadBuffer,
		r.TermStateSerializer,
		r.BlockHeader,
		r.FieldInfos,
		r.ScratchTermState)
}

var (
	_ spi.TermsEnum                     = (*STIntersectBlockReader)(nil)
	_ uniformsplit.BlockReaderOverrides = (*STIntersectBlockReader)(nil)
)
