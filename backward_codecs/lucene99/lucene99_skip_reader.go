// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// lucene99BlockSize is ForUtil.BLOCK_SIZE for the Lucene 9.9 postings format.
const lucene99BlockSize = 128

// Lucene99SkipReader implements the skip list reader for the Lucene 9.9 block
// postings format that stores positions and payloads.
//
// The skip position semantics differ slightly from MultiLevelSkipListReader:
// when df is an exact multiple of blockSize, the final block has no skip
// entry. trim trims df to prevent the base reader from attempting to read a
// non-existent skip point.
//
// Port of org.apache.lucene.backward_codecs.lucene99.Lucene99SkipReader
// (Lucene 10.4.0).
type Lucene99SkipReader struct {
	base *codecs.MultiLevelSkipListReader

	// Per-level pointer accumulators.
	docPointer      []int64
	posPointer      []int64
	payPointer      []int64
	posBufferUpto   []int
	payloadByteUpto []int

	// Snapshot of the last accepted skip data.
	lastDocPointer      int64
	lastPosPointer      int64
	lastPayPointer      int64
	lastPosBufferUpto   int
	lastPayloadByteUpto int

	// readImpactsHook is called from readSkipData to consume (or buffer) the
	// impact bytes for a given level. Lucene99ScoreSkipReader overrides this.
	readImpactsHook func(level int, skipStream store.IndexInput) error
}

// NewLucene99SkipReader constructs a Lucene99SkipReader.
//
// Port of Lucene99SkipReader(IndexInput, int, boolean, boolean, boolean).
func NewLucene99SkipReader(
	skipStream store.IndexInput,
	maxSkipLevels int,
	hasPos bool,
	hasOffsets bool,
	hasPayloads bool,
) *Lucene99SkipReader {
	r := &Lucene99SkipReader{
		docPointer: make([]int64, maxSkipLevels),
	}

	if hasPos {
		r.posPointer = make([]int64, maxSkipLevels)
		r.posBufferUpto = make([]int, maxSkipLevels)
		if hasPayloads {
			r.payloadByteUpto = make([]int, maxSkipLevels)
		}
		if hasOffsets || hasPayloads {
			r.payPointer = make([]int64, maxSkipLevels)
		}
	}

	r.base = codecs.NewMultiLevelSkipListReader(
		skipStream,
		maxSkipLevels,
		lucene99BlockSize,
		8,
		r.readSkipData,
	)
	r.base.SetOnSetLastSkipData(r.setLastSkipData)
	r.base.SetOnSeekChild(r.seekChild)

	// Default hook discards impact bytes.
	r.readImpactsHook = r.readImpacts

	return r
}

// trim adjusts df to prevent the base reader from attempting to read a
// non-existent skip point after the last full block boundary.
//
// Port of Lucene99SkipReader.trim(int).
func trimLucene99(df int) int {
	if df%lucene99BlockSize == 0 {
		return df - 1
	}
	return df
}

// Init prepares the reader for a new posting list.
//
// Port of Lucene99SkipReader.init(long, long, long, long, int).
func (r *Lucene99SkipReader) Init(
	skipPointer int64,
	docBasePointer int64,
	posBasePointer int64,
	payBasePointer int64,
	df int,
) error {
	if err := r.base.Init(skipPointer, trimLucene99(df)); err != nil {
		return err
	}
	r.lastDocPointer = docBasePointer
	r.lastPosPointer = posBasePointer
	r.lastPayPointer = payBasePointer

	for i := range r.docPointer {
		r.docPointer[i] = docBasePointer
	}
	if r.posPointer != nil {
		for i := range r.posPointer {
			r.posPointer[i] = posBasePointer
		}
		if r.payPointer != nil {
			for i := range r.payPointer {
				r.payPointer[i] = payBasePointer
			}
		}
	}
	return nil
}

// SkipTo advances the cursor past all skip entries whose doc id < target.
// Returns the number of postings skipped.
func (r *Lucene99SkipReader) SkipTo(target int) (int, error) {
	return r.base.SkipTo(target)
}

// GetDocPointer returns the doc file pointer after the last successful SkipTo.
func (r *Lucene99SkipReader) GetDocPointer() int64 { return r.lastDocPointer }

// GetPosPointer returns the positions file pointer after the last successful SkipTo.
func (r *Lucene99SkipReader) GetPosPointer() int64 { return r.lastPosPointer }

// GetPosBufferUpto returns the in-block position offset after the last SkipTo.
func (r *Lucene99SkipReader) GetPosBufferUpto() int { return r.lastPosBufferUpto }

// GetPayPointer returns the payload/offset file pointer after the last SkipTo.
func (r *Lucene99SkipReader) GetPayPointer() int64 { return r.lastPayPointer }

// GetPayloadByteUpto returns the in-block payload byte offset after the last SkipTo.
func (r *Lucene99SkipReader) GetPayloadByteUpto() int { return r.lastPayloadByteUpto }

// GetNextSkipDoc returns the document id of the next skip entry (level 0).
func (r *Lucene99SkipReader) GetNextSkipDoc() int { return r.base.GetDoc() }

// Close releases all skip-level streams.
func (r *Lucene99SkipReader) Close() error { return r.base.Close() }

// seekChild mirrors Lucene99SkipReader.seekChild(int).
func (r *Lucene99SkipReader) seekChild(level int) {
	r.docPointer[level] = r.lastDocPointer
	if r.posPointer != nil {
		r.posPointer[level] = r.lastPosPointer
		r.posBufferUpto[level] = r.lastPosBufferUpto
		if r.payloadByteUpto != nil {
			r.payloadByteUpto[level] = r.lastPayloadByteUpto
		}
		if r.payPointer != nil {
			r.payPointer[level] = r.lastPayPointer
		}
	}
}

// setLastSkipData mirrors Lucene99SkipReader.setLastSkipData(int).
func (r *Lucene99SkipReader) setLastSkipData(level int) {
	r.lastDocPointer = r.docPointer[level]
	if r.posPointer != nil {
		r.lastPosPointer = r.posPointer[level]
		r.lastPosBufferUpto = r.posBufferUpto[level]
		if r.payPointer != nil {
			r.lastPayPointer = r.payPointer[level]
		}
		if r.payloadByteUpto != nil {
			r.lastPayloadByteUpto = r.payloadByteUpto[level]
		}
	}
}

// readSkipData mirrors Lucene99SkipReader.readSkipData(int, IndexInput).
func (r *Lucene99SkipReader) readSkipData(level int, skipStream store.IndexInput) (int, error) {
	delta, err := store.ReadVInt(skipStream)
	if err != nil {
		return 0, err
	}
	docDelta, err := store.ReadVLong(skipStream)
	if err != nil {
		return 0, err
	}
	r.docPointer[level] += docDelta

	if r.posPointer != nil {
		posDelta, err := store.ReadVLong(skipStream)
		if err != nil {
			return 0, err
		}
		r.posPointer[level] += posDelta

		posUpto, err := store.ReadVInt(skipStream)
		if err != nil {
			return 0, err
		}
		r.posBufferUpto[level] = int(posUpto)

		if r.payloadByteUpto != nil {
			payByte, err := store.ReadVInt(skipStream)
			if err != nil {
				return 0, err
			}
			r.payloadByteUpto[level] = int(payByte)
		}

		if r.payPointer != nil {
			payDelta, err := store.ReadVLong(skipStream)
			if err != nil {
				return 0, err
			}
			r.payPointer[level] += payDelta
		}
	}

	if err := r.readImpactsHook(level, skipStream); err != nil {
		return 0, err
	}
	return int(delta), nil
}

// readImpacts is the default hook: skips over the impact bytes in the stream.
//
// Port of Lucene99SkipReader.readImpacts(int, IndexInput).
func (r *Lucene99SkipReader) readImpacts(_ int, skipStream store.IndexInput) error {
	n, err := store.ReadVInt(skipStream)
	if err != nil {
		return err
	}
	return skipStream.SetPosition(skipStream.GetFilePointer() + int64(n))
}
