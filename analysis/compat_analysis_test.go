package analysis

import "testing"

func TestWhitespaceAnalyzer_New(t *testing.T) {
	a := NewWhitespaceAnalyzer()
	if a == nil {
		t.Fatal("NewWhitespaceAnalyzer returned nil")
	}
}

func TestStandardAnalyzer_New(t *testing.T) {
	a := NewStandardAnalyzer()
	if a == nil {
		t.Fatal("NewStandardAnalyzer returned nil")
	}
}

func TestSimpleAnalyzer_New(t *testing.T) {
	a := NewSimpleAnalyzer()
	if a == nil {
		t.Fatal("NewSimpleAnalyzer returned nil")
	}
}

func TestStopAnalyzer_New(t *testing.T) {
	a := NewStopAnalyzer()
	if a == nil {
		t.Fatal("NewStopAnalyzer returned nil")
	}
}
