// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene50PostingsFormat constants used by the skip reader.
const (
	// BlockSize is the number of documents per block (matches BLOCK_SIZE in Java).
	BlockSize = 128

	// VersionStart is the first supported version.
	VersionStart = 0

	// VersionImpactSkipData is the version that added impact data to skip lists.
	VersionImpactSkipData = 1

	// VersionCurrent is the current format version.
	VersionCurrent = VersionImpactSkipData
)

// Lucene50SkipReader implements the skip list reader for the Lucene 5.0 block
// postings format that stores positions and payloads.
//
// The skip position semantics differ slightly from MultiLevelSkipListReader:
// when df is an exact multiple of BlockSize, the final block has no skip
// entry. trim(df) corrects for this before initialising the base reader.
//
// Port of
// org.apache.lucene.backward_codecs.lucene50.Lucene50SkipReader
// (Lucene 10.4.0).
type Lucene50SkipReader struct {
	base *codecs.MultiLevelSkipListReader

	version int

	// Per-level pointer accumulators (base file pointers per level).
	docPointer      []int64
	posPointer      []int64
	payPointer      []int64
	posBufferUpto   []int
	payloadByteUpto []int

	// "Last accepted" pointer snapshots — updated by setLastSkipData.
	lastDocPointer      int64
	lastPosPointer      int64
	lastPayPointer      int64
	lastPosBufferUpto   int
	lastPayloadByteUpto int

	// readImpactsHook is called from readSkipData to consume (or buffer) the
	// impact bytes for a given level. Subtype implementations (e.g.
	// Lucene50ScoreSkipReader) replace this hook to capture impact data instead
	// of discarding it. Default implementation skips the bytes.
	readImpactsHook func(level int, skipStream store.IndexInput) error
}

// NewLucene50SkipReader constructs a Lucene50SkipReader.
//
// Port of Lucene50SkipReader(int, IndexInput, int, boolean, boolean, boolean).
func NewLucene50SkipReader(
	version int,
	skipStream store.IndexInput,
	maxSkipLevels int,
	hasPos bool,
	hasOffsets bool,
	hasPayloads bool,
) *Lucene50SkipReader {
	r := &Lucene50SkipReader{
		version:    version,
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

	// Construct the base reader with skipInterval=BlockSize, skipMultiplier=8.
	// readSkipData is wired to r.readSkipData; the hooks mirror seekChild and
	// setLastSkipData from the Java abstract class.
	r.base = codecs.NewMultiLevelSkipListReader(
		skipStream,
		maxSkipLevels,
		BlockSize,
		8,
		r.readSkipData,
	)
	r.base.SetOnSetLastSkipData(r.setLastSkipData)
	r.base.SetOnSeekChild(r.seekChild)

	// Default hook discards impact bytes. Lucene50ScoreSkipReader overrides
	// this to buffer them.
	r.readImpactsHook = r.readImpacts

	return r
}

// trim adjusts df to prevent the base reader from attempting to read a
// non-existent skip point after the last full block boundary.
//
// Port of Lucene50SkipReader.trim(int).
func trim(df int) int {
	if df%BlockSize == 0 {
		return df - 1
	}
	return df
}

// Init prepares the reader for a new posting list.
//
// Port of Lucene50SkipReader.init(long, long, long, long, int).
func (r *Lucene50SkipReader) Init(
	skipPointer int64,
	docBasePointer int64,
	posBasePointer int64,
	payBasePointer int64,
	df int,
) error {
	if err := r.base.Init(skipPointer, trim(df)); err != nil {
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
//
// Port of MultiLevelSkipListReader.skipTo(int) (inherited, no override in Java).
func (r *Lucene50SkipReader) SkipTo(target int) (int, error) {
	return r.base.SkipTo(target)
}

// GetDocPointer returns the doc file pointer after the last successful SkipTo.
func (r *Lucene50SkipReader) GetDocPointer() int64 {
	return r.lastDocPointer
}

// GetPosPointer returns the positions file pointer after the last successful
// SkipTo.
func (r *Lucene50SkipReader) GetPosPointer() int64 {
	return r.lastPosPointer
}

// GetPosBufferUpto returns the in-block position offset after the last
// successful SkipTo.
func (r *Lucene50SkipReader) GetPosBufferUpto() int {
	return r.lastPosBufferUpto
}

// GetPayPointer returns the payload/offset file pointer after the last
// successful SkipTo.
func (r *Lucene50SkipReader) GetPayPointer() int64 {
	return r.lastPayPointer
}

// GetPayloadByteUpto returns the in-block payload byte offset after the last
// successful SkipTo.
func (r *Lucene50SkipReader) GetPayloadByteUpto() int {
	return r.lastPayloadByteUpto
}

// GetNextSkipDoc returns the document id of the next skip entry (level 0).
// This is skipDoc[0] in the base reader.
func (r *Lucene50SkipReader) GetNextSkipDoc() int {
	return r.base.GetDoc()
}

// Close releases all skip-level streams.
func (r *Lucene50SkipReader) Close() error {
	return r.base.Close()
}

// seekChild mirrors Java's Lucene50SkipReader.seekChild(int).
// Called by the base reader when it repositions a child level.
func (r *Lucene50SkipReader) seekChild(level int) {
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

// setLastSkipData mirrors Java's Lucene50SkipReader.setLastSkipData(int).
// Called by the base reader before advancing past the current skip entry.
func (r *Lucene50SkipReader) setLastSkipData(level int) {
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

// readSkipData mirrors Java's Lucene50SkipReader.readSkipData(int, IndexInput).
// Reads per-level skip data and returns the doc id delta.
func (r *Lucene50SkipReader) readSkipData(level int, skipStream store.IndexInput) (int, error) {
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

// readImpacts skips over the impact data in the skip stream.
//
// Port of Lucene50SkipReader.readImpacts(int, IndexInput).
// The base implementation simply discards the impact bytes — they are not
// used in the plain skip reader (see Lucene50ScoreSkipReader for the scoring
// variant that reads them).
func (r *Lucene50SkipReader) readImpacts(_ int, skipStream store.IndexInput) error {
	if r.version >= VersionImpactSkipData {
		n, err := store.ReadVInt(skipStream)
		if err != nil {
			return err
		}
		return skipStream.SetPosition(skipStream.GetFilePointer() + int64(n))
	}
	return nil
}
