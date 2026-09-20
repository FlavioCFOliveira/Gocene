// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// errUniformSplitTermsIntersectAutomatonType renders the
// IllegalArgumentException thrown by
// UniformSplitTerms.checkIntersectAutomatonType
// (UniformSplitTerms.java:90).
var errUniformSplitTermsIntersectAutomatonType = errors.New("please use CompiledAutomaton.getTermsEnum instead")

// UniformSplitTermsOverrides is the set of UniformSplitTerms methods that
// Apache Lucene 10.5.0 subclasses override and that Lucene then invokes
// virtually on `this`: iterator(), which
// org.apache.lucene.index.Terms#getMin() calls in its default body
// `iterator().next()` (Terms.java:143) and which
// STUniformSplitTerms overrides (STUniformSplitTerms.java:66).
//
// Java resolves that call virtually. Go embedding does not, so
// UniformSplitTerms keeps a back-pointer to the most-derived instance in
// [UniformSplitTerms.Overrides] and makes the call through it. See
// [BlockReaderOverrides] for the same mechanism on the block reader.
type UniformSplitTermsOverrides interface {
	// Iterator mirrors UniformSplitTerms.iterator
	// (UniformSplitTerms.java:69).
	Iterator() (spi.TermsEnum, error)
}

// UniformSplitTermsBase is the Go rendering of the Java type
// org.apache.lucene.codecs.uniformsplit.UniformSplitTerms wherever Apache
// Lucene 10.5.0 uses it as a supertype rather than as a concrete class. The
// one such use is the value type of UniformSplitTermsReader.fieldToTermsMap,
// declared `Map<String, UniformSplitTerms>`
// (UniformSplitTermsReader.java:62) and filled with UniformSplitTerms by
// UniformSplitTermsReader.fillFieldMap (UniformSplitTermsReader.java:177) and
// with its subclass STUniformSplitTerms by
// STUniformSplitTermsReader.fillFieldMap (STUniformSplitTermsReader.java:100).
//
// Go structs do not subtype, so the Java supertype is expressed as this
// interface. UniformSplitTerms declares no public member beyond those it
// inherits from org.apache.lucene.index.Terms, so the interface is exactly the
// Gocene spi.Terms contract, named for the Lucene class it stands for.
type UniformSplitTermsBase interface {
	spi.Terms
}

// UniformSplitTerms is the Terms based on the Uniform Split technique.
//
// The IndexDictionary index dictionary is lazy loaded only when
// TermsEnum.SeekCeil or TermsEnum.SeekExact are called (it is not loaded for a
// direct terms enumeration).
//
// See UniformSplitTermsWriter.
//
// Mirrors org.apache.lucene.codecs.uniformsplit.UniformSplitTerms from Apache
// Lucene 10.5.0, which extends org.apache.lucene.index.Terms.
type UniformSplitTerms struct {
	// Overrides is the back-pointer to the most-derived instance, through
	// which UniformSplitTerms makes the calls Java resolves virtually. See
	// [UniformSplitTermsOverrides]. NewUniformSplitTerms sets it to the
	// UniformSplitTerms itself; a subclass constructor overwrites it with
	// the subclass.
	Overrides UniformSplitTermsOverrides

	BlockInput                store.IndexInput
	FieldMetadata             *FieldMetadata
	PostingsReader            codecs.PostingsReaderBase
	BlockDecoder              BlockDecoder
	DictionaryBrowserSupplier IndexDictionaryBrowserSupplier
}

// NewUniformSplitTerms mirrors the protected UniformSplitTerms constructor
// (UniformSplitTerms.java:51).
//
// blockDecoder is an optional block decoder, may be nil if none. It can be used
// for decompression or decryption.
func NewUniformSplitTerms(
	blockInput store.IndexInput,
	fieldMetadata *FieldMetadata,
	postingsReader codecs.PostingsReaderBase,
	blockDecoder BlockDecoder,
	dictionaryBrowserSupplier IndexDictionaryBrowserSupplier,
) *UniformSplitTerms {
	// assert fieldMetadata != null;
	// assert fieldMetadata.getFieldInfo() != null;
	// assert fieldMetadata.getLastTerm() != null;
	// assert dictionaryBrowserSupplier != null;
	t := &UniformSplitTerms{
		BlockInput:                blockInput,
		FieldMetadata:             fieldMetadata,
		PostingsReader:            postingsReader,
		BlockDecoder:              blockDecoder,
		DictionaryBrowserSupplier: dictionaryBrowserSupplier,
	}
	t.Overrides = t
	return t
}

// Iterator mirrors UniformSplitTerms.iterator (UniformSplitTerms.java:68).
func (t *UniformSplitTerms) Iterator() (spi.TermsEnum, error) {
	return NewBlockReader(t.DictionaryBrowserSupplier, t.BlockInput, t.PostingsReader, t.FieldMetadata, t.BlockDecoder)
}

// Intersect mirrors UniformSplitTerms.intersect(CompiledAutomaton, BytesRef)
// (UniformSplitTerms.java:74). Java's start term is a bare BytesRef; Gocene's
// spi.Terms carries it as a *spi.Term (field + bytes), so the bytes are
// unwrapped before they reach IntersectBlockReader.
func (t *UniformSplitTerms) Intersect(compiled *automaton.CompiledAutomaton, startTerm *spi.Term) (spi.TermsEnum, error) {
	if err := t.CheckIntersectAutomatonType(compiled); err != nil {
		return nil, err
	}
	var start *util.BytesRef
	if startTerm != nil {
		start = startTerm.BytesValue()
	}
	return NewIntersectBlockReader(
		compiled,
		start,
		t.DictionaryBrowserSupplier,
		t.BlockInput,
		t.PostingsReader,
		t.FieldMetadata,
		t.BlockDecoder)
}

// CheckIntersectAutomatonType mirrors
// UniformSplitTerms.checkIntersectAutomatonType
// (UniformSplitTerms.java:87). Java throws IllegalArgumentException, which
// Gocene reports as an error.
func (t *UniformSplitTerms) CheckIntersectAutomatonType(compiled *automaton.CompiledAutomaton) error {
	// This check is consistent with other impls and precondition stated in javadoc.
	if compiled.Type != automaton.AutomatonTypeNormal {
		return errUniformSplitTermsIntersectAutomatonType
	}
	return nil
}

// GetMax mirrors UniformSplitTerms.getMax (UniformSplitTerms.java:94).
func (t *UniformSplitTerms) GetMax() (*spi.Term, error) {
	lastTerm := t.FieldMetadata.GetLastTerm()
	if lastTerm == nil {
		return nil, nil
	}
	return spi.NewTermFromBytesRef(t.Field(), lastTerm), nil
}

// Size mirrors UniformSplitTerms.size (UniformSplitTerms.java:99).
func (t *UniformSplitTerms) Size() int64 {
	return t.FieldMetadata.GetNumTerms()
}

// GetSumTotalTermFreq mirrors UniformSplitTerms.getSumTotalTermFreq
// (UniformSplitTerms.java:104). Java declares no checked exception on it;
// Gocene's spi.Terms carries the error return on every statistic.
func (t *UniformSplitTerms) GetSumTotalTermFreq() (int64, error) {
	return t.FieldMetadata.GetSumTotalTermFreq(), nil
}

// GetSumDocFreq mirrors UniformSplitTerms.getSumDocFreq
// (UniformSplitTerms.java:109).
func (t *UniformSplitTerms) GetSumDocFreq() (int64, error) {
	return t.FieldMetadata.GetSumDocFreq(), nil
}

// GetDocCount mirrors UniformSplitTerms.getDocCount
// (UniformSplitTerms.java:114).
func (t *UniformSplitTerms) GetDocCount() (int, error) {
	return int(t.FieldMetadata.GetDocCount()), nil
}

// HasFreqs mirrors UniformSplitTerms.hasFreqs (UniformSplitTerms.java:119).
func (t *UniformSplitTerms) HasFreqs() bool {
	return t.FieldMetadata.GetFieldInfo().IndexOptions().Subsumes(index.IndexOptionsDocsAndFreqs)
}

// HasOffsets mirrors UniformSplitTerms.hasOffsets
// (UniformSplitTerms.java:124).
func (t *UniformSplitTerms) HasOffsets() bool {
	return t.FieldMetadata.GetFieldInfo().IndexOptions().
		Subsumes(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
}

// HasPositions mirrors UniformSplitTerms.hasPositions
// (UniformSplitTerms.java:132).
func (t *UniformSplitTerms) HasPositions() bool {
	return t.FieldMetadata.GetFieldInfo().IndexOptions().
		Subsumes(index.IndexOptionsDocsAndFreqsAndPositions)
}

// HasPayloads mirrors UniformSplitTerms.hasPayloads
// (UniformSplitTerms.java:140).
func (t *UniformSplitTerms) HasPayloads() bool {
	return t.FieldMetadata.GetFieldInfo().HasPayloads()
}

// Field returns the name of the field this Terms instance represents.
//
// org.apache.lucene.index.Terms declares no field() accessor, so
// UniformSplitTerms overrides nothing here; Gocene's spi.Terms contract does
// declare one, and it is answered from the FieldMetadata's FieldInfo
// (UniformSplitTerms.java:42).
func (t *UniformSplitTerms) Field() string {
	return t.FieldMetadata.GetFieldInfo().Name()
}

// GetMin is the default org.apache.lucene.index.Terms#getMin()
// (Terms.java:143) that UniformSplitTerms inherits: `return iterator().next()`.
func (t *UniformSplitTerms) GetMin() (*spi.Term, error) {
	te, err := t.Overrides.Iterator()
	if err != nil {
		return nil, err
	}
	return te.Next()
}

// GetIteratorWithSeek returns an iterator positioned at or after seekTerm.
//
// org.apache.lucene.index.Terms declares no such member, so UniformSplitTerms
// overrides nothing here; the Gocene spi.Terms contract does declare it, and it
// is answered with the two Lucene operations a Java caller would spell out —
// Terms.iterator() followed by TermsEnum.seekCeil(BytesRef).
func (t *UniformSplitTerms) GetIteratorWithSeek(seekTerm *spi.Term) (spi.TermsEnum, error) {
	te, err := t.Overrides.Iterator()
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
// absent. Like GetIteratorWithSeek this is a Gocene-only spi.Terms member; it
// is answered with iterator() -> seekExact(BytesRef) -> postings(int).
func (t *UniformSplitTerms) GetPostingsReader(termText string, flags int) (spi.PostingsEnum, error) {
	te, err := t.Overrides.Iterator()
	if err != nil {
		return nil, err
	}
	found, err := te.SeekExact(spi.NewTerm(t.Field(), termText))
	if err != nil || !found {
		return nil, err
	}
	return te.Postings(flags)
}

var (
	_ spi.Terms                  = (*UniformSplitTerms)(nil)
	_ UniformSplitTermsBase      = (*UniformSplitTerms)(nil)
	_ UniformSplitTermsOverrides = (*UniformSplitTerms)(nil)
)
