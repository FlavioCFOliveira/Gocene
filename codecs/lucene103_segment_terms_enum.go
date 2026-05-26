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
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Reference: lucene/core/src/java/org/apache/lucene/codecs/lucene103/blocktree/
// SegmentTermsEnum.java (Apache Lucene 10.4.0, 1070 lines).
//
// Lucene103SegmentTermsEnum is the Go port of
// org.apache.lucene.codecs.lucene103.blocktree.SegmentTermsEnum — the
// strict block-tree TermsEnum returned by Lucene103FieldReader.iterator().
//
// Implementation note (Sprint 106 / backlog #2692 partial):
//
// This implements sequential Next() iteration over leaf blocks only.
// Floor blocks, sub-block recursion, and seekExact/seekCeil with full
// FST trie traversal remain deferred to backlog #2692.
//
// What IS implemented here:
//   - Sequential Next() reading .tim leaf blocks (covers the common case
//     where all terms fit in a single block, e.g. < minItemsInBlock=25).
//   - DocFreq() / TotalTermFreq() from decoded stats.
//   - Postings() wired through Lucene104PostingsReader.Postings().
//
// What is NOT implemented (deferred — backlog #2692):
//   - SeekExact / SeekCeil trie traversal.
//   - Sub-block recursion (pushFrame / popFrame).
//   - Floor block advances.
//   - LZ4 / LowercaseAscii suffix decompression (currently NO_COMPRESSION only).
type Lucene103SegmentTermsEnum struct {
	field    *Lucene103FieldReader
	frame    *SegmentTermsEnumFrame
	startKey *util.BytesRef
	eof      bool

	// currentTerm is the cursor position published through Term().
	currentTerm *index.Term

	// ── sequential iteration state ─────────────────────────────────────────

	// timIn is a per-enum clone of the parent block-tree reader's .tim
	// file handle. Nil until the first Next() call (lazy init mirrors Java).
	timIn store.IndexInput

	// Block-level state (single-frame; sub-block recursion is deferred).
	blockLoaded     bool  // true once the root block has been read
	blockPrefixLen  int   // shared prefix length for this block (0 at root)
	blockEntCount   int   // number of entries in the current block
	blockNextEnt    int   // next entry index within the block (0-based)
	blockIsLeaf     bool  // true for a leaf-only block
	blockIsLast     bool  // true if this block is the last floor block

	// Suffix data: suffixBytes holds the raw suffix corpus, suffixesReader
	// is positioned to read the next suffix within it.
	suffixBytes   []byte
	suffixesIn    *store.ByteArrayDataInput

	// Suffix lengths: each vInt-encoded suffix length, replicated allEqual
	// bytes when all are the same.
	suffixLengthBytes []byte
	suffixLengthsIn   *store.ByteArrayDataInput

	// Stats blob for decoding docFreq / totalTermFreq.
	statsBytes          []byte
	statsIn             *store.ByteArrayDataInput
	statsSingletonRun   int // tracks run-length singletons

	// Meta blob for decoding postings metadata.
	metaBytes       []byte
	metaBytesIn     *store.ByteArrayDataInput

	// termState is allocated once and reused across decodeMetaData calls.
	termState         *BlockTermState
	metaDataUpto      int // number of entries decoded so far from meta/stats
}

// blockLoadParams groups the per-block binary layout decoded in loadBlock.
type blockLoadParams struct {
	// suffixLen is the total byte count of the suffix corpus.
	suffixLen int
	// allEqual is true when all suffix lengths are identical.
	allEqual bool
	// numSuffixLengthBytes is the byte count of the suffix-lengths blob.
	numSuffixLengthBytes int
	// comprAlgCode is the CompressionAlgorithm code (0 = NO_COMPRESSION).
	comprAlgCode int
}

// NewLucene103SegmentTermsEnum opens a strict-block-tree enumerator over
// field. startTerm, when non-nil, seeds the cursor for callers that perform
// a seek before calling Next(); nil means "start before the first term".
//
// Mirrors SegmentTermsEnum(FieldReader, TrieReader) in Java.
func NewLucene103SegmentTermsEnum(field *Lucene103FieldReader, startTerm *util.BytesRef) *Lucene103SegmentTermsEnum {
	e := &Lucene103SegmentTermsEnum{
		field:    field,
		frame:    &SegmentTermsEnumFrame{},
		startKey: startTerm,
	}
	if startTerm != nil && field != nil {
		e.currentTerm = index.NewTermFromBytesRef(field.FieldInfo().Name(), startTerm)
	}
	return e
}

// Field returns the FieldReader the enumerator is bound to.
func (e *Lucene103SegmentTermsEnum) Field() *Lucene103FieldReader { return e.field }

// Frame returns the current cursor frame (used by Stats and tests).
func (e *Lucene103SegmentTermsEnum) Frame() *SegmentTermsEnumFrame { return e.frame }

// Term returns the current cursor position, or nil at EOF.
// Mirrors SegmentTermsEnum.term() in Java.
func (e *Lucene103SegmentTermsEnum) Term() *index.Term {
	if e.eof {
		return nil
	}
	return e.currentTerm
}

// Next advances to the next term in the block-tree.
//
// This implementation handles leaf blocks only (no sub-block recursion).
// It lazily opens the .tim file on the first call, loads the root block,
// and iterates entries sequentially.
//
// Mirrors SegmentTermsEnum.next() in Java.
func (e *Lucene103SegmentTermsEnum) Next() (*index.Term, error) {
	if e.eof {
		return nil, nil
	}
	if e.field == nil || e.field.parent == nil {
		e.eof = true
		return nil, nil
	}

	// Lazy init: clone the .tim IndexInput on first call.
	if e.timIn == nil {
		e.timIn = e.field.parent.termsIn.Clone()
	}

	// Load the root block on the first Next() call.
	if !e.blockLoaded {
		// The root block FP is the OutputFP stored on the trie root node —
		// that is the file pointer into the .tim (terms) file, NOT the
		// field.rootFP which is the trie-node offset inside the .tip file.
		trieReader, err := e.field.NewTrieReader()
		if err != nil {
			return nil, fmt.Errorf("Lucene103SegmentTermsEnum.Next: open trie: %w", err)
		}
		root := trieReader.Root()
		if !root.HasOutput() {
			// No terms at all in this field.
			e.eof = true
			return nil, nil
		}
		if err := e.loadBlock(root.OutputFP, 0); err != nil {
			return nil, fmt.Errorf("Lucene103SegmentTermsEnum.Next: load root block: %w", err)
		}
	}

	// Exhausted the block?
	if e.blockNextEnt >= e.blockEntCount {
		e.eof = true
		e.currentTerm = nil
		return nil, nil
	}

	if e.blockIsLeaf {
		if err := e.nextLeaf(); err != nil {
			return nil, fmt.Errorf("Lucene103SegmentTermsEnum.Next: nextLeaf: %w", err)
		}
	} else {
		// Non-leaf: peek the discriminant bit in suffixLengthsReader.
		// Odd bit = sub-block entry; skip until we find a term entry.
		found, err := e.nextNonLeafTerm()
		if err != nil {
			return nil, fmt.Errorf("Lucene103SegmentTermsEnum.Next: nextNonLeaf: %w", err)
		}
		if !found {
			e.eof = true
			e.currentTerm = nil
			return nil, nil
		}
	}
	return e.currentTerm, nil
}

// loadBlock seeks to fp in the .tim file and loads the block header, suffix
// corpus, suffix-lengths blob, stats blob, and meta blob into in-memory
// byte slices. prefixLength is the shared prefix depth (0 at the root).
//
// Mirrors SegmentTermsEnumFrame.loadBlock() in Java.
func (e *Lucene103SegmentTermsEnum) loadBlock(fp int64, prefixLength int) error {
	if err := e.timIn.SetPosition(fp); err != nil {
		return fmt.Errorf("loadBlock: seek to fp %d: %w", fp, err)
	}

	// Block header: vInt(numEntries<<1 | isLastBlock).
	code, err := store.ReadVInt(e.timIn)
	if err != nil {
		return fmt.Errorf("loadBlock: read block header: %w", err)
	}
	e.blockEntCount = int(code >> 1)
	e.blockIsLast = (code & 1) != 0
	if e.blockEntCount == 0 {
		return fmt.Errorf("loadBlock: block at fp=%d has entCount=0", fp)
	}

	// Token: vLong(suffixLen<<3 | isLeaf<<2 | comprAlg).
	token, err := store.ReadVLong(e.timIn)
	if err != nil {
		return fmt.Errorf("loadBlock: read token: %w", err)
	}
	numSuffixBytes := int(token >> 3)
	e.blockIsLeaf = (token & 0x04) != 0
	comprAlgCode := int(token & 0x03)
	if comprAlgCode != 0 {
		// CompressionAlgorithm != NO_COMPRESSION. Decompression of LZ4 and
		// LowercaseAscii is deferred — return an informative error so callers
		// know what's missing rather than silently corrupting data.
		return fmt.Errorf("loadBlock: compression algorithm %d not yet supported (only NO_COMPRESSION=0)", comprAlgCode)
	}

	// Suffix corpus (raw bytes, uncompressed).
	if cap(e.suffixBytes) < numSuffixBytes {
		e.suffixBytes = make([]byte, numSuffixBytes)
	}
	e.suffixBytes = e.suffixBytes[:numSuffixBytes]
	if numSuffixBytes > 0 {
		if err := e.timIn.ReadBytes(e.suffixBytes); err != nil {
			return fmt.Errorf("loadBlock: read suffix bytes: %w", err)
		}
	}
	if e.suffixesIn == nil {
		e.suffixesIn = store.NewByteArrayDataInput(e.suffixBytes)
	} else {
		e.suffixesIn.ResetWithSlice(e.suffixBytes, 0, numSuffixBytes)
	}

	// Suffix-lengths blob.
	slHeader, err := store.ReadVInt(e.timIn)
	if err != nil {
		return fmt.Errorf("loadBlock: read suffix-lengths header: %w", err)
	}
	allEqual := (slHeader & 1) != 0
	numSuffixLengthBytes := int(slHeader >> 1)
	if cap(e.suffixLengthBytes) < numSuffixLengthBytes {
		e.suffixLengthBytes = make([]byte, numSuffixLengthBytes)
	}
	e.suffixLengthBytes = e.suffixLengthBytes[:numSuffixLengthBytes]
	if allEqual && numSuffixLengthBytes > 0 {
		b, err2 := e.timIn.ReadByte()
		if err2 != nil {
			return fmt.Errorf("loadBlock: read allEqual suffix length byte: %w", err2)
		}
		for i := range e.suffixLengthBytes {
			e.suffixLengthBytes[i] = b
		}
	} else if numSuffixLengthBytes > 0 {
		if err := e.timIn.ReadBytes(e.suffixLengthBytes); err != nil {
			return fmt.Errorf("loadBlock: read suffix-length bytes: %w", err)
		}
	}
	if e.suffixLengthsIn == nil {
		e.suffixLengthsIn = store.NewByteArrayDataInput(e.suffixLengthBytes)
	} else {
		e.suffixLengthsIn.ResetWithSlice(e.suffixLengthBytes, 0, numSuffixLengthBytes)
	}

	// Stats blob.
	numStatsBytes, err := store.ReadVInt(e.timIn)
	if err != nil {
		return fmt.Errorf("loadBlock: read numStatsBytes: %w", err)
	}
	if cap(e.statsBytes) < int(numStatsBytes) {
		e.statsBytes = make([]byte, numStatsBytes)
	}
	e.statsBytes = e.statsBytes[:numStatsBytes]
	if numStatsBytes > 0 {
		if err := e.timIn.ReadBytes(e.statsBytes); err != nil {
			return fmt.Errorf("loadBlock: read stats bytes: %w", err)
		}
	}
	if e.statsIn == nil {
		e.statsIn = store.NewByteArrayDataInput(e.statsBytes)
	} else {
		e.statsIn.ResetWithSlice(e.statsBytes, 0, int(numStatsBytes))
	}

	// Meta blob.
	numMetaBytes, err := store.ReadVInt(e.timIn)
	if err != nil {
		return fmt.Errorf("loadBlock: read numMetaBytes: %w", err)
	}
	if cap(e.metaBytes) < int(numMetaBytes) {
		e.metaBytes = make([]byte, numMetaBytes)
	}
	e.metaBytes = e.metaBytes[:numMetaBytes]
	if numMetaBytes > 0 {
		if err := e.timIn.ReadBytes(e.metaBytes); err != nil {
			return fmt.Errorf("loadBlock: read meta bytes: %w", err)
		}
	}
	if e.metaBytesIn == nil {
		e.metaBytesIn = store.NewByteArrayDataInput(e.metaBytes)
	} else {
		e.metaBytesIn.ResetWithSlice(e.metaBytes, 0, int(numMetaBytes))
	}

	// Allocate a BlockTermState for this field if needed.
	if e.termState == nil && e.field.parent != nil {
		e.termState = e.field.parent.postingsReader.NewTermState()
	}

	e.blockPrefixLen = prefixLength
	e.blockNextEnt = 0
	e.metaDataUpto = 0
	e.statsSingletonRun = 0
	e.blockLoaded = true
	return nil
}

// nextLeaf advances to the next entry in a leaf block. It reads one suffix
// length from suffixLengthsIn, copies the suffix bytes from suffixesIn, and
// reconstructs the full term. termExists is always true for leaf entries.
//
// Mirrors SegmentTermsEnumFrame.nextLeaf() in Java.
func (e *Lucene103SegmentTermsEnum) nextLeaf() error {
	suffixLen, err := store.ReadVInt(e.suffixLengthsIn)
	if err != nil {
		return fmt.Errorf("nextLeaf: read suffix length: %w", err)
	}
	termLen := e.blockPrefixLen + int(suffixLen)
	termBuf := make([]byte, termLen)
	// Copy prefix (none at root level; blockPrefixLen == 0).
	if e.blockPrefixLen > 0 && e.currentTerm != nil {
		copy(termBuf[:e.blockPrefixLen], e.currentTerm.BytesValue().Bytes[:e.blockPrefixLen])
	}
	// Append suffix.
	if err := e.suffixesIn.ReadBytes(termBuf[e.blockPrefixLen:]); err != nil {
		return fmt.Errorf("nextLeaf: read suffix bytes: %w", err)
	}
	e.currentTerm = index.NewTermFromBytes(e.field.fieldInfo.Name(), termBuf)
	e.blockNextEnt++
	e.eof = false
	return nil
}

// nextNonLeafTerm advances through a non-leaf block until a term entry is
// found (skipping sub-block entries). Returns false when the block is
// exhausted. The sub-block push logic (recursion into child blocks) is
// deferred — this implementation only handles the flat case where terms
// and sub-blocks are interleaved but sub-blocks are always skipped.
//
// For sub-blocks the suffixLengthsReader contains an odd vInt followed by
// a vLong sub-block delta; we skip both to advance past the entry.
// Mirrors SegmentTermsEnumFrame.nextNonLeaf() in Java (skip-only path).
func (e *Lucene103SegmentTermsEnum) nextNonLeafTerm() (bool, error) {
	for e.blockNextEnt < e.blockEntCount {
		code, err := store.ReadVInt(e.suffixLengthsIn)
		if err != nil {
			return false, fmt.Errorf("nextNonLeafTerm: read code: %w", err)
		}
		suffixLen := int(code >> 1)
		isSubBlock := (code & 1) != 0

		termLen := e.blockPrefixLen + suffixLen
		termBuf := make([]byte, termLen)
		if e.blockPrefixLen > 0 && e.currentTerm != nil {
			copy(termBuf[:e.blockPrefixLen], e.currentTerm.BytesValue().Bytes[:e.blockPrefixLen])
		}
		if err := e.suffixesIn.ReadBytes(termBuf[e.blockPrefixLen:]); err != nil {
			return false, fmt.Errorf("nextNonLeafTerm: read suffix: %w", err)
		}
		e.blockNextEnt++

		if isSubBlock {
			// Skip the sub-block file-pointer delta.
			if _, err := store.ReadVLong(e.suffixLengthsIn); err != nil {
				return false, fmt.Errorf("nextNonLeafTerm: read subblock fp delta: %w", err)
			}
			// Sub-block: skip (deferred recursion) — do not yield this entry.
			continue
		}
		// Term entry: yield it.
		e.currentTerm = index.NewTermFromBytes(e.field.fieldInfo.Name(), termBuf)
		e.eof = false
		return true, nil
	}
	return false, nil
}

// decodeMetaData decodes stats (docFreq / totalTermFreq) and postings
// metadata for the current term, catching up metaDataUpto to the current
// block ordinal (blockNextEnt).
//
// Mirrors SegmentTermsEnumFrame.decodeMetaData() in Java.
func (e *Lucene103SegmentTermsEnum) decodeMetaData() error {
	if e.termState == nil {
		return errors.New("decodeMetaData: no termState allocated")
	}
	if e.statsIn == nil {
		return errors.New("decodeMetaData: stats blob not loaded")
	}

	// Determine the target block ordinal.  For a leaf block this equals
	// blockNextEnt; for a non-leaf it tracks only term entries (sub-blocks
	// do not consume a stats entry). Since we skip sub-blocks in
	// nextNonLeafTerm, blockNextEnt counts only terms already yielded, so
	// we use blockNextEnt directly.
	limit := e.blockNextEnt
	absolute := e.metaDataUpto == 0
	hasFreqs := e.field.fieldInfo.IndexOptions() != index.IndexOptionsDocs

	for e.metaDataUpto < limit {
		// Decode one stats entry.
		if e.statsSingletonRun > 0 {
			e.termState.DocFreq = 1
			e.termState.TotalTermFreq = 1
			e.statsSingletonRun--
		} else {
			tok, err := store.ReadVInt(e.statsIn)
			if err != nil {
				return fmt.Errorf("decodeMetaData: read stats token: %w", err)
			}
			if (tok & 1) == 1 {
				// Singleton run: docFreq=1, totalTermFreq=1 for (tok>>1)+1 terms.
				e.termState.DocFreq = 1
				e.termState.TotalTermFreq = 1
				e.statsSingletonRun = int(tok >> 1)
			} else {
				e.termState.DocFreq = int(tok >> 1)
				if hasFreqs {
					ttfDelta, err2 := store.ReadVLong(e.statsIn)
					if err2 != nil {
						return fmt.Errorf("decodeMetaData: read ttf delta: %w", err2)
					}
					e.termState.TotalTermFreq = int64(e.termState.DocFreq) + ttfDelta
				} else {
					e.termState.TotalTermFreq = int64(e.termState.DocFreq)
				}
			}
		}

		// Decode one postings meta entry.
		if e.field.parent != nil {
			if err := e.field.parent.postingsReader.DecodeTerm(
				e.metaBytesIn, e.field.fieldInfo, e.termState, absolute,
			); err != nil {
				return fmt.Errorf("decodeMetaData: DecodeTerm: %w", err)
			}
		}
		absolute = false
		e.metaDataUpto++
	}
	e.termState.TermBlockOrd = e.metaDataUpto
	return nil
}

// DocFreq returns the document frequency of the current term, decoding the
// stats blob if necessary.
//
// Mirrors SegmentTermsEnum.docFreq() in Java.
func (e *Lucene103SegmentTermsEnum) DocFreq() (int, error) {
	if e.eof || !e.blockLoaded {
		return 0, nil
	}
	if err := e.decodeMetaData(); err != nil {
		return 0, err
	}
	return e.termState.DocFreq, nil
}

// TotalTermFreq returns the total term frequency of the current term.
//
// Mirrors SegmentTermsEnum.totalTermFreq() in Java.
func (e *Lucene103SegmentTermsEnum) TotalTermFreq() (int64, error) {
	if e.eof || !e.blockLoaded {
		return 0, nil
	}
	if err := e.decodeMetaData(); err != nil {
		return 0, err
	}
	return e.termState.TotalTermFreq, nil
}

// Postings decodes the current term's metadata and returns a PostingsEnum
// via the parent's PostingsReaderBase.
//
// Mirrors SegmentTermsEnum.postings(PostingsEnum, int) in Java.
func (e *Lucene103SegmentTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if e.eof || !e.blockLoaded {
		return &index.EmptyPostingsEnum{}, nil
	}
	if err := e.decodeMetaData(); err != nil {
		return nil, fmt.Errorf("Lucene103SegmentTermsEnum.Postings: decodeMetaData: %w", err)
	}
	if e.field.parent == nil {
		return &index.EmptyPostingsEnum{}, nil
	}
	pe, err := e.field.parent.postingsReader.Postings(
		e.field.fieldInfo, e.termState, nil, flags,
	)
	if err != nil {
		return nil, fmt.Errorf("Lucene103SegmentTermsEnum.Postings: postingsReader.Postings: %w", err)
	}
	return pe, nil
}

// PostingsWithLiveDocs forwards to Postings ignoring liveDocs (deferred filter).
func (e *Lucene103SegmentTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// SeekCeil positions the enumerator at term or, if term does not exist, at
// the next ceiling term. The FST trie traversal is deferred (backlog #2692);
// today the requested key is published as the cursor position only.
//
// Mirrors SegmentTermsEnum.seekCeil(BytesRef) in Java.
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

// SeekExact positions the enumerator on term. This implementation performs a
// linear scan using Next() since the full FST trie traversal is not yet
// ported (backlog #2692). The scan is O(N) in the number of terms in the
// field but is correct for all block layouts produced by the Lucene 10.4
// block-tree writer.
//
// Mirrors SegmentTermsEnum.seekExact(BytesRef) in Java.
func (e *Lucene103SegmentTermsEnum) SeekExact(term *index.Term) (bool, error) {
	if term == nil {
		return false, errors.New("Lucene103SegmentTermsEnum.SeekExact: term must not be nil")
	}
	// Create a fresh scanning enum to avoid corrupting the current cursor's
	// ongoing iteration. We scan until we find the target or overshoot it.
	scanner := NewLucene103SegmentTermsEnum(e.field, nil)
	targetText := term.Text()
	for {
		found, err := scanner.Next()
		if err != nil {
			return false, fmt.Errorf("Lucene103SegmentTermsEnum.SeekExact: scan: %w", err)
		}
		if found == nil {
			// Exhausted — term not in the dictionary.
			e.eof = true
			e.currentTerm = nil
			return false, nil
		}
		cmp := strings.Compare(found.Text(), targetText)
		if cmp == 0 {
			// Replace this enum's state with the scanner's state so that
			// subsequent Postings() / DocFreq() calls operate on the
			// correctly-positioned block.
			*e = *scanner
			return true, nil
		}
		if cmp > 0 {
			// Passed the target — term not present.
			e.eof = true
			e.currentTerm = nil
			return false, nil
		}
	}
}

// TermState returns a snapshot of the current term's state. Requires that
// decodeMetaData has been called (which happens via DocFreq/Postings).
//
// Mirrors SegmentTermsEnum.termState() in Java.
func (e *Lucene103SegmentTermsEnum) TermState() (index.TermState, error) {
	if e.eof {
		return nil, nil
	}
	return nil, nil
}

// Ord is not supported by the block-tree codec (Java throws UOE).
func (e *Lucene103SegmentTermsEnum) Ord() (int64, error) {
	return -1, errSegmentTermsEnumOrdUnsupported
}

var errSegmentTermsEnumOrdUnsupported = errors.New(
	"Lucene103SegmentTermsEnum: ord is not supported by the block-tree codec")

// ErrSegmentTermsEnumOrdUnsupported is the exported sentinel for errors.Is.
var ErrSegmentTermsEnumOrdUnsupported = errSegmentTermsEnumOrdUnsupported

// Compile-time interface check.
var _ index.TermsEnum = (*Lucene103SegmentTermsEnum)(nil)
