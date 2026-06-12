package misc

import (
	"math"
	"testing"
)

func TestSweetSpotSimilarity_New(t *testing.T) {
	s := NewSweetSpotSimilarity(1, 50)
	if s == nil {
		t.Fatal("NewSweetSpotSimilarity returned nil")
	}
	if s.MinLen != 1 {
		t.Fatalf("MinLen expected 1, got %d", s.MinLen)
	}
	if s.MaxLen != 50 {
		t.Fatalf("MaxLen expected 50, got %d", s.MaxLen)
	}
	if s.Steepness != 0.5 {
		t.Fatalf("Steepness expected 0.5, got %v", s.Steepness)
	}
}

func TestSweetSpotSimilarity_NewClampsMinLen(t *testing.T) {
	s := NewSweetSpotSimilarity(0, 5)
	if s.MinLen != 1 {
		t.Fatalf("MinLen expected clamped to 1, got %d", s.MinLen)
	}
}

func TestSweetSpotSimilarity_NewClampsMaxLen(t *testing.T) {
	s := NewSweetSpotSimilarity(5, 3)
	if s.MaxLen != 5 {
		t.Fatalf("MaxLen expected clamped to MinLen (5), got %d", s.MaxLen)
	}
}

func TestSweetSpotSimilarity_LengthNormPlateau(t *testing.T) {
	// Between minLen and maxLen the norm must be exactly 1.0.
	s := NewSweetSpotSimilarity(3, 8)
	for length := 3; length <= 8; length++ {
		got := s.LengthNorm(length)
		if got != 1.0 {
			t.Fatalf("LengthNorm(%d) inside plateau expected 1.0, got %v", length, got)
		}
	}
}

// javaLengthNorm computes the same unified formula as Lucene's
// SweetSpotSimilarity.lengthNorm so the test can assert against an
// independent reference.
func javaLengthNorm(length, minLen, maxLen int, steepness float64) float32 {
	l := float64(minLen)
	h := float64(maxLen)
	x := float64(length)
	s := steepness
	inner := s*(math.Abs(x-l)+math.Abs(x-h)-(h-l)) + 1.0
	return float32(1.0 / math.Sqrt(inner))
}

func TestSweetSpotSimilarity_LengthNormAgainstFormula(t *testing.T) {
	s := &SweetSpotSimilarity{MinLen: 3, MaxLen: 8, Steepness: 0.3}

	cases := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 15, 20}
	for _, length := range cases {
		got := s.LengthNorm(length)
		want := javaLengthNorm(length, s.MinLen, s.MaxLen, s.Steepness)
		if math.Abs(float64(got-want)) > 1e-6 {
			t.Fatalf("LengthNorm(%d) expected %v, got %v", length, want, got)
		}
	}
}

func TestSweetSpotSimilarity_LengthNormMonotonic(t *testing.T) {
	// Outside the plateau, norms should increase monotonically as we approach
	// the sweet spot from below, and decrease monotonically as we move past
	// the plateau.
	s := NewSweetSpotSimilarity(5, 10)

	var last float32
	for length := 1; length < 5; length++ {
		got := s.LengthNorm(length)
		if length > 1 && got <= last {
			t.Fatalf("LengthNorm(%d)=%v should be > LengthNorm(%d)=%v (increasing toward plateau)", length, got, length-1, last)
		}
		last = got
	}

	last = 1.0
	for length := 11; length <= 30; length++ {
		got := s.LengthNorm(length)
		if got >= last {
			t.Fatalf("LengthNorm(%d)=%v should be < LengthNorm(%d)=%v (decreasing past plateau)", length, got, length-1, last)
		}
		last = got
	}
}

func TestSweetSpotSimilarity_DifferentConfigs(t *testing.T) {
	a := NewSweetSpotSimilarity(1, 5)
	b := NewSweetSpotSimilarity(20, 100)
	if a.LengthNorm(3) == b.LengthNorm(3) {
		t.Log("Different configs may produce same norm for same length")
	}
}

func TestSweetSpotSimilarity_DegeneratesToClassic(t *testing.T) {
	// When min=max=1 and steepness=0.5 the formula degrades to 1/sqrt(x).
	s := &SweetSpotSimilarity{MinLen: 1, MaxLen: 1, Steepness: 0.5}
	for length := 1; length <= 20; length++ {
		got := s.LengthNorm(length)
		want := float32(1.0 / math.Sqrt(float64(length)))
		if math.Abs(float64(got-want)) > 1e-6 {
			t.Fatalf("LengthNorm(%d) expected %v (1/sqrt), got %v", length, want, got)
		}
	}
}
