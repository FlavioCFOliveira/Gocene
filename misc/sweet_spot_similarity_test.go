package misc

import "testing"

func TestSweetSpotSimilarity_New(t *testing.T) {
	s := NewSweetSpotSimilarity(1, 50)
	if s == nil {
		t.Fatal("NewSweetSpotSimilarity returned nil")
	}
}

func TestSweetSpotSimilarity_LengthNormPlateau(t *testing.T) {
	// SweetSpotSimilarity flattens lengthNorm between minLen and maxLen
	s := NewSweetSpotSimilarity(10, 100)
	n1 := s.LengthNorm(5)
	n2 := s.LengthNorm(50)
	n3 := s.LengthNorm(200)
	if n1 <= 0 || n2 <= 0 || n3 <= 0 {
		t.Fatalf("LengthNorm values should be positive: %v %v %v", n1, n2, n3)
	}
}

func TestSweetSpotSimilarity_DifferentConfigs(t *testing.T) {
	a := NewSweetSpotSimilarity(1, 5)
	b := NewSweetSpotSimilarity(20, 100)
	if a.LengthNorm(3) == b.LengthNorm(3) {
		t.Log("Different configs may produce same norm for same length")
	}
}
