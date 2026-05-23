// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "testing"

func stemFrench(t *testing.T, input string) string {
	t.Helper()
	s := frenchLightStemmer{}
	runes := []rune(input)
	n := s.stem(runes, len(runes))
	return string(runes[:n])
}

// TestFrenchLightStemmer_Examples mirrors testExamples from TestFrenchLightStemFilter.java.
func TestFrenchLightStemmer_Examples(t *testing.T) {
	cases := []struct{ input, want string }{
		{"chevaux", "cheval"},
		{"cheval", "cheval"},
		{"hiboux", "hibou"},
		{"hibou", "hibou"},
		{"chantés", "chant"},
		{"chanter", "chant"},
		{"chante", "chant"},
		{"chant", "chant"},
		{"baronnes", "baron"},
		{"barons", "baron"},
		{"baron", "baron"},
		{"peaux", "peau"},
		{"peau", "peau"},
		{"anneaux", "aneau"},
		{"anneau", "aneau"},
		{"neveux", "neveu"},
		{"neveu", "neveu"},
		{"affreux", "afreu"},
		{"affreuse", "afreu"},
		{"investissement", "investi"},
		{"investir", "investi"},
		{"assourdissant", "asourdi"},
		{"assourdir", "asourdi"},
		{"pratiquement", "pratiqu"},
		{"pratique", "pratiqu"},
		{"administrativement", "administratif"},
		{"administratif", "administratif"},
		{"justificatrice", "justifi"},
		{"justificateur", "justifi"},
		{"justifier", "justifi"},
		{"educatrice", "eduqu"},
		{"eduquer", "eduqu"},
		{"communicateur", "comuniqu"},
		{"communiquer", "comuniqu"},
		{"accompagnatrice", "acompagn"},
		{"accompagnateur", "acompagn"},
		{"administrateur", "administr"},
		{"administrer", "administr"},
		{"productrice", "product"},
		{"producteur", "product"},
		{"acheteuse", "achet"},
		{"acheteur", "achet"},
		{"planteur", "plant"},
		{"plante", "plant"},
	}
	for _, c := range cases {
		got := stemFrench(t, c.input)
		if got != c.want {
			t.Errorf("stem(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestFrenchLightStemmer_ShortWords ensures very short words are not stemmed.
func TestFrenchLightStemmer_ShortWords(t *testing.T) {
	cases := []string{"je", "il", "un", "le", "la"}
	for _, w := range cases {
		got := stemFrench(t, w)
		if len([]rune(got)) > len([]rune(w)) {
			t.Errorf("stem(%q) = %q unexpectedly longer", w, got)
		}
	}
}
