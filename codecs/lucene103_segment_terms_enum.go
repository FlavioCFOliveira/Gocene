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

package codecs

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Reference: lucene/core/src/java/org/apache/lucene/codecs/lucene103/blocktree/
// SegmentTermsEnum.java (Apache Lucene 10.4.0, 1070 lines).
//
// Lucene103SegmentTermsEnum is the Go port of
// org.apache.lucene.codecs.lucene103.blocktree.SegmentTermsEnum — the
// strict block-tree TermsEnum returned by Lucene103FieldReader.iterator()
// and (via [Lucene103IntersectTermsEnum]) FieldReader.intersect().
//
// The Java enumerator walks the on-disk .tim file using a stack of
// SegmentTermsEnumFrames; each Next() pull either advances within the
// current frame's suffix run or pops/pushes frames following the trie
// transitions stored in the .tip block index. The full Java traversal
// includes block prefetching, MMap-aware slicing, and Automaton-driven
// pruning when wrapped by IntersectTermsEnum.
//
// Scope of this port (Sprint 55 GOC-3327, option (c) — most-used
// surface):
//
//   - Public surface aligned with the Java class so callers in
//     [Lucene103FieldReader] and downstream tests can program against a
//     real index.TermsEnum (Term(), SeekCeil, SeekExact, Next, DocFreq,
//     TotalTermFreq, Postings, PostingsWithLiveDocs, plus TermState and
//     Ord which mirror the BaseTermsEnum surface).
//   - "Cached seek" semantics for SeekCeil / SeekExact: the enumerator
//     stores the requested key as currentTerm, mirroring the Java pattern
//     where the caller can read term() immediately after a positive
//     seekExact. This is what lets [Lucene103FieldReader.GetPostingsReader]
//     wire the seek -> postings call chain even before the FST walker
//     lands.
//   - Explicit, documented deviations for the byte-level traversal:
//     Next() returns end-of-enumeration, DocFreq / TotalTermFreq return
//     0, Postings returns an EmptyPostingsEnum. Each method explains
//     which Lucene method it stands in for, so the deferred port has a
//     one-line target on each call site.
//
// What is intentionally NOT in this file (deferred — backlog #2692):
//
//   - .tim/.tip block load (loadBlock / loadNextFloorBlock /
//     scanToFloorFrame / scanToTerm), the frame stack growth
//     (getFrame / pushFrame), and the trieReader-driven prepareSeekExact
//     state machine.
//   - decodeMetaData, Postings/Impacts production through
//     Lucene103PostingsReader (the postings reader itself is also a
//     forward dep on Sprint 55 / GOC-3328).
//   - printSeekState debugging path (Java assert-only).
type Lucene103SegmentTermsEnum struct {
	// currentTerm is the cursor position published through Term(). We
	// hold it on the concrete struct rather than reusing the package-
	// private index.TermsEnumBase.currentTerm because mutating that
	// field requires a setter that does not exist in the index package;
	// adding one would widen index.TermsEnumBase's surface for the
	// benefit of a single codec, which the back-compat rule for the
	// codec package forbids.
	currentTerm *index.Term

	field    *Lucene103FieldReader
	frame    *SegmentTermsEnumFrame
	startKey *util.BytesRef
	eof      bool
}

// NewLucene103SegmentTermsEnum opens a strict-block-tree enumerator over
// field. startTerm, when non-nil, is the byte-prefix the iterator should
// position at (used by intersect() and by GetIteratorWithSeek); a nil
// startTerm produces an enumerator positioned before the first term.
//
// Mirrors the Java constructor SegmentTermsEnum(FieldReader, TrieReader).
// The TrieReader is omitted here: the trie is opened lazily by the
// deferred deep-port through field.NewTrieReader() at the point of the
// first .tim block load.
func NewLucene103SegmentTermsEnum(field *Lucene103FieldReader, startTerm *util.BytesRef) *Lucene103SegmentTermsEnum {
	e := &Lucene103SegmentTermsEnum{
		field:    field,
		frame:    &SegmentTermsEnumFrame{},
		startKey: startTerm,
	}
	if startTerm != nil && field != nil {
		// Seed currentTerm so Term() returns the seek key immediately;
		// mirrors the Java "cached after seekExact" behaviour relied on
		// by FieldReader.GetPostingsReader callers.
		e.currentTerm = index.NewTermFromBytesRef(field.FieldInfo().Name(), startTerm)
	}
	return e
}

// Field returns the FieldReader the enumerator is bound to. There is no
// Java equivalent for this getter — the Java field is package-private —
// but Gocene needs it so other codec-package files (IntersectTermsEnum,
// future PostingsReader wiring) can reach the parent without a back
// pointer.
func (e *Lucene103SegmentTermsEnum) Field() *Lucene103FieldReader { return e.field }

// Frame returns the current cursor frame. Used by [Lucene103BlockTreeStats]
// and tests to inspect the per-block state without exporting the field.
func (e *Lucene103SegmentTermsEnum) Frame() *SegmentTermsEnumFrame { return e.frame }

// Term returns the current cursor position, or nil after EOF. The Java
// counterpart asserts !eof; Gocene chooses the soft contract so naive
// callers that loop on Next() until nil and then read Term() get nil
// instead of a stale key.
//
// Mirrors SegmentTermsEnum.term() in Java.
func (e *Lucene103SegmentTermsEnum) Term() *index.Term {
	if e.eof {
		return nil
	}
	return e.currentTerm
}

// Next advances to the next term. The strict-block-tree port is
// deferred to backlog #2692; the surface here returns
// (nil, nil) — the Lucene "no more terms" signal — and marks the
// enumerator as EOF so subsequent Term() calls report nil.
//
// Mirrors SegmentTermsEnum.next() in Java.
func (e *Lucene103SegmentTermsEnum) Next() (*index.Term, error) {
	e.eof = true
	e.currentTerm = nil
	return nil, nil
}

// SeekCeil positions the enumerator at term or, if term does not exist,
// at the next ceiling term. The deferred port (backlog #2692) will run
// the full prepareSeekExact + scanToTerm pipeline; today we publish term
// as the cursor position so callers can immediately read it through
// Term() and pull a (stub) PostingsEnum.
//
// Mirrors SegmentTermsEnum.seekCeil(BytesRef) in Java — but does not yet
// return SeekStatus distinguishing FOUND vs NOT_FOUND. Gocene encodes
// the simpler signature *Term + error already used across the
// index.TermsEnum interface.
func (e *Lucene103SegmentTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	if term == nil {
		return nil, errors.New("Lucene103SegmentTermsEnum.SeekCeil: term must not be nil")
	}
	e.eof = false
	e.startKey = term.BytesValue()
	cloned := term.Clone()
	e.currentTerm = cloned
	return cloned, nil
}

// SeekExact positions the enumerator on term if it exists and returns
// true; otherwise leaves it unpositioned and returns false.
//
// The deferred FST walker is what would actually probe the trie for
// presence; without it we conservatively return false. We DO publish the
// requested key as the cursor — matching Lucene's behaviour after a
// seekExact(BytesRef, TermState) call where the index walk is skipped —
// so a caller doing "seekExact -> Postings" still gets a coherent
// (empty) chain.
//
// Mirrors SegmentTermsEnum.seekExact(BytesRef) in Java.
func (e *Lucene103SegmentTermsEnum) SeekExact(term *index.Term) (bool, error) {
	if term == nil {
		return false, errors.New("Lucene103SegmentTermsEnum.SeekExact: term must not be nil")
	}
	e.eof = false
	e.startKey = term.BytesValue()
	e.currentTerm = term.Clone()
	return false, nil
}

// DocFreq returns the docFreq of the current term. The stub returns 0
// because there is no decoded BlockTermState until backlog #2692 lands;
// callers must treat 0 as "unknown / no decoded metadata" and not
// confuse it with a real "term seen in zero documents" reading.
//
// Mirrors SegmentTermsEnum.docFreq() in Java.
func (e *Lucene103SegmentTermsEnum) DocFreq() (int, error) { return 0, nil }

// TotalTermFreq returns the totalTermFreq of the current term. Same
// deferred-state caveat as DocFreq applies.
//
// Mirrors SegmentTermsEnum.totalTermFreq() in Java.
func (e *Lucene103SegmentTermsEnum) TotalTermFreq() (int64, error) { return 0, nil }

// Postings returns a PostingsEnum for the current term. The stub
// returns an empty enumerator because [Lucene103PostingsReader] is the
// deferred behavioural port (Sprint 55 GOC-3328 covers the .doc/.pos
// load).
//
// Mirrors SegmentTermsEnum.postings(PostingsEnum, int) in Java — the
// reuse-argument is dropped because Gocene's interface allocates a fresh
// PostingsEnum per call.
func (e *Lucene103SegmentTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	return &index.EmptyPostingsEnum{}, nil
}

// PostingsWithLiveDocs forwards to Postings; the stub ignores liveDocs.
// The deferred port will wire the liveDocs filter at the
// Lucene103PostingsReader boundary.
//
// No direct Java equivalent — Lucene encodes liveDocs filtering inside
// PostingsEnum itself; Gocene exposes it as a separate entry point.
func (e *Lucene103SegmentTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// TermState returns a TermState snapshot of the current term. The stub
// returns nil with no error: the Java method requires a positioned,
// metadata-decoded term and returns currentFrame.state.clone(); both
// pieces (currentFrame + decoded state) land with backlog #2692.
//
// Mirrors SegmentTermsEnum.termState() in Java.
func (e *Lucene103SegmentTermsEnum) TermState() (index.TermState, error) {
	if e.eof {
		return nil, nil
	}
	return nil, nil
}

// Ord returns the ordinal of the current term in the field. The Java
// strict block-tree enum throws UnsupportedOperationException because
// the format does not store ordinals; Gocene mirrors that by returning
// a sentinel error so callers can branch on it explicitly instead of
// recovering from a panic.
//
// Mirrors SegmentTermsEnum.ord() in Java.
func (e *Lucene103SegmentTermsEnum) Ord() (int64, error) {
	return -1, errSegmentTermsEnumOrdUnsupported
}

// errSegmentTermsEnumOrdUnsupported is the sentinel returned by
// [Lucene103SegmentTermsEnum.Ord] — mirrors the Java
// UnsupportedOperationException the strict block-tree enum throws.
var errSegmentTermsEnumOrdUnsupported = errors.New("Lucene103SegmentTermsEnum: ord is not supported by the block-tree codec")

// ErrSegmentTermsEnumOrdUnsupported exposes the sentinel above for use
// with errors.Is in callers that need to branch on "this codec has no
// ordinals" without string matching.
var ErrSegmentTermsEnumOrdUnsupported = errSegmentTermsEnumOrdUnsupported

// Lucene103IntersectTermsEnum has moved to its own file
// (lucene103_intersect_terms_enum.go) so the strict block-tree
// SegmentTermsEnum port and the automaton-driven traversal can grow
// independently — see Sprint 56 / GOC-3323.

// Compile-time interface check for SegmentTermsEnum. The matching
// check for Lucene103IntersectTermsEnum lives in
// lucene103_intersect_terms_enum.go.
var _ index.TermsEnum = (*Lucene103SegmentTermsEnum)(nil)
