// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ru

import "testing"

func stemString(st *RussianLightStemmer, word string) string {
	runes := []rune(word)
	n := st.Stem(runes, len(runes))
	return string(runes[:n])
}

// TestRussianLightStemmer_RemoveCase verifies case-ending removal (4-char
// suffix, 3-char suffix, 2-char suffix, 1-char suffix).
func TestRussianLightStemmer_RemoveCase(t *testing.T) {
	st := NewRussianLightStemmer()

	tests := []struct {
		input string
		want  string
	}{
		// 4-char suffix (length > 6)
		{"студиями", "студ"},   // иями
		{"мотоциклами", "мотоцикл"}, // ами (3-char, length > 5)
		// 3-char suffix (length > 5)
		{"красного", "красн"}, // ого
		{"красному", "красн"}, // ому
		// 2-char suffix (length > 4)
		{"красная", "красн"}, // ая
		{"красные", "красн"}, // ые
		{"красного", "красн"}, // ого (falls to 3-char first)
		// 1-char suffix (length > 3)
		{"слова", "слов"}, // а
		{"словами", "слов"}, // ами via 3-char
	}
	for _, tt := range tests {
		got := stemString(st, tt.input)
		if got != tt.want {
			t.Errorf("Stem(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestRussianLightStemmer_Normalize verifies the normalize step.
func TestRussianLightStemmer_Normalize(t *testing.T) {
	st := NewRussianLightStemmer()

	tests := []struct {
		input string
		want  string
	}{
		// trailing ь should be removed (after removeCase leaves length > 3)
		{"рубль", "рубл"},
		// trailing и should be removed
		{"жизни", "жизн"},
		// double н — ий stripped by removeCase (length 7→5 "осенн"),
		// then normalize strips trailing н (double нн, length 5→4 "осен")
		{"осенний", "осен"},
	}
	for _, tt := range tests {
		got := stemString(st, tt.input)
		if got != tt.want {
			t.Errorf("Stem(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestRussianLightStemmer_ShortWords verifies words below length threshold
// are not modified.
func TestRussianLightStemmer_ShortWords(t *testing.T) {
	st := NewRussianLightStemmer()
	for _, word := range []string{"он", "она", "мы"} {
		got := stemString(st, word)
		if got != word {
			t.Errorf("Stem(%q) = %q, want unchanged %q", word, got, word)
		}
	}
}
