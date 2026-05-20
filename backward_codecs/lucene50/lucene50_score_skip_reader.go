// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene50ScoreSkipReader extends Lucene50SkipReader to also decode and expose
// per-level impact data (freq+norm pairs). Callers can obtain an Impacts
// view via GetImpacts.
//
// Port of
// org.apache.lucene.backward_codecs.lucene50.Lucene50ScoreSkipReader
// (Lucene 10.4.0).
type Lucene50ScoreSkipReader struct {
	*Lucene50SkipReader

	// impactData stores the raw encoded impact bytes per level.
	impactData [][]byte

	// impactDataLength tracks the valid length within impactData[level].
	impactDataLength []int

	// badi is the re-usable reader for decoding buffered impact bytes.
	badi *store.ByteArrayDataInput

	// numLevels is the number of active skip levels as of the last SkipTo.
	numLevels int

	// perLevelImpacts caches the decoded FreqAndNormBuffer per level.
	perLevelImpacts []*index.FreqAndNormBuffer

	// impactsView is the Impacts implementation returned by GetImpacts.
	impactsView index.Impacts
}

// NewLucene50ScoreSkipReader constructs a Lucene50ScoreSkipReader.
// Panics if version < VersionImpactSkipData because impacts are not indexed
// for older versions.
//
// Port of Lucene50ScoreSkipReader(int, IndexInput, int, boolean, boolean, boolean).
func NewLucene50ScoreSkipReader(
	version int,
	skipStream store.IndexInput,
	maxSkipLevels int,
	hasPos bool,
	hasOffsets bool,
	hasPayloads bool,
) *Lucene50ScoreSkipReader {
	if version < VersionImpactSkipData {
		panic(fmt.Sprintf(
			"lucene50: cannot score-skip for version %d: impacts not indexed (need >= %d)",
			version, VersionImpactSkipData,
		))
	}

	base := NewLucene50SkipReader(version, skipStream, maxSkipLevels, hasPos, hasOffsets, hasPayloads)

	r := &Lucene50ScoreSkipReader{
		Lucene50SkipReader: base,
		impactData:         make([][]byte, maxSkipLevels),
		impactDataLength:   make([]int, maxSkipLevels),
		badi:               store.NewByteArrayDataInput(nil),
		numLevels:          1,
		perLevelImpacts:    make([]*index.FreqAndNormBuffer, maxSkipLevels),
	}

	for i := range r.impactData {
		r.impactData[i] = []byte{}
	}
	for i := range r.perLevelImpacts {
		b := index.NewFreqAndNormBuffer()
		b.Add(math.MaxInt32, 1)
		r.perLevelImpacts[i] = b
	}

	// Override the base reader's readImpacts hook so that readSkipData calls
	// r.readImpacts (this type) instead of the default discard implementation.
	r.Lucene50SkipReader.readImpactsHook = r.readImpacts

	// Build the Impacts view. The closure captures r so it sees live state.
	r.impactsView = &scoreSkipImpacts{r: r}

	return r
}

// SkipTo advances the skip cursor and updates numLevels.
//
// Port of Lucene50ScoreSkipReader.skipTo(int).
func (r *Lucene50ScoreSkipReader) SkipTo(target int) (int, error) {
	result, err := r.Lucene50SkipReader.SkipTo(target)
	if err != nil {
		return result, err
	}
	n := r.base.NumberOfSkipLevels()
	if n > 0 {
		r.numLevels = n
	} else {
		// End of postings: fill with dummy data like SlowImpactsEnum.
		r.numLevels = 1
		b := r.perLevelImpacts[0]
		b.Size = 1
		if len(b.Freqs) == 0 {
			b.GrowNoCopy(1)
		}
		b.Freqs[0] = math.MaxInt32
		b.Norms[0] = 1
		r.impactDataLength[0] = 0
	}
	return result, nil
}

// GetImpacts returns the Impacts view for the current skip position.
func (r *Lucene50ScoreSkipReader) GetImpacts() index.Impacts {
	return r.impactsView
}

// readImpacts overrides Lucene50SkipReader.readImpacts to buffer the raw
// encoded impact bytes rather than discarding them.
//
// Port of Lucene50ScoreSkipReader.readImpacts(int, IndexInput).
func (r *Lucene50ScoreSkipReader) readImpacts(level int, skipStream store.IndexInput) error {
	n, err := store.ReadVInt(skipStream)
	if err != nil {
		return err
	}
	length := int(n)
	if cap(r.impactData[level]) < length {
		r.impactData[level] = make([]byte, oversize(length))
	}
	r.impactData[level] = r.impactData[level][:length]
	if err := skipStream.ReadBytes(r.impactData[level]); err != nil {
		return err
	}
	r.impactDataLength[level] = length
	return nil
}

// decodeImpacts decodes the raw impact bytes stored for level into a
// FreqAndNormBuffer.
//
// Port of the static Lucene50ScoreSkipReader.readImpacts(ByteArrayDataInput, FreqAndNormBuffer).
func (r *Lucene50ScoreSkipReader) decodeImpacts(level int) *index.FreqAndNormBuffer {
	if r.impactDataLength[level] > 0 {
		r.badi.ResetWithSlice(r.impactData[level], 0, r.impactDataLength[level])
		r.perLevelImpacts[level] = decodeImpactsFromReader(r.badi, r.perLevelImpacts[level])
		r.impactDataLength[level] = 0
	}
	return r.perLevelImpacts[level]
}

// decodeImpactsFromReader decodes a sequence of (freq, norm) impact pairs
// from in and stores them in reuse.
//
// Encoding:
//   - Each entry is a VInt freqDelta.
//   - If bit 0 of freqDelta is set: freq += 1 + (freqDelta >> 1); norm advances
//     by 1 + ZLong (zig-zag-decoded VLong).
//   - Otherwise: freq += 1 + (freqDelta >> 1); norm++.
//
// Port of the static Lucene50ScoreSkipReader.readImpacts(ByteArrayDataInput, FreqAndNormBuffer).
func decodeImpactsFromReader(
	in *store.ByteArrayDataInput,
	reuse *index.FreqAndNormBuffer,
) *index.FreqAndNormBuffer {
	if reuse == nil {
		reuse = index.NewFreqAndNormBuffer()
	}
	maxNumImpacts := in.Length()
	reuse.GrowNoCopy(maxNumImpacts)

	var freq int
	var norm int64
	size := 0

	for in.GetPosition() < in.Length() {
		freqDelta, _ := store.ReadVInt(in) // BADI never returns I/O error
		fd := int(freqDelta)
		freq += 1 + (fd >> 1)
		if fd&0x01 != 0 {
			normDelta, _ := store.ReadVLong(in)
			norm += 1 + zigzagDecodeLong(normDelta)
		} else {
			norm++
		}
		reuse.Freqs[size] = freq
		reuse.Norms[size] = norm
		size++
	}
	reuse.Size = size
	return reuse
}

// zigzagDecodeLong decodes a zig-zag-encoded int64 (mirrors Java's
// BitUtil.zigZagDecode or DataInput.readZLong).
func zigzagDecodeLong(n int64) int64 {
	return (n >> 1) ^ -(n & 1)
}

// oversize returns a grown capacity for a byte slice, mirroring
// ArrayUtil.oversize(n, Byte.BYTES) from Lucene.
func oversize(minSize int) int {
	if minSize < 8 {
		return 8
	}
	extra := minSize >> 3
	return minSize + extra + 1
}

// scoreSkipImpacts implements index.Impacts backed by Lucene50ScoreSkipReader.
type scoreSkipImpacts struct {
	r *Lucene50ScoreSkipReader
}

// NumLevels returns the number of active skip levels.
func (i *scoreSkipImpacts) NumLevels() int {
	return i.r.numLevels
}

// GetDocIDUpTo returns the max doc id covered by skip-level impacts.
func (i *scoreSkipImpacts) GetDocIDUpTo(level int) int {
	return i.r.base.GetSkipDoc(level)
}

// GetImpacts lazily decodes and returns the impact buffer for the given level.
func (i *scoreSkipImpacts) GetImpacts(level int) *index.FreqAndNormBuffer {
	return i.r.decodeImpacts(level)
}

// compile-time assertion
var _ index.Impacts = (*scoreSkipImpacts)(nil)
