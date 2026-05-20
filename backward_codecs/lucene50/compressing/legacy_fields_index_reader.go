// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"fmt"

	bcpacked "github.com/FlavioCFOliveira/Gocene/backward_codecs/packed"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// LegacyFieldsIndex is the minimal interface shared by
// LegacyFieldsIndexReader and any future implementations in this package.
// It mirrors the Java abstract class
// org.apache.lucene.backward_codecs.lucene50.compressing.FieldsIndex.
type LegacyFieldsIndex interface {
	// GetStartPointer returns the file offset of the chunk that contains docID.
	GetStartPointer(docID int) (int64, error)

	// CheckIntegrity verifies the index.
	CheckIntegrity() error

	// Clone returns a snapshot of this index (may return self for immutable readers).
	Clone() LegacyFieldsIndex

	// Close releases resources.
	Close() error
}

// LegacyFieldsIndexReader reads and queries the fields-index written by the
// Lucene 5.0 compressing stored-fields format.
//
// Port of
// org.apache.lucene.backward_codecs.lucene50.compressing.LegacyFieldsIndexReader
// (Lucene 10.4.0).
//
// The index is a sequence of blocks, each covering a contiguous range of
// chunks. Within each block the absolute doc-base and start-pointer are
// recovered from a block average plus a zig-zag-encoded delta stored in a
// LegacyPacked64 array.
type LegacyFieldsIndexReader struct {
	maxDoc              int
	docBases            []int
	startPointers       []int64
	avgChunkDocs        []int
	avgChunkSizes       []int64
	docBasesDeltas      []packed.Reader
	startPointersDeltas []packed.Reader
}

// NewLegacyFieldsIndexReader constructs a LegacyFieldsIndexReader by
// consuming fieldsIndexIn, which must be positioned at the start of the index
// payload (i.e. after any index header has already been read by the caller).
// si is used only to obtain maxDoc.
//
// The caller is responsible for closing fieldsIndexIn after this constructor
// returns.
//
// Port of LegacyFieldsIndexReader(IndexInput, SegmentInfo).
func NewLegacyFieldsIndexReader(fieldsIndexIn store.DataInput, si *index.SegmentInfo) (*LegacyFieldsIndexReader, error) {
	maxDoc := si.DocCount()

	// PackedInts version used when the index was serialized.
	packedIntsVersionRaw, err := store.ReadVInt(fieldsIndexIn)
	if err != nil {
		return nil, fmt.Errorf("legacyFieldsIndex: read packedIntsVersion: %w", err)
	}
	packedIntsVersion := int(packedIntsVersionRaw)

	// Pre-allocated slices; grown on demand like Java ArrayUtil.oversize.
	docBases := make([]int, 16)
	startPointers := make([]int64, 16)
	avgChunkDocs := make([]int, 16)
	avgChunkSizes := make([]int64, 16)
	docBasesDeltas := make([]packed.Reader, 16)
	startPointersDeltas := make([]packed.Reader, 16)

	blockCount := 0

	for {
		numChunksRaw, err := store.ReadVInt(fieldsIndexIn)
		if err != nil {
			return nil, fmt.Errorf("legacyFieldsIndex: read numChunks: %w", err)
		}
		numChunks := int(numChunksRaw)
		if numChunks == 0 {
			break
		}

		// Grow backing slices if needed.
		if blockCount == len(docBases) {
			newSize := util.Oversize(blockCount+1, 8)
			docBases = growInts(docBases, newSize)
			startPointers = growInt64s(startPointers, newSize)
			avgChunkDocs = growInts(avgChunkDocs, newSize)
			avgChunkSizes = growInt64s(avgChunkSizes, newSize)
			docBasesDeltas = growReaders(docBasesDeltas, newSize)
			startPointersDeltas = growReaders(startPointersDeltas, newSize)
		}

		// Doc bases.
		docBaseRaw, err := store.ReadVInt(fieldsIndexIn)
		if err != nil {
			return nil, fmt.Errorf("legacyFieldsIndex: block %d read docBase: %w", blockCount, err)
		}
		docBases[blockCount] = int(docBaseRaw)

		avgChunkDocsRaw, err := store.ReadVInt(fieldsIndexIn)
		if err != nil {
			return nil, fmt.Errorf("legacyFieldsIndex: block %d read avgChunkDocs: %w", blockCount, err)
		}
		avgChunkDocs[blockCount] = int(avgChunkDocsRaw)

		bitsPerDocBaseRaw, err := store.ReadVInt(fieldsIndexIn)
		if err != nil {
			return nil, fmt.Errorf("legacyFieldsIndex: block %d read bitsPerDocBase: %w", blockCount, err)
		}
		bitsPerDocBase := int(bitsPerDocBaseRaw)
		if bitsPerDocBase > 32 {
			return nil, fmt.Errorf("legacyFieldsIndex: block %d corrupted bitsPerDocBase=%d", blockCount, bitsPerDocBase)
		}

		docBasesDeltas[blockCount], err = bcpacked.GetReaderNoHeader(fieldsIndexIn, packedIntsVersion, numChunks, bitsPerDocBase)
		if err != nil {
			return nil, fmt.Errorf("legacyFieldsIndex: block %d read docBasesDeltas: %w", blockCount, err)
		}

		// Start pointers.
		spRaw, err := store.ReadVLong(fieldsIndexIn)
		if err != nil {
			return nil, fmt.Errorf("legacyFieldsIndex: block %d read startPointer: %w", blockCount, err)
		}
		startPointers[blockCount] = spRaw

		avgChunkSizeRaw, err := store.ReadVLong(fieldsIndexIn)
		if err != nil {
			return nil, fmt.Errorf("legacyFieldsIndex: block %d read avgChunkSize: %w", blockCount, err)
		}
		avgChunkSizes[blockCount] = avgChunkSizeRaw

		bitsPerStartPointerRaw, err := store.ReadVInt(fieldsIndexIn)
		if err != nil {
			return nil, fmt.Errorf("legacyFieldsIndex: block %d read bitsPerStartPointer: %w", blockCount, err)
		}
		bitsPerStartPointer := int(bitsPerStartPointerRaw)
		if bitsPerStartPointer > 64 {
			return nil, fmt.Errorf("legacyFieldsIndex: block %d corrupted bitsPerStartPointer=%d", blockCount, bitsPerStartPointer)
		}

		startPointersDeltas[blockCount], err = bcpacked.GetReaderNoHeader(fieldsIndexIn, packedIntsVersion, numChunks, bitsPerStartPointer)
		if err != nil {
			return nil, fmt.Errorf("legacyFieldsIndex: block %d read startPointersDeltas: %w", blockCount, err)
		}

		blockCount++
	}

	return &LegacyFieldsIndexReader{
		maxDoc:              maxDoc,
		docBases:            docBases[:blockCount],
		startPointers:       startPointers[:blockCount],
		avgChunkDocs:        avgChunkDocs[:blockCount],
		avgChunkSizes:       avgChunkSizes[:blockCount],
		docBasesDeltas:      docBasesDeltas[:blockCount],
		startPointersDeltas: startPointersDeltas[:blockCount],
	}, nil
}

// block returns the block index whose docBases entry is the largest value ≤
// docID, using binary search.
//
// Port of LegacyFieldsIndexReader.block(int).
func (r *LegacyFieldsIndexReader) block(docID int) int {
	lo, hi := 0, len(r.docBases)-1
	for lo <= hi {
		mid := (lo + hi) >> 1
		midVal := r.docBases[mid]
		if midVal == docID {
			return mid
		} else if midVal < docID {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return hi
}

// relativeDocBase returns the doc offset of relativeChunk within block b,
// computed as avgChunkDocs[b]*relativeChunk + zigzag(delta).
//
// Port of LegacyFieldsIndexReader.relativeDocBase(int, int).
func (r *LegacyFieldsIndexReader) relativeDocBase(b, relativeChunk int) int {
	expected := r.avgChunkDocs[b] * relativeChunk
	delta := util.ZigZagDecodeInt64(r.docBasesDeltas[b].Get(relativeChunk))
	return expected + int(delta)
}

// relativeStartPointer returns the byte-offset delta of relativeChunk within
// block b.
//
// Port of LegacyFieldsIndexReader.relativeStartPointer(int, int).
func (r *LegacyFieldsIndexReader) relativeStartPointer(b, relativeChunk int) int64 {
	expected := r.avgChunkSizes[b] * int64(relativeChunk)
	delta := util.ZigZagDecodeInt64(r.startPointersDeltas[b].Get(relativeChunk))
	return expected + delta
}

// relativeChunk returns the chunk index within block b that covers relativeDoc
// (doc offset from docBases[b]), using binary search.
//
// Port of LegacyFieldsIndexReader.relativeChunk(int, int).
func (r *LegacyFieldsIndexReader) relativeChunk(b, relativeDoc int) int {
	lo, hi := 0, r.docBasesDeltas[b].Size()-1
	for lo <= hi {
		mid := (lo + hi) >> 1
		midVal := r.relativeDocBase(b, mid)
		if midVal == relativeDoc {
			return mid
		} else if midVal < relativeDoc {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return hi
}

// GetStartPointer returns the file offset of the chunk that contains docID.
//
// Port of LegacyFieldsIndexReader.getStartPointer(int).
func (r *LegacyFieldsIndexReader) GetStartPointer(docID int) (int64, error) {
	if docID < 0 || docID >= r.maxDoc {
		return 0, fmt.Errorf("legacyFieldsIndex: docID %d out of range [0, %d)", docID, r.maxDoc)
	}
	b := r.block(docID)
	if b < 0 {
		// No block found: segment has no stored-fields blocks.
		return 0, nil
	}
	rc := r.relativeChunk(b, docID-r.docBases[b])
	return r.startPointers[b] + r.relativeStartPointer(b, rc), nil
}

// CheckIntegrity is a no-op: the index is validated at open time.
//
// Port of LegacyFieldsIndexReader.checkIntegrity().
func (r *LegacyFieldsIndexReader) CheckIntegrity() error { return nil }

// Clone returns the receiver — LegacyFieldsIndexReader is immutable.
//
// Port of LegacyFieldsIndexReader.clone() which returns this.
func (r *LegacyFieldsIndexReader) Clone() LegacyFieldsIndex { return r }

// Close is a no-op — there are no resources to release.
//
// Port of LegacyFieldsIndexReader.close().
func (r *LegacyFieldsIndexReader) Close() error { return nil }

// String returns a debug-friendly representation.
func (r *LegacyFieldsIndexReader) String() string {
	return fmt.Sprintf("LegacyFieldsIndexReader(blocks=%d)", len(r.docBases))
}

// compile-time assertion
var _ LegacyFieldsIndex = (*LegacyFieldsIndexReader)(nil)

// ── helpers ──────────────────────────────────────────────────────────────────

func growInts(s []int, newSize int) []int {
	n := make([]int, newSize)
	copy(n, s)
	return n
}

func growInt64s(s []int64, newSize int) []int64 {
	n := make([]int64, newSize)
	copy(n, s)
	return n
}

func growReaders(s []packed.Reader, newSize int) []packed.Reader {
	n := make([]packed.Reader, newSize)
	copy(n, s)
	return n
}
