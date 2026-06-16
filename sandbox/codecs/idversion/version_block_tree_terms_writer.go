// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.codecs.idversion.VersionBlockTreeTermsWriter.
package idversion

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// Wire-format constants for the VersionBlockTree terms dictionary.
//
// Mirrors the static finals in
// org.apache.lucene.sandbox.codecs.idversion.VersionBlockTreeTermsWriter.
const (
	// vbtTermsExtension is the file extension for the terms data file.
	vbtTermsExtension = "tiv"

	// vbtTermsCodecName is the codec name written in the .tiv header.
	vbtTermsCodecName = "VersionBlockTreeTermsDict"

	// vbtTermsIndexExtension is the file extension for the terms index file.
	vbtTermsIndexExtension = "tipv"

	// vbtTermsIndexCodecName is the codec name written in the .tipv header.
	vbtTermsIndexCodecName = "VersionBlockTreeTermsIndex"

	// vbtVersionStart is the first (and current) wire format version.
	vbtVersionStart = int32(1)

	// vbtVersionCurrent is the current wire format version.
	vbtVersionCurrent = vbtVersionStart

	// vbtDefaultMinBlockSize is the suggested minimum items-per-block.
	// Mirrors Lucene103BlockTreeTermsWriter.DEFAULT_MIN_BLOCK_SIZE.
	vbtDefaultMinBlockSize = 25

	// vbtDefaultMaxBlockSize is the suggested maximum items-per-block.
	// Mirrors Lucene103BlockTreeTermsWriter.DEFAULT_MAX_BLOCK_SIZE.
	vbtDefaultMaxBlockSize = 48

	// vbtOutputFlagsNumBitsW is the number of flag bits in the block-FP
	// output stored in the FST. Mirrors OUTPUT_FLAGS_NUM_BITS (= 2).
	vbtOutputFlagsNumBitsW = 2

	// vbtOutputFlagsMask masks the two flag bits.
	vbtOutputFlagsMask = 0x3

	// vbtOutputFlagIsFloorW marks the block as a floor block.
	vbtOutputFlagIsFloorW = 0x1

	// vbtOutputFlagHasTermsW marks the block as containing terms.
	vbtOutputFlagHasTermsW = 0x2
)

// vbtFSTOutputsW is the PairOutputs used by the writer for the FST index.
// Mirrors VersionBlockTreeTermsWriter.FST_OUTPUTS.
var vbtFSTOutputsW = fst.NewPairOutputs(
	fst.ByteSequenceOutputs(),
	fst.PositiveIntOutputs(),
)

// vbtNoOutputW is the no-output sentinel.
// Mirrors VersionBlockTreeTermsWriter.NO_OUTPUT.
var vbtNoOutputW = vbtFSTOutputsW.GetNoOutput()

// vbtFieldMetaData captures the per-field data written to the directory
// section at the end of .tiv and .tipv.
//
// Mirrors the private FieldMetaData record inside VersionBlockTreeTermsWriter.
type vbtFieldMetaData struct {
	fieldInfo    *index.FieldInfo
	rootCode     *fst.Pair[*util.BytesRef, int64]
	numTerms     int64
	indexStartFP int64
	minTerm      *util.BytesRef
	maxTerm      *util.BytesRef
}

// VersionBlockTreeTermsWriter writes the IDVersion block-tree term dictionary.
//
// The format is identical to Lucene103BlockTreeTermsWriter except for two
// differences:
//  1. The file extensions are .tiv (terms) and .tipv (terms index) instead
//     of .tim and .tip.
//  2. The FST output type is PairOutputs<BytesRef,Long> so that each index
//     node also records the maximum version in its sub-tree (enabling fast
//     version-gated seeks without loading blocks).
//
// Mirrors org.apache.lucene.sandbox.codecs.idversion.VersionBlockTreeTermsWriter.
type VersionBlockTreeTermsWriter struct {
	out      store.IndexOutput
	indexOut store.IndexOutput

	maxDoc          int
	minItemsInBlock int
	maxItemsInBlock int

	postingsWriter codecs.PushPostingsWriterBase
	fieldInfos     *schema.FieldInfos

	fields []*vbtFieldMetaData
	closed bool

	// scratchBytes is reused by compileIndex to serialise floor data blobs.
	scratchBytes *store.ByteBuffersDataOutput
	// scratchIntsRef is reused by compileIndex to convert BytesRef → IntsRef.
	scratchIntsRef *util.IntsRefBuilder
}

// NewVersionBlockTreeTermsWriter creates the writer, opening the .tiv and
// .tipv files and writing their codec headers.
//
// Mirrors VersionBlockTreeTermsWriter(SegmentWriteState, PostingsWriterBase, int, int).
func NewVersionBlockTreeTermsWriter(
	state *codecs.SegmentWriteState,
	postingsWriter codecs.PushPostingsWriterBase,
	minItemsInBlock, maxItemsInBlock int,
) (*VersionBlockTreeTermsWriter, error) {
	if err := validateBlockSizeSettings(minItemsInBlock, maxItemsInBlock); err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsWriter: %w", err)
	}

	termsFileName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, vbtTermsExtension,
	)
	rawOut, err := state.Directory.CreateOutput(termsFileName, store.IOContext{})
	if err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsWriter: create %s: %w", termsFileName, err)
	}
	// Wrap with a checksum layer so WriteFooter can embed the CRC.
	out := store.NewChecksumIndexOutput(rawOut)

	var rawIndexOut store.IndexOutput
	var indexOut store.IndexOutput
	success := false
	defer func() {
		if !success {
			closeQuietly(out, rawOut)
			closeQuietly(indexOut, rawIndexOut)
		}
	}()

	if err := codecs.WriteIndexHeader(
		out, vbtTermsCodecName, vbtVersionCurrent,
		state.SegmentInfo.GetID(), state.SegmentSuffix,
	); err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsWriter: write terms header: %w", err)
	}

	indexFileName := index.SegmentFileName(
		state.SegmentInfo.Name(), state.SegmentSuffix, vbtTermsIndexExtension,
	)
	rawIndexOut, err = state.Directory.CreateOutput(indexFileName, store.IOContext{})
	if err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsWriter: create %s: %w", indexFileName, err)
	}
	indexOut = store.NewChecksumIndexOutput(rawIndexOut)

	if err := codecs.WriteIndexHeader(
		indexOut, vbtTermsIndexCodecName, vbtVersionCurrent,
		state.SegmentInfo.GetID(), state.SegmentSuffix,
	); err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsWriter: write index header: %w", err)
	}

	if err := postingsWriter.Init(out, state); err != nil {
		return nil, fmt.Errorf("NewVersionBlockTreeTermsWriter: postingsWriter.Init: %w", err)
	}

	success = true
	return &VersionBlockTreeTermsWriter{
		out:             out,
		indexOut:        indexOut,
		maxDoc:          state.SegmentInfo.DocCount(),
		minItemsInBlock: minItemsInBlock,
		maxItemsInBlock: maxItemsInBlock,
		postingsWriter:  postingsWriter,
		fieldInfos:      state.FieldInfos,
		scratchBytes:    store.NewByteBuffersDataOutput(),
		scratchIntsRef:  util.NewIntsRefBuilder(),
	}, nil
}

// Write serialises all terms of field from terms into the on-disk files.
//
// Mirrors VersionBlockTreeTermsWriter.write(Fields, NormsProducer).
func (w *VersionBlockTreeTermsWriter) Write(field string, terms schema.Terms) error {
	if w.closed {
		return fmt.Errorf("VersionBlockTreeTermsWriter.Write: writer is closed")
	}

	fi := w.fieldInfos.GetByName(field)
	if fi == nil {
		return fmt.Errorf("VersionBlockTreeTermsWriter.Write: unknown field %q", field)
	}

	te, err := terms.GetIterator()
	if err != nil {
		return fmt.Errorf("VersionBlockTreeTermsWriter.Write: field %q GetIterator: %w", field, err)
	}

	tw := newVBTTermsWriter(w, fi)
	for {
		term, nerr := te.Next()
		if nerr != nil {
			return fmt.Errorf("VersionBlockTreeTermsWriter.Write: field %q Next: %w", field, nerr)
		}
		if term == nil {
			break
		}
		if werr := tw.writeTerm(term.BytesValue(), te); werr != nil {
			return fmt.Errorf("VersionBlockTreeTermsWriter.Write: field %q writeTerm: %w", field, werr)
		}
	}

	return tw.finish()
}

// Close flushes the per-field directory section and codec footers to both
// output files, then closes all resources.
//
// Mirrors VersionBlockTreeTermsWriter.close().
func (w *VersionBlockTreeTermsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	var firstErr error
	setErr := func(e error) {
		if firstErr == nil {
			firstErr = e
		}
	}

	success := false
	defer func() {
		if success {
			closeAll(setErr, w.out, w.indexOut, w.postingsWriter)
		} else {
			closeQuietly(w.out, w.indexOut, w.postingsWriter)
		}
	}()

	// Write the directory start (== current write pointer in .tiv).
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
		// rootCode.output1 is the BytesRef (encoded block FP + flags).
		rc1 := field.rootCode.Output1
		if err := store.WriteVInt(w.out, int32(rc1.Length)); err != nil {
			setErr(err)
			return firstErr
		}
		if err := w.out.WriteBytes(rc1.Bytes[rc1.Offset : rc1.Offset+rc1.Length]); err != nil {
			setErr(err)
			return firstErr
		}
		// rootCode.output2 is the maxVersion long.
		if err := store.WriteVLong(w.out, field.rootCode.Output2); err != nil {
			setErr(err)
			return firstErr
		}
		// indexStartFP goes into the index file.
		if err := store.WriteVLong(w.indexOut, field.indexStartFP); err != nil {
			setErr(err)
			return firstErr
		}
		// minTerm / maxTerm.
		if err := writeBytesRefVBT(w.out, field.minTerm); err != nil {
			setErr(err)
			return firstErr
		}
		if err := writeBytesRefVBT(w.out, field.maxTerm); err != nil {
			setErr(err)
			return firstErr
		}
	}

	// Write trailing directory offset (seekDir pointer).
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

// closeQuietly closes all closers, silently dropping any errors.
func closeQuietly(closers ...interface{ Close() error }) {
	for _, c := range closers {
		if c != nil {
			_ = c.Close()
		}
	}
}

// closeAll closes all closers, collecting the first non-nil error.
func closeAll(setErr func(error), closers ...interface{ Close() error }) {
	for _, c := range closers {
		if c != nil {
			setErr(c.Close())
		}
	}
}

// newFixedBitSetOrPanic allocates a FixedBitSet of numBits and panics on error.
// Used only for internal book-keeping where the size is always valid.
func newFixedBitSetOrPanic(numBits int) *util.FixedBitSet {
	bs, err := util.NewFixedBitSet(numBits)
	if err != nil {
		panic(fmt.Sprintf("newFixedBitSetOrPanic: %v", err))
	}
	return bs
}

// writeBytesRefVBT writes a BytesRef as (vint length, raw bytes).
func writeBytesRefVBT(out store.IndexOutput, b *util.BytesRef) error {
	if b == nil {
		return store.WriteVInt(out, 0)
	}
	if err := store.WriteVInt(out, int32(b.Length)); err != nil {
		return err
	}
	return out.WriteBytes(b.Bytes[b.Offset : b.Offset+b.Length])
}

// vbtEncodeOutput encodes a file pointer and two flag bits into a single long
// for inclusion in an FST output BytesRef.
//
// Mirrors VersionBlockTreeTermsWriter.encodeOutput(long, boolean, boolean).
func vbtEncodeOutput(fp int64, hasTerms, isFloor bool) int64 {
	out := fp << vbtOutputFlagsNumBitsW
	if hasTerms {
		out |= vbtOutputFlagHasTermsW
	}
	if isFloor {
		out |= vbtOutputFlagIsFloorW
	}
	return out
}

// ─────────────────────────── PendingEntry ────────────────────────────────

// vbtPendingEntry is the interface shared by vbtPendingTerm and vbtPendingBlock.
// Mirrors VersionBlockTreeTermsWriter.PendingEntry.
type vbtPendingEntry interface {
	isTerm() bool
}

// vbtPendingTerm records one term and its postings state.
// Mirrors VersionBlockTreeTermsWriter.PendingTerm.
type vbtPendingTerm struct {
	termBytes []byte
	state     *codecs.BlockTermState
}

func (*vbtPendingTerm) isTerm() bool { return true }

// vbtPendingBlock records one pending sub-block (with its compiled FST index).
// Mirrors VersionBlockTreeTermsWriter.PendingBlock.
type vbtPendingBlock struct {
	prefix     *util.BytesRef
	fp         int64
	index      *fst.FST[*fst.Pair[*util.BytesRef, int64]]
	subIndices []*fst.FST[*fst.Pair[*util.BytesRef, int64]]
	hasTerms   bool
	isFloor    bool
	// floorLeadByte is -1 for the first (or only) block of a floor group.
	floorLeadByte int
	maxVersion    int64
}

func (*vbtPendingBlock) isTerm() bool { return false }

// compileIndex builds the FST for this (possibly floor) block group.
// Mirrors VersionBlockTreeTermsWriter.PendingBlock.compileIndex.
func (b *vbtPendingBlock) compileIndex(
	blocks []*vbtPendingBlock,
	scratch *store.ByteBuffersDataOutput,
	scratchInts *util.IntsRefBuilder,
	outputs *fst.PairOutputsImpl[*util.BytesRef, int64],
) error {
	encoded := vbtEncodeOutput(b.fp, b.hasTerms, b.isFloor)
	if err := scratch.WriteVLong(encoded); err != nil {
		return fmt.Errorf("compileIndex: write encoded FP: %w", err)
	}

	maxVersionIndex := b.maxVersion
	if b.isFloor {
		if err := store.WriteVInt(scratch, int32(len(blocks)-1)); err != nil {
			return fmt.Errorf("compileIndex: write floor count: %w", err)
		}
		for i := 1; i < len(blocks); i++ {
			sub := blocks[i]
			if sub.maxVersion > maxVersionIndex {
				maxVersionIndex = sub.maxVersion
			}
			if err := scratch.WriteByte(byte(sub.floorLeadByte)); err != nil {
				return fmt.Errorf("compileIndex: write floor lead byte: %w", err)
			}
			delta := (sub.fp - b.fp) << 1
			if sub.hasTerms {
				delta |= 1
			}
			if err := scratch.WriteVLong(delta); err != nil {
				return fmt.Errorf("compileIndex: write floor delta: %w", err)
			}
		}
	}

	raw := scratch.ToArrayCopy()
	if len(raw) == 0 {
		return fmt.Errorf("compileIndex: empty scratch buffer")
	}
	scratch.Reset()

	// Build the FST: one entry keyed by prefix → PairOutputs(raw, maxVersion).
	compiler := fst.NewFSTCompilerBuilder[*fst.Pair[*util.BytesRef, int64]](
		fst.InputTypeByte1, outputs,
	).Build()

	inputRef := &util.BytesRef{Bytes: b.prefix.Bytes, Offset: b.prefix.Offset, Length: b.prefix.Length}
	intsInput := fst.ToIntsRef(inputRef, scratchInts)
	// Lucene stores Long.MAX_VALUE - maxVersionIndex in output2 so that
	// higher versions have lower output values (enabling min-output semantics).
	pairOut := outputs.NewPair(
		util.NewBytesRef(raw),
		int64(math.MaxInt64)-maxVersionIndex,
	)
	if err := compiler.Add(intsInput, pairOut); err != nil {
		return fmt.Errorf("compileIndex: Add prefix entry: %w", err)
	}

	// Merge in all sub-indices.
	for _, blk := range blocks {
		if blk.subIndices == nil {
			continue
		}
		for _, subIdx := range blk.subIndices {
			if appendErr := appendIndex(compiler, subIdx, scratchInts); appendErr != nil {
				return fmt.Errorf("compileIndex: append sub-index: %w", appendErr)
			}
		}
		blk.subIndices = nil
	}

	meta, cerr := compiler.Compile()
	if cerr != nil {
		return fmt.Errorf("compileIndex: Compile: %w", cerr)
	}
	if meta == nil {
		return fmt.Errorf("compileIndex: Compile returned nil metadata (no entries)")
	}

	var ferr error
	b.index, ferr = fst.FromFSTReader[*fst.Pair[*util.BytesRef, int64]](meta, compiler.GetFSTReader())
	if ferr != nil {
		return fmt.Errorf("compileIndex: FromFSTReader: %w", ferr)
	}
	b.subIndices = nil
	return nil
}

// appendIndex merges all entries from subIndex into the FSTCompiler.
func appendIndex(
	compiler *fst.FSTCompiler[*fst.Pair[*util.BytesRef, int64]],
	subIndex *fst.FST[*fst.Pair[*util.BytesRef, int64]],
	scratchInts *util.IntsRefBuilder,
) error {
	enum, err := fst.NewBytesRefFSTEnum[*fst.Pair[*util.BytesRef, int64]](subIndex)
	if err != nil {
		return fmt.Errorf("appendIndex: NewBytesRefFSTEnum: %w", err)
	}
	for {
		ent, nerr := enum.Next()
		if nerr != nil {
			return fmt.Errorf("appendIndex: Next: %w", nerr)
		}
		if ent == nil {
			break
		}
		intsInput := fst.ToIntsRef(ent.Input, scratchInts)
		if aerr := compiler.Add(intsInput, ent.Output); aerr != nil {
			return fmt.Errorf("appendIndex: Add: %w", aerr)
		}
	}
	return nil
}

// ──────────────────────── TermsWriter ────────────────────────────────────

// vbtTermsWriter handles one field at a time.
// Mirrors VersionBlockTreeTermsWriter.TermsWriter.
type vbtTermsWriter struct {
	parent   *VersionBlockTreeTermsWriter
	fi       *index.FieldInfo
	docsSeen *util.FixedBitSet
	numTerms int64

	pending      []vbtPendingEntry
	prefixStarts []int
	lastTerm     *util.BytesRefBuilder
	newBlocks    []*vbtPendingBlock

	firstPendingTerm *vbtPendingTerm
	lastPendingTerm  *vbtPendingTerm

	indexStartFP int64

	suffixWriter *store.ByteBuffersDataOutput
	metaWriter   *store.ByteBuffersDataOutput
}

// newVBTTermsWriter constructs a per-field writer.
func newVBTTermsWriter(parent *VersionBlockTreeTermsWriter, fi *index.FieldInfo) *vbtTermsWriter {
	tw := &vbtTermsWriter{
		parent:       parent,
		fi:           fi,
		docsSeen:     newFixedBitSetOrPanic(parent.maxDoc),
		prefixStarts: make([]int, 8),
		lastTerm:     &util.BytesRefBuilder{},
		suffixWriter: store.NewByteBuffersDataOutput(),
		metaWriter:   store.NewByteBuffersDataOutput(),
	}
	// Notify the postings writer that we are switching to this field.
	_, _ = parent.postingsWriter.SetField(fi)
	return tw
}

// writeTerm processes one term from the TermsEnum.
// Mirrors VersionBlockTreeTermsWriter.TermsWriter.write(BytesRef, TermsEnum, NormsProducer).
func (tw *vbtTermsWriter) writeTerm(term *util.BytesRef, termsEnum schema.TermsEnum) error {
	pw := tw.parent.postingsWriter

	state := pw.NewTermState()
	if err := pw.StartTerm(nil); err != nil {
		return fmt.Errorf("writeTerm: StartTerm: %w", err)
	}

	postingsFlags := index.PostingsFlagPositions | index.PostingsFlagPayloads
	postingsEnum, err := termsEnum.Postings(postingsFlags)
	if err != nil {
		return fmt.Errorf("writeTerm: Postings: %w", err)
	}

	fi := tw.fi
	hasFreqs := fi.IndexOptions() >= schema.IndexOptionsDocsAndFreqs
	hasPositions := fi.IndexOptions() >= schema.IndexOptionsDocsAndFreqsAndPositions
	hasOffsets := fi.IndexOptions() >= schema.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
	hasPayloads := fi.HasPayloads()

	_, _, werr := codecs.WriteTerm(pw, postingsEnum, hasFreqs, hasPositions, hasOffsets, hasPayloads, nil)
	if werr != nil {
		return fmt.Errorf("writeTerm: WriteTerm: %w", werr)
	}

	// Retrieve the per-term sidecar to check if the doc was alive.
	extra := globalTermStateRegistry.lookup(state)
	if extra == nil || extra.DocID == -1 {
		// Term had no live documents (deleted); skip it.
		return nil
	}

	state.DocFreq = 1
	state.TotalTermFreq = 1
	if err := pw.FinishTerm(state); err != nil {
		return fmt.Errorf("writeTerm: FinishTerm: %w", err)
	}

	tw.pushTerm(term)

	termBytes := make([]byte, term.Length)
	copy(termBytes, term.Bytes[term.Offset:term.Offset+term.Length])
	pt := &vbtPendingTerm{termBytes: termBytes, state: state}
	tw.pending = append(tw.pending, pt)
	tw.numTerms++
	if tw.firstPendingTerm == nil {
		tw.firstPendingTerm = pt
	}
	tw.lastPendingTerm = pt
	return nil
}

// pushTerm updates prefixStarts and flushes blocks when a prefix ends.
// Mirrors VersionBlockTreeTermsWriter.TermsWriter.pushTerm.
func (tw *vbtTermsWriter) pushTerm(text *util.BytesRef) {
	limit := tw.lastTerm.Length()
	if text.Length < limit {
		limit = text.Length
	}

	pos := 0
	for pos < limit && tw.lastTerm.ByteAt(pos) == text.Bytes[text.Offset+pos] {
		pos++
	}

	for i := tw.lastTerm.Length() - 1; i >= pos; i-- {
		topSize := len(tw.pending) - tw.prefixStarts[i]
		if topSize >= tw.parent.minItemsInBlock {
			_ = tw.writeBlocks(i+1, topSize)
			tw.prefixStarts[i] -= topSize - 1
		}
	}

	if len(tw.prefixStarts) < text.Length {
		grown := make([]int, util.Oversize(text.Length, 1))
		copy(grown, tw.prefixStarts)
		tw.prefixStarts = grown
	}

	for i := pos; i < text.Length; i++ {
		tw.prefixStarts[i] = len(tw.pending)
	}
	tw.lastTerm.CopyBytesRef(text)
}

// writeBlocks writes the top count entries from the pending stack as one or
// more (possibly floor) blocks.
// Mirrors VersionBlockTreeTermsWriter.TermsWriter.writeBlocks(int, int).
func (tw *vbtTermsWriter) writeBlocks(prefixLength, count int) error {
	lastSuffixLeadLabel := -1
	hasTerms := false
	hasSubBlocks := false

	start := len(tw.pending) - count
	end := len(tw.pending)
	nextBlockStart := start
	nextFloorLeadLabel := -1

	for i := start; i < end; i++ {
		ent := tw.pending[i]
		var suffixLeadLabel int
		if ent.isTerm() {
			pt := ent.(*vbtPendingTerm)
			if len(pt.termBytes) == prefixLength {
				suffixLeadLabel = -1
			} else {
				suffixLeadLabel = int(pt.termBytes[prefixLength]) & 0xff
			}
		} else {
			pb := ent.(*vbtPendingBlock)
			suffixLeadLabel = int(pb.prefix.Bytes[pb.prefix.Offset+prefixLength]) & 0xff
		}

		if suffixLeadLabel != lastSuffixLeadLabel {
			itemsInBlock := i - nextBlockStart
			if itemsInBlock >= tw.parent.minItemsInBlock && end-nextBlockStart > tw.parent.maxItemsInBlock {
				isFloor := itemsInBlock < count
				blk, err := tw.writeBlock(prefixLength, isFloor, nextFloorLeadLabel, nextBlockStart, i, hasTerms, hasSubBlocks)
				if err != nil {
					return err
				}
				tw.newBlocks = append(tw.newBlocks, blk)
				hasTerms = false
				hasSubBlocks = false
				nextFloorLeadLabel = suffixLeadLabel
				nextBlockStart = i
			}
			lastSuffixLeadLabel = suffixLeadLabel
		}

		if ent.isTerm() {
			hasTerms = true
		} else {
			hasSubBlocks = true
		}
	}

	if nextBlockStart < end {
		itemsInBlock := end - nextBlockStart
		isFloor := itemsInBlock < count
		blk, err := tw.writeBlock(prefixLength, isFloor, nextFloorLeadLabel, nextBlockStart, end, hasTerms, hasSubBlocks)
		if err != nil {
			return err
		}
		tw.newBlocks = append(tw.newBlocks, blk)
	}

	if len(tw.newBlocks) == 0 {
		return fmt.Errorf("writeBlocks: newBlocks is empty")
	}

	firstBlock := tw.newBlocks[0]
	if err := firstBlock.compileIndex(
		tw.newBlocks, tw.parent.scratchBytes, tw.parent.scratchIntsRef, vbtFSTOutputsW,
	); err != nil {
		return fmt.Errorf("writeBlocks: compileIndex: %w", err)
	}

	tw.pending = tw.pending[:len(tw.pending)-count]
	tw.pending = append(tw.pending, firstBlock)
	tw.newBlocks = tw.newBlocks[:0]
	return nil
}

// writeBlock writes one block (leaf or non-leaf) to the .tiv file.
// Mirrors VersionBlockTreeTermsWriter.TermsWriter.writeBlock(…).
func (tw *vbtTermsWriter) writeBlock(
	prefixLength int,
	isFloor bool,
	floorLeadLabel,
	start, end int,
	hasTerms, hasSubBlocks bool,
) (*vbtPendingBlock, error) {
	out := tw.parent.out
	startFP := out.GetFilePointer()

	hasFloorLeadLabel := isFloor && floorLeadLabel != -1
	prefixLen := prefixLength
	if hasFloorLeadLabel {
		prefixLen++
	}

	// Build the prefix BytesRef.
	prefixBytes := make([]byte, prefixLen)
	if prefixLength > 0 {
		copy(prefixBytes, tw.lastTerm.Bytes()[:prefixLength])
	}
	prefix := &util.BytesRef{Bytes: prefixBytes, Offset: 0, Length: prefixLength}

	numEntries := end - start
	code := int32(numEntries << 1)
	if end == len(tw.pending) {
		code |= 1 // last block
	}
	if err := store.WriteVInt(out, code); err != nil {
		return nil, fmt.Errorf("writeBlock: write entCount: %w", err)
	}

	isLeafBlock := !hasSubBlocks
	var subIndices []*fst.FST[*fst.Pair[*util.BytesRef, int64]]
	absolute := true
	maxVersionInBlock := int64(-1)

	if isLeafBlock {
		// Leaf: only terms.
		for i := start; i < end; i++ {
			pt := tw.pending[i].(*vbtPendingTerm)
			extra := globalTermStateRegistry.lookup(pt.state)
			if extra != nil && extra.IDVersion > maxVersionInBlock {
				maxVersionInBlock = extra.IDVersion
			}
			suffix := len(pt.termBytes) - prefixLength
			if err := store.WriteVInt(tw.suffixWriter, int32(suffix)); err != nil {
				return nil, fmt.Errorf("writeBlock (leaf): write suffix len: %w", err)
			}
			if err := tw.suffixWriter.WriteBytes(pt.termBytes[prefixLength : prefixLength+suffix]); err != nil {
				return nil, fmt.Errorf("writeBlock (leaf): write suffix bytes: %w", err)
			}
			if err := tw.parent.postingsWriter.EncodeTerm(byteBuffersIndexOutputAdapter{tw.metaWriter}, tw.fi, pt.state, absolute); err != nil {
				return nil, fmt.Errorf("writeBlock (leaf): EncodeTerm: %w", err)
			}
			absolute = false
		}
	} else {
		// Non-leaf: mixed terms and sub-blocks.
		for i := start; i < end; i++ {
			ent := tw.pending[i]
			if ent.isTerm() {
				pt := ent.(*vbtPendingTerm)
				extra := globalTermStateRegistry.lookup(pt.state)
				if extra != nil && extra.IDVersion > maxVersionInBlock {
					maxVersionInBlock = extra.IDVersion
				}
				suffix := len(pt.termBytes) - prefixLength
				// Borrow LSB=0 to signal "term".
				if err := store.WriteVInt(tw.suffixWriter, int32(suffix<<1)); err != nil {
					return nil, fmt.Errorf("writeBlock (non-leaf term): write suffix: %w", err)
				}
				if err := tw.suffixWriter.WriteBytes(pt.termBytes[prefixLength : prefixLength+suffix]); err != nil {
					return nil, fmt.Errorf("writeBlock (non-leaf term): write suffix bytes: %w", err)
				}
				if err := tw.parent.postingsWriter.EncodeTerm(byteBuffersIndexOutputAdapter{tw.metaWriter}, tw.fi, pt.state, absolute); err != nil {
					return nil, fmt.Errorf("writeBlock (non-leaf term): EncodeTerm: %w", err)
				}
				absolute = false
			} else {
				pb := ent.(*vbtPendingBlock)
				if pb.maxVersion > maxVersionInBlock {
					maxVersionInBlock = pb.maxVersion
				}
				suffix := pb.prefix.Length - prefixLength
				// Borrow LSB=1 to signal "sub-block".
				if err := store.WriteVInt(tw.suffixWriter, int32((suffix<<1)|1)); err != nil {
					return nil, fmt.Errorf("writeBlock (non-leaf block): write suffix: %w", err)
				}
				if err := tw.suffixWriter.WriteBytes(pb.prefix.Bytes[prefixLength : prefixLength+suffix]); err != nil {
					return nil, fmt.Errorf("writeBlock (non-leaf block): write suffix bytes: %w", err)
				}
				delta := startFP - pb.fp
				if err := tw.suffixWriter.WriteVLong(delta); err != nil {
					return nil, fmt.Errorf("writeBlock (non-leaf block): write delta: %w", err)
				}
				subIndices = append(subIndices, pb.index)
			}
		}
		if len(subIndices) == 0 {
			return nil, fmt.Errorf("writeBlock: non-leaf block has no sub-indices")
		}
	}

	// Write suffix blob: (size<<1 | isLeaf).
	suffixCode := int32(tw.suffixWriter.Size()<<1) | 0
	if isLeafBlock {
		suffixCode |= 1
	}
	if err := store.WriteVInt(out, suffixCode); err != nil {
		return nil, fmt.Errorf("writeBlock: write suffix code: %w", err)
	}
	if err := tw.suffixWriter.CopyTo(out); err != nil {
		return nil, fmt.Errorf("writeBlock: copy suffix: %w", err)
	}
	tw.suffixWriter.Reset()

	// Write meta blob.
	if err := store.WriteVInt(out, int32(tw.metaWriter.Size())); err != nil {
		return nil, fmt.Errorf("writeBlock: write meta size: %w", err)
	}
	if err := tw.metaWriter.CopyTo(out); err != nil {
		return nil, fmt.Errorf("writeBlock: copy meta: %w", err)
	}
	tw.metaWriter.Reset()

	// Append floor lead label to prefix if needed.
	if hasFloorLeadLabel {
		prefix.Bytes[prefixLength] = byte(floorLeadLabel)
		prefix.Length = prefixLen
	}

	return &vbtPendingBlock{
		prefix:        prefix,
		maxVersion:    maxVersionInBlock,
		fp:            startFP,
		hasTerms:      hasTerms,
		isFloor:       isFloor,
		floorLeadByte: floorLeadLabel,
		subIndices:    subIndices,
	}, nil
}

// finish flushes the root block and records field metadata.
// Mirrors VersionBlockTreeTermsWriter.TermsWriter.finish().
func (tw *vbtTermsWriter) finish() error {
	if tw.numTerms == 0 {
		return nil
	}

	// Flush remaining pending entries as root block.
	if err := tw.writeBlocks(0, len(tw.pending)); err != nil {
		return fmt.Errorf("finish: writeBlocks root: %w", err)
	}

	if len(tw.pending) != 1 || tw.pending[0].isTerm() {
		return fmt.Errorf("finish: expected exactly one root block, got %d entries", len(tw.pending))
	}
	root := tw.pending[0].(*vbtPendingBlock)
	if root.prefix.Length != 0 {
		return fmt.Errorf("finish: root block prefix length must be 0, got %d", root.prefix.Length)
	}
	if root.index == nil {
		return fmt.Errorf("finish: root block has nil FST index")
	}

	// Save the FST index to the .tipv file.
	tw.indexStartFP = tw.parent.indexOut.GetFilePointer()
	if err := root.index.Save(tw.parent.indexOut, tw.parent.indexOut); err != nil {
		return fmt.Errorf("finish: save FST: %w", err)
	}

	// Build minTerm / maxTerm from first/last pending term bytes.
	minTerm := util.NewBytesRef(tw.firstPendingTerm.termBytes)
	maxTerm := util.NewBytesRef(tw.lastPendingTerm.termBytes)

	rootCode, hasEmpty := root.index.GetEmptyOutput()
	if !hasEmpty {
		return fmt.Errorf("finish: root FST has no empty output (root block prefix must be empty string)")
	}

	tw.parent.fields = append(tw.parent.fields, &vbtFieldMetaData{
		fieldInfo:    tw.fi,
		rootCode:     rootCode,
		numTerms:     tw.numTerms,
		indexStartFP: tw.indexStartFP,
		minTerm:      minTerm,
		maxTerm:      maxTerm,
	})
	return nil
}

// ─────────────────────── IndexOutput adapter ─────────────────────────────────

// byteBuffersIndexOutputAdapter is a thin shim that lets *store.ByteBuffersDataOutput
// satisfy the store.IndexOutput interface. Only the write methods needed by
// EncodeTerm are forwarded; the rest are no-ops / zero-returns.
//
// Mirrors the package-private byteBuffersDataOutputAsIndexOutput in
// codecs.Lucene103BlockTreeTermsWriter.
type byteBuffersIndexOutputAdapter struct {
	inner *store.ByteBuffersDataOutput
}

var _ store.IndexOutput = byteBuffersIndexOutputAdapter{}

func (a byteBuffersIndexOutputAdapter) WriteByte(b byte) error    { return a.inner.WriteByte(b) }
func (a byteBuffersIndexOutputAdapter) WriteBytes(b []byte) error { return a.inner.WriteBytes(b) }
func (a byteBuffersIndexOutputAdapter) WriteBytesN(b []byte, n int) error {
	return a.inner.WriteBytesN(b, n)
}
func (a byteBuffersIndexOutputAdapter) WriteShort(v int16) error { return a.inner.WriteShort(v) }
func (a byteBuffersIndexOutputAdapter) WriteInt(v int32) error   { return a.inner.WriteInt(v) }
func (a byteBuffersIndexOutputAdapter) WriteLong(v int64) error  { return a.inner.WriteLong(v) }
func (a byteBuffersIndexOutputAdapter) WriteString(s string) error {
	return a.inner.WriteString(s)
}
func (a byteBuffersIndexOutputAdapter) WriteVInt(i int32) error  { return a.inner.WriteVInt(i) }
func (a byteBuffersIndexOutputAdapter) WriteVLong(i int64) error { return a.inner.WriteVLong(i) }

// GetFilePointer returns the current size of the buffer as a proxy for the
// write position (ByteBuffersDataOutput is append-only).
func (a byteBuffersIndexOutputAdapter) GetFilePointer() int64 { return a.inner.Size() }

// Length, SetPosition, Close, and GetName satisfy the store.RandomAccess,
// store.Closable, and store.NamedOutput interfaces. All are no-ops or
// proxies because this adapter is used only as a scratch write buffer.
func (a byteBuffersIndexOutputAdapter) Length() int64             { return a.inner.Size() }
func (a byteBuffersIndexOutputAdapter) SetPosition(_ int64) error { return nil }
func (a byteBuffersIndexOutputAdapter) Close() error              { return nil }
func (a byteBuffersIndexOutputAdapter) GetName() string           { return "<scratch>" }

// ─── compile-time interface assertions ───────────────────────────────────────

var _ codecs.FieldsConsumer = (*VersionBlockTreeTermsWriter)(nil)
