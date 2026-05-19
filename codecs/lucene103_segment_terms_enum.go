// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Lucene103SegmentTermsEnum is the Go port of
// org.apache.lucene.codecs.lucene103.blocktree.SegmentTermsEnum — the
// strict block-tree TermsEnum used by Lucene103FieldReader.iterator() and
// FieldReader.intersect() (the latter via [Lucene103IntersectTermsEnum]).
//
// The enumerator walks the on-disk .tim file using a stack of
// SegmentTermsEnumFrames; each Next() pull either advances within the
// current frame's suffix run or pops/pushes frames following the FST
// transitions stored in the .tip block index. The full Java traversal
// includes block prefetching, MMap-aware slicing, and Automaton-driven
// pruning when wrapped by IntersectTermsEnum.
//
// This implementation is a typed stub: it satisfies the [index.TermsEnum]
// contract, owns the FieldReader pointer and an empty cursor frame, and
// returns end-of-enumeration on every advance. The behavioural port (FST
// traversal + suffix decode + Postings/Impacts wiring with
// [Lucene103PostingsReader]) is the deferred deep-port follow-up.
type Lucene103SegmentTermsEnum struct {
	index.TermsEnumBase

	field    *Lucene103FieldReader
	frame    *SegmentTermsEnumFrame
	startKey *util.BytesRef
}

// NewLucene103SegmentTermsEnum opens a strict-block-tree enumerator over
// field. startTerm, when non-nil, is the byte-prefix the iterator should
// position at (used by intersect()); a nil startTerm produces an
// enumerator positioned before the first term.
func NewLucene103SegmentTermsEnum(field *Lucene103FieldReader, startTerm *util.BytesRef) *Lucene103SegmentTermsEnum {
	return &Lucene103SegmentTermsEnum{
		field:    field,
		frame:    &SegmentTermsEnumFrame{},
		startKey: startTerm,
	}
}

// Field returns the FieldReader the enumerator is bound to.
func (e *Lucene103SegmentTermsEnum) Field() *Lucene103FieldReader { return e.field }

// Next advances to the next term. The stub returns nil (end of
// enumeration) because the on-disk .tim/.tip pull is the deferred port.
func (e *Lucene103SegmentTermsEnum) Next() (*index.Term, error) {
	return nil, nil
}

// SeekCeil positions the enumerator at term or the next ceiling term.
// The stub returns nil because the FST traversal is the deferred port.
func (e *Lucene103SegmentTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	return nil, nil
}

// SeekExact returns true when term exists. The stub returns false
// because the FST traversal is the deferred port.
func (e *Lucene103SegmentTermsEnum) SeekExact(term *index.Term) (bool, error) {
	return false, nil
}

// DocFreq returns the docFreq of the current term. The stub returns 0
// because there is no positioned term until the deferred port lands.
func (e *Lucene103SegmentTermsEnum) DocFreq() (int, error) { return 0, nil }

// TotalTermFreq returns the totalTermFreq of the current term. The stub
// returns 0 for the same reason as DocFreq.
func (e *Lucene103SegmentTermsEnum) TotalTermFreq() (int64, error) { return 0, nil }

// Postings returns a PostingsEnum for the current term. The stub
// returns an empty enumerator because Lucene103PostingsReader.Postings
// is the deferred behavioural port.
func (e *Lucene103SegmentTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	return &index.EmptyPostingsEnum{}, nil
}

// PostingsWithLiveDocs forwards to Postings; the stub ignores liveDocs.
func (e *Lucene103SegmentTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// Lucene103IntersectTermsEnum has moved to its own file
// (lucene103_intersect_terms_enum.go) so the strict block-tree
// SegmentTermsEnum port and the automaton-driven traversal can grow
// independently — see Sprint 56 / GOC-3323.

// Compile-time interface check for SegmentTermsEnum. The matching
// check for Lucene103IntersectTermsEnum lives in
// lucene103_intersect_terms_enum.go.
var _ index.TermsEnum = (*Lucene103SegmentTermsEnum)(nil)
