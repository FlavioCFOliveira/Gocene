// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// FreqAndNormBuffer is the wrapper around parallel arrays storing term
// frequencies and length normalization factors used by Impacts. Mirrors
// org.apache.lucene.index.FreqAndNormBuffer from Apache Lucene 10.4.0.
//
// Both slices are kept identical in length once any growth happens. The
// caller is responsible for honouring the Size invariant: Freqs[:Size] and
// Norms[:Size] are the valid entries.
type FreqAndNormBuffer struct {
	// Freqs holds per-impact term frequencies.
	Freqs []int

	// Norms holds per-impact length-normalisation factors.
	Norms []int64

	// Size is the number of valid entries in Freqs/Norms.
	Size int
}

// NewFreqAndNormBuffer returns an empty FreqAndNormBuffer.
func NewFreqAndNormBuffer() *FreqAndNormBuffer { return &FreqAndNormBuffer{} }

// GrowNoCopy ensures both arrays can hold at least minSize entries. Existing
// contents are discarded if a reallocation is required.
func (b *FreqAndNormBuffer) GrowNoCopy(minSize int) {
	if len(b.Freqs) >= minSize {
		return
	}
	cap := oversizeImpacts(minSize)
	b.Freqs = make([]int, cap)
	b.Norms = make([]int64, cap)
}

// Add appends a (freq, norm) pair, growing the underlying arrays if necessary.
func (b *FreqAndNormBuffer) Add(freq int, norm int64) {
	if len(b.Freqs) == b.Size {
		grown := oversizeImpacts(b.Size + 1)
		nfreqs := make([]int, grown)
		nnorms := make([]int64, grown)
		copy(nfreqs, b.Freqs[:b.Size])
		copy(nnorms, b.Norms[:b.Size])
		b.Freqs = nfreqs
		b.Norms = nnorms
	}
	b.Freqs[b.Size] = freq
	b.Norms[b.Size] = norm
	b.Size++
}

// oversizeImpacts is a tiny growth heuristic equivalent to ArrayUtil.oversize
// (with bytesPerElement=4): grow by 12.5% plus a small floor.
func oversizeImpacts(minSize int) int {
	if minSize < 8 {
		return 8
	}
	extra := minSize >> 3
	return minSize + extra + 1
}

// Impacts conveys information about upcoming impacts (i.e. (freq, norm)
// pairs that may trigger non-zero scores) within a postings list. Mirrors
// org.apache.lucene.index.Impacts from Apache Lucene 10.4.0.
type Impacts interface {
	// NumLevels returns the number of levels of impact summary information.
	// Always > 0 and may differ across positions in the same postings list.
	NumLevels() int

	// GetDocIDUpTo returns the maximum inclusive doc ID up to which the
	// impacts returned by GetImpacts(level) are valid. Non-decreasing in level.
	GetDocIDUpTo(level int) int

	// GetImpacts returns the (freq, norm) impacts for the given level. The
	// returned buffer is never empty and is only guaranteed to be valid until
	// the iterator advances.
	GetImpacts(level int) *FreqAndNormBuffer
}

// ImpactsSource produces Impacts and supports shallow-advance to allow callers
// to retrieve more precise impact information for upcoming docs. Mirrors
// org.apache.lucene.index.ImpactsSource from Apache Lucene 10.4.0.
type ImpactsSource interface {
	// AdvanceShallow shallow-advances to target. Cheaper than calling Advance
	// on the underlying iterator and lets subsequent GetImpacts calls ignore
	// doc IDs less than target.
	AdvanceShallow(target int) error

	// GetImpacts returns Impacts for upcoming doc IDs greater than or equal
	// to the maximum of the current docID and the last AdvanceShallow target.
	GetImpacts() (Impacts, error)
}

// ImpactsEnum is the PostingsEnum extension that also exposes ImpactsSource.
// Mirrors org.apache.lucene.index.ImpactsEnum from Apache Lucene 10.4.0.
//
// Lucene defines an abstract class; Gocene uses an interface to align with
// how PostingsEnum and ImpactsSource are modelled. Implementations must
// satisfy both PostingsEnum and ImpactsSource.
type ImpactsEnum interface {
	PostingsEnum
	ImpactsSource
}
