package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FreqAndNormBuffer is a wrapper around parallel arrays storing term frequencies and length normalization factors.
// This is the Go port of Lucene's org.apache.lucene.index.FreqAndNormBuffer.
type FreqAndNormBuffer struct {
	// Freqs is the array of term frequencies.
	Freqs []int
	// Norms is the array of length normalization factors.
	Norms []int64
	// Size is the number of valid entries in the doc ID and float-valued feature arrays.
	Size int
}

// NewFreqAndNormBuffer creates a new FreqAndNormBuffer.
func NewFreqAndNormBuffer() *FreqAndNormBuffer {
	return &FreqAndNormBuffer{
		Freqs: []int{},
		Norms: []int64{},
		Size:  0,
	}
}

// GrowNoCopy grows both arrays to ensure that they can store at least the given number of entries.
func (f *FreqAndNormBuffer) GrowNoCopy(minSize int) {
	if len(f.Freqs) < minSize {
		newSize := util.Oversize(minSize, 4)
		f.Freqs = make([]int, newSize)
		f.Norms = make([]int64, newSize)
	}
}

// Add adds the given pair of term frequency and norm at the end of this buffer, growing underlying
// arrays if necessary.
func (f *FreqAndNormBuffer) Add(freq int, norm int64) {
	if len(f.Freqs) == f.Size {
		// Use append to handle growth idiomaticly in Go
		f.Freqs = append(f.Freqs, 0)
		f.Norms = append(f.Norms, 0)
	}
	f.Freqs[f.Size] = freq
	f.Norms[f.Size] = norm
	f.Size++
}
