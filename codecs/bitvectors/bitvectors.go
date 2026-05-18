// Package bitvectors implements org.apache.lucene.codecs.bitvectors:
// flat / HNSW vector formats specialised for 1-bit quantised vectors.
package bitvectors

// FlatBitVectorsScorer computes the dot-product score between two 1-bit
// quantised vectors. Mirrors
// org.apache.lucene.codecs.bitvectors.FlatBitVectorsScorer.
type FlatBitVectorsScorer struct{}

// Score returns the population-count of the AND of two equal-length bit
// vectors expressed as []byte.
func (FlatBitVectorsScorer) Score(a, b []byte) int {
	if len(a) != len(b) {
		return 0
	}
	count := 0
	for i := range a {
		count += popcount8(a[i] & b[i])
	}
	return count
}

func popcount8(b byte) int {
	b = (b & 0x55) + ((b >> 1) & 0x55)
	b = (b & 0x33) + ((b >> 2) & 0x33)
	return int((b & 0x0f) + ((b >> 4) & 0x0f))
}

// HnswBitVectorsFormat is the HNSW codec specialised for bit-vector lanes.
// Mirrors org.apache.lucene.codecs.bitvectors.HnswBitVectorsFormat.
type HnswBitVectorsFormat struct {
	MaxConn  int
	BeamWidth int
}

// NewHnswBitVectorsFormat builds the format with the supplied HNSW
// parameters; Lucene defaults are M=16, efConstruction=100.
func NewHnswBitVectorsFormat(maxConn, beamWidth int) *HnswBitVectorsFormat {
	if maxConn < 1 {
		maxConn = 16
	}
	if beamWidth < 1 {
		beamWidth = 100
	}
	return &HnswBitVectorsFormat{MaxConn: maxConn, BeamWidth: beamWidth}
}
