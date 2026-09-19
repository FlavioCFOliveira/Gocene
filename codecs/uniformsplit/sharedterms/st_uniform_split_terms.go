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

// STUniformSplitTerms extends uniformsplit.UniformSplitTerms for a
// shared-terms dictionary, with all the fields of a term in the same block
// line.
//
// Mirrors
// org.apache.lucene.codecs.uniformsplit.sharedterms.STUniformSplitTerms from
// Apache Lucene 10.5.0, which extends
// org.apache.lucene.codecs.uniformsplit.UniformSplitTerms.
type STUniformSplitTerms struct {
	*uniformsplit.UniformSplitTerms

	// UnionFieldMetadata mirrors the protected final field
	// STUniformSplitTerms.unionFieldMetadata (STUniformSplitTerms.java:38).
	UnionFieldMetadata *uniformsplit.FieldMetadata

	// FieldInfos mirrors the protected final field
	// STUniformSplitTerms.fieldInfos (STUniformSplitTerms.java:39).
	FieldInfos *index.FieldInfos
}

// NewSTUniformSplitTerms mirrors the protected STUniformSplitTerms(IndexInput,
// FieldMetadata, FieldMetadata, PostingsReaderBase, BlockDecoder, FieldInfos,
// IndexDictionary.BrowserSupplier) constructor (STUniformSplitTerms.java:41).
func NewSTUniformSplitTerms(
	blockInput store.IndexInput,
	fieldMetadata *uniformsplit.FieldMetadata,
	unionFieldMetadata *uniformsplit.FieldMetadata,
	postingsReader codecs.PostingsReaderBase,
	blockDecoder uniformsplit.BlockDecoder,
	fieldInfos *index.FieldInfos,
	dictionaryBrowserSupplier uniformsplit.IndexDictionaryBrowserSupplier,
) *STUniformSplitTerms {
	t := &STUniformSplitTerms{
		UniformSplitTerms: uniformsplit.NewUniformSplitTerms(
			blockInput, fieldMetadata, postingsReader, blockDecoder, dictionaryBrowserSupplier),
		UnionFieldMetadata: unionFieldMetadata,
		FieldInfos:         fieldInfos,
	}
	// Java's `this` inside UniformSplitTerms' bodies is the
	// STUniformSplitTerms being constructed; see
	// uniformsplit.UniformSplitTermsOverrides.
	t.Overrides = t
	return t
}

// Intersect mirrors STUniformSplitTerms.intersect(CompiledAutomaton, BytesRef)
// (STUniformSplitTerms.java:56). Java's start term is a bare BytesRef;
// Gocene's spi.Terms carries it as a *spi.Term, so the bytes are unwrapped
// before they reach STIntersectBlockReader, exactly as
// uniformsplit.UniformSplitTerms.Intersect does.
func (t *STUniformSplitTerms) Intersect(
	compiled *automaton.CompiledAutomaton,
	startTerm *spi.Term,
) (spi.TermsEnum, error) {
	if err := t.CheckIntersectAutomatonType(compiled); err != nil {
		return nil, err
	}
	var start *util.BytesRef
	if startTerm != nil {
		start = startTerm.BytesValue()
	}
	return NewSTIntersectBlockReader(
		compiled,
		start,
		t.DictionaryBrowserSupplier,
		t.BlockInput,
		t.PostingsReader,
		t.FieldMetadata,
		t.BlockDecoder,
		t.FieldInfos)
}

// Iterator mirrors STUniformSplitTerms.iterator (STUniformSplitTerms.java:70).
func (t *STUniformSplitTerms) Iterator() (spi.TermsEnum, error) {
	return NewSTBlockReader(
		t.DictionaryBrowserSupplier,
		t.BlockInput,
		t.PostingsReader,
		t.FieldMetadata,
		t.BlockDecoder,
		t.FieldInfos)
}

// createMergingBlockReader mirrors the package-private
// STUniformSplitTerms.createMergingBlockReader (STUniformSplitTerms.java:80).
func (t *STUniformSplitTerms) createMergingBlockReader() (*STMergingBlockReader, error) {
	return NewSTMergingBlockReader(
		t.DictionaryBrowserSupplier,
		t.BlockInput,
		t.PostingsReader,
		t.UnionFieldMetadata,
		t.BlockDecoder,
		t.FieldInfos)
}

var (
	_ spi.Terms                               = (*STUniformSplitTerms)(nil)
	_ uniformsplit.UniformSplitTermsBase      = (*STUniformSplitTerms)(nil)
	_ uniformsplit.UniformSplitTermsOverrides = (*STUniformSplitTerms)(nil)
)
