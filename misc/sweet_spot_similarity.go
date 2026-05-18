package misc

import "math"

// SweetSpotSimilarity is the BM25-derived similarity that flattens lengthNorm
// within a "sweet spot" length range. Mirrors
// org.apache.lucene.misc.SweetSpotSimilarity.
type SweetSpotSimilarity struct {
	MinLen   int
	MaxLen   int
	Steepness float64
}

// NewSweetSpotSimilarity builds the similarity with default steepness 0.5.
func NewSweetSpotSimilarity(minLen, maxLen int) *SweetSpotSimilarity {
	if minLen < 1 {
		minLen = 1
	}
	if maxLen < minLen {
		maxLen = minLen
	}
	return &SweetSpotSimilarity{MinLen: minLen, MaxLen: maxLen, Steepness: 0.5}
}

// LengthNorm computes the length-normalised score factor.
func (s *SweetSpotSimilarity) LengthNorm(length int) float32 {
	if length >= s.MinLen && length <= s.MaxLen {
		return 1.0
	}
	if length < s.MinLen {
		diff := float64(s.MinLen - length)
		return float32(1.0 / (1.0 + s.Steepness*diff))
	}
	diff := float64(length - s.MaxLen)
	return float32(1.0 / (1.0 + s.Steepness*math.Sqrt(diff)))
}
