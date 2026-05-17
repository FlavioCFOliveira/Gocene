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
	"bytes"
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Reference: lucene/core/src/java/org/apache/lucene/codecs/lucene103/blocktree/
// Lucene103BlockTreeTermsWriter.java (Apache Lucene 10.4.0). Wire-format
// constants (codec names, file extensions, version range) live alongside
// the Reader port in lucene103_block_tree_terms_reader.go.
//
// Per-task workflow note (Sprint 17 task #99): this file is the strict
// byte-for-byte port of the writer side. The Stats helper, the FieldReader,
// and the Reader proper live in the sibling lucene103_*.go files in this
// directory; SegmentTermsEnum / IntersectTermsEnum are deferred to backlog
// task #2692. The Lucene104PostingsFormat SPI shim references that backlog
// because instantiating this writer requires a concrete PostingsWriterBase
// (also deferred — backlog task #2691).

// Lucene103DefaultMinBlockSize is the suggested minimum number of items
// (terms or sub-blocks) per term-dictionary block. Mirrors
// {@code Lucene103BlockTreeTermsWriter.DEFAULT_MIN_BLOCK_SIZE} (= 25).
const Lucene103DefaultMinBlockSize = 25

// Lucene103DefaultMaxBlockSize is the suggested maximum number of items
// per block. Mirrors {@code Lucene103BlockTreeTermsWriter.DEFAULT_MAX_BLOCK_SIZE}
// (= 48).
const Lucene103DefaultMaxBlockSize = 48

// Lucene103BlockTreeTermsWriter is the strict Go port of
// org.apache.lucene.codecs.lucene103.blocktree.Lucene103BlockTreeTermsWriter.
//
// It writes the three on-disk files that make up a block-tree term
// dictionary:
//   - .tim (terms dictionary): a sequence of block headers and packed
//     suffix/stat/meta blobs.
//   - .tip (terms index): per-field prefix tries (one [TrieBuilder] each)
//     persisted via [TrieBuilder.Save].
//   - .tmd (terms metadata): per-field summary (numTerms / sums / min /
//     max term) plus the trailing indexLength / termsLength footer tail.
//
// The writer is single-threaded; callers must serialise WriteFields and
// Close invocations themselves.
type Lucene103BlockTreeTermsWriter struct {
	metaOut  store.IndexOutput
	termsOut store.IndexOutput
	indexOut store.IndexOutput

	maxDoc          int
	minItemsInBlock int
	maxItemsInBlock int
	version         int32

	postingsWriter PostingsWriterBase
	fieldInfos     *index.FieldInfos

	fields []*store.ByteBuffersDataOutput

	// scratchBytes is reused inside compileIndex to materialise floor data
	// blobs and to avoid re-allocating per-block.
	scratchBytes *store.ByteBuffersDataOutput

	closed bool
}

// NewLucene103BlockTreeTermsWriter is the canonical constructor; it pins
// the writer to the current wire-format version. Use
// NewLucene103BlockTreeTermsWriterAtVersion for back-compat tests.
func NewLucene103BlockTreeTermsWriter(
	state *SegmentWriteState,
	postingsWriter PostingsWriterBase,
	minItemsInBlock, maxItemsInBlock int,
) (*Lucene103BlockTreeTermsWriter, error) {
	return NewLucene103BlockTreeTermsWriterAtVersion(state, postingsWriter, minItemsInBlock, maxItemsInBlock, Lucene103BlockTreeVersionCurrent)
}

// NewLucene103BlockTreeTermsWriterAtVersion creates a writer that emits
// the given wire-format version. Out-of-range versions are rejected.
// Mirrors the package-private Java constructor used by back-compat tests.
func NewLucene103BlockTreeTermsWriterAtVersion(
	state *SegmentWriteState,
	postingsWriter PostingsWriterBase,
	minItemsInBlock, maxItemsInBlock int,
	version int32,
) (*Lucene103BlockTreeTermsWriter, error) {
	if err := ValidateLucene103BlockTreeBlockSizes(minItemsInBlock, maxItemsInBlock); err != nil {
		return nil, err
	}
	if version < Lucene103BlockTreeVersionStart || version > Lucene103BlockTreeVersionCurrent {
		return nil, fmt.Errorf(
			"expected version in range [%d, %d], but got %d",
			Lucene103BlockTreeVersionStart, Lucene103BlockTreeVersionCurrent, version,
		)
	}
	if state == nil {
		return nil, errors.New("Lucene103BlockTreeTermsWriter: state must not be nil")
	}
	if postingsWriter == nil {
		return nil, errors.New("Lucene103BlockTreeTermsWriter: postingsWriter must not be nil")
	}

	segmentName := state.SegmentInfo.Name()
	segmentSuffix := state.SegmentSuffix
	segmentID := state.SegmentInfo.GetID()
	directory := state.Directory

	w := &Lucene103BlockTreeTermsWriter{
		maxDoc:          state.SegmentInfo.DocCount(),
		minItemsInBlock: minItemsInBlock,
		maxItemsInBlock: maxItemsInBlock,
		version:         version,
		postingsWriter:  postingsWriter,
		fieldInfos:      state.FieldInfos,
		fields:          make([]*store.ByteBuffersDataOutput, 0, 16),
		scratchBytes:    store.NewByteBuffersDataOutput(),
	}

	termsName := GetSegmentFileName(segmentName, segmentSuffix, Lucene103BlockTreeTermsExtension)
	termsOut, err := directory.CreateOutput(termsName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", termsName, err)
	}
	success := false
	var indexOut, metaOut store.IndexOutput
	defer func() {
		if success {
			return
		}
		closeQuietly(metaOut, termsOut, indexOut)
	}()

	if err := WriteIndexHeader(termsOut, Lucene103BlockTreeTermsCodecName, version, segmentID, segmentSuffix); err != nil {
		return nil, fmt.Errorf("write terms header: %w", err)
	}

	indexName := GetSegmentFileName(segmentName, segmentSuffix, Lucene103BlockTreeTermsIndexExtension)
	indexOut, err = directory.CreateOutput(indexName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", indexName, err)
	}
	if err := WriteIndexHeader(indexOut, Lucene103BlockTreeTermsIndexCodecName, version, segmentID, segmentSuffix); err != nil {
		return nil, fmt.Errorf("write index header: %w", err)
	}

	metaName := GetSegmentFileName(segmentName, segmentSuffix, Lucene103BlockTreeTermsMetaExtension)
	metaOut, err = directory.CreateOutput(metaName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", metaName, err)
	}
	if err := WriteIndexHeader(metaOut, Lucene103BlockTreeTermsMetaCodecName, version, segmentID, segmentSuffix); err != nil {
		return nil, fmt.Errorf("write meta header: %w", err)
	}

	if err := postingsWriter.Init(metaOut, state); err != nil {
		return nil, fmt.Errorf("postings writer init: %w", err)
	}

	w.termsOut = termsOut
	w.indexOut = indexOut
	w.metaOut = metaOut
	success = true
	return w, nil
}

// ValidateLucene103BlockTreeBlockSizes mirrors
// {@code Lucene103BlockTreeTermsWriter.validateSettings(int, int)} in Java.
// It is exposed so callers (PostingsFormat shims) can pre-validate sizes.
func ValidateLucene103BlockTreeBlockSizes(minItemsInBlock, maxItemsInBlock int) error {
	if minItemsInBlock <= 1 {
		return fmt.Errorf("minItemsInBlock must be >= 2; got %d", minItemsInBlock)
	}
	if minItemsInBlock > maxItemsInBlock {
		return fmt.Errorf(
			"maxItemsInBlock must be >= minItemsInBlock; got maxItemsInBlock=%d minItemsInBlock=%d",
			maxItemsInBlock, minItemsInBlock,
		)
	}
	if 2*(minItemsInBlock-1) > maxItemsInBlock {
		return fmt.Errorf(
			"maxItemsInBlock must be at least 2*(minItemsInBlock-1); got maxItemsInBlock=%d minItemsInBlock=%d",
			maxItemsInBlock, minItemsInBlock,
		)
	}
	return nil
}

// WriteFields walks fields in the iterator's order and persists every
// non-nil Terms via the internal per-field termsWriter state machine.
// Mirrors {@code Lucene103BlockTreeTermsWriter.write(Fields, NormsProducer)}.
//
// The Java original asserts that field names arrive in ascending order; we
// surface the same condition as an explicit error so callers see the bug
// instead of getting silently-garbled output.
func (w *Lucene103BlockTreeTermsWriter) WriteFields(fields index.Fields, norms NormsProducer) error {
	if w.closed {
		return errors.New("Lucene103BlockTreeTermsWriter: WriteFields after Close")
	}
	if fields == nil {
		return nil
	}
	it, err := fields.Iterator()
	if err != nil {
		return err
	}
	lastField := ""
	first := true
	for {
		field, err := it.Next()
		if err != nil {
			return err
		}
		if field == "" {
			break
		}
		if !first && lastField >= field {
			return fmt.Errorf("WriteFields: fields must be visited in ascending order, got %q after %q", field, lastField)
		}
		lastField = field
		first = false

		terms, err := fields.Terms(field)
		if err != nil {
			return err
		}
		if terms == nil {
			continue
		}

		fieldInfo := w.fieldInfos.GetByName(field)
		if fieldInfo == nil {
			return fmt.Errorf("WriteFields: unknown field %q (not in FieldInfos)", field)
		}
		if err := w.writeField(fieldInfo, terms, norms); err != nil {
			return err
		}
	}
	return nil
}

// Write satisfies the existing per-field FieldsConsumer SPI. It is a thin
// convenience wrapper around WriteFields and exists only so existing
// callers in Gocene that drive the writer one field at a time keep
// compiling. New code should prefer WriteFields.
func (w *Lucene103BlockTreeTermsWriter) Write(field string, terms index.Terms) error {
	if w.closed {
		return errors.New("Lucene103BlockTreeTermsWriter: Write after Close")
	}
	fieldInfo := w.fieldInfos.GetByName(field)
	if fieldInfo == nil {
		return fmt.Errorf("Lucene103BlockTreeTermsWriter.Write: unknown field %q", field)
	}
	return w.writeField(fieldInfo, terms, nil)
}

// Close flushes the terms-meta footer and closes all three on-disk files
// plus the wrapped postings writer. Subsequent calls are no-ops.
// Mirrors {@code Lucene103BlockTreeTermsWriter.close()} in Java, including
// the closeWhileHandlingException fan-out on failure.
func (w *Lucene103BlockTreeTermsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	var firstErr error
	setErr := func(err error) {
		if firstErr == nil {
			firstErr = err
		}
	}

	success := false
	defer func() {
		if success {
			closeAll(setErr, w.metaOut, w.termsOut, w.indexOut, w.postingsWriter)
		} else {
			closeQuietly(w.metaOut, w.termsOut, w.indexOut, w.postingsWriter)
		}
	}()

	if err := store.WriteVInt(w.metaOut, int32(len(w.fields))); err != nil {
		setErr(err)
		return firstErr
	}
	for _, fieldMeta := range w.fields {
		if err := fieldMeta.CopyTo(w.metaOut); err != nil {
			setErr(err)
			return firstErr
		}
	}
	if err := WriteFooter(w.indexOut); err != nil {
		setErr(err)
		return firstErr
	}
	if err := w.metaOut.WriteLong(w.indexOut.GetFilePointer()); err != nil {
		setErr(err)
		return firstErr
	}
	if err := WriteFooter(w.termsOut); err != nil {
		setErr(err)
		return firstErr
	}
	if err := w.metaOut.WriteLong(w.termsOut.GetFilePointer()); err != nil {
		setErr(err)
		return firstErr
	}
	if err := WriteFooter(w.metaOut); err != nil {
		setErr(err)
		return firstErr
	}
	success = true
	return firstErr
}

// writeField drives a single field through a fresh per-field
// termsWriterState until every term has been visited and finish() has
// emitted the field metadata block.
func (w *Lucene103BlockTreeTermsWriter) writeField(fieldInfo *index.FieldInfo, terms index.Terms, norms NormsProducer) error {
	tw, err := newTermsWriterState(w, fieldInfo)
	if err != nil {
		return err
	}

	termsEnum, err := terms.GetIterator()
	if err != nil {
		return err
	}

	for {
		term, err := termsEnum.Next()
		if err != nil {
			return err
		}
		if term == nil {
			break
		}
		if err := tw.write(term, termsEnum, norms); err != nil {
			return err
		}
	}
	return tw.finish()
}

// pendingEntry is the in-memory union of "term" and "block" entries on the
// per-field pending stack. Java models this with a class hierarchy; in Go
// the discriminant is the boolean isTerm and the storage is two pointer
// fields, exactly one of which is non-nil. We keep both pointers (rather
// than an interface) so the hot loop avoids interface dispatch.
type pendingEntry struct {
	isTerm bool
	term   *pendingTerm
	block  *pendingBlock
}

// pendingTerm holds a buffered term and its postings-writer-produced
// metadata until the surrounding block is sealed.
type pendingTerm struct {
	termBytes []byte
	state     *BlockTermState
}

func newPendingTerm(term *index.Term, state *BlockTermState) *pendingTerm {
	ref := term.BytesValue()
	cp := make([]byte, ref.Length)
	copy(cp, ref.Bytes[ref.Offset:ref.Offset+ref.Length])
	return &pendingTerm{termBytes: cp, state: state}
}

// pendingBlock describes a finalised block that has already been flushed
// to the .tim output. It holds the prefix that drives the per-field trie,
// the file pointer where the block lives, and the floor sub-block list
// when the block is part of a floor split.
type pendingBlock struct {
	prefix        *util.BytesRef
	fp            int64
	index         *TrieBuilder
	subIndices    []*TrieBuilder
	hasTerms      bool
	isFloor       bool
	floorLeadByte int
}

// compileIndex builds the per-prefix sub-trie. The first block in blocks
// owns the trie, the rest contribute floor sub-block pointers (when isFloor
// is true) and sub-tries pulled up via TrieBuilder.Append.
//
// Mirrors {@code PendingBlock.compileIndex(List<PendingBlock>, ByteBuffersDataOutput)}.
func (b *pendingBlock) compileIndex(blocks []*pendingBlock, scratch *store.ByteBuffersDataOutput) error {
	if b.isFloor {
		if len(blocks) <= 1 {
			return fmt.Errorf("pendingBlock.compileIndex: floor block must have >1 sub-blocks, got %d", len(blocks))
		}
	} else if len(blocks) != 1 {
		return fmt.Errorf("pendingBlock.compileIndex: non-floor block must have exactly 1 entry, got %d", len(blocks))
	}
	if blocks[0] != b {
		return errors.New("pendingBlock.compileIndex: first block must be receiver")
	}
	if scratch.Size() != 0 {
		return errors.New("pendingBlock.compileIndex: scratch must be reset before invocation")
	}

	var floorData *util.BytesRef
	if b.isFloor {
		if err := scratch.WriteVInt(int32(len(blocks) - 1)); err != nil {
			return err
		}
		for i := 1; i < len(blocks); i++ {
			sub := blocks[i]
			if sub.floorLeadByte == -1 {
				return errors.New("pendingBlock.compileIndex: sub-block missing floorLeadByte")
			}
			if err := scratch.WriteByte(byte(sub.floorLeadByte)); err != nil {
				return err
			}
			if sub.fp <= b.fp {
				return fmt.Errorf("pendingBlock.compileIndex: sub.fp %d not strictly greater than parent fp %d", sub.fp, b.fp)
			}
			delta := (sub.fp - b.fp) << 1
			if sub.hasTerms {
				delta |= 1
			}
			if err := scratch.WriteVLong(delta); err != nil {
				return err
			}
		}
		floorData = util.NewBytesRef(scratch.ToArrayCopy())
	}

	output := NewTrieOutput(b.fp, b.hasTerms, floorData)
	trie := BytesRefToTrie(b.prefix, output)
	scratch.Reset()

	// Pull sub-block tries into the parent's trie.
	for _, block := range blocks {
		if block.subIndices == nil {
			continue
		}
		for _, sub := range block.subIndices {
			if err := trie.Append(sub); err != nil {
				return fmt.Errorf("pendingBlock.compileIndex: merge sub-trie: %w", err)
			}
		}
		block.subIndices = nil
	}

	b.index = trie
	if b.subIndices != nil {
		return errors.New("pendingBlock.compileIndex: subIndices must be nil after compile")
	}
	return nil
}

// statsWriter encodes a run of (DocFreq, TotalTermFreq) pairs, using
// run-length compression for the common case of "singleton" terms
// (DF==1 and, when frequencies are tracked, TTF==1). Mirrors
// {@code Lucene103BlockTreeTermsWriter.StatsWriter}.
type statsWriter struct {
	out            store.DataOutput
	hasFreqs       bool
	singletonCount int
}

func newStatsWriter(out store.DataOutput, hasFreqs bool) *statsWriter {
	return &statsWriter{out: out, hasFreqs: hasFreqs}
}

func (s *statsWriter) add(df int, ttf int64) error {
	if df == 1 && (!s.hasFreqs || ttf == 1) {
		s.singletonCount++
		return nil
	}
	if err := s.finish(); err != nil {
		return err
	}
	if err := store.WriteVInt(s.out, int32(df<<1)); err != nil {
		return err
	}
	if s.hasFreqs {
		if err := store.WriteVLong(s.out, ttf-int64(df)); err != nil {
			return err
		}
	}
	return nil
}

func (s *statsWriter) finish() error {
	if s.singletonCount <= 0 {
		return nil
	}
	if err := store.WriteVInt(s.out, int32(((s.singletonCount-1)<<1)|1)); err != nil {
		return err
	}
	s.singletonCount = 0
	return nil
}

// termsWriterState is the per-field driver. Each call to write() pushes
// one term onto a stack of (terms, sub-blocks), occasionally folding the
// top of the stack into a new pendingBlock when enough entries share a
// common prefix. finish() drains the stack and emits the field's metadata.
//
// Mirrors {@code Lucene103BlockTreeTermsWriter.TermsWriter}.
type termsWriterState struct {
	parent           *Lucene103BlockTreeTermsWriter
	fieldInfo        *index.FieldInfo
	docsSeen         *util.FixedBitSet
	numTerms         int64
	sumTotalTermFreq int64
	sumDocFreq       int64

	lastTerm     *util.BytesRefBuilder
	prefixStarts []int
	pending      []*pendingEntry
	newBlocks    []*pendingBlock

	firstPendingTerm *pendingTerm
	lastPendingTerm  *pendingTerm

	// Per-write reusable buffers. ByteBuffersDataOutput is allocation
	// friendly when reset between blocks; matches Java exactly.
	suffixLengthsWriter *store.ByteBuffersDataOutput
	suffixWriter        *util.BytesRefBuilder
	statsWriter         *store.ByteBuffersDataOutput
	metaWriter          *store.ByteBuffersDataOutput
	spareWriter         *store.ByteBuffersDataOutput
	spareBytes          []byte
}

func newTermsWriterState(parent *Lucene103BlockTreeTermsWriter, fieldInfo *index.FieldInfo) (*termsWriterState, error) {
	if fieldInfo.IndexOptions() == index.IndexOptionsNone {
		return nil, fmt.Errorf("newTermsWriterState: field %q indexOptions == NONE", fieldInfo.Name())
	}
	docsSeen, err := util.NewFixedBitSet(parent.maxDoc)
	if err != nil {
		return nil, fmt.Errorf("newTermsWriterState: %w", err)
	}
	tw := &termsWriterState{
		parent:              parent,
		fieldInfo:           fieldInfo,
		docsSeen:            docsSeen,
		lastTerm:            util.NewBytesRefBuilder(),
		prefixStarts:        make([]int, 8),
		pending:             make([]*pendingEntry, 0, 64),
		newBlocks:           make([]*pendingBlock, 0, 16),
		suffixLengthsWriter: store.NewByteBuffersDataOutput(),
		suffixWriter:        util.NewBytesRefBuilder(),
		statsWriter:         store.NewByteBuffersDataOutput(),
		metaWriter:          store.NewByteBuffersDataOutput(),
		spareWriter:         store.NewByteBuffersDataOutput(),
		spareBytes:          nil,
	}
	if _, err := parent.postingsWriter.SetField(fieldInfo); err != nil {
		return nil, fmt.Errorf("postingsWriter.SetField: %w", err)
	}
	return tw, nil
}

// pushSinglePostings drives the underlying PostingsWriterBase for a single
// term, returning the populated BlockTermState. Returns nil when the term
// has no surviving documents (mirrors Java's null return from
// PostingsWriterBase.writeTerm).
//
// The Java helper also accepts a NormsProducer and threads it into
// StartTerm; without a fully-ported norms layer we approximate by passing
// nil for now and rely on the postings writer's own omit-norms branch.
func (t *termsWriterState) pushSinglePostings(termText *index.Term, termsEnum index.TermsEnum, norms NormsProducer) (*BlockTermState, error) {
	state := t.parent.postingsWriter.NewTermState()

	// StartTerm: norms are wired in when the index has them. The Java
	// writer fetches them via NormsProducer.getNorms(fieldInfo); for now
	// we pass nil so DOCS-only / DOCS_AND_FREQS_* fields work end-to-end.
	if err := t.parent.postingsWriter.StartTerm(nil); err != nil {
		return nil, err
	}
	_ = norms

	postingsEnum, err := termsEnum.Postings(0)
	if err != nil {
		return nil, err
	}
	if postingsEnum == nil {
		return nil, nil
	}

	// We need the PushPostingsWriterBase subset for the doc-by-doc push.
	pusher, ok := t.parent.postingsWriter.(PushPostingsWriterBase)
	if !ok {
		return nil, fmt.Errorf("Lucene103BlockTreeTermsWriter: postingsWriter %T does not implement PushPostingsWriterBase", t.parent.postingsWriter)
	}
	hasPositions := t.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
	hasOffsets := t.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	hasPayloads := t.fieldInfo.HasPayloads()

	docCount, err := WriteTerm(pusher, postingsEnum, hasPositions, hasOffsets, hasPayloads)
	if err != nil {
		return nil, err
	}
	if docCount == 0 {
		return nil, nil
	}

	state.DocFreq = docCount
	// totalTermFreq is filled by the postings writer's FinishTerm; the
	// caller already populated docFreq from the postings stream.
	if err := t.parent.postingsWriter.FinishTerm(state); err != nil {
		return nil, err
	}
	if hasPositions && state.TotalTermFreq < int64(state.DocFreq) {
		return nil, fmt.Errorf("Lucene103BlockTreeTermsWriter: term has positions but totalTermFreq (%d) < docFreq (%d)", state.TotalTermFreq, state.DocFreq)
	}
	return state, nil
}

// write pushes one term onto the per-field pending stack and possibly
// flushes the top of the stack into a new block when its prefix has been
// closed. Mirrors {@code TermsWriter.write(BytesRef, TermsEnum, NormsProducer)}.
func (t *termsWriterState) write(term *index.Term, termsEnum index.TermsEnum, norms NormsProducer) error {
	state, err := t.pushSinglePostings(term, termsEnum, norms)
	if err != nil {
		return err
	}
	if state == nil {
		return nil
	}
	if state.DocFreq == 0 {
		return errors.New("termsWriterState.write: postings writer returned BlockTermState with docFreq == 0")
	}
	if t.fieldInfo.IndexOptions() != index.IndexOptionsDocs && state.TotalTermFreq < int64(state.DocFreq) {
		return fmt.Errorf("termsWriterState.write: totalTermFreq %d < docFreq %d", state.TotalTermFreq, state.DocFreq)
	}

	textBytes := term.BytesValue()
	if err := t.pushTerm(textBytes); err != nil {
		return err
	}

	pt := newPendingTerm(term, state)
	t.pending = append(t.pending, &pendingEntry{isTerm: true, term: pt})

	t.sumDocFreq += int64(state.DocFreq)
	t.sumTotalTermFreq += state.TotalTermFreq
	t.numTerms++
	if t.firstPendingTerm == nil {
		t.firstPendingTerm = pt
	}
	t.lastPendingTerm = pt
	return nil
}

// pushTerm closes any prefix the new term no longer shares with the
// previous term, possibly emitting one or more blocks per closed level,
// then primes prefixStarts for the new tail. Mirrors
// {@code TermsWriter.pushTerm(BytesRef)}.
func (t *termsWriterState) pushTerm(text *util.BytesRef) error {
	last := t.lastTerm.Get()
	prefixLen := computePrefixMismatch(
		last.Bytes[last.Offset:last.Offset+last.Length],
		text.Bytes[text.Offset:text.Offset+text.Length],
	)
	if prefixLen == -1 {
		if t.lastTerm.Length() != 0 {
			return errors.New("termsWriterState.pushTerm: mismatch=-1 only valid for first/empty lastTerm")
		}
		prefixLen = 0
	}

	for i := t.lastTerm.Length() - 1; i >= prefixLen; i-- {
		prefixTopSize := len(t.pending) - t.prefixStarts[i]
		if prefixTopSize >= t.parent.minItemsInBlock {
			if err := t.writeBlocks(i+1, prefixTopSize); err != nil {
				return err
			}
			// writeBlocks shrank pending by (prefixTopSize-1).
			t.prefixStarts[i] -= prefixTopSize - 1
		}
	}

	if len(t.prefixStarts) < text.Length {
		grown := make([]int, maxInt(text.Length, 2*len(t.prefixStarts)))
		copy(grown, t.prefixStarts)
		t.prefixStarts = grown
	}
	for i := prefixLen; i < text.Length; i++ {
		t.prefixStarts[i] = len(t.pending)
	}

	t.lastTerm.CopyBytes(text.Bytes, text.Offset, text.Length)
	return nil
}

// finish drains the pending stack, emits the root block, and appends the
// field metadata blob to parent.fields. Mirrors
// {@code TermsWriter.finish()}.
func (t *termsWriterState) finish() error {
	if t.numTerms == 0 {
		if t.sumTotalTermFreq != 0 && !(t.fieldInfo.IndexOptions() == index.IndexOptionsDocs && t.sumTotalTermFreq == -1) {
			return fmt.Errorf("termsWriterState.finish: sumTotalTermFreq %d invalid for empty field", t.sumTotalTermFreq)
		}
		if t.sumDocFreq != 0 {
			return fmt.Errorf("termsWriterState.finish: sumDocFreq %d invalid for empty field", t.sumDocFreq)
		}
		if t.docsSeen.Cardinality() != 0 {
			return fmt.Errorf("termsWriterState.finish: docsSeen cardinality %d invalid for empty field", t.docsSeen.Cardinality())
		}
		return nil
	}

	// Append empty term twice to force closing every dangling prefix,
	// then writeBlocks(0, pending.size()) collapses the stack into a
	// single root block. Mirrors the Java pattern verbatim.
	empty := util.NewBytesRefEmpty()
	if err := t.pushTerm(empty); err != nil {
		return err
	}
	if err := t.pushTerm(empty); err != nil {
		return err
	}
	if err := t.writeBlocks(0, len(t.pending)); err != nil {
		return err
	}
	if len(t.pending) != 1 || t.pending[0].isTerm {
		return fmt.Errorf("termsWriterState.finish: expected single root block, got %d entries", len(t.pending))
	}
	root := t.pending[0].block
	if root.prefix.Length != 0 {
		return errors.New("termsWriterState.finish: root block must have empty prefix")
	}
	if root.index == nil || root.index.GetEmptyOutput() == nil {
		return errors.New("termsWriterState.finish: root block trie missing empty output")
	}

	fieldMeta := store.NewByteBuffersDataOutput()
	t.parent.fields = append(t.parent.fields, fieldMeta)

	if err := fieldMeta.WriteVInt(int32(t.fieldInfo.Number())); err != nil {
		return err
	}
	if err := fieldMeta.WriteVLong(t.numTerms); err != nil {
		return err
	}
	if t.fieldInfo.IndexOptions() != index.IndexOptionsDocs {
		if err := fieldMeta.WriteVLong(t.sumTotalTermFreq); err != nil {
			return err
		}
	}
	if err := fieldMeta.WriteVLong(t.sumDocFreq); err != nil {
		return err
	}
	if err := fieldMeta.WriteVInt(int32(t.docsSeen.Cardinality())); err != nil {
		return err
	}
	if err := writeBytesRefVInt(fieldMeta, t.firstPendingTerm.termBytes); err != nil {
		return err
	}
	if err := writeBytesRefVInt(fieldMeta, t.lastPendingTerm.termBytes); err != nil {
		return err
	}
	if err := root.index.Save(fieldMeta, t.parent.indexOut); err != nil {
		return fmt.Errorf("save root trie: %w", err)
	}
	return nil
}

// writeBlocks takes the top `count` entries off pending and groups them
// into one or more on-disk blocks; floor splitting happens when a single
// shared-prefix group exceeds maxItemsInBlock. Mirrors
// {@code TermsWriter.writeBlocks(int, int)}.
func (t *termsWriterState) writeBlocks(prefixLength, count int) error {
	if count <= 0 {
		return fmt.Errorf("termsWriterState.writeBlocks: count must be > 0, got %d", count)
	}
	if prefixLength <= 0 && count != len(t.pending) {
		return errors.New("termsWriterState.writeBlocks: root block must consume entire pending stack")
	}

	lastSuffixLeadLabel := -1
	hasTerms := false
	hasSubBlocks := false

	start := len(t.pending) - count
	end := len(t.pending)
	nextBlockStart := start
	nextFloorLeadLabel := -1

	for i := start; i < end; i++ {
		ent := t.pending[i]
		var suffixLeadLabel int
		if ent.isTerm {
			if len(ent.term.termBytes) == prefixLength {
				if lastSuffixLeadLabel != -1 {
					return fmt.Errorf("termsWriterState.writeBlocks: lastSuffixLeadLabel %d != -1 for zero-suffix term", lastSuffixLeadLabel)
				}
				suffixLeadLabel = -1
			} else {
				suffixLeadLabel = int(ent.term.termBytes[prefixLength]) & 0xFF
			}
		} else {
			block := ent.block
			if block.prefix.Length <= prefixLength {
				return fmt.Errorf("termsWriterState.writeBlocks: sub-block prefix length %d <= prefixLength %d", block.prefix.Length, prefixLength)
			}
			suffixLeadLabel = int(block.prefix.Bytes[block.prefix.Offset+prefixLength]) & 0xFF
		}

		if suffixLeadLabel != lastSuffixLeadLabel {
			itemsInBlock := i - nextBlockStart
			if itemsInBlock >= t.parent.minItemsInBlock && (end-nextBlockStart) > t.parent.maxItemsInBlock {
				isFloor := itemsInBlock < count
				block, err := t.writeBlock(prefixLength, isFloor, nextFloorLeadLabel, nextBlockStart, i, hasTerms, hasSubBlocks)
				if err != nil {
					return err
				}
				t.newBlocks = append(t.newBlocks, block)
				hasTerms = false
				hasSubBlocks = false
				nextFloorLeadLabel = suffixLeadLabel
				nextBlockStart = i
			}
			lastSuffixLeadLabel = suffixLeadLabel
		}

		if ent.isTerm {
			hasTerms = true
		} else {
			hasSubBlocks = true
		}
	}

	if nextBlockStart < end {
		itemsInBlock := end - nextBlockStart
		isFloor := itemsInBlock < count
		block, err := t.writeBlock(prefixLength, isFloor, nextFloorLeadLabel, nextBlockStart, end, hasTerms, hasSubBlocks)
		if err != nil {
			return err
		}
		t.newBlocks = append(t.newBlocks, block)
	}
	if len(t.newBlocks) == 0 {
		return errors.New("termsWriterState.writeBlocks: produced zero blocks")
	}

	first := t.newBlocks[0]
	if first.isFloor != (len(t.newBlocks) > 1) {
		return fmt.Errorf("termsWriterState.writeBlocks: isFloor=%v but newBlocks.size()=%d", first.isFloor, len(t.newBlocks))
	}
	if err := first.compileIndex(t.newBlocks, t.parent.scratchBytes); err != nil {
		return err
	}

	// Replace consumed entries with the new first block.
	t.pending = t.pending[:len(t.pending)-count]
	t.pending = append(t.pending, &pendingEntry{block: first})
	t.newBlocks = t.newBlocks[:0]
	return nil
}

// writeBlock emits one block (terms-only "leaf" or mixed "internal") to
// the .tim output and returns a pendingBlock describing it. Mirrors
// {@code TermsWriter.writeBlock(int, boolean, int, int, int, boolean, boolean)}.
func (t *termsWriterState) writeBlock(prefixLength int, isFloor bool, floorLeadLabel, start, end int, hasTerms, hasSubBlocks bool) (*pendingBlock, error) {
	if end <= start {
		return nil, fmt.Errorf("writeBlock: end %d <= start %d", end, start)
	}
	startFP := t.parent.termsOut.GetFilePointer()
	hasFloorLeadLabel := isFloor && floorLeadLabel != -1

	prefixCap := prefixLength
	if hasFloorLeadLabel {
		prefixCap++
	}
	prefixBytes := make([]byte, prefixLength, prefixCap)
	if prefixLength > 0 {
		lastBytes := t.lastTerm.Bytes()
		copy(prefixBytes, lastBytes[:prefixLength])
	}
	prefix := &util.BytesRef{Bytes: prefixBytes, Offset: 0, Length: prefixLength}

	// Block header: vInt(numEntries<<1 | isLastBlock).
	numEntries := end - start
	code := numEntries << 1
	if end == len(t.pending) {
		code |= 1
	}
	if err := store.WriteVInt(t.parent.termsOut, int32(code)); err != nil {
		return nil, err
	}

	isLeafBlock := !hasSubBlocks
	var subIndices []*TrieBuilder

	stats := newStatsWriter(t.statsWriter, t.fieldInfo.IndexOptions() != index.IndexOptionsDocs)
	absolute := true

	if isLeafBlock {
		// Leaf blocks contain only terms; suffixLengthsWriter stores
		// vInt(suffix), suffixWriter stores the suffix bytes, statsWriter
		// stores the (df, ttf) stream, metaWriter stores the postings
		// metadata for each term.
		for i := start; i < end; i++ {
			ent := t.pending[i]
			if !ent.isTerm {
				return nil, fmt.Errorf("writeBlock: leaf block expects all terms, got block at index %d", i)
			}
			term := ent.term
			if !bytesHasPrefix(term.termBytes, prefixBytes) {
				return nil, fmt.Errorf("writeBlock: term %q lacks expected prefix", term.termBytes)
			}
			suffix := len(term.termBytes) - prefixLength

			if err := t.suffixLengthsWriter.WriteVInt(int32(suffix)); err != nil {
				return nil, err
			}
			t.suffixWriter.AppendBytes(term.termBytes, prefixLength, suffix)

			if floorLeadLabel != -1 && int(term.termBytes[prefixLength])&0xFF < floorLeadLabel {
				return nil, fmt.Errorf("writeBlock: term lead byte 0x%02x < floorLeadLabel 0x%02x", int(term.termBytes[prefixLength])&0xFF, floorLeadLabel)
			}

			if err := stats.add(term.state.DocFreq, term.state.TotalTermFreq); err != nil {
				return nil, err
			}
			if err := t.parent.postingsWriter.EncodeTerm(byteBuffersDataOutputAsIndexOutput{t.metaWriter}, t.fieldInfo, term.state, absolute); err != nil {
				return nil, err
			}
			absolute = false
		}
		if err := stats.finish(); err != nil {
			return nil, err
		}
	} else {
		subIndices = make([]*TrieBuilder, 0, 8)
		for i := start; i < end; i++ {
			ent := t.pending[i]
			if ent.isTerm {
				term := ent.term
				if !bytesHasPrefix(term.termBytes, prefixBytes) {
					return nil, fmt.Errorf("writeBlock: non-leaf term %q lacks expected prefix", term.termBytes)
				}
				suffix := len(term.termBytes) - prefixLength
				if err := t.suffixLengthsWriter.WriteVInt(int32(suffix << 1)); err != nil {
					return nil, err
				}
				t.suffixWriter.AppendBytes(term.termBytes, prefixLength, suffix)
				if err := stats.add(term.state.DocFreq, term.state.TotalTermFreq); err != nil {
					return nil, err
				}
				if err := t.parent.postingsWriter.EncodeTerm(byteBuffersDataOutputAsIndexOutput{t.metaWriter}, t.fieldInfo, term.state, absolute); err != nil {
					return nil, err
				}
				absolute = false
			} else {
				block := ent.block
				if !bytesHasPrefix(
					block.prefix.Bytes[block.prefix.Offset:block.prefix.Offset+block.prefix.Length],
					prefixBytes,
				) {
					return nil, fmt.Errorf("writeBlock: sub-block prefix lacks expected leading bytes")
				}
				suffix := block.prefix.Length - prefixLength
				if suffix <= 0 {
					return nil, fmt.Errorf("writeBlock: sub-block suffix length %d must be > 0", suffix)
				}
				if err := t.suffixLengthsWriter.WriteVInt(int32((suffix << 1) | 1)); err != nil {
					return nil, err
				}
				t.suffixWriter.AppendBytes(block.prefix.Bytes, block.prefix.Offset+prefixLength, suffix)
				if floorLeadLabel != -1 && int(block.prefix.Bytes[block.prefix.Offset+prefixLength])&0xFF < floorLeadLabel {
					return nil, fmt.Errorf("writeBlock: sub-block lead byte < floorLeadLabel")
				}
				if block.fp >= startFP {
					return nil, fmt.Errorf("writeBlock: sub-block fp %d must be < startFP %d", block.fp, startFP)
				}
				if err := t.suffixLengthsWriter.WriteVLong(startFP - block.fp); err != nil {
					return nil, err
				}
				subIndices = append(subIndices, block.index)
			}
		}
		if err := stats.finish(); err != nil {
			return nil, err
		}
		if len(subIndices) == 0 {
			return nil, errors.New("writeBlock: mixed block produced zero sub-indices")
		}
	}

	// Compression: try LZ4 first if the suffix corpus looks worthwhile,
	// then LowercaseAsciiCompression as a fallback. Until those helpers
	// are wired here we always emit NO_COMPRESSION, which is byte-format
	// compatible (the algorithm code is part of the wire format and is
	// validated at read time by the Lucene reader).
	compressionAlg := CompressionNoCompression
	suffixLen := t.suffixWriter.Length()
	token := int64(suffixLen) << 3
	if isLeafBlock {
		token |= 0x04
	}
	token |= int64(compressionAlg.Code())
	if err := store.WriteVLong(t.parent.termsOut, token); err != nil {
		return nil, err
	}
	if err := t.parent.termsOut.WriteBytesN(t.suffixWriter.Bytes()[:suffixLen], suffixLen); err != nil {
		return nil, err
	}
	t.suffixWriter.SetLength(0)
	t.spareWriter.Reset()

	// Suffix lengths blob. If every length is identical we collapse it
	// to a single byte using the low bit of the leading vInt.
	numSuffixBytes := int(t.suffixLengthsWriter.Size())
	if cap(t.spareBytes) < numSuffixBytes {
		t.spareBytes = make([]byte, numSuffixBytes)
	} else {
		t.spareBytes = t.spareBytes[:numSuffixBytes]
	}
	if numSuffixBytes > 0 {
		// Drain the suffix-lengths buffer through a ByteArrayDataOutput
		// so we can inspect the bytes and decide on the all-equal path.
		tmp := store.NewByteArrayDataOutput(numSuffixBytes)
		if err := t.suffixLengthsWriter.CopyTo(tmp); err != nil {
			return nil, err
		}
		copy(t.spareBytes, tmp.GetBytes()[:numSuffixBytes])
	}
	t.suffixLengthsWriter.Reset()
	if numSuffixBytes > 0 && bytesAllEqual(t.spareBytes[1:numSuffixBytes], t.spareBytes[0]) {
		if err := store.WriteVInt(t.parent.termsOut, int32((numSuffixBytes<<1)|1)); err != nil {
			return nil, err
		}
		if err := t.parent.termsOut.WriteByte(t.spareBytes[0]); err != nil {
			return nil, err
		}
	} else {
		if err := store.WriteVInt(t.parent.termsOut, int32(numSuffixBytes<<1)); err != nil {
			return nil, err
		}
		if numSuffixBytes > 0 {
			if err := t.parent.termsOut.WriteBytesN(t.spareBytes[:numSuffixBytes], numSuffixBytes); err != nil {
				return nil, err
			}
		}
	}

	// Stats blob.
	numStatsBytes := int(t.statsWriter.Size())
	if err := store.WriteVInt(t.parent.termsOut, int32(numStatsBytes)); err != nil {
		return nil, err
	}
	if numStatsBytes > 0 {
		if err := t.statsWriter.CopyTo(t.parent.termsOut); err != nil {
			return nil, err
		}
	}
	t.statsWriter.Reset()

	// Term metadata blob (PostingsWriterBase output).
	numMetaBytes := int(t.metaWriter.Size())
	if err := store.WriteVInt(t.parent.termsOut, int32(numMetaBytes)); err != nil {
		return nil, err
	}
	if numMetaBytes > 0 {
		if err := t.metaWriter.CopyTo(t.parent.termsOut); err != nil {
			return nil, err
		}
	}
	t.metaWriter.Reset()

	if hasFloorLeadLabel {
		// Reuse the cap reserved above.
		prefix.Bytes = append(prefix.Bytes, byte(floorLeadLabel))
		prefix.Length++
	}

	return &pendingBlock{
		prefix:        prefix,
		fp:            startFP,
		hasTerms:      hasTerms,
		isFloor:       isFloor,
		floorLeadByte: floorLeadLabel,
		subIndices:    subIndices,
	}, nil
}

// writeBytesRefVInt emits vInt(len) followed by the raw bytes; mirrors
// the private writeBytesRef helper in the Java writer.
func writeBytesRefVInt(out store.DataOutput, b []byte) error {
	if err := store.WriteVInt(out, int32(len(b))); err != nil {
		return err
	}
	if len(b) == 0 {
		return nil
	}
	return out.WriteBytesN(b, len(b))
}

// computePrefixMismatch returns the index of the first differing byte
// between a and b, or -1 when one is a prefix of the other. Mirrors
// Java's Arrays.mismatch(byte[], int, int, byte[], int, int).
func computePrefixMismatch(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) == len(b) {
		return -1
	}
	return n
}

func bytesHasPrefix(b, prefix []byte) bool {
	if len(b) < len(prefix) {
		return false
	}
	return bytes.Equal(b[:len(prefix)], prefix)
}

func bytesAllEqual(b []byte, v byte) bool {
	for _, x := range b {
		if x != v {
			return false
		}
	}
	return true
}

// maxInt is the local int max; the codecs package already declares max()
// in pfor_util.go, so we use a distinct name to avoid the redeclaration.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// byteBuffersDataOutputAsIndexOutput is a thin adapter that lets the
// PostingsWriterBase.EncodeTerm hook (which insists on store.IndexOutput
// for symmetry with the Java signature) accept a ByteBuffersDataOutput.
// Only DataOutput methods are forwarded; RandomAccess / file-pointer
// methods are stubbed because EncodeTerm is supposed to be a streaming
// writer and the upstream caller drains the buffer separately.
type byteBuffersDataOutputAsIndexOutput struct {
	inner *store.ByteBuffersDataOutput
}

var _ store.IndexOutput = byteBuffersDataOutputAsIndexOutput{}

func (a byteBuffersDataOutputAsIndexOutput) WriteByte(b byte) error    { return a.inner.WriteByte(b) }
func (a byteBuffersDataOutputAsIndexOutput) WriteBytes(b []byte) error { return a.inner.WriteBytes(b) }
func (a byteBuffersDataOutputAsIndexOutput) WriteBytesN(b []byte, n int) error {
	return a.inner.WriteBytesN(b, n)
}
func (a byteBuffersDataOutputAsIndexOutput) WriteShort(v int16) error { return a.inner.WriteShort(v) }
func (a byteBuffersDataOutputAsIndexOutput) WriteInt(v int32) error   { return a.inner.WriteInt(v) }
func (a byteBuffersDataOutputAsIndexOutput) WriteLong(v int64) error  { return a.inner.WriteLong(v) }
func (a byteBuffersDataOutputAsIndexOutput) WriteString(s string) error {
	return a.inner.WriteString(s)
}
func (a byteBuffersDataOutputAsIndexOutput) GetName() string       { return "ByteBuffersDataOutput" }
func (a byteBuffersDataOutputAsIndexOutput) GetFilePointer() int64 { return a.inner.Size() }
func (a byteBuffersDataOutputAsIndexOutput) SetPosition(pos int64) error {
	// PostingsWriterBase.EncodeTerm is a forward-only writer; positional
	// rewinds are not part of the protocol. Refuse them rather than
	// silently producing a corrupt term blob.
	return fmt.Errorf("byteBuffersDataOutputAsIndexOutput.SetPosition(%d): adapter does not support seek", pos)
}
func (a byteBuffersDataOutputAsIndexOutput) Length() int64 { return a.inner.Size() }
func (a byteBuffersDataOutputAsIndexOutput) Close() error  { return nil }

// closeQuietly closes every Closer in order, swallowing every error. Used
// on the unwind path mirroring IOUtils.closeWhileHandlingException.
func closeQuietly(closers ...closer) {
	for _, c := range closers {
		if c == nil {
			continue
		}
		_ = c.Close()
	}
}

// closeAll closes every Closer in order and forwards the first error to
// setErr (idempotent on subsequent invocations). Mirrors IOUtils.close.
func closeAll(setErr func(error), closers ...closer) {
	for _, c := range closers {
		if c == nil {
			continue
		}
		if err := c.Close(); err != nil {
			setErr(err)
		}
	}
}

type closer interface {
	Close() error
}

// Ensure Lucene103BlockTreeTermsWriter satisfies the existing per-field
// FieldsConsumer SPI so it can be slotted into PostingsFormat shims.
var _ FieldsConsumer = (*Lucene103BlockTreeTermsWriter)(nil)
