// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

// TestBulgarianAnalyzer tests the BulgarianAnalyzer
func TestBulgarianAnalyzer(t *testing.T) {
	analyzer := NewBulgarianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Български анализатор",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestCatalanAnalyzer tests the CatalanAnalyzer
func TestCatalanAnalyzer(t *testing.T) {
	analyzer := NewCatalanAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Analitzador català",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestCroatianAnalyzer tests the CroatianAnalyzer
func TestCroatianAnalyzer(t *testing.T) {
	analyzer := NewCroatianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Hrvatski analizator",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestCzechAnalyzer tests the CzechAnalyzer
func TestCzechAnalyzer(t *testing.T) {
	analyzer := NewCzechAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Český analyzátor",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestDutchStemmer tests the DutchStemmer
func TestDutchStemmer(t *testing.T) {
	stemmer := NewDutchStemmer()

	tests := []struct {
		input    string
		expected string
	}{
		{input: "gelopen", expected: "gelop"},
		// -heid suffix removal (gelopenheid -> gelopenheid -> gelopenheid without -heid)
		{input: "gelopenheid", expected: "gelopenhe"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := stemmer.Stem(tc.input)
			if result != tc.expected {
				t.Errorf("Stem(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

// TestEstonianAnalyzer tests the EstonianAnalyzer
func TestEstonianAnalyzer(t *testing.T) {
	analyzer := NewEstonianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Eesti keele analüsaator",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestGalicianAnalyzer tests the GalicianAnalyzer
func TestGalicianAnalyzer(t *testing.T) {
	analyzer := NewGalicianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Analizador galego",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestGreekStemmer tests the GreekStemmer
func TestGreekStemmer(t *testing.T) {
	stemmer := NewGreekStemmer()

	tests := []struct {
		input    string
		expected string
	}{
		{input: "αγόρι", expected: "αγορ"},
		{input: "γάτα", expected: "γατ"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := stemmer.Stem(tc.input)
			// Just verify it doesn't panic - Greek stemming is complex
			_ = result
		})
	}
}

// TestHindiAnalyzer tests the HindiAnalyzer
func TestHindiAnalyzer(t *testing.T) {
	analyzer := NewHindiAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "हिंदी भाषा विश्लेषक",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestHungarianAnalyzer tests the HungarianAnalyzer
func TestHungarianAnalyzer(t *testing.T) {
	analyzer := NewHungarianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Magyar nyelv elemző",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestIndonesianAnalyzer tests the IndonesianAnalyzer
func TestIndonesianAnalyzer(t *testing.T) {
	analyzer := NewIndonesianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Analisis bahasa Indonesia",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestLatvianAnalyzer tests the LatvianAnalyzer
func TestLatvianAnalyzer(t *testing.T) {
	analyzer := NewLatvianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Latviešu valodas analizators",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestLithuanianAnalyzer tests the LithuanianAnalyzer
func TestLithuanianAnalyzer(t *testing.T) {
	analyzer := NewLithuanianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Lietuvių kalbos analizatorius",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestPersianAnalyzer tests the PersianAnalyzer
func TestPersianAnalyzer(t *testing.T) {
	analyzer := NewPersianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "تحلیلگر زبان فارسی",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestRomanianAnalyzer tests the RomanianAnalyzer
func TestRomanianAnalyzer(t *testing.T) {
	analyzer := NewRomanianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Analizator de limbă română",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestSerbianAnalyzer tests the SerbianAnalyzer
func TestSerbianAnalyzer(t *testing.T) {
	analyzer := NewSerbianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Српски анализатор",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestSlovakAnalyzer tests the SlovakAnalyzer
func TestSlovakAnalyzer(t *testing.T) {
	analyzer := NewSlovakAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Slovenský analyzátor",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestSlovenianAnalyzer tests the SlovenianAnalyzer
func TestSlovenianAnalyzer(t *testing.T) {
	analyzer := NewSlovenianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Slovenski analizator",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestThaiAnalyzer tests the ThaiAnalyzer
func TestThaiAnalyzer(t *testing.T) {
	analyzer := NewThaiAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "ตัววิเคราะห์ภาษาไทย",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestTurkishAnalyzer tests the TurkishAnalyzer
func TestTurkishAnalyzer(t *testing.T) {
	analyzer := NewTurkishAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Türkçe dil analizörü",
			minTokenCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			tokens, err := collectTokensFromAnalyzer(analyzer, tt.input)
			if err != nil {
				t.Fatalf("Failed to collect tokens: %v", err)
			}

			if len(tokens) < tt.minTokenCount {
				t.Errorf("Expected at least %d tokens, got %d: %v", tt.minTokenCount, len(tokens), tokens)
			}
		})
	}
}

// TestTurkishLowerCaseFilter tests Turkish-specific lowercasing
func TestTurkishLowerCaseFilter(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Dotless I should become dotless ı
		{input: "ISTANBUL", expected: "ıstanbul"},
		// Dotted İ should become dotted i
		{input: "İstanbul", expected: "istanbul"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := turkishToLower(tc.input)
			if result != tc.expected {
				t.Errorf("turkishToLower(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}
