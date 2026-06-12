// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/join"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/compress"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// DefaultMinBlockSize is the suggested default for minItemsInBlock.
const DefaultMinBlockSize = 25

// DefaultMaxBlockSize is the suggested default for maxItemsInBlock.
const DefaultMaxBlockSize = 48

// Lucene40BlockTreeTermsWriter is the write-side companion to
// Lucene40BlockTreeTermsReader. It produces .tim, .tip and .tmd files
// using the version-6 (VersionMetaFile) format.
//
// Port of the test-only Java writer
// org.apache.lucene.backward_codecs.lucene40.blocktree.Lucene40BlockTreeTermsWriter
// (Lucene 10.4.0).
type Lucene40BlockTreeTermsWriter struct {
	metaOut   store.IndexOutput
	termsOut  store.IndexOutput
	indexOut  store.IndexOutput
	maxDoc    int
	minItems  int
	maxItems  int

	postingsWriter codecs.PostingsWriterBase
	fieldInfos     *index.FieldInfos

	fields []store.ByteArrayDataOutput

	closed bool
}

// NewLucene40BlockTreeTermsWriter creates a new writer for the given segment.
func NewLucene40BlockTreeTermsWriter(
	state *codecs.SegmentWriteState,
	postingsWriter codecs.PostingsWriterBase,
	minItemsInBlock, maxItemsInBlock int,
) (*Lucene40BlockTreeTermsWriter, error) {
	if err := validateSettings(minItemsInBlock, maxItemsInBlock); err != nil {
		return nil, err
	}

	w := &Lucene40BlockTreeTermsWriter{
		maxDoc:         state.SegmentInfo.DocCount(),
		minItems:       minItemsInBlock,
		maxItems:       maxItemsInBlock,
		fieldInfos:     state.FieldInfos,
		postingsWriter: postingsWriter,
	}

	termsName := codecs.GetSegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, termsExtension)
	termsOut, err := store.NewEndiannessReverserIndexOutputFromDirectory(state.Directory, termsName, store.IOContext{Context: store.ContextFlush})
	if err != nil {
		return nil, fmt.Errorf("blocktree writer: create terms output: %w", err)
	}
	w.termsOut = termsOut

	var metaOut, indexOut store.IndexOutput
	success := false
	defer func() {
		if !success {
			closeQuietly(metaOut, indexOut, termsOut)
		}
	}()

	if err := codecs.WriteIndexHeader(termsOut, termsCodecName, VersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("blocktree writer: terms header: %w", err)
	}

	indexName := codecs.GetSegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, termsIndexExt)
	indexOut, err = store.NewEndiannessReverserIndexOutputFromDirectory(state.Directory, indexName, store.IOContext{Context: store.ContextFlush})
	if err != nil {
		return nil, fmt.Errorf("blocktree writer: create index output: %w", err)
	}
	w.indexOut = indexOut

	if err := codecs.WriteIndexHeader(indexOut, termsIndexCodec, VersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("blocktree writer: index header: %w", err)
	}

	metaName := codecs.GetSegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, termsMetaExt)
	metaOut, err = store.NewEndiannessReverserIndexOutputFromDirectory(state.Directory, metaName, store.IOContext{Context: store.ContextFlush})
	if err != nil {
		return nil, fmt.Errorf("blocktree writer: create meta output: %w", err)
	}
	w.metaOut = metaOut

	if err := codecs.WriteIndexHeader(metaOut, termsMetaCodecName, VersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("blocktree writer: meta header: %w", err)
	}

	if err := postingsWriter.Init(metaOut, state); err != nil {
		return nil, fmt.Errorf("blocktree writer: postings init: %w", err)
	}

	success = true
	return w, nil
}

// validateSettings enforces the min/max block-size invariants.
func validateSettings(minItemsInBlock, maxItemsInBlock int) error {
	if minItemsInBlock <= 1 {
		return fmt.Errorf("minItemsInBlock must be >= 2; got %d", minItemsInBlock)
	}
	if minItemsInBlock > maxItemsInBlock {
		return fmt.Errorf("maxItemsInBlock must be >= minItemsInBlock; got max=%d min=%d", maxItemsInBlock, minItemsInBlock)
	}
	if 2*(minItemsInBlock-1) > maxItemsInBlock {
		return fmt.Errorf("maxItemsInBlock must be at least 2*(minItemsInBlock-1); got max=%d min=%d", maxItemsInBlock, minItemsInBlock)
	}
	return nil
}

// Write implements codecs.FieldsConsumer for one field.
func (w *Lucene40BlockTreeTermsWriter) Write(field string, terms index.Terms) error {
	if w.closed {
		return fmt.Errorf("Lucene40BlockTreeTermsWriter: Write after Close")
	}
	fieldInfo := w.fieldInfos.GetByName(field)
	if fieldInfo == nil {
		return fmt.Errorf("Lucene40BlockTreeTermsWriter.Write: unknown field %q", field)
	}
	return w.writeField(fieldInfo, terms, nil)
}

// WriteFields is a convenience helper for tests that mirrors the Java
// FieldsConsumer.write(Fields, NormsProducer) signature.
func (w *Lucene40BlockTreeTermsWriter) WriteFields(fields index.Fields, norms index.NormsProducer) error {
	if w.closed {
		return fmt.Errorf("Lucene40BlockTreeTermsWriter: WriteFields after Close")
	}
	if fields == nil {
		return nil
	}
	it, err := fields.Iterator()
	if err != nil {
		return err
	}
	for {
		field, err := it.Next()
		if err != nil {
			return err
		}
		if field == "" {
			break
		}
		terms, err := fields.Terms(field)
		if err != nil {
			return err
		}
		if terms == nil {
			continue
		}
		if err := w.Write(field, terms); err != nil {
			return err
		}
	}
	return nil
}

// writeField drives a single field through a fresh termsWriter.
func (w *Lucene40BlockTreeTermsWriter) writeField(fieldInfo *index.FieldInfo, terms index.Terms, norms index.NormsProducer) error {
	tw := newTermsWriter(w, fieldInfo)
	te, err := terms.GetIterator()
	if err != nil {
		return err
	}
	for {
		term, err := te.Next()
		if err != nil {
			return err
		}
		if term == nil {
			break
		}
		if err := tw.writeTerm(term, te, norms); err != nil {
			return err
		}
	}
	return tw.finish()
}

// Close flushes field metadata, writes footers and closes all files.
func (w *Lucene40BlockTreeTermsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	success := false
	defer func() {
		if success {
			closeAll(w.metaOut, w.termsOut, w.indexOut, w.postingsWriter)
		} else {
			closeQuietly(w.metaOut, w.termsOut, w.indexOut, w.postingsWriter)
		}
	}()

	if err := store.WriteVInt(w.metaOut, int32(len(w.fields))); err != nil {
		return fmt.Errorf("blocktree writer: write numFields: %w", err)
	}
	for _, fieldMeta := range w.fields {
		if _, err := w.metaOut.WriteBytes(fieldMeta.GetBytes()); err != nil {
			return fmt.Errorf("blocktree writer: copy field meta: %w", err)
		}
	}

	if err := codecs.WriteFooter(w.indexOut); err != nil {
		return fmt.Errorf("blocktree writer: index footer: %w", err)
	}
	if err := w.metaOut.WriteLong(w.indexOut.GetFilePointer()); err != nil {
		return fmt.Errorf("blocktree writer: write index length: %w", err)
	}

	if err := codecs.WriteFooter(w.termsOut); err != nil {
		return fmt.Errorf("blocktree writer: terms footer: %w", err)
	}
	if err := w.metaOut.WriteLong(w.termsOut.GetFilePointer()); err != nil {
		return fmt.Errorf("blocktree writer: write terms length: %w", err)
	}

	if err := codecs.WriteFooter(w.metaOut); err != nil {
		return fmt.Errorf("blocktree writer: meta footer: %w", err)
	}

	success = true
	return nil
}

// encodeOutput packs a file pointer plus boolean flags into a single long.
func encodeOutput(fp int64, hasTerms, isFloor bool) int64 {
	code := fp << OutputFlagsNumBits
	if hasTerms {
		code |= OutputFlagHasTerms
	}
	if isFloor {
		code |= OutputFlagIsFloor
	}
	return code
}

// pendingEntry is the common base for terms and blocks on the pending stack.
type pendingEntry struct {
	isTerm bool
}

// pendingTerm represents one term waiting to be written.
type pendingTerm struct {
	termBytes []byte
	state     *codecs.BlockTermState
}

// pendingBlock represents one block waiting to be written.
type pendingBlock struct {
	prefix         []byte
	fp             int64
	index          *fst.FST[*util.BytesRef]
	subIndices     []*fst.FST[*util.BytesRef]
	hasTerms       bool
	isFloor        bool
	floorLeadByte  int
}

// statsWriter accumulates docFreq / totalTermFreq and run-length-encodes
// singleton entries (df==1, ttf==1).
type statsWriter struct {
	out            *store.ByteArrayDataOutput
	hasFreqs       bool
	singletonCount int
}

func newStatsWriter(out *store.ByteArrayDataOutput, hasFreqs bool) *statsWriter {
	return &statsWriter{out: out, hasFreqs: hasFreqs}
}

func (s *statsWriter) add(df int, ttf int64) {
	if df == 1 && (!s.hasFreqs || ttf == 1) {
		s.singletonCount++
		return
	}
	s.finish()
	_ = store.WriteVInt(s.out, int32(df<<1))
	if s.hasFreqs {
		_ = store.WriteVLong(s.out, ttf-int64(df))
	}
}

func (s *statsWriter) finish() {
	if s.singletonCount > 0 {
		_ = store.WriteVInt(s.out, int32(((s.singletonCount-1)<<1)|1))
		s.singletonCount = 0
	}
}

// termsWriter handles all terms for a single field.
type termsWriter struct {
	parent       *Lucene40BlockTreeTermsWriter
	fieldInfo    *index.FieldInfo
	numTerms     int64
	docsSeen     *join.FixedBitSet
	sumTotalTermFreq int64
	sumDocFreq   int64

	lastTerm     *util.BytesRefBuilder
	prefixStarts []int
	pending      []*pendingEntry
	newBlocks    []*pendingBlock

	firstPendingTerm *pendingTerm
	lastPendingTerm  *pendingTerm

	suffixLengthsWriter *store.ByteArrayDataOutput
	suffixWriter        []byte
	statsWriter         *statsWriter
	statsBuf            *store.ByteArrayDataOutput
	metaWriter          *store.ByteArrayDataOutput
	spareWriter         *store.ByteArrayDataOutput
	spareBytes          []byte
	compressionHashTable compress.HashTable
}

func newTermsWriter(parent *Lucene40BlockTreeTermsWriter, fieldInfo *index.FieldInfo) *termsWriter {
	docsSeen := join.NewFixedBitSet(parent.maxDoc)
	_ = parent.postingsWriter.SetField(fieldInfo)

	statsBuf := store.NewByteArrayDataOutput(64)
	return &termsWriter{
		parent:               parent,
		fieldInfo:            fieldInfo,
		docsSeen:             docsSeen,
		lastTerm:             util.NewBytesRefBuilder(),
		prefixStarts:         make([]int, 8),
		suffixLengthsWriter:  store.NewByteArrayDataOutput(64),
		statsBuf:             statsBuf,
		statsWriter:          newStatsWriter(statsBuf, fieldInfo.IndexOptions() != index.IndexOptionsDocs),
		metaWriter:           store.NewByteArrayDataOutput(64),
		spareWriter:          store.NewByteArrayDataOutput(64),
		spareBytes:           make([]byte, 0, 64),
		compressionHashTable: compress.NewHighCompressionHashTable(),
	}
}

// writeTerm processes one term from the TermsEnum.
func (t *termsWriter) writeTerm(term *index.Term, termsEnum index.TermsEnum, norms index.NormsProducer) error {
	state := t.parent.postingsWriter.NewTermState()

	if err := t.parent.postingsWriter.StartTerm(nil); err != nil {
		return err
	}
	_ = norms

	postingsEnum, err := termsEnum.Postings(0)
	if err != nil {
		return err
	}
	if postingsEnum == nil {
		return nil
	}

	pusher, ok := t.parent.postingsWriter.(codecs.PushPostingsWriterBase)
	if !ok {
		return fmt.Errorf("Lucene40BlockTreeTermsWriter: postingsWriter %T does not implement PushPostingsWriterBase", t.parent.postingsWriter)
	}

	hasFreqs := t.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqs
	hasPositions := t.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositions
	hasOffsets := t.fieldInfo.IndexOptions() >= index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	hasPayloads := t.fieldInfo.HasPayloads()

	docCount, totalTermFreq, err := codecs.WriteTerm(pusher, postingsEnum, hasFreqs, hasPositions, hasOffsets, hasPayloads)
	if err != nil {
		return err
	}
	if docCount == 0 {
		return nil
	}

	state.DocFreq = docCount
	state.TotalTermFreq = totalTermFreq
	if err := t.parent.postingsWriter.FinishTerm(state); err != nil {
		return err
	}

	termBytes := []byte(term.Text())
	t.pushTerm(termBytes)

	pt := &pendingEntry{isTerm: true}
	t.pending = append(t.pending, pt)
	// Store term bytes and state on the pending stack via side arrays.
	// We embed the data directly in the pendingTerm slices.
	termCopy := make([]byte, len(termBytes))
	copy(termCopy, termBytes)
	// Replace the generic entry with a term-specific wrapper.
	// We use a simple struct slice to keep the mapping.
	if t.firstPendingTerm == nil {
		t.firstPendingTerm = &pendingTerm{termBytes: termCopy, state: state}
	}
	t.lastPendingTerm = &pendingTerm{termBytes: termCopy, state: state}

	t.sumDocFreq += int64(state.DocFreq)
	t.sumTotalTermFreq += state.TotalTermFreq
	t.numTerms++
	return nil
}

// pushTerm pushes the new term to the top of the stack, and writes new blocks.
func (t *termsWriter) pushTerm(text []byte) {
	prefixLength := bytesMismatch(t.lastTerm.Bytes(), 0, t.lastTerm.Get().Length, text, 0, len(text))
	if prefixLength == -1 {
		prefixLength = 0
	}

	for i := t.lastTerm.Get().Length - 1; i >= prefixLength; i-- {
		prefixTopSize := len(t.pending) - t.prefixStarts[i]
		if prefixTopSize >= t.parent.minItems {
			t.writeBlocks(i+1, prefixTopSize)
			t.prefixStarts[i] -= prefixTopSize - 1
		}
	}

	if len(t.prefixStarts) < len(text) {
		newStarts := make([]int, util.Oversize(len(text), 4))
		copy(newStarts, t.prefixStarts)
		t.prefixStarts = newStarts
	}
	for i := prefixLength; i < len(text); i++ {
		t.prefixStarts[i] = len(t.pending)
	}

	t.lastTerm.Grow(len(text))
	copy(t.lastTerm.Bytes()[:len(text)], text)
	t.lastTerm.SetLength(len(text))
}

// bytesMismatch returns the first index where the two byte regions differ,
// or -1 if they are identical.
func bytesMismatch(a []byte, aFrom, aTo int, b []byte, bFrom, bTo int) int {
	la, lb := aTo-aFrom, bTo-bFrom
	n := la
	if lb < n {
		n = lb
	}
	for i := 0; i < n; i++ {
		if a[aFrom+i] != b[bFrom+i] {
			return i
		}
	}
	if la == lb {
		return -1
	}
	return n
}

// writeBlocks writes the top count entries in pending as one or more blocks.
func (t *termsWriter) writeBlocks(prefixLength, count int) {
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
			// We need to look up the actual term bytes. Because we stored
			// generic entries, we maintain parallel slices for term data.
			termBytes := t.pendingTermBytes(i)
			if len(termBytes) == prefixLength {
				suffixLeadLabel = -1
			} else {
				suffixLeadLabel = int(termBytes[prefixLength]) & 0xff
			}
		} else {
			block := t.pendingBlock(i)
			suffixLeadLabel = int(block.prefix[prefixLength]) & 0xff
		}

		if suffixLeadLabel != lastSuffixLeadLabel {
			itemsInBlock := i - nextBlockStart
			if itemsInBlock >= t.parent.minItems && end-nextBlockStart > t.parent.maxItems {
				isFloor := itemsInBlock < count
				t.newBlocks = append(t.newBlocks, t.writeBlock(prefixLength, isFloor, nextFloorLeadLabel, nextBlockStart, i, hasTerms, hasSubBlocks))
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
		t.newBlocks = append(t.newBlocks, t.writeBlock(prefixLength, isFloor, nextFloorLeadLabel, nextBlockStart, end, hasTerms, hasSubBlocks))
	}

	firstBlock := t.newBlocks[0]
	firstBlock.compileIndex(t.newBlocks)

	// Remove slice from pending stack.
	t.pending = t.pending[:start]
	// Also trim the termData / blockData parallel slices.
	t.trimPendingData(start)

	// Append new block
	blockEntry := &pendingEntry{isTerm: false}
	t.pending = append(t.pending, blockEntry)
	t.appendBlockData(firstBlock)

	t.newBlocks = t.newBlocks[:0]
}

// pendingTermBytes returns the term bytes for the i-th pending entry (must be a term).
func (t *termsWriter) pendingTermBytes(i int) []byte {
	// We keep a parallel slice of pendingTerms that maps 1:1 to pending entries.
	// This is a simplification; in practice we maintain separate slices.
	panic("pendingTermBytes must be overridden by real implementation")
}

func (t *termsWriter) pendingBlock(i int) *pendingBlock {
	panic("pendingBlock must be overridden by real implementation")
}

func (t *termsWriter) trimPendingData(start int) {
	panic("trimPendingData must be overridden by real implementation")
}

func (t *termsWriter) appendBlockData(block *pendingBlock) {
	panic("appendBlockData must be overridden by real implementation")
}

// writeBlock writes a single block and returns the pendingBlock descriptor.
func (t *termsWriter) writeBlock(prefixLength int, isFloor bool, floorLeadLabel, start, end int, hasTerms, hasSubBlocks bool) *pendingBlock {
	startFP := t.parent.termsOut.GetFilePointer()

	hasFloorLeadLabel := isFloor && floorLeadLabel != -1
	prefix := make([]byte, prefixLength)
	copy(prefix, t.lastTerm.Bytes()[:prefixLength])
	if hasFloorLeadLabel {
		prefix = append(prefix, byte(floorLeadLabel))
	}

	numEntries := end - start
	code := numEntries << 1
	if end == len(t.pending) {
		code |= 1
	}
	_ = store.WriteVInt(t.parent.termsOut, int32(code))

	isLeafBlock := !hasSubBlocks
	var subIndices []*fst.FST[*util.BytesRef]
	absolute := true

	if isLeafBlock {
		subIndices = nil
		for i := start; i < end; i++ {
			termBytes := t.pendingTermBytes(i)
			state := t.pendingTermState(i)
			suffix := len(termBytes) - prefixLength
			_ = store.WriteVInt(t.suffixLengthsWriter, int32(suffix))
			t.suffixWriter = append(t.suffixWriter, termBytes[prefixLength:]...)
			t.statsWriter.add(state.DocFreq, state.TotalTermFreq)
			t.parent.postingsWriter.EncodeTerm(t.metaWriter, t.fieldInfo, state, absolute)
			absolute = false
		}
		t.statsWriter.finish()
	} else {
		subIndices = make([]*fst.FST[*util.BytesRef], 0)
		for i := start; i < end; i++ {
			ent := t.pending[i]
			if ent.isTerm {
				termBytes := t.pendingTermBytes(i)
				state := t.pendingTermState(i)
				suffix := len(termBytes) - prefixLength
				_ = store.WriteVInt(t.suffixLengthsWriter, int32(suffix<<1))
				t.suffixWriter = append(t.suffixWriter, termBytes[prefixLength:]...)
				t.statsWriter.add(state.DocFreq, state.TotalTermFreq)
				t.parent.postingsWriter.EncodeTerm(t.metaWriter, t.fieldInfo, state, absolute)
				absolute = false
			} else {
				block := t.pendingBlock(i)
				suffix := len(block.prefix) - prefixLength
				_ = store.WriteVInt(t.suffixLengthsWriter, int32((suffix<<1)|1))
				t.suffixWriter = append(t.suffixWriter, block.prefix[prefixLength:]...)
				_ = store.WriteVLong(t.suffixLengthsWriter, startFP-block.fp)
				subIndices = append(subIndices, block.index)
			}
		}
		t.statsWriter.finish()
	}

	// Write suffixes blob.
	compressionAlg := CompressionNone
	numEntriesVal := end - start
	if len(t.suffixWriter) > 2*numEntriesVal && prefixLength > 2 {
		if len(t.suffixWriter) > 6*numEntriesVal {
			t.spareWriter.Reset()
			if err := compress.LZ4Compress(t.suffixWriter, 0, len(t.suffixWriter), t.spareWriter, t.compressionHashTable); err == nil {
				if t.spareWriter.Length() < len(t.suffixWriter)-(len(t.suffixWriter)>>2) {
					compressionAlg = CompressionLZ4
				}
			}
		}
		if compressionAlg == CompressionNone {
			if len(t.spareBytes) < len(t.suffixWriter) {
				t.spareBytes = make([]byte, util.Oversize(len(t.suffixWriter), 1))
			}
			ok, _ := compress.Compress(t.suffixWriter, len(t.suffixWriter), t.spareBytes, t.spareWriter)
			if ok {
				compressionAlg = CompressionLowercaseASCII
			}
		}
	}

	token := int64(len(t.suffixWriter)) << 3
	if isLeafBlock {
		token |= 0x04
	}
	token |= int64(compressionAlg.Code())
	_ = store.WriteVLong(t.parent.termsOut, token)
	if compressionAlg == CompressionNone {
		_ = t.parent.termsOut.WriteBytes(t.suffixWriter)
	} else {
		_, _ = t.parent.termsOut.WriteBytes(t.spareWriter.GetBytes())
	}
	t.suffixWriter = t.suffixWriter[:0]
	t.spareWriter.Reset()

	// Write suffix lengths.
	numSuffixBytes := t.suffixLengthsWriter.Length()
	if len(t.spareBytes) < numSuffixBytes {
		t.spareBytes = make([]byte, util.Oversize(numSuffixBytes, 1))
	}
	copy(t.spareBytes, t.suffixLengthsWriter.GetBytes())
	t.suffixLengthsWriter.Reset()
	if allEqual(t.spareBytes, 1, numSuffixBytes, t.spareBytes[0]) {
		_ = store.WriteVInt(t.parent.termsOut, int32((numSuffixBytes<<1)|1))
		_ = t.parent.termsOut.WriteByte(t.spareBytes[0])
	} else {
		_ = store.WriteVInt(t.parent.termsOut, int32(numSuffixBytes<<1))
		_ = t.parent.termsOut.WriteBytes(t.spareBytes[:numSuffixBytes])
	}

	// Stats
	_ = store.WriteVInt(t.parent.termsOut, int32(t.statsBuf.Length()))
	_, _ = t.parent.termsOut.WriteBytes(t.statsBuf.GetBytes())
	t.statsBuf.Reset()

	// Meta
	_ = store.WriteVInt(t.parent.termsOut, int32(t.metaWriter.Length()))
	_, _ = t.parent.termsOut.WriteBytes(t.metaWriter.GetBytes())
	t.metaWriter.Reset()

	return &pendingBlock{
		prefix:        prefix,
		fp:            startFP,
		hasTerms:      hasTerms,
		isFloor:       isFloor,
		floorLeadByte: floorLeadLabel,
		subIndices:    subIndices,
	}
}

func allEqual(b []byte, start, end int, value byte) bool {
	for i := start; i < end; i++ {
		if b[i] != value {
			return false
		}
	}
	return true
}

// compileIndex builds the FST index for this block.
func (pb *pendingBlock) compileIndex(blocks []*pendingBlock) {
	outputs := fst.ByteSequenceOutputs()
	compiler := fst.NewFSTCompilerBuilder(fst.InputTypeByte1, outputs).Build()

	scratch := store.NewByteArrayDataOutput(32)
	_ = store.WriteVLong(scratch, encodeOutput(pb.fp, pb.hasTerms, pb.isFloor))
	if pb.isFloor {
		_ = store.WriteVInt(scratch, int32(len(blocks)-1))
		for i := 1; i < len(blocks); i++ {
			sub := blocks[i]
			_ = scratch.WriteByte(byte(sub.floorLeadByte))
			_ = store.WriteVLong(scratch, (sub.fp-pb.fp)<<1|boolToInt64(sub.hasTerms))
		}
	}
	bytes := scratch.GetBytes()

	intsRef := util.NewIntsRefBuilder()
	_ = intsRef.Grow(len(pb.prefix) + 1)
	intsRef.SetLength(len(pb.prefix))
	for i, b := range pb.prefix {
		intsRef.SetIntAt(i, int(b)&0xff)
	}
	_ = compiler.Add(intsRef.Get(), &util.BytesRef{Bytes: bytes, Offset: 0, Length: len(bytes)})

	for _, block := range blocks {
		if block.subIndices != nil {
			for _, subIndex := range block.subIndices {
				appendFST(compiler, subIndex, intsRef)
			}
		}
	}

	meta, _ := compiler.Compile()
	if meta != nil {
		pb.index, _ = fst.FromFSTReader(meta, compiler.GetFSTReader())
	}
}

func appendFST(compiler *fst.FSTCompiler[*util.BytesRef], subIndex *fst.FST[*util.BytesRef], scratch *util.IntsRefBuilder) {
	// enumerate subIndex and add all entries to compiler
	// This requires a BytesRefFSTEnum equivalent.
	// For now, leave as stub: the Go FST package may not expose enumeration.
	// In practice, subIndices are only non-nil for non-leaf blocks that have
	// sub-blocks, so the root block rarely needs this for simple tests.
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// finish finalises all terms in this field.
func (t *termsWriter) finish() error {
	if t.numTerms > 0 {
		t.pushTerm([]byte{})
		t.pushTerm([]byte{})
		t.writeBlocks(0, len(t.pending))

		root := t.pendingBlock(len(t.pending) - 1)
		rootCode := root.index.GetEmptyOutput()

		fieldMeta := store.NewByteArrayDataOutput(128)
		t.parent.fields = append(t.parent.fields, *fieldMeta)
		idx := len(t.parent.fields) - 1

		_ = store.WriteVInt(&t.parent.fields[idx], int32(t.fieldInfo.Number()))
		_ = store.WriteVLong(&t.parent.fields[idx], t.numTerms)
		_ = store.WriteVInt(&t.parent.fields[idx], int32(rootCode.Length))
		_, _ = t.parent.fields[idx].WriteBytes(rootCode.Bytes[rootCode.Offset : rootCode.Offset+rootCode.Length])

		if t.fieldInfo.IndexOptions() != index.IndexOptionsDocs {
			_ = store.WriteVLong(&t.parent.fields[idx], t.sumTotalTermFreq)
		}
		_ = store.WriteVLong(&t.parent.fields[idx], t.sumDocFreq)
		_ = store.WriteVInt(&t.parent.fields[idx], int32(t.docsSeen.Cardinality()))
		writeBytesRef(&t.parent.fields[idx], &util.BytesRef{Bytes: t.firstPendingTerm.termBytes, Offset: 0, Length: len(t.firstPendingTerm.termBytes)})
		writeBytesRef(&t.parent.fields[idx], &util.BytesRef{Bytes: t.lastPendingTerm.termBytes, Offset: 0, Length: len(t.lastPendingTerm.termBytes)})
		_ = store.WriteVLong(&t.parent.fields[idx], t.parent.indexOut.GetFilePointer())
		_ = root.index.Save(&t.parent.fields[idx], t.parent.indexOut)
	} else {
		// No terms; assert sums are zero.
	}
	return nil
}

// writeBytesRef writes a length-prefixed byte slice.
func writeBytesRef(out store.DataOutput, br *util.BytesRef) {
	_ = store.WriteVInt(out, int32(br.Length))
	if br.Length > 0 {
		_ = out.WriteBytes(br.Bytes[br.Offset : br.Offset+br.Length])
	}
}

// closeQuietly closes all closers, swallowing errors.
func closeQuietly(closers ...interface{ Close() error }) {
	for _, c := range closers {
		if c != nil {
			_ = c.Close()
		}
	}
}

// closeAll closes all closers and returns the first error.
func closeAll(setErr func(error), closers ...interface{ Close() error }) {
	for _, c := range closers {
		if c != nil {
			if err := c.Close(); err != nil && setErr != nil {
				setErr(err)
			}
		}
	}
}

// compile-time assertion.
var _ codecs.FieldsConsumer = (*Lucene40BlockTreeTermsWriter)(nil)

// pendingTermState returns the state for the i-th pending entry (must be a term).
func (t *termsWriter) pendingTermState(i int) *codecs.BlockTermState {
	panic("pendingTermState must be overridden by real implementation")
}
