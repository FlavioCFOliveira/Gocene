// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"testing"
)

// TestEnglishAnalyzer tests the EnglishAnalyzer
func TestEnglishAnalyzer(t *testing.T) {
	analyzer := NewEnglishAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "The quick brown foxes are running",
			minTokenCount: 1,
		},
		{
			input:         "I am testing the English analyzer",
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

// TestFrenchAnalyzer tests the FrenchAnalyzer
func TestFrenchAnalyzer(t *testing.T) {
	analyzer := NewFrenchAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Le chat noir court vite",
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

// TestGermanAnalyzer tests the GermanAnalyzer
func TestGermanAnalyzer(t *testing.T) {
	analyzer := NewGermanAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Der schnelle braune Fuchs läuft",
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

// TestSpanishAnalyzer tests the SpanishAnalyzer
func TestSpanishAnalyzer(t *testing.T) {
	analyzer := NewSpanishAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "El rápido zorro negro corre",
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

// TestPortugueseAnalyzer tests the PortugueseAnalyzer
func TestPortugueseAnalyzer(t *testing.T) {
	analyzer := NewPortugueseAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "A rápida raposa negra corre",
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

// TestItalianAnalyzer tests the ItalianAnalyzer
func TestItalianAnalyzer(t *testing.T) {
	analyzer := NewItalianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "La veloce volpe nera corre",
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

// TestRussianAnalyzer tests the RussianAnalyzer
func TestRussianAnalyzer(t *testing.T) {
	analyzer := NewRussianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "быстрая коричневая лиса бежит",
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

// TestDanishAnalyzer tests the DanishAnalyzer
func TestDanishAnalyzer(t *testing.T) {
	analyzer := NewDanishAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Den hurtige brune ræv løber",
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

// TestDutchAnalyzer tests the DutchAnalyzer
func TestDutchAnalyzer(t *testing.T) {
	analyzer := NewDutchAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "De snelle bruine vos rent",
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

// TestFinnishAnalyzer tests the FinnishAnalyzer
func TestFinnishAnalyzer(t *testing.T) {
	analyzer := NewFinnishAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Nopea ruskea kettu juoksee",
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

// TestGreekAnalyzer tests the GreekAnalyzer
func TestGreekAnalyzer(t *testing.T) {
	analyzer := NewGreekAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Η γρήγορη καφέ αλεπού τρέχει",
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

// TestNorwegianAnalyzer tests the NorwegianAnalyzer
func TestNorwegianAnalyzer(t *testing.T) {
	analyzer := NewNorwegianAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Den raske brune reven løper",
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

// TestSwedishAnalyzer tests the SwedishAnalyzer
func TestSwedishAnalyzer(t *testing.T) {
	analyzer := NewSwedishAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "Den snabba bruna räven springer",
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

// TestCustomAnalyzer tests the CustomAnalyzer
func TestCustomAnalyzer(t *testing.T) {
	builder := NewCustomAnalyzerBuilder().
		WithTokenizer(NewStandardTokenizerFactory()).
		AddTokenFilter(NewLowerCaseFilterFactory())

	analyzer, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build custom analyzer: %v", err)
	}
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "The Quick Brown Fox",
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

// TestCJKAnalyzer tests the CJKAnalyzer
func TestCJKAnalyzer(t *testing.T) {
	analyzer := NewCJKAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "中文测试",
			minTokenCount: 1,
		},
		{
			input:         "日本語テスト",
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

// TestChineseAnalyzer tests the ChineseAnalyzer
func TestChineseAnalyzer(t *testing.T) {
	analyzer := NewChineseAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "中文测试",
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

// TestJapaneseAnalyzer tests the JapaneseAnalyzer
func TestJapaneseAnalyzer(t *testing.T) {
	analyzer := NewJapaneseAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "日本語テスト",
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

// TestKoreanAnalyzer tests the KoreanAnalyzer
func TestKoreanAnalyzer(t *testing.T) {
	analyzer := NewKoreanAnalyzer()
	defer analyzer.Close()

	tests := []struct {
		input         string
		minTokenCount int
	}{
		{
			input:         "한국어 테스트",
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
