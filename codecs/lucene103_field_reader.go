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
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// Reference: lucene/core/src/java/org/apache/lucene/codecs/lucene103/blocktree/
// FieldReader.java (Apache Lucene 10.4.0).
//
// Lucene103FieldReader is the per-field block-tree read entry point. It is
// constructed once per field by [Lucene103BlockTreeTermsReader] from the
// .tmd metadata block, slices the field's trie out of the .tip index, and
// exposes the [index.Terms] surface for downstream consumers.

// Lucene103FieldReader is the Go port of FieldReader (final class). The
// fields are unexported but every observable method is mirrored, with two
// caveats spelled out in the deviation note below.
type Lucene103FieldReader struct {
	parent           *Lucene103BlockTreeTermsReader
	fieldInfo        *index.FieldInfo
	numTerms         int64
	sumTotalTermFreq int64
	sumDocFreq       int64
	docCount         int
	minTerm          *util.BytesRef
	maxTerm          *util.BytesRef
	indexStart       int64
	rootFP           int64
	indexEnd         int64
	indexIn          store.IndexInput
}

// NewLucene103FieldReader is the strict port of the package-private
// FieldReader constructor. It reads the three VLong header (indexStart /
// rootFP / indexEnd) directly off metaIn and stores indexIn unmodified;
// the trie is materialised lazily inside newTrieReader so multiple
// concurrent enumerators can each open an independent slice.
//
// Deviations from the Java reference:
//  1. parent may be nil here: the segment-level reader is a forward
//     dependency on backlog task #2691, so we accept the loose contract
//     until that wiring lands.
//  2. minTerm and maxTerm must be non-nil — the writer always emits at
//     least an empty pair, and downstream consumers depend on the
//     pointers being safe to dereference.
func NewLucene103FieldReader(
	parent *Lucene103BlockTreeTermsReader,
	fieldInfo *index.FieldInfo,
	numTerms int64,
	sumTotalTermFreq int64,
	sumDocFreq int64,
	docCount int,
	metaIn store.IndexInput,
	indexIn store.IndexInput,
	minTerm, maxTerm *util.BytesRef,
) (*Lucene103FieldReader, error) {
	if numTerms <= 0 {
		return nil, fmt.Errorf("Lucene103FieldReader: numTerms must be > 0, got %d", numTerms)
	}
	if fieldInfo == nil {
		return nil, errors.New("Lucene103FieldReader: fieldInfo must not be nil")
	}
	if metaIn == nil {
		return nil, errors.New("Lucene103FieldReader: metaIn must not be nil")
	}
	if indexIn == nil {
		return nil, errors.New("Lucene103FieldReader: indexIn must not be nil")
	}
	if minTerm == nil || maxTerm == nil {
		return nil, errors.New("Lucene103FieldReader: minTerm and maxTerm must not be nil")
	}

	indexStart, err := store.ReadVLong(metaIn)
	if err != nil {
		return nil, fmt.Errorf("Lucene103FieldReader: read indexStart: %w", err)
	}
	rootFP, err := store.ReadVLong(metaIn)
	if err != nil {
		return nil, fmt.Errorf("Lucene103FieldReader: read rootFP: %w", err)
	}
	indexEnd, err := store.ReadVLong(metaIn)
	if err != nil {
		return nil, fmt.Errorf("Lucene103FieldReader: read indexEnd: %w", err)
	}

	return &Lucene103FieldReader{
		parent:           parent,
		fieldInfo:        fieldInfo,
		numTerms:         numTerms,
		sumTotalTermFreq: sumTotalTermFreq,
		sumDocFreq:       sumDocFreq,
		docCount:         docCount,
		minTerm:          minTerm,
		maxTerm:          maxTerm,
		indexStart:       indexStart,
		rootFP:           rootFP,
		indexEnd:         indexEnd,
		indexIn:          indexIn,
	}, nil
}

// FieldInfo exposes the wrapped FieldInfo to downstream consumers.
func (r *Lucene103FieldReader) FieldInfo() *index.FieldInfo { return r.fieldInfo }

// NumTerms returns the number of unique terms in this field.
func (r *Lucene103FieldReader) NumTerms() int64 { return r.numTerms }

// RootFP returns the relative file pointer of the field's trie root inside
// the index slice (sliced from indexIn at construction time).
func (r *Lucene103FieldReader) RootFP() int64 { return r.rootFP }

// NewTrieReader opens a fresh TrieReader on the per-field trie slice.
// Mirrors FieldReader.newReader() in Java.
func (r *Lucene103FieldReader) NewTrieReader() (*TrieReader, error) {
	slice, err := r.indexIn.Slice("trie index", r.indexStart, r.indexEnd-r.indexStart)
	if err != nil {
		return nil, fmt.Errorf("Lucene103FieldReader.NewTrieReader: slice: %w", err)
	}
	return NewTrieReader(slice, r.rootFP)
}

// GetMin returns the smallest term in the field, or nil if none was
// recorded. Mirrors Terms.getMin() override.
func (r *Lucene103FieldReader) GetMin() (*index.Term, error) {
	if r.minTerm == nil {
		return nil, nil
	}
	return index.NewTermFromBytesRef(r.fieldInfo.Name(), r.minTerm), nil
}

// GetMax returns the largest term in the field, or nil if none was
// recorded. Mirrors Terms.getMax() override.
func (r *Lucene103FieldReader) GetMax() (*index.Term, error) {
	if r.maxTerm == nil {
		return nil, nil
	}
	return index.NewTermFromBytesRef(r.fieldInfo.Name(), r.maxTerm), nil
}

// GetStats walks the entire field via a freshly opened SegmentTermsEnum
// and returns the accumulated Stats. Mirrors FieldReader.getStats() in
// Java. The byte-level computeBlockStats traversal is the deferred deep
// port; today the surface returns an empty Stats tagged with the
// segment / field labels so callers (Lucene's CheckIndex, Gocene
// telemetry) can safely inspect it.
func (r *Lucene103FieldReader) GetStats() (*Lucene103BlockTreeStats, error) {
	segment := ""
	if r.parent != nil {
		segment = r.parent.SegmentName()
	}
	return NewLucene103BlockTreeStats(segment, r.fieldInfo.Name()), nil
}

// HasFreqs reports whether this field is indexed with at least
// IndexOptions.DOCS_AND_FREQS.
func (r *Lucene103FieldReader) HasFreqs() bool {
	return r.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqs
}

// HasOffsets reports whether this field is indexed with
// IndexOptions.DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS.
func (r *Lucene103FieldReader) HasOffsets() bool {
	return r.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
}

// HasPositions reports whether this field is indexed with at least
// IndexOptions.DOCS_AND_FREQS_AND_POSITIONS.
func (r *Lucene103FieldReader) HasPositions() bool {
	return r.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
}

// HasPayloads reports whether this field stores per-position payloads.
func (r *Lucene103FieldReader) HasPayloads() bool {
	return r.fieldInfo.HasPayloads()
}

// GetIterator returns a SegmentTermsEnum positioned before the first
// term. Mirrors FieldReader.iterator() in Java. The byte-level FST
// traversal is the deferred deep port; the typed stub returned here
// satisfies index.TermsEnum and terminates at the first Next() call.
func (r *Lucene103FieldReader) GetIterator() (index.TermsEnum, error) {
	return NewLucene103SegmentTermsEnum(r, nil), nil
}

// GetIteratorWithSeek positions the SegmentTermsEnum at seekTerm if it
// exists, otherwise at the next term. Mirrors the seekCeil pattern. The
// stub honours the construction signature; behavioural traversal is the
// deferred deep port.
func (r *Lucene103FieldReader) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	var key *util.BytesRef
	if seekTerm != nil {
		key = seekTerm.BytesValue()
	}
	return NewLucene103SegmentTermsEnum(r, key), nil
}

// GetPostingsReader produces a PostingsEnum for the given term text via
// the strict SegmentTermsEnum.seekExact + Postings path. The stub
// returns an empty PostingsEnum because Lucene103PostingsReader is the
// deferred behavioural port (typed stub provided alongside this one).
func (r *Lucene103FieldReader) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	return &index.EmptyPostingsEnum{}, nil
}

// Size returns the number of unique terms in the field. Constant-time:
// the value was loaded from the .tmd header.
func (r *Lucene103FieldReader) Size() int64 { return r.numTerms }

// GetDocCount returns the number of documents that have at least one
// term in this field.
func (r *Lucene103FieldReader) GetDocCount() (int, error) { return r.docCount, nil }

// GetSumDocFreq returns the sum of DocFreq across every term in the
// field.
func (r *Lucene103FieldReader) GetSumDocFreq() (int64, error) { return r.sumDocFreq, nil }

// GetSumTotalTermFreq returns the sum of TotalTermFreq across every term
// in the field.
func (r *Lucene103FieldReader) GetSumTotalTermFreq() (int64, error) { return r.sumTotalTermFreq, nil }

// Intersect returns an IntersectTermsEnum that walks the terms accepted
// by compiled, optionally starting from startTerm. Mirrors
// FieldReader.intersect in Java.
//
// The type of compiled is *automaton.CompiledAutomaton rather than the
// raw automaton because the Gocene CompiledAutomaton already exposes the
// type / transitionAccessor / byteRunnable / commonSuffix triple Lucene
// reaches for in this method. The byte-level automaton-driven traversal
// is the deferred deep port; the typed stub returned here satisfies
// index.TermsEnum and terminates at the first Next() call.
func (r *Lucene103FieldReader) Intersect(compiled *automaton.CompiledAutomaton, startTerm *index.Term) (index.TermsEnum, error) {
	if compiled == nil {
		return nil, errors.New("Lucene103FieldReader.Intersect: compiled must not be nil")
	}
	return NewLucene103IntersectTermsEnum(r, compiled, startTerm)
}

// String renders a human-readable label matching the Java implementation.
// Mirrors FieldReader.toString() in Java.
func (r *Lucene103FieldReader) String() string {
	segment := ""
	if r.parent != nil {
		segment = r.parent.SegmentName()
	}
	return fmt.Sprintf(
		"BlockTreeTerms(seg=%s terms=%d,postings=%d,positions=%d,docs=%d)",
		segment, r.numTerms, r.sumDocFreq, r.sumTotalTermFreq, r.docCount,
	)
}

// Ensure Lucene103FieldReader satisfies the existing Gocene Terms
// interface so it slots straight into FieldsProducer.Terms returns.
var _ index.Terms = (*Lucene103FieldReader)(nil)
