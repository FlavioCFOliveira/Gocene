// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene99

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Lucene99ScoreSkipReader extends Lucene99SkipReader to also decode and
// expose per-level impact data (freq+norm pairs). Callers obtain an Impacts
// view via GetImpacts.
//
// Port of
// org.apache.lucene.backward_codecs.lucene99.Lucene99ScoreSkipReader
// (Lucene 10.4.0).
type Lucene99ScoreSkipReader struct {
	*Lucene99SkipReader

	// impactData stores the raw encoded impact bytes per level.
	impactData [][]byte

	// impactDataLength tracks the valid byte count within impactData[level].
	impactDataLength []int

	// badi is the re-usable decoder for impact bytes.
	badi *store.ByteArrayDataInput

	// numLevels is the number of active skip levels as of the last SkipTo.
	numLevels int

	// perLevelImpacts caches the decoded FreqAndNormBuffer per level.
	perLevelImpacts []*index.FreqAndNormBuffer

	// impactsView is the Impacts implementation returned by GetImpacts.
	impactsView index.Impacts
}

// NewLucene99ScoreSkipReader constructs a Lucene99ScoreSkipReader.
//
// Port of Lucene99ScoreSkipReader(IndexInput, int, boolean, boolean, boolean).
func NewLucene99ScoreSkipReader(
	skipStream store.IndexInput,
	maxSkipLevels int,
	hasPos bool,
	hasOffsets bool,
	hasPayloads bool,
) *Lucene99ScoreSkipReader {
	base := NewLucene99SkipReader(skipStream, maxSkipLevels, hasPos, hasOffsets, hasPayloads)

	r := &Lucene99ScoreSkipReader{
		Lucene99SkipReader: base,
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

	// Override the base reader's impact hook to buffer raw bytes instead of
	// discarding them.
	r.Lucene99SkipReader.readImpactsHook = r.readImpactsLevel

	// Build the Impacts view backed by this reader.
	r.impactsView = &lucene99ScoreSkipImpacts{r: r}

	return r
}

// SkipTo advances the skip cursor and updates numLevels.
//
// Port of Lucene99ScoreSkipReader.skipTo(int).
func (r *Lucene99ScoreSkipReader) SkipTo(target int) (int, error) {
	result, err := r.Lucene99SkipReader.SkipTo(target)
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
		b.GrowNoCopy(1)
		b.Freqs[0] = math.MaxInt32
		b.Norms[0] = 1
		r.impactDataLength[0] = 0
	}
	return result, nil
}

// GetImpacts returns the Impacts view for the current skip position.
func (r *Lucene99ScoreSkipReader) GetImpacts() index.Impacts {
	return r.impactsView
}

// readImpactsLevel overrides Lucene99SkipReader.readImpacts to buffer the
// raw encoded impact bytes rather than discarding them.
//
// Port of Lucene99ScoreSkipReader.readImpacts(int, IndexInput).
func (r *Lucene99ScoreSkipReader) readImpactsLevel(level int, skipStream store.IndexInput) error {
	n, err := store.ReadVInt(skipStream)
	if err != nil {
		return err
	}
	length := int(n)
	if cap(r.impactData[level]) < length {
		r.impactData[level] = make([]byte, util.Oversize(length, 1))
	}
	r.impactData[level] = r.impactData[level][:length]
	if err := skipStream.ReadBytes(r.impactData[level]); err != nil {
		return err
	}
	r.impactDataLength[level] = length
	return nil
}

// decodeImpacts lazily decodes the buffered impact bytes for the given level.
func (r *Lucene99ScoreSkipReader) decodeImpacts(level int) *index.FreqAndNormBuffer {
	if r.impactDataLength[level] > 0 {
		r.badi.ResetWithSlice(r.impactData[level], 0, r.impactDataLength[level])
		r.perLevelImpacts[level] = decodeImpacts99(r.badi, r.perLevelImpacts[level])
		r.impactDataLength[level] = 0
	}
	return r.perLevelImpacts[level]
}

// decodeImpacts99 decodes a sequence of (freq, norm) impact pairs from in
// and stores them in reuse.
//
// Encoding (same as all Lucene 8.x–9.x formats):
//   - Each entry is a VInt freqDelta.
//   - If bit 0 of freqDelta is set: freq += 1 + (freqDelta >> 1); norm
//     advances by 1 + ZigZag(VLong).
//   - Otherwise: freq += 1 + (freqDelta >> 1); norm++.
//
// Port of the static Lucene99ScoreSkipReader.readImpacts(ByteArrayDataInput, FreqAndNormBuffer).
func decodeImpacts99(
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
		freqDelta, _ := in.ReadVInt() // ByteArrayDataInput never returns I/O error
		fd := int(freqDelta)
		freq += 1 + (fd >> 1)
		if fd&0x01 != 0 {
			normDelta, _ := in.ReadVLong() // ByteArrayDataInput never returns I/O error
			norm += 1 + util.ZigZagDecodeInt64(normDelta)
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

// lucene99ScoreSkipImpacts implements index.Impacts backed by
// Lucene99ScoreSkipReader.
type lucene99ScoreSkipImpacts struct {
	r *Lucene99ScoreSkipReader
}

// NumLevels returns the number of active skip levels.
func (i *lucene99ScoreSkipImpacts) NumLevels() int { return i.r.numLevels }

// GetDocIDUpTo returns the max doc id covered by skip-level impacts.
func (i *lucene99ScoreSkipImpacts) GetDocIDUpTo(level int) int {
	return i.r.base.GetSkipDoc(level)
}

// GetImpacts lazily decodes and returns the impact buffer for the given level.
func (i *lucene99ScoreSkipImpacts) GetImpacts(level int) *index.FreqAndNormBuffer {
	return i.r.decodeImpacts(level)
}

// compile-time assertion
var _ index.Impacts = (*lucene99ScoreSkipImpacts)(nil)
