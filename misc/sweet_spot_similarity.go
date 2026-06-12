package misc

import "math"

// SweetSpotSimilarity is the BM25-derived similarity that flattens lengthNorm
// within a "sweet spot" length range. Mirrors
// org.apache.lucene.misc.SweetSpotSimilarity.
type SweetSpotSimilarity struct {
	MinLen    int
	MaxLen    int
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

// LengthNorm computes the length-normalised score factor using Lucene's
// unified formula:
//
//	1 / sqrt( steepness * (abs(x-min) + abs(x-max) - (max-min)) + 1 )
//
// When length is inside the [min, max] plateau the inner expression evaluates
// to zero, yielding 1.0.
func (s *SweetSpotSimilarity) LengthNorm(length int) float32 {
	min := float64(s.MinLen)
	max := float64(s.MaxLen)
	steep := s.Steepness
	x := float64(length)

	inner := steep * (math.Abs(x-min) + math.Abs(x-max) - (max-min))
	return float32(1.0 / math.Sqrt(inner+1.0))
}
