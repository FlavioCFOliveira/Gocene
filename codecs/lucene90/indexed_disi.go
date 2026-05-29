// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//   Licensed to the Apache Software Foundation (ASF) under one or more
//   contributor license agreements. See the NOTICE file distributed with
//   this work for additional information regarding copyright ownership.
//   The ASF licenses this file to You under the Apache License, Version
//   2.0 (the "License"); you may not use this file except in compliance
//   with the License. You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//   implied. See the License for the specific language governing
//   permissions and limitations under the License.

package lucene90

import (
	"errors"
	"fmt"
	"io"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// IndexedDISI is the disk-based DocIdSetIterator from
// org.apache.lucene.codecs.lucene90.IndexedDISI. The on-disk format encodes
// the doc-id stream as a sequence of logical 65536-doc blocks; each block
// independently picks one of three encodings depending on its density:
//
//   - ALL    — the block contains exactly 65536 docs (header only, no body).
//   - DENSE  — 4096 or more docs; encoded as a 1024-long FixedBitSet, with an
//     optional rank table for sub-block lookups.
//   - SPARSE — otherwise; the lower 16 bits of each doc id are stored as a
//     short and consumed by binary scan.
//
// An optional jump-table (one (index, offset) int pair per block) is stored
// at the end of the slice so callers can skip whole blocks in O(1). When
// jumpTableEntryCount <= 0 there is no jump table.
//
// This iterator additionally exposes the ordinal of the current document
// (the count of set bits before it) via Index, which is the property that
// makes it useful for sparse doc-values consumers.
//
// Wire-format parity: the byte stream produced by WriteBitSet is identical
// to the Apache Lucene 10.4.0 reference. All multi-byte numerics are
// little-endian on the wire (IndexInput.readShort/readInt/readLong are
// little-endian in Lucene 10.x's data-input contract; see rmp #4786).
//
// Deviations from the Java reference (documented):
//
//   - The `intoBitSet` family is not ported. Gocene's
//     search.DocIdSetIterator interface does not yet expose IntoBitSet, and
//     the production consumers we have (sparse doc-values readers) drive
//     IndexedDISI via NextDoc / Advance / Index. IntoBitSet returns
//     ErrIntoBitSetNotSupported when invoked.
//   - The KnnVectorValues.DocIndexIterator adapter is omitted because no
//     consumer in Gocene currently uses it; the equivalent is a thin
//     wrapper that anyone can write in two lines around an *IndexedDISI.
//   - `IndexInput.prefetch` is invoked through a type-assert against the
//     optional store.PrefetchableIndexInput interface (Lucene 10.4.0 has
//     prefetch as a default no-op on RandomAccessInput / IndexInput).
type IndexedDISI struct {
	// slice is the per-DISI IndexInput that holds the data blocks (without
	// the jump-table). All seek positions are relative to its start.
	slice store.IndexInput
	// jumpTable provides absolute (index, offset) jump entries for blocks;
	// may be nil when jumpTableEntryCount <= 0.
	jumpTable           store.RandomAccessInput
	jumpTableEntryCount int
	// denseRankPower is the log2 of the doc-id stride covered by each rank
	// entry in DENSE blocks (e.g. 9 = every 512 docs). -1 disables ranks.
	denseRankPower byte
	denseRankTable []byte
	cost           int64

	// Iterator state — mutates on every call.
	doc            int   // current docID; -1 = unstarted, NO_MORE_DOCS = exhausted
	block          int   // the high 16 bits of the current doc, or -1
	blockEnd       int64 // file position immediately after the current block body
	denseBitmapOff int64 // file position of the first long of the DENSE bitmap (DENSE only)
	nextBlockIndex int
	method         indexedDISIMethod
	index          int

	// SPARSE state
	exists              bool
	nextExistDocInBlock int

	// DENSE state
	word          uint64
	wordIndex     int
	numberOfOnes  int
	denseOrigoIdx int

	// ALL state
	gap int
}

// blockSize is the doc-id span covered by a single logical block.
const blockSize = 65536

// denseBlockLongs is the number of 64-bit words a DENSE bitset stores per
// block (65536 / 64).
const denseBlockLongs = blockSize / 64 // 1024

// DefaultDenseRankPower is the recommended rank-power for DENSE blocks: one
// rank entry every 512 docIDs (8 longs). Matches Lucene's
// IndexedDISI.DEFAULT_DENSE_RANK_POWER.
const DefaultDenseRankPower byte = 9

// maxArrayLength is the SPARSE/DENSE switchover threshold: blocks with at
// most maxArrayLength docs are SPARSE; larger ones are DENSE or ALL.
const maxArrayLength = (1 << 12) - 1 // 4095

// noMoreDocs is the sentinel returned by Lucene's DocIdSetIterator when the
// stream is exhausted; mirrors search.NO_MORE_DOCS.
const noMoreDocs = search.NO_MORE_DOCS // 2^31 - 1

// ErrIntoBitSetNotSupported is returned when a caller invokes the
// IntoBitSet hook. The Lucene 10.4.0 IndexedDISI provides this entry point
// but the Gocene DocIdSetIterator surface does not (yet) expose it; the
// existing IndexedDISI consumers in the Go port do not exercise this path.
var ErrIntoBitSetNotSupported = errors.New("lucene90: IndexedDISI.IntoBitSet is not yet implemented")

// writeShortLE / writeIntLE / writeLongLE emit little-endian multi-byte
// integers via the DataOutput's WriteByte primitive. The wire format of
// Lucene 10+ is little-endian for all multi-byte numerics. As of rmp #4786
// the store IndexOutput.WriteShort/WriteInt/WriteLong are also little-endian,
// so these helpers are now equivalent to the bare methods; they are retained
// because this is wire-format-sensitive code where explicit byte order is
// clearer than relying on the (now correct) interface default.
func writeShortLE(out store.IndexOutput, v int16) error {
	uv := uint16(v)
	if err := out.WriteByte(byte(uv)); err != nil {
		return err
	}
	return out.WriteByte(byte(uv >> 8))
}

func writeIntLE(out store.IndexOutput, v int32) error {
	uv := uint32(v)
	if err := out.WriteByte(byte(uv)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(uv >> 8)); err != nil {
		return err
	}
	if err := out.WriteByte(byte(uv >> 16)); err != nil {
		return err
	}
	return out.WriteByte(byte(uv >> 24))
}

func writeLongLE(out store.IndexOutput, v int64) error {
	uv := uint64(v)
	for i := 0; i < 8; i++ {
		if err := out.WriteByte(byte(uv >> (8 * uint(i)))); err != nil {
			return err
		}
	}
	return nil
}

// indexedDISIMethod identifies the per-block encoding scheme.
type indexedDISIMethod int

const (
	methodSparse indexedDISIMethod = iota
	methodDense
	methodAll
)

// -----------------------------------------------------------------------------
// Writer
// -----------------------------------------------------------------------------

// WriteBitSet writes the docIDs from it into out in 65536-doc blocks, using
// DefaultDenseRankPower for DENSE blocks. Returns the number of jump-table
// entries appended after the blocks (which the caller must record so the
// reader can be constructed identically).
//
// Mirrors IndexedDISI.writeBitSet(DocIdSetIterator, IndexOutput).
func WriteBitSet(it search.DocIdSetIterator, out store.IndexOutput) (int16, error) {
	return WriteBitSetWithRank(it, out, DefaultDenseRankPower)
}

// WriteBitSetWithRank is the explicit-rank variant of WriteBitSet. Valid
// denseRankPower values are 7..15 (every 128..32768 docIDs) or -1 to
// disable DENSE ranks entirely.
func WriteBitSetWithRank(it search.DocIdSetIterator, out store.IndexOutput, denseRankPower byte) (int16, error) {
	if denseRankPower != 0xFF /* int8 -1 */ {
		if denseRankPower < 7 || denseRankPower > 15 {
			return 0, fmt.Errorf("lucene90: invalid denseRankPower=%d (want 7..15 or -1)", int8(denseRankPower))
		}
	}

	origo := out.GetFilePointer()
	totalCardinality := 0
	blockCardinality := 0
	buffer, err := util.NewFixedBitSet(blockSize)
	if err != nil {
		return 0, err
	}
	jumps := make([]int, 1*2) // dynamically grown
	lastBlock := 0

	doc, err := it.NextDoc()
	if err != nil {
		return 0, err
	}
	for doc != noMoreDocs {
		blockHi := doc >> 16
		// Bring buffer in line with docs in [block*65536, (block+1)*65536).
		// We collect by direct Set on each doc — equivalent to the Java
		// intoBitSet(upTo, buffer, doc & 0xFFFF0000) path applied to a
		// DocIdSetIterator without intoBitSet support.
		for doc != noMoreDocs && (doc>>16) == blockHi {
			buffer.Set(doc & 0xFFFF)
			doc, err = it.NextDoc()
			if err != nil {
				return 0, err
			}
		}
		blockCardinality = buffer.Cardinality()
		jumps = addBlockJumps(jumps, out.GetFilePointer()-origo, totalCardinality, lastBlock, blockHi+1)
		lastBlock = blockHi + 1
		if err := flushIndexedDISIBlock(blockHi, buffer, blockCardinality, denseRankPower, out); err != nil {
			return 0, err
		}
		buffer.ClearAll()
		totalCardinality += blockCardinality
	}

	// Sentinel block: a SPARSE block containing the single docID
	// NO_MORE_DOCS (0xFFFFFFFF interpreted as the high block 0x7FFF and
	// low 0xFFFF). The reader uses it as the natural EOF.
	jumps = addBlockJumps(jumps, out.GetFilePointer()-origo, totalCardinality, lastBlock, lastBlock+1)
	buffer.Set(noMoreDocs & 0xFFFF)
	if err := flushIndexedDISIBlock(noMoreDocs>>16, buffer, 1, denseRankPower, out); err != nil {
		return 0, err
	}
	return flushBlockJumps(jumps, lastBlock+1, out)
}

// flushIndexedDISIBlock writes one logical block to out. Wire format:
//
//	uint16 block               // high 16 bits of the docIDs in this block
//	uint16 cardinality - 1     // number of docs in the block, minus one
//	[body]
//
// For SPARSE (cardinality <= maxArrayLength) the body is a sequence of
// uint16 low-doc-bits. For DENSE the body is the optional rank table
// followed by 1024 little-endian uint64 words. For ALL (cardinality ==
// blockSize) the body is empty.
func flushIndexedDISIBlock(block int, buffer *util.FixedBitSet, cardinality int, denseRankPower byte, out store.IndexOutput) error {
	if block < 0 || block >= blockSize {
		return fmt.Errorf("lucene90: block=%d out of range", block)
	}
	if err := writeShortLE(out, int16(block)); err != nil {
		return err
	}
	if cardinality <= 0 || cardinality > blockSize {
		return fmt.Errorf("lucene90: invalid cardinality=%d", cardinality)
	}
	if err := writeShortLE(out, int16(cardinality-1)); err != nil {
		return err
	}

	if cardinality > maxArrayLength {
		if cardinality != blockSize { // not ALL
			if denseRankPower != 0xFF {
				rank := createDenseRank(buffer, denseRankPower)
				if err := out.WriteBytes(rank); err != nil {
					return err
				}
			}
			for _, w := range buffer.Bits() {
				if err := writeLongLE(out, int64(w)); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// SPARSE: iterate set bits and write each low-16 as a short.
	doc := buffer.NextSetBit(0)
	for doc != noMoreDocs && doc >= 0 && doc < blockSize {
		if err := writeShortLE(out, int16(doc)); err != nil {
			return err
		}
		if doc == blockSize-1 {
			break
		}
		doc = buffer.NextSetBit(doc + 1)
	}
	return nil
}

// createDenseRank builds the per-block rank table: one (high byte, low byte)
// pair every 2^denseRankPower bits. Returns the table as a contiguous byte
// slice ready to be flushed.
func createDenseRank(buffer *util.FixedBitSet, denseRankPower byte) []byte {
	longsPerRank := 1 << (denseRankPower - 6)
	rankMark := longsPerRank - 1
	rankIndexShift := int(denseRankPower) - 7 // 6 for the long + 1 for 2 bytes/entry
	rank := make([]byte, denseBlockLongs>>rankIndexShift)
	wordsArr := buffer.Bits()
	bitCount := 0
	for word := 0; word < denseBlockLongs; word++ {
		if (word & rankMark) == 0 {
			rank[word>>rankIndexShift] = byte(bitCount >> 8)
			rank[(word>>rankIndexShift)+1] = byte(bitCount & 0xFF)
		}
		bitCount += bits.OnesCount64(wordsArr[word])
	}
	return rank
}

// addBlockJumps records (totalCardinality, offset) for every block in
// [startBlock, endBlock). Empty blocks share the same entry as the previous
// non-empty one — same trick as the Java reference uses ArrayUtil.grow
// here; we use slice append.
func addBlockJumps(jumps []int, offset int64, index, startBlock, endBlock int) []int {
	need := (endBlock + 1) * 2
	for cap(jumps) < need {
		jumps = append(jumps[:cap(jumps)], make([]int, cap(jumps))...)
	}
	if len(jumps) < need {
		jumps = jumps[:need]
	}
	for b := startBlock; b < endBlock; b++ {
		jumps[b*2] = index
		jumps[b*2+1] = int(offset)
	}
	return jumps
}

// flushBlockJumps writes the jump table at the end of the IndexedDISI
// stream. When blockCount == 2 (only one real block plus NO_MORE_DOCS) the
// jump table is suppressed to avoid wasted space, exactly as the Java
// reference does.
func flushBlockJumps(jumps []int, blockCount int, out store.IndexOutput) (int16, error) {
	if blockCount == 2 {
		blockCount = 0
	}
	for i := 0; i < blockCount; i++ {
		if err := writeIntLE(out, int32(jumps[i*2])); err != nil {
			return 0, err
		}
		if err := writeIntLE(out, int32(jumps[i*2+1])); err != nil {
			return 0, err
		}
	}
	return int16(blockCount), nil
}

// -----------------------------------------------------------------------------
// Reader constructors
// -----------------------------------------------------------------------------

// NewIndexedDISI opens an IndexedDISI over [offset, offset+length) of in,
// reading the jumpTableEntryCount jump entries at the tail of that range.
// The receiver owns independent slices of in so the caller's file pointer
// is unaffected.
func NewIndexedDISI(in store.IndexInput, offset, length int64, jumpTableEntryCount int, denseRankPower byte, cost int64) (*IndexedDISI, error) {
	blockSliceIn, err := CreateBlockSlice(in, "docs", offset, length, jumpTableEntryCount)
	if err != nil {
		return nil, err
	}
	jt, err := CreateJumpTable(in, offset, length, jumpTableEntryCount)
	if err != nil {
		return nil, err
	}
	return NewIndexedDISIWithSlices(blockSliceIn, jt, jumpTableEntryCount, denseRankPower, cost)
}

// NewIndexedDISIWithSlices is the explicit-slice constructor. Use this
// overload when the caller has already split the underlying input into a
// block slice and a (possibly nil) jump-table slice — typically inside a
// merge instance that wants to share both with sibling readers.
func NewIndexedDISIWithSlices(blockSlice store.IndexInput, jumpTable store.RandomAccessInput, jumpTableEntryCount int, denseRankPower byte, cost int64) (*IndexedDISI, error) {
	if denseRankPower != 0xFF { // int8(-1)
		if denseRankPower < 7 || denseRankPower > 15 {
			return nil, fmt.Errorf("lucene90: invalid denseRankPower=%d (want 7..15 or -1)", int8(denseRankPower))
		}
	}

	// Optional prefetch: mirrors the Java reference's call to
	// IndexInput.prefetch(0, 1) / RandomAccessInput.prefetch(0, 1). Gocene's
	// store package exposes the hint only on RandomAccessInput
	// (PrefetchableRandomAccessInput); IndexInput has no peer interface yet
	// — the IndexInput prefetch is a Lucene 10 read-ahead hint that
	// degrades to a no-op when absent, so omitting it changes performance
	// but not correctness.
	if jumpTable != nil && jumpTable.Length() > 0 {
		if p, ok := jumpTable.(store.PrefetchableRandomAccessInput); ok {
			_ = p.Prefetch(0, 1)
		}
	}

	var rankTable []byte
	if denseRankPower != 0xFF {
		rankIndexShift := int(denseRankPower) - 7
		rankTable = make([]byte, denseBlockLongs>>rankIndexShift)
	}

	return &IndexedDISI{
		slice:               blockSlice,
		jumpTable:           jumpTable,
		jumpTableEntryCount: jumpTableEntryCount,
		denseRankPower:      denseRankPower,
		denseRankTable:      rankTable,
		cost:                cost,
		doc:                 -1,
		block:               -1,
		nextBlockIndex:      -1,
		index:               -1,
		nextExistDocInBlock: -1,
		wordIndex:           -1,
	}, nil
}

// CreateBlockSlice returns the IndexInput slice that contains only the
// block bodies, excluding the jump table. Mirrors
// IndexedDISI.createBlockSlice.
func CreateBlockSlice(slice store.IndexInput, desc string, offset, length int64, jumpTableEntryCount int) (store.IndexInput, error) {
	jumpTableBytes := int64(0)
	if jumpTableEntryCount > 0 {
		jumpTableBytes = int64(jumpTableEntryCount) * 4 * 2
	}
	return slice.Slice(desc, offset, length-jumpTableBytes)
}

// CreateJumpTable returns a RandomAccessInput over the jump-table region at
// the end of the IndexedDISI stream, or nil when there is no jump table.
// Mirrors IndexedDISI.createJumpTable.
//
// Because store.IndexInput does not expose RandomAccessSlice in Gocene, the
// jump-table bytes are read once into a byte slice and wrapped in a
// ByteArrayRandomAccessInput — small in absolute terms (8 bytes per block)
// and bounded by jumpTableEntryCount.
func CreateJumpTable(slice store.IndexInput, offset, length int64, jumpTableEntryCount int) (store.RandomAccessInput, error) {
	if jumpTableEntryCount <= 0 {
		return nil, nil
	}
	jumpTableBytes := int64(jumpTableEntryCount) * 4 * 2

	// Read the bytes directly without disturbing the caller's file pointer.
	saved := slice.GetFilePointer()
	if err := slice.SetPosition(offset + length - jumpTableBytes); err != nil {
		return nil, err
	}
	buf := make([]byte, jumpTableBytes)
	if err := slice.ReadBytes(buf); err != nil {
		return nil, err
	}
	_ = slice.SetPosition(saved)
	return store.NewByteArrayRandomAccessInput(buf), nil
}

// -----------------------------------------------------------------------------
// Iterator API
// -----------------------------------------------------------------------------

// DocID returns the current document ID, -1 if unstarted, or NO_MORE_DOCS
// when exhausted.
func (d *IndexedDISI) DocID() int { return d.doc }

// Index returns the ordinal of the current document (the number of set bits
// before and including DocID()), or -1 if the iterator is unstarted.
func (d *IndexedDISI) Index() int { return d.index }

// Cost returns the cost estimate passed to the constructor.
func (d *IndexedDISI) Cost() int64 { return d.cost }

// NextDoc is Advance(doc+1).
func (d *IndexedDISI) NextDoc() (int, error) { return d.Advance(d.doc + 1) }

// Advance moves the iterator to the first document with ID >= target,
// returning its ID or NO_MORE_DOCS.
func (d *IndexedDISI) Advance(target int) (int, error) {
	targetBlock := target & 0xFFFF0000
	if d.block < targetBlock {
		if err := d.advanceBlock(targetBlock); err != nil {
			return 0, err
		}
	}
	if d.block == targetBlock {
		ok, err := d.advanceWithinBlock(target)
		if err != nil {
			return 0, err
		}
		if ok {
			return d.doc, nil
		}
		if err := d.readBlockHeader(); err != nil {
			return 0, err
		}
	}
	ok, err := d.advanceWithinBlock(d.block)
	if err != nil {
		return 0, err
	}
	if !ok {
		// Defensive: the sentinel ensures we always find a doc at
		// d.block (NO_MORE_DOCS).
		d.doc = noMoreDocs
		return noMoreDocs, nil
	}
	return d.doc, nil
}

// AdvanceExact positions the iterator at target if it exists, returning
// whether the document is set. The internal cursor moves to target either
// way, mirroring the Java semantics.
func (d *IndexedDISI) AdvanceExact(target int) (bool, error) {
	targetBlock := target & 0xFFFF0000
	if d.block < targetBlock {
		if err := d.advanceBlock(targetBlock); err != nil {
			return false, err
		}
	}
	found := false
	if d.block == targetBlock {
		ok, err := d.advanceExactWithinBlock(target)
		if err != nil {
			return false, err
		}
		found = ok
	}
	d.doc = target
	return found, nil
}

// DocIDRunEnd returns one past the end of the current run of consecutive
// doc IDs.
func (d *IndexedDISI) DocIDRunEnd() int {
	switch d.method {
	case methodAll:
		return (d.doc | 0xFFFF) + 1
	case methodDense:
		if d.word == ^uint64(0) {
			return (d.doc | 0x3F) + 1
		}
		return d.doc + 1
	default: // SPARSE
		return d.doc + 1
	}
}

// IntoBitSet — not supported on Gocene's iterator surface. See package doc.
func (d *IndexedDISI) IntoBitSet(_ int, _ []uint64, _ int) error {
	return ErrIntoBitSetNotSupported
}

// Close releases the underlying input slices owned by this DISI.
func (d *IndexedDISI) Close() error {
	var firstErr error
	if d.slice != nil {
		if err := d.slice.Close(); err != nil {
			firstErr = err
		}
		d.slice = nil
	}
	// jumpTable is owned externally (ByteArrayRandomAccessInput holds only
	// a byte slice); no Close needed.
	return firstErr
}

// -----------------------------------------------------------------------------
// Internal traversal
// -----------------------------------------------------------------------------

// advanceBlock positions the slice at the start of the block containing
// targetBlock (a high-16 multiple). Uses the jump table when the target is
// at least two blocks ahead; otherwise falls back to a linear walk.
func (d *IndexedDISI) advanceBlock(targetBlock int) error {
	blockIndex := targetBlock >> 16
	if d.jumpTable != nil && blockIndex >= (d.block>>16)+2 {
		inRangeBlockIndex := blockIndex
		if inRangeBlockIndex >= d.jumpTableEntryCount {
			inRangeBlockIndex = d.jumpTableEntryCount - 1
		}
		idx, err := d.jumpTable.ReadIntAt(int64(inRangeBlockIndex) * 4 * 2)
		if err != nil {
			return err
		}
		off, err := d.jumpTable.ReadIntAt(int64(inRangeBlockIndex)*4*2 + 4)
		if err != nil {
			return err
		}
		d.nextBlockIndex = int(idx) - 1 // compensated by +1 inside readBlockHeader
		if err := d.slice.SetPosition(int64(off)); err != nil {
			return err
		}
		return d.readBlockHeader()
	}

	// Fallback: walk linearly.
	for {
		if err := d.slice.SetPosition(d.blockEnd); err != nil {
			return err
		}
		if err := d.readBlockHeader(); err != nil {
			return err
		}
		if d.block >= targetBlock {
			return nil
		}
	}
}

// readBlockHeader parses the {block, cardinality-1} header at the current
// file pointer and primes the per-method state needed by advanceWithinBlock.
func (d *IndexedDISI) readBlockHeader() error {
	blockShort, err := d.slice.ReadShort()
	if err != nil {
		return err
	}
	d.block = int(uint16(blockShort)) << 16
	if d.block < 0 {
		return fmt.Errorf("lucene90: corrupt IndexedDISI: negative block=%d", d.block)
	}
	cardMinus1, err := d.slice.ReadShort()
	if err != nil {
		return err
	}
	numValues := 1 + int(uint16(cardMinus1))
	d.index = d.nextBlockIndex
	d.nextBlockIndex = d.index + numValues

	switch {
	case numValues <= maxArrayLength:
		d.method = methodSparse
		d.blockEnd = d.slice.GetFilePointer() + int64(numValues)*2
		d.nextExistDocInBlock = -1
	case numValues == blockSize:
		d.method = methodAll
		d.blockEnd = d.slice.GetFilePointer()
		d.gap = d.block - d.index - 1
	default:
		d.method = methodDense
		d.denseBitmapOff = d.slice.GetFilePointer()
		if d.denseRankTable != nil {
			d.denseBitmapOff += int64(len(d.denseRankTable))
		}
		d.blockEnd = d.denseBitmapOff + (1 << 13) // 1024 longs = 8192 bytes
		if d.denseRankPower != 0xFF {
			if err := d.slice.ReadBytes(d.denseRankTable); err != nil {
				return err
			}
		}
		d.wordIndex = -1
		d.numberOfOnes = d.index + 1
		d.denseOrigoIdx = d.numberOfOnes
	}
	return nil
}

// advanceWithinBlock seeks to the first doc >= target within the current
// block. Returns false if the target exceeds every doc in the block, in
// which case the caller should advance to the next block.
func (d *IndexedDISI) advanceWithinBlock(target int) (bool, error) {
	switch d.method {
	case methodAll:
		d.doc = target
		d.index = target - d.gap
		return true, nil

	case methodSparse:
		targetInBlock := target & 0xFFFF
		for d.index < d.nextBlockIndex {
			docShort, err := d.slice.ReadShort()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return false, nil
				}
				return false, err
			}
			doc := int(uint16(docShort))
			d.index++
			if doc >= targetInBlock {
				d.doc = d.block | doc
				d.exists = true
				d.nextExistDocInBlock = doc
				return true, nil
			}
		}
		return false, nil

	case methodDense:
		targetInBlock := target & 0xFFFF
		targetWordIndex := targetInBlock >> 6

		if d.denseRankPower != 0xFF && targetWordIndex-d.wordIndex >= (1<<(int(d.denseRankPower)-6)) {
			if err := d.denseRankSkip(targetInBlock); err != nil {
				return false, err
			}
		}

		for i := d.wordIndex + 1; i <= targetWordIndex; i++ {
			w, err := d.slice.ReadLong()
			if err != nil {
				return false, err
			}
			d.word = uint64(w)
			d.numberOfOnes += bits.OnesCount64(d.word)
		}
		d.wordIndex = targetWordIndex

		// leftBits = word >>> target (Java unsigned shift; the shift count
		// is implicitly modulo 64 in Java, so the low 6 bits of `target`
		// are what matter).
		leftBits := d.word >> uint(target&63)
		if leftBits != 0 {
			d.doc = target + bits.TrailingZeros64(leftBits)
			d.index = d.numberOfOnes - bits.OnesCount64(leftBits)
			return true, nil
		}

		// No set bit at or after target within this word; scan forward.
		for {
			d.wordIndex++
			if d.wordIndex >= denseBlockLongs {
				return false, nil
			}
			w, err := d.slice.ReadLong()
			if err != nil {
				return false, err
			}
			d.word = uint64(w)
			if d.word != 0 {
				d.index = d.numberOfOnes
				d.numberOfOnes += bits.OnesCount64(d.word)
				d.doc = d.block | (d.wordIndex << 6) | bits.TrailingZeros64(d.word)
				return true, nil
			}
		}
	}
	return false, nil
}

// advanceExactWithinBlock is the exact variant used by AdvanceExact.
func (d *IndexedDISI) advanceExactWithinBlock(target int) (bool, error) {
	switch d.method {
	case methodAll:
		d.index = target - d.gap
		return true, nil

	case methodSparse:
		targetInBlock := target & 0xFFFF
		if d.nextExistDocInBlock > targetInBlock {
			return false, nil
		}
		if target == d.doc {
			return d.exists, nil
		}
		for d.index < d.nextBlockIndex {
			docShort, err := d.slice.ReadShort()
			if err != nil {
				return false, err
			}
			doc := int(uint16(docShort))
			d.index++
			if doc >= targetInBlock {
				d.nextExistDocInBlock = doc
				if doc != targetInBlock {
					d.index--
					if err := d.slice.SetPosition(d.slice.GetFilePointer() - 2); err != nil {
						return false, err
					}
					break
				}
				d.exists = true
				return true, nil
			}
		}
		d.exists = false
		return false, nil

	case methodDense:
		targetInBlock := target & 0xFFFF
		targetWordIndex := targetInBlock >> 6

		if d.denseRankPower != 0xFF && targetWordIndex-d.wordIndex >= (1<<(int(d.denseRankPower)-6)) {
			if err := d.denseRankSkip(targetInBlock); err != nil {
				return false, err
			}
		}

		for i := d.wordIndex + 1; i <= targetWordIndex; i++ {
			w, err := d.slice.ReadLong()
			if err != nil {
				return false, err
			}
			d.word = uint64(w)
			d.numberOfOnes += bits.OnesCount64(d.word)
		}
		d.wordIndex = targetWordIndex

		leftBits := d.word >> uint(target&63)
		d.index = d.numberOfOnes - bits.OnesCount64(leftBits)
		return (leftBits & 1) != 0, nil
	}
	return false, nil
}

// denseRankSkip uses the per-DENSE-block rank table to seek forward to the
// nearest 2^denseRankPower boundary at or before targetInBlock without
// reading every intermediate long.
func (d *IndexedDISI) denseRankSkip(targetInBlock int) error {
	rankIndex := targetInBlock >> int(d.denseRankPower)
	rank := int(d.denseRankTable[rankIndex<<1]&0xFF)<<8 |
		int(d.denseRankTable[(rankIndex<<1)+1]&0xFF)

	rankAlignedWordIndex := rankIndex << int(d.denseRankPower) >> 6
	if err := d.slice.SetPosition(d.denseBitmapOff + int64(rankAlignedWordIndex)*8); err != nil {
		return err
	}
	w, err := d.slice.ReadLong()
	if err != nil {
		return err
	}
	rankWord := uint64(w)

	d.wordIndex = rankAlignedWordIndex
	d.word = rankWord
	d.numberOfOnes = d.denseOrigoIdx + rank + bits.OnesCount64(rankWord)
	return nil
}
