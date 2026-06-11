package classification

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

func TestBooleanPerceptronClassifier_New(t *testing.T) {
	analyzer := analysis.NewWhitespaceAnalyzer()
	c := NewBooleanPerceptronClassifier(
		nil, analyzer, nil, nil, "class", "text")
	if c == nil {
		t.Fatal("NewBooleanPerceptronClassifier returned nil")
	}
}

func TestKNNClassifier_New(t *testing.T) {
	analyzer := analysis.NewWhitespaceAnalyzer()
	c := NewKNearestNeighborClassifier(
		nil, analyzer, nil, 3, 0, 0, "class")
	if c == nil {
		t.Fatal("NewKNearestNeighborClassifier returned nil")
	}
}

func TestKNNFuzzyClassifier_New(t *testing.T) {
	analyzer := analysis.NewWhitespaceAnalyzer()
	c := NewKNearestFuzzyClassifier(
		nil, analyzer, nil, 5, "class")
	if c == nil {
		t.Fatal("NewKNearestFuzzyClassifier returned nil")
	}
}
