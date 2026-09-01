// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// FreqAndNormBuffer is a wrapper around parallel arrays storing term frequencies
// and length normalization factors.
// Mirrors org.apache.lucene.index.FreqAndNormBuffer from Apache Lucene 10.5.0.
type FreqAndNormBuffer struct {
	// Freqs stores term frequencies.
	Freqs []int32
	// Norms stores length normalization factors.
	Norms []int64
	// Size is the number of valid entries in the arrays.
	Size int
}

// NewFreqAndNormBuffer constructs a FreqAndNormBuffer.
func NewFreqAndNormBuffer() *FreqAndNormBuffer {
	return &FreqAndNormBuffer{}
}

// GrowNoCopy grows both arrays to ensure that they can store at least the given number of entries.
func (b *FreqAndNormBuffer) GrowNoCopy(minSize int) {
	if len(b.Freqs) < minSize {
		// Use a simple growth strategy similar to Lucene's oversize
		newSize := minSize
		if newSize < 10 {
			newSize = 10
		} else {
			newSize = (newSize * 3) / 2 + 1
		}
		b.Freqs = make([]int32, newSize)
		b.Norms = make([]int64, newSize)
	}
}

// Add adds the given pair of term frequency and norm at the end of this buffer,
// growing underlying arrays if necessary.
func (b *FreqAndNormBuffer) Add(freq int32, norm int64) {
	if len(b.Freqs) == b.Size {
		// grow
		newSize := len(b.Freqs) * 2
		if newSize == 0 {
			newSize = 10
		}
		newFreqs := make([]int32, newSize)
		newNorms := make([]int64, newSize)
		copy(newFreqs, b.Freqs)
		copy(newNorms, b.Norms)
		b.Freqs = newFreqs
		b.Norms = newNorms
	}
	b.Freqs[b.Size] = freq
	b.Norms[b.Size] = norm
	b.Size++
}
