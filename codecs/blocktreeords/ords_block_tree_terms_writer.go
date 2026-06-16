// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package blocktreeords

import (
	"bytes"
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// Reference: lucene/codecs/src/java/org/apache/lucene/codecs/blocktreeords/
// OrdsBlockTreeTermsWriter.java (Apache Lucene 10.4.0).
//
// This writer ports the "BlockTreeOrds" postings-format term dictionary,
// augmenting the standard BlockTree terms dict with per-term ordinal
// support. Each term receives a dense ordinal [0, numTerms), and the FST
// index maps prefixes to ordinal ranges instead of raw file pointers.
//
// Key differences from Lucene103BlockTreeTermsWriter:
//   - File extensions: .tio (terms) and .tipo (index); no .tmd.
//   - Metadata is written inline at the end of .tio (no separate meta file).
//   - Uses FST[*FSTOrdsOutput] for the index instead of TrieBuilder.
//   - encodeOutput packs (fp << 2) | flag bits (hasTerms, isFloor).
//   - Stats are written directly (no singleton run-length compression).
//   - No LZ4 / LowercaseAscii compression or all-equal suffix optimization.
//   - Sub-blocks encode totFloorTermCount after the FP delta.
//
// The writer is single-threaded; callers must serialise Write and Close
// invocations themselves.

// ---- constants ----

// ordsBlockTreeDefaultMinBlockSize mirrors Java's DEFAULT_MIN_BLOCK_SIZE = 25.
const ordsBlockTreeDefaultMinBlockSize = 25

// ordsBlockTreeDefaultMaxBlockSize mirrors Java's DEFAULT_MAX_BLOCK_SIZE = 48.
const ordsBlockTreeDefaultMaxBlockSize = 48

// ---- helper: encodeOutput ----

// encodeOutput packs (fp << OUTPUT_FLAGS_NUM_BITS) | flags. Mirrors the
// Java static method OrdsBlockTreeTermsWriter.encodeOutput.
func encodeOutput(fp int64, hasTerms, isFloor bool) int64 {
	out := fp << outputFlagsNumBits
	if hasTerms {
		out |= outputFlagHasTerms
	}
	if isFloor {
		out |= outputFlagIsFloor
	}
	return out
}

// ---- pending-entry types ----

// ordsPendingEntry is the in-memory union of "term" and "block" entries on
// the per-field pending stack. The discriminant is the boolean isTerm.
type ordsPendingEntry struct {
	isTerm bool
	term   *ordsPendingTerm
	block  *ordsPendingBlock
}

// ordsPendingTerm holds a buffered term and its postings-writer metadata.
type ordsPendingTerm struct {
	termBytes []byte
	state     *codecs.BlockTermState
}

func newOrdsPendingTerm(term *index.Term, state *codecs.BlockTermState) *ordsPendingTerm {
	ref := term.BytesValue()
	cp := make([]byte, ref.Length)
	copy(cp, ref.Bytes[ref.Offset:ref.Offset+ref.Length])
	return &ordsPendingTerm{termBytes: cp, state: state}
}

// ordsSubIndex pairs a sub-block FST with the ordinal offset at which the
// sub-block's first term appears within the enclosing block.
type ordsSubIndex struct {
	index        *gfst.FST[*FSTOrdsOutput]
	termOrdStart int64
}

// ordsPendingBlock describes a finalised block that has already been flushed
// to the .tio output. It holds the prefix, the file pointer where the block
// lives, the per-block FST index, floor metadata, and cumulative term counts.
type ordsPendingBlock struct {
	prefix        *util.BytesRef
	fp            int64
	index         *gfst.FST[*FSTOrdsOutput]
	subIndices    []*ordsSubIndex
	hasTerms      bool
	isFloor       bool
	floorLeadByte int
	totalTermCount    int64 // number of terms in THIS block (not including siblings in a floor group)
	totFloorTermCount int64 // total terms across all blocks in the floor group (set in compileIndex)
}

// compileIndex builds the per-prefix FST (with FSTOrdsOutput values) for
// this block and its floor siblings.
//
// Mirrors OrdsBlockTreeTermsWriter.PendingBlock.compileIndex.
func (b *ordsPendingBlock) compileIndex(blocks []*ordsPendingBlock, scratchBytes *store.ByteBuffersDataOutput, scratchIntsRef *util.IntsRefBuilder) error {
	if b.isFloor && len(blocks) <= 1 {
		return fmt.Errorf("ordsPendingBlock.compileIndex: floor block must have >1 sub-blocks, got %d", len(blocks))
	}
	if !b.isFloor && len(blocks) != 1 {
		return fmt.Errorf("ordsPendingBlock.compileIndex: non-floor block must have exactly 1 entry, got %d", len(blocks))
	}
	if blocks[0] != b {
		return errors.New("ordsPendingBlock.compileIndex: first block must be receiver")
	}
	if scratchBytes.Size() != 0 {
		return errors.New("ordsPendingBlock.compileIndex: scratch must be reset before invocation")
	}

	lastSumTotalTermCount := int64(0)
	sumTotalTermCount := b.totalTermCount

	if err := scratchBytes.WriteVLong(encodeOutput(b.fp, b.hasTerms, b.isFloor)); err != nil {
		return err
	}

	if b.isFloor {
		if err := scratchBytes.WriteVInt(int32(len(blocks) - 1)); err != nil {
			return err
		}
		for i := 1; i < len(blocks); i++ {
			sub := blocks[i]
			if sub.floorLeadByte == -1 {
				return errors.New("ordsPendingBlock.compileIndex: sub-block missing floorLeadByte")
			}
			if err := scratchBytes.WriteByte(byte(sub.floorLeadByte)); err != nil {
				return err
			}
			// Write ord offset delta.
			if err := scratchBytes.WriteVLong(sumTotalTermCount - lastSumTotalTermCount); err != nil {
				return err
			}
			lastSumTotalTermCount = sumTotalTermCount
			sumTotalTermCount += sub.totalTermCount
			if sub.fp <= b.fp {
				return fmt.Errorf("ordsPendingBlock.compileIndex: sub.fp %d not > parent fp %d", sub.fp, b.fp)
			}
			// Write fp delta with hasTerms flag in bit 0.
			delta := (sub.fp - b.fp) << 1
			if sub.hasTerms {
				delta |= 1
			}
			if err := scratchBytes.WriteVLong(delta); err != nil {
				return err
			}
		}
	}

	// Build the FST for this block level.
	compiler := gfst.NewFSTCompilerBuilder[*FSTOrdsOutput](
		gfst.InputTypeByte1, fstOrdsOutputsSingleton,
	).Build()

	payload := scratchBytes.ToArrayCopy()
	if len(payload) == 0 {
		return errors.New("ordsPendingBlock.compileIndex: empty scratchBytes payload")
	}

	if err := compiler.Add(
		gfst.ToIntsRef(b.prefix, scratchIntsRef),
		fstOrdsOutputsSingleton.newOutput(
			&util.BytesRef{Bytes: payload, Offset: 0, Length: len(payload)},
			0,
			math.MaxInt64-(sumTotalTermCount-1)),
	); err != nil {
		return fmt.Errorf("ordsPendingBlock.compileIndex: add prefix entry: %w", err)
	}
	scratchBytes.Reset()

	// Append sub-block FST entries.
	termOrdOffset := int64(0)
	for _, block := range blocks {
		if block.subIndices != nil {
			for _, si := range block.subIndices {
				if err := b.appendFST(compiler, si.index, termOrdOffset+si.termOrdStart, scratchIntsRef); err != nil {
					return fmt.Errorf("ordsPendingBlock.compileIndex: append sub-index: %w", err)
				}
			}
			block.subIndices = nil
		}
		termOrdOffset += block.totalTermCount
	}
	b.totFloorTermCount = termOrdOffset

	meta, err := compiler.Compile()
	if err != nil {
		return fmt.Errorf("ordsPendingBlock.compileIndex: compile FST: %w", err)
	}
	if meta == nil {
		return errors.New("ordsPendingBlock.compileIndex: FST compiled to nil metadata")
	}

	fstInst, err := gfst.FromFSTReader(meta, compiler.GetFSTReader())
	if err != nil {
		return fmt.Errorf("ordsPendingBlock.compileIndex: from FST reader: %w", err)
	}
	if fstInst == nil {
		return errors.New("ordsPendingBlock.compileIndex: FST is nil after compile")
	}

	b.index = fstInst
	if b.subIndices != nil {
		return errors.New("ordsPendingBlock.compileIndex: subIndices must be nil after compile")
	}
	return nil
}

// appendFST iterates the entries of subIndex and adds each one to the parent
// FSTCompiler with adjusted ordinal offsets.
func (b *ordsPendingBlock) appendFST(
	compiler *gfst.FSTCompiler[*FSTOrdsOutput],
	subIndex *gfst.FST[*FSTOrdsOutput],
	termOrdOffset int64,
	scratchIntsRef *util.IntsRefBuilder,
) error {
	subEnum, err := gfst.NewBytesRefFSTEnum(subIndex)
	if err != nil {
		return err
	}
	for {
		ent, err := subEnum.Next()
		if err != nil {
			return err
		}
		if ent == nil {
			break
		}
		output := ent.Output
		newOutput := fstOrdsOutputsSingleton.newOutput(
			output.Bytes,
			termOrdOffset+output.StartOrd,
			output.EndOrd-termOrdOffset,
		)
		if err := compiler.Add(
			gfst.ToIntsRef(ent.Input, scratchIntsRef),
			newOutput,
		); err != nil {
			return fmt.Errorf("appendFST: add sub-entry: %w", err)
		}
	}
	return nil
}

// ---- field metadata ----

// ordsFieldMetaData carries the per-field summary written during Close.
// Mirrors Java's FieldMetaData record.
type ordsFieldMetaData struct {
	fieldInfo        *index.FieldInfo
	rootCode         *FSTOrdsOutput // the FST's empty-output value
	numTerms         int64
	indexStartFP     int64
	sumTotalTermFreq int64
	sumDocFreq       int64
	docCount         int
	minTerm          *util.BytesRef
	maxTerm          *util.BytesRef
}

// ---- top-level writer ----

// ordsBlockTreeTermsWriter is the Go port of
// org.apache.lucene.codecs.blocktreeords.OrdsBlockTreeTermsWriter.
//
// It writes two on-disk files:
//   - .tio (terms dictionary): block headers, suffix/stats/meta blobs,
//     field-metadata trailer, and codec footer.
//   - .tipo (terms index): per-field FSTs with FSTOrdsOutput values.
type ordsBlockTreeTermsWriter struct {
	out      store.IndexOutput
	indexOut store.IndexOutput

	maxDoc          int
	minItemsInBlock int
	maxItemsInBlock int
	version         int32

	postingsWriter codecs.PostingsWriterBase
	fieldInfos     *index.FieldInfos

	fields []*ordsFieldMetaData

	scratchBytes   *store.ByteBuffersDataOutput
	scratchIntsRef *util.IntsRefBuilder

	closed bool
}

// newOrdsBlockTreeTermsWriter is the canonical constructor. It opens the
// .tio and .tipo outputs, writes codec headers, and calls
// postingsWriter.Init(out, state) so the postings writer writes its own
// header into the .tio stream.
func newOrdsBlockTreeTermsWriter(
	state *codecs.SegmentWriteState,
	postingsWriter codecs.PostingsWriterBase,
	minItemsInBlock, maxItemsInBlock int,
) (*ordsBlockTreeTermsWriter, error) {
	if err := validateOrdsBlockSizes(minItemsInBlock, maxItemsInBlock); err != nil {
		return nil, err
	}
	if state == nil {
		return nil, errors.New("ordsBlockTreeTermsWriter: state must not be nil")
	}
	if postingsWriter == nil {
		return nil, errors.New("ordsBlockTreeTermsWriter: postingsWriter must not be nil")
	}

	segmentName := state.SegmentInfo.Name()
	segmentSuffix := state.SegmentSuffix
	segmentID := state.SegmentInfo.GetID()
	directory := state.Directory

	w := &ordsBlockTreeTermsWriter{
		maxDoc:          state.SegmentInfo.DocCount(),
		minItemsInBlock: minItemsInBlock,
		maxItemsInBlock: maxItemsInBlock,
		version:         versionCurrent,
		postingsWriter:  postingsWriter,
		fieldInfos:      state.FieldInfos,
		fields:          make([]*ordsFieldMetaData, 0, 16),
		scratchBytes:    store.NewByteBuffersDataOutput(),
		scratchIntsRef:  util.NewIntsRefBuilder(),
	}

	// Create .tio (terms) output.
	termsName := segmentFileName(segmentName, segmentSuffix, termsExtension)
	rawOut, err := directory.CreateOutput(termsName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", termsName, err)
	}
	out := store.NewChecksumIndexOutput(rawOut)

	success := false
	var rawIndexOut store.IndexOutput
	var indexOut store.IndexOutput
	defer func() {
		if success {
			return
		}
		ordsCloseQuietly(out, indexOut)
	}()

	if err := codecs.WriteIndexHeader(out, termsCodecName, w.version, segmentID, segmentSuffix); err != nil {
		return nil, fmt.Errorf("write terms header: %w", err)
	}

	// Create .tipo (index) output.
	indexName := segmentFileName(segmentName, segmentSuffix, termsIndexExtension)
	rawIndexOut, err = directory.CreateOutput(indexName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", indexName, err)
	}
	indexOut = store.NewChecksumIndexOutput(rawIndexOut)
	if err := codecs.WriteIndexHeader(indexOut, termsIndexCodecName, w.version, segmentID, segmentSuffix); err != nil {
		return nil, fmt.Errorf("write index header: %w", err)
	}

	// Postings writer writes its own format/header into the terms output.
	if err := postingsWriter.Init(out, state); err != nil {
		return nil, fmt.Errorf("postings writer init: %w", err)
	}

	w.out = out
	w.indexOut = indexOut
	success = true
	return w, nil
}

// validateOrdsBlockSizes mirrors Lucene103BlockTreeTermsWriter.validateSettings.
func validateOrdsBlockSizes(minItemsInBlock, maxItemsInBlock int) error {
	if minItemsInBlock <= 1 {
		return fmt.Errorf("ordsBlockTreeTermsWriter: minItemsInBlock must be >= 2; got %d", minItemsInBlock)
	}
	if minItemsInBlock > maxItemsInBlock {
		return fmt.Errorf("ordsBlockTreeTermsWriter: maxItemsInBlock must be >= minItemsInBlock; got max=%d min=%d", maxItemsInBlock, minItemsInBlock)
	}
	if 2*(minItemsInBlock-1) > maxItemsInBlock {
		return fmt.Errorf("ordsBlockTreeTermsWriter: maxItemsInBlock must be at least 2*(minItemsInBlock-1); got max=%d min=%d", maxItemsInBlock, minItemsInBlock)
	}
	return nil
}

// segmentFileName builds a segment file name from its parts. Mirrors
// IndexFileNames.segmentFileName in Lucene and codecs.GetSegmentFileName.
func segmentFileName(segmentName, segmentSuffix, ext string) string {
	if segmentSuffix != "" {
		return segmentName + "_" + segmentSuffix + "." + ext
	}
	return segmentName + "." + ext
}

// Write satisfies the FieldsConsumer SPI: it drives the per-field writer
// for a single field.
func (w *ordsBlockTreeTermsWriter) Write(field string, terms index.Terms) error {
	if w.closed {
		return errors.New("ordsBlockTreeTermsWriter: Write after Close")
	}
	fieldInfo := w.fieldInfos.GetByName(field)
	if fieldInfo == nil {
		return fmt.Errorf("ordsBlockTreeTermsWriter.Write: unknown field %q", field)
	}
	return w.writeField(fieldInfo, terms)
}

// WriteFields walks fields in the iterator's order and persists every
// non-nil Terms via the per-field termsWriter state machine. Fields must
// be visited in ascending order.
func (w *ordsBlockTreeTermsWriter) WriteFields(fields index.Fields) error {
	if w.closed {
		return errors.New("ordsBlockTreeTermsWriter: WriteFields after Close")
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
			return fmt.Errorf("ordsBlockTreeTermsWriter.WriteFields: fields must be in ascending order, got %q after %q", field, lastField)
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
			return fmt.Errorf("ordsBlockTreeTermsWriter.WriteFields: unknown field %q", field)
		}
		if err := w.writeField(fieldInfo, terms); err != nil {
			return err
		}
	}
	return nil
}

// Close flushes the field-metadata trailer to .tio, writes footers to both
// files, and closes all outputs. Subsequent calls are no-ops.
func (w *ordsBlockTreeTermsWriter) Close() error {
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
			ordsCloseAll(setErr, w.out, w.indexOut, w.postingsWriter)
		} else {
			ordsCloseQuietly(w.out, w.indexOut, w.postingsWriter)
		}
	}()

	dirStart := w.out.GetFilePointer()
	indexDirStart := w.indexOut.GetFilePointer()

	if err := store.WriteVInt(w.out, int32(len(w.fields))); err != nil {
		setErr(err)
		return firstErr
	}

	for _, field := range w.fields {
		if err := store.WriteVInt(w.out, int32(field.fieldInfo.Number())); err != nil {
			setErr(err)
			return firstErr
		}
		if err := store.WriteVLong(w.out, field.numTerms); err != nil {
			setErr(err)
			return firstErr
		}
		// Write rootCode bytes (the FST empty-output's BytesRef payload).
		rootBytes := field.rootCode.Bytes
		if err := store.WriteVInt(w.out, int32(rootBytes.Length)); err != nil {
			setErr(err)
			return firstErr
		}
		if rootBytes.Length > 0 {
			if err := w.out.WriteBytesN(rootBytes.Bytes[rootBytes.Offset:rootBytes.Offset+rootBytes.Length], rootBytes.Length); err != nil {
				setErr(err)
				return firstErr
			}
		}
		if field.fieldInfo.IndexOptions() != index.IndexOptionsDocs {
			if err := store.WriteVLong(w.out, field.sumTotalTermFreq); err != nil {
				setErr(err)
				return firstErr
			}
		}
		if err := store.WriteVLong(w.out, field.sumDocFreq); err != nil {
			setErr(err)
			return firstErr
		}
		if err := store.WriteVInt(w.out, int32(field.docCount)); err != nil {
			setErr(err)
			return firstErr
		}
		if err := store.WriteVLong(w.indexOut, field.indexStartFP); err != nil {
			setErr(err)
			return firstErr
		}
		ordsWriteBytesRef(w.out, field.minTerm)
		ordsWriteBytesRef(w.out, field.maxTerm)
	}

	if err := w.out.WriteLong(dirStart); err != nil {
		setErr(err)
		return firstErr
	}
	if err := codecs.WriteFooter(w.out); err != nil {
		setErr(err)
		return firstErr
	}
	if err := w.indexOut.WriteLong(indexDirStart); err != nil {
		setErr(err)
		return firstErr
	}
	if err := codecs.WriteFooter(w.indexOut); err != nil {
		setErr(err)
		return firstErr
	}
	success = true
	return firstErr
}

// writeField drives a single field through a fresh per-field
// ordsTermsWriter until every term has been visited and finish() has
// emitted the field metadata.
func (w *ordsBlockTreeTermsWriter) writeField(fieldInfo *index.FieldInfo, terms index.Terms) error {
	tw, err := newOrdsTermsWriter(w, fieldInfo)
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
		if err := tw.write(term, termsEnum); err != nil {
			return err
		}
	}
	return tw.finish()
}

// ---- per-field TermsWriter ----

// ordsTermsWriter is the per-field driver. Each call to write() pushes
// one term onto a stack of (terms, sub-blocks), occasionally folding the
// top into a new ordsPendingBlock. finish() drains the stack and emits
// the field's metadata.
//
// Mirrors OrdsBlockTreeTermsWriter.TermsWriter.
type ordsTermsWriter struct {
	parent           *ordsBlockTreeTermsWriter
	fieldInfo        *index.FieldInfo
	docsSeen         *util.FixedBitSet
	numTerms         int64
	sumTotalTermFreq int64
	sumDocFreq       int64
	indexStartFP     int64

	lastTerm     *util.BytesRefBuilder
	prefixStarts []int
	pending      []*ordsPendingEntry
	newBlocks    []*ordsPendingBlock

	firstPendingTerm *ordsPendingTerm
	lastPendingTerm  *ordsPendingTerm

	// Reusable per-block buffers (mirrors Java's ByteBuffersDataOutput fields).
	suffixWriter *store.ByteBuffersDataOutput
	statsWriter  *store.ByteBuffersDataOutput
	metaWriter   *store.ByteBuffersDataOutput
}

func newOrdsTermsWriter(parent *ordsBlockTreeTermsWriter, fieldInfo *index.FieldInfo) (*ordsTermsWriter, error) {
	if fieldInfo.IndexOptions() == index.IndexOptionsNone {
		return nil, fmt.Errorf("newOrdsTermsWriter: field %q indexOptions == NONE", fieldInfo.Name())
	}
	docsSeen, err := util.NewFixedBitSet(parent.maxDoc)
	if err != nil {
		return nil, fmt.Errorf("newOrdsTermsWriter: %w", err)
	}
	tw := &ordsTermsWriter{
		parent:       parent,
		fieldInfo:    fieldInfo,
		docsSeen:     docsSeen,
		lastTerm:     util.NewBytesRefBuilder(),
		prefixStarts: make([]int, 8),
		pending:      make([]*ordsPendingEntry, 0, 64),
		newBlocks:    make([]*ordsPendingBlock, 0, 16),
		suffixWriter: store.NewByteBuffersDataOutput(),
		statsWriter:  store.NewByteBuffersDataOutput(),
		metaWriter:   store.NewByteBuffersDataOutput(),
	}
	if _, err := parent.postingsWriter.SetField(fieldInfo); err != nil {
		return nil, fmt.Errorf("postingsWriter.SetField: %w", err)
	}
	return tw, nil
}

// pushSinglePostings drives the underlying PostingsWriterBase for a single
// term and returns the populated BlockTermState. Returns nil when the term
// has no surviving documents.
func (t *ordsTermsWriter) pushSinglePostings(termText *index.Term, termsEnum index.TermsEnum) (*codecs.BlockTermState, error) {
	state := t.parent.postingsWriter.NewTermState()

	if err := t.parent.postingsWriter.StartTerm(nil); err != nil {
		return nil, err
	}

	postingsEnum, err := termsEnum.Postings(0)
	if err != nil {
		return nil, err
	}
	if postingsEnum == nil {
		return nil, nil
	}

	pusher, ok := t.parent.postingsWriter.(codecs.PushPostingsWriterBase)
	if !ok {
		return nil, fmt.Errorf("ordsBlockTreeTermsWriter: postingsWriter %T does not implement PushPostingsWriterBase", t.parent.postingsWriter)
	}

	hasFreqs := t.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqs
	hasPositions := t.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
	hasOffsets := t.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	hasPayloads := t.fieldInfo.HasPayloads()

	docCount, totalTermFreq, err := codecs.WriteTerm(pusher, postingsEnum, hasFreqs, hasPositions, hasOffsets, hasPayloads, nil)
	if err != nil {
		return nil, err
	}
	if docCount == 0 {
		return nil, nil
	}

	state.DocFreq = docCount
	state.TotalTermFreq = totalTermFreq
	if err := t.parent.postingsWriter.FinishTerm(state); err != nil {
		return nil, err
	}
	if hasPositions && state.TotalTermFreq < int64(state.DocFreq) {
		return nil, fmt.Errorf("ordsBlockTreeTermsWriter: term has positions but totalTermFreq (%d) < docFreq (%d)", state.TotalTermFreq, state.DocFreq)
	}
	return state, nil
}

// write pushes one term onto the per-field pending stack and updates
// cumulative stats. Mirrors TermsWriter.write(BytesRef, TermsEnum).
func (t *ordsTermsWriter) write(term *index.Term, termsEnum index.TermsEnum) error {
	state, err := t.pushSinglePostings(term, termsEnum)
	if err != nil {
		return err
	}
	if state == nil {
		return nil
	}
	if state.DocFreq == 0 {
		return errors.New("ordsTermsWriter.write: postings writer returned BlockTermState with docFreq == 0")
	}
	if t.fieldInfo.IndexOptions() != index.IndexOptionsDocs && state.TotalTermFreq < int64(state.DocFreq) {
		return fmt.Errorf("ordsTermsWriter.write: totalTermFreq %d < docFreq %d", state.TotalTermFreq, state.DocFreq)
	}

	textBytes := term.BytesValue()
	if err := t.pushTerm(textBytes); err != nil {
		return err
	}

	pt := newOrdsPendingTerm(term, state)
	t.pending = append(t.pending, &ordsPendingEntry{isTerm: true, term: pt})

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
// previous term, possibly emitting one or more blocks, then primes
// prefixStarts for the new tail.
func (t *ordsTermsWriter) pushTerm(text *util.BytesRef) error {
	last := t.lastTerm.Get()
	prefixLen := ordsComputePrefixMismatch(
		last.Bytes[last.Offset:last.Offset+last.Length],
		text.Bytes[text.Offset:text.Offset+text.Length],
	)
	if prefixLen == -1 {
		if t.lastTerm.Length() != 0 {
			return errors.New("ordsTermsWriter.pushTerm: mismatch=-1 only valid for first/empty lastTerm")
		}
		prefixLen = 0
	}

	for i := t.lastTerm.Length() - 1; i >= prefixLen; i-- {
		prefixTopSize := len(t.pending) - t.prefixStarts[i]
		if prefixTopSize >= t.parent.minItemsInBlock {
			if err := t.writeBlocks(i+1, prefixTopSize); err != nil {
				return err
			}
			t.prefixStarts[i] -= prefixTopSize - 1
		}
	}

	if len(t.prefixStarts) < text.Length {
		grown := make([]int, ordsMaxInt(text.Length, 2*len(t.prefixStarts)))
		copy(grown, t.prefixStarts)
		t.prefixStarts = grown
	}
	for i := prefixLen; i < text.Length; i++ {
		t.prefixStarts[i] = len(t.pending)
	}

	t.lastTerm.CopyBytes(text.Bytes, text.Offset, text.Length)
	return nil
}

// writeBlocks takes the top count entries off pending and groups them into
// one or more on-disk blocks; floor splitting happens when a single
// shared-prefix group exceeds maxItemsInBlock.
func (t *ordsTermsWriter) writeBlocks(prefixLength, count int) error {
	if count <= 0 {
		return fmt.Errorf("ordsTermsWriter.writeBlocks: count must be > 0, got %d", count)
	}
	if prefixLength <= 0 && count != len(t.pending) {
		return errors.New("ordsTermsWriter.writeBlocks: root block must consume entire pending stack")
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
					return fmt.Errorf("ordsTermsWriter.writeBlocks: lastSuffixLeadLabel %d != -1 for zero-suffix term", lastSuffixLeadLabel)
				}
				suffixLeadLabel = -1
			} else {
				suffixLeadLabel = int(ent.term.termBytes[prefixLength]) & 0xFF
			}
		} else {
			block := ent.block
			if block.prefix.Length <= prefixLength {
				return fmt.Errorf("ordsTermsWriter.writeBlocks: sub-block prefix length %d <= prefixLength %d", block.prefix.Length, prefixLength)
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
		return errors.New("ordsTermsWriter.writeBlocks: produced zero blocks")
	}

	first := t.newBlocks[0]
	if first.isFloor != (len(t.newBlocks) > 1) {
		return fmt.Errorf("ordsTermsWriter.writeBlocks: isFloor=%v but newBlocks.size()=%d", first.isFloor, len(t.newBlocks))
	}
	if err := first.compileIndex(t.newBlocks, t.parent.scratchBytes, t.parent.scratchIntsRef); err != nil {
		return err
	}

	// Replace consumed entries with the new first block.
	t.pending = t.pending[:len(t.pending)-count]
	t.pending = append(t.pending, &ordsPendingEntry{block: first})
	t.newBlocks = t.newBlocks[:0]
	return nil
}

// writeBlock emits one block to the .tio output and returns an
// ordsPendingBlock describing it.
func (t *ordsTermsWriter) writeBlock(prefixLength int, isFloor bool, floorLeadLabel, start, end int, hasTerms, hasSubBlocks bool) (*ordsPendingBlock, error) {
	if end <= start {
		return nil, fmt.Errorf("ordsTermsWriter.writeBlock: end %d <= start %d", end, start)
	}
	startFP := t.parent.out.GetFilePointer()
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
	if err := store.WriteVInt(t.parent.out, int32(code)); err != nil {
		return nil, err
	}

	isLeafBlock := !hasSubBlocks
	var subIndices []*ordsSubIndex
	var totalTermCount int64

	absolute := true

	if isLeafBlock {
		// Leaf block: only terms, no sub-blocks.
		subIndices = nil
		for i := start; i < end; i++ {
			ent := t.pending[i]
			if !ent.isTerm {
				return nil, fmt.Errorf("ordsTermsWriter.writeBlock: leaf block expects all terms, got block at index %d", i)
			}
			term := ent.term
			if !ordsBytesHasPrefix(term.termBytes, prefixBytes) {
				return nil, fmt.Errorf("ordsTermsWriter.writeBlock: term %q lacks expected prefix", term.termBytes)
			}
			suffix := len(term.termBytes) - prefixLength

			// Write suffix length + bytes.
			if err := t.suffixWriter.WriteVInt(int32(suffix)); err != nil {
				return nil, err
			}
			if err := t.suffixWriter.WriteBytes(term.termBytes[prefixLength:]); err != nil {
				return nil, err
			}

			if floorLeadLabel != -1 && int(term.termBytes[prefixLength])&0xFF < floorLeadLabel {
				return nil, fmt.Errorf("ordsTermsWriter.writeBlock: term lead byte < floorLeadLabel")
			}

			// Write stats directly (no singleton compression).
			if err := t.statsWriter.WriteVInt(int32(term.state.DocFreq)); err != nil {
				return nil, err
			}
			if t.fieldInfo.IndexOptions() != index.IndexOptionsDocs {
				if err := t.statsWriter.WriteVLong(term.state.TotalTermFreq - int64(term.state.DocFreq)); err != nil {
					return nil, err
				}
			}

			// Write term metadata via postings writer.
			if err := t.parent.postingsWriter.EncodeTerm(bbdoAsIndexOutput{t.metaWriter}, t.fieldInfo, term.state, absolute); err != nil {
				return nil, err
			}
			absolute = false
		}
		totalTermCount = int64(end - start)
	} else {
		// Mixed block: terms and sub-blocks.
		subIndices = make([]*ordsSubIndex, 0, 8)
		totalTermCount = 0
		for i := start; i < end; i++ {
			ent := t.pending[i]
			if ent.isTerm {
				term := ent.term
				if !ordsBytesHasPrefix(term.termBytes, prefixBytes) {
					return nil, fmt.Errorf("ordsTermsWriter.writeBlock: non-leaf term %q lacks expected prefix", term.termBytes)
				}
				suffix := len(term.termBytes) - prefixLength

				// For non-leaf, bit 0 = 0 means this entry is a term.
				if err := t.suffixWriter.WriteVInt(int32(suffix << 1)); err != nil {
					return nil, err
				}
				if err := t.suffixWriter.WriteBytes(term.termBytes[prefixLength:]); err != nil {
					return nil, err
				}

				if floorLeadLabel != -1 && int(term.termBytes[prefixLength])&0xFF < floorLeadLabel {
					return nil, fmt.Errorf("ordsTermsWriter.writeBlock: term lead byte < floorLeadLabel")
				}

				if err := t.statsWriter.WriteVInt(int32(term.state.DocFreq)); err != nil {
					return nil, err
				}
				if t.fieldInfo.IndexOptions() != index.IndexOptionsDocs {
					if err := t.statsWriter.WriteVLong(term.state.TotalTermFreq - int64(term.state.DocFreq)); err != nil {
						return nil, err
					}
				}

				if err := t.parent.postingsWriter.EncodeTerm(bbdoAsIndexOutput{t.metaWriter}, t.fieldInfo, term.state, absolute); err != nil {
					return nil, err
				}
				absolute = false
				totalTermCount++
			} else {
				block := ent.block
				if !ordsBytesHasPrefix(
					block.prefix.Bytes[block.prefix.Offset:block.prefix.Offset+block.prefix.Length],
					prefixBytes,
				) {
					return nil, fmt.Errorf("ordsTermsWriter.writeBlock: sub-block prefix lacks expected leading bytes")
				}
				suffix := block.prefix.Length - prefixLength
				if suffix <= 0 {
					return nil, fmt.Errorf("ordsTermsWriter.writeBlock: sub-block suffix length %d must be > 0", suffix)
				}

				// For non-leaf, bit 0 = 1 means this entry is a sub-block.
				if err := t.suffixWriter.WriteVInt(int32((suffix << 1) | 1)); err != nil {
					return nil, err
				}
				if err := t.suffixWriter.WriteBytes(block.prefix.Bytes[block.prefix.Offset+prefixLength : block.prefix.Offset+block.prefix.Length]); err != nil {
					return nil, err
				}

				if floorLeadLabel != -1 && int(block.prefix.Bytes[block.prefix.Offset+prefixLength])&0xFF < floorLeadLabel {
					return nil, fmt.Errorf("ordsTermsWriter.writeBlock: sub-block lead byte < floorLeadLabel")
				}
				if block.fp >= startFP {
					return nil, fmt.Errorf("ordsTermsWriter.writeBlock: sub-block fp %d must be < startFP %d", block.fp, startFP)
				}

				// Write FP delta.
				if err := t.suffixWriter.WriteVLong(startFP - block.fp); err != nil {
					return nil, err
				}
				// Write totFloorTermCount.
				if err := t.suffixWriter.WriteVLong(block.totFloorTermCount); err != nil {
					return nil, err
				}

				subIndices = append(subIndices, &ordsSubIndex{index: block.index, termOrdStart: totalTermCount})
				totalTermCount += block.totFloorTermCount
			}
		}
		if len(subIndices) == 0 {
			return nil, errors.New("ordsTermsWriter.writeBlock: mixed block produced zero sub-indices")
		}
	}

	// Write suffix blob: vInt(size << 1 | leafBit), then raw suffix bytes.
	suffixSize := int(t.suffixWriter.Size())
	token := suffixSize << 1
	if isLeafBlock {
		token |= 1
	}
	if err := store.WriteVInt(t.parent.out, int32(token)); err != nil {
		return nil, err
	}
	if suffixSize > 0 {
		if err := t.suffixWriter.CopyTo(t.parent.out); err != nil {
			return nil, err
		}
	}
	t.suffixWriter.Reset()

	// Write stats blob.
	statsSize := int(t.statsWriter.Size())
	if err := store.WriteVInt(t.parent.out, int32(statsSize)); err != nil {
		return nil, err
	}
	if statsSize > 0 {
		if err := t.statsWriter.CopyTo(t.parent.out); err != nil {
			return nil, err
		}
	}
	t.statsWriter.Reset()

	// Write meta blob.
	metaSize := int(t.metaWriter.Size())
	if err := store.WriteVInt(t.parent.out, int32(metaSize)); err != nil {
		return nil, err
	}
	if metaSize > 0 {
		if err := t.metaWriter.CopyTo(t.parent.out); err != nil {
			return nil, err
		}
	}
	t.metaWriter.Reset()

	if hasFloorLeadLabel {
		prefix.Bytes = append(prefix.Bytes, byte(floorLeadLabel))
		prefix.Length++
	}

	return &ordsPendingBlock{
		prefix:        prefix,
		fp:            startFP,
		hasTerms:      hasTerms,
		isFloor:       isFloor,
		floorLeadByte: floorLeadLabel,
		totalTermCount:    totalTermCount,
		subIndices:    subIndices,
	}, nil
}

// finish drains the pending stack, emits the root block, saves the FST to
// the index output, and appends the field metadata to the parent's fields
// list.
func (t *ordsTermsWriter) finish() error {
	if t.numTerms == 0 {
		if t.sumTotalTermFreq != 0 && !(t.fieldInfo.IndexOptions() == index.IndexOptionsDocs && t.sumTotalTermFreq == -1) {
			return fmt.Errorf("ordsTermsWriter.finish: sumTotalTermFreq %d invalid for empty field", t.sumTotalTermFreq)
		}
		if t.sumDocFreq != 0 {
			return fmt.Errorf("ordsTermsWriter.finish: sumDocFreq %d invalid for empty field", t.sumDocFreq)
		}
		if t.docsSeen.Cardinality() != 0 {
			return fmt.Errorf("ordsTermsWriter.finish: docsSeen cardinality %d invalid for empty field", t.docsSeen.Cardinality())
		}
		return nil
	}

	// Push empty term twice to force closing all dangling prefixes.
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
		return fmt.Errorf("ordsTermsWriter.finish: expected single root block, got %d entries", len(t.pending))
	}
	root := t.pending[0].block
	if root.prefix.Length != 0 {
		return errors.New("ordsTermsWriter.finish: root block must have empty prefix")
	}
	if root.index == nil {
		return errors.New("ordsTermsWriter.finish: root block FST is nil")
	}

	rootCode, hasEmptyOutput := root.index.GetEmptyOutput()
	if !hasEmptyOutput || rootCode == nil {
		return errors.New("ordsTermsWriter.finish: root block FST missing empty output")
	}

	// Save FST to index output.
	t.indexStartFP = t.parent.indexOut.GetFilePointer()
	if err := root.index.Save(t.parent.indexOut, t.parent.indexOut); err != nil {
		return fmt.Errorf("ordsTermsWriter.finish: save FST: %w", err)
	}

	if t.firstPendingTerm == nil {
		return errors.New("ordsTermsWriter.finish: firstPendingTerm is nil")
	}
	if t.lastPendingTerm == nil {
		return errors.New("ordsTermsWriter.finish: lastPendingTerm is nil")
	}

	minTerm := &util.BytesRef{
		Bytes:  t.firstPendingTerm.termBytes,
		Offset: 0,
		Length: len(t.firstPendingTerm.termBytes),
	}
	maxTerm := &util.BytesRef{
		Bytes:  t.lastPendingTerm.termBytes,
		Offset: 0,
		Length: len(t.lastPendingTerm.termBytes),
	}

	t.parent.fields = append(t.parent.fields, &ordsFieldMetaData{
		fieldInfo:        t.fieldInfo,
		rootCode:         rootCode,
		numTerms:         t.numTerms,
		indexStartFP:     t.indexStartFP,
		sumTotalTermFreq: t.sumTotalTermFreq,
		sumDocFreq:       t.sumDocFreq,
		docCount:         t.docsSeen.Cardinality(),
		minTerm:          minTerm,
		maxTerm:          maxTerm,
	})
	return nil
}

// ---- static helpers ----

// ordsWriteBytesRef writes a BytesRef as vInt(len) + raw bytes. Mirrors
// the private writeBytesRef helper in the Java writer.
func ordsWriteBytesRef(out store.DataOutput, b *util.BytesRef) {
	if err := store.WriteVInt(out, int32(b.Length)); err != nil {
		// Panic is acceptable here because this is only called from Close
		// after all term-processing errors have already been checked, and
		// a write failure at this point is fatal anyway.
		panic(fmt.Errorf("ordsWriteBytesRef: write length: %w", err))
	}
	if b.Length > 0 {
		if err := out.WriteBytesN(b.Bytes[b.Offset:b.Offset+b.Length], b.Length); err != nil {
			panic(fmt.Errorf("ordsWriteBytesRef: write bytes: %w", err))
		}
	}
}

// ordsComputePrefixMismatch returns the index of the first differing byte
// between a and b, or -1 when one is a prefix of the other.
func ordsComputePrefixMismatch(a, b []byte) int {
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

// ordsBytesHasPrefix returns true when b starts with prefix.
func ordsBytesHasPrefix(b, prefix []byte) bool {
	if len(b) < len(prefix) {
		return false
	}
	return bytes.Equal(b[:len(prefix)], prefix)
}

// ordsMaxInt returns the larger of a and b.
func ordsMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// bbdoAsIndexOutput wraps a *store.ByteBuffersDataOutput to satisfy
// store.IndexOutput for EncodeTerm calls. Only streaming writes are
// forwarded; random-access methods are stubs.
type bbdoAsIndexOutput struct {
	*store.ByteBuffersDataOutput
}

var _ store.IndexOutput = bbdoAsIndexOutput{}

func (b bbdoAsIndexOutput) GetFilePointer() int64 { return b.ByteBuffersDataOutput.Size() }
func (b bbdoAsIndexOutput) SetPosition(_ int64) error {
	return fmt.Errorf("bbdoAsIndexOutput: SetPosition not supported")
}
func (b bbdoAsIndexOutput) Length() int64   { return b.ByteBuffersDataOutput.Size() }
func (b bbdoAsIndexOutput) GetName() string { return "<bbdo>" }
func (b bbdoAsIndexOutput) Close() error    { return nil }

// ---- close helpers ----

// ordsCloser is the common interface for anything that can be closed.
type ordsCloser interface {
	Close() error
}

// ordsCloseQuietly closes every closer in order, swallowing all errors.
func ordsCloseQuietly(closers ...ordsCloser) {
	for _, c := range closers {
		if c == nil {
			continue
		}
		_ = c.Close()
	}
}

// ordsCloseAll closes every closer in order and forwards the first error
// to setErr. Mirrors IOUtils.close.
func ordsCloseAll(setErr func(error), closers ...ordsCloser) {
	for _, c := range closers {
		if c == nil {
			continue
		}
		if err := c.Close(); err != nil {
			setErr(err)
		}
	}
}

// Compile-time check that ordsBlockTreeTermsWriter satisfies
// codecs.FieldsConsumer (via the Write + Close methods).
var _ codecs.FieldsConsumer = (*ordsBlockTreeTermsWriter)(nil)
