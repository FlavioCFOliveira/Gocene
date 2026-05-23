// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package br provides Brazilian Portuguese analysis components.
package br

import "strings"

// BrazilianStemmer is a stemmer for Brazilian Portuguese words.
//
// This is the Go port of
// org.apache.lucene.analysis.br.BrazilianStemmer from
// Apache Lucene 10.4.0.
type BrazilianStemmer struct {
	// TERM is the original term joined with CT for debug purposes.
	term string
	ct   string
	r1   string
	r2   string
	rv   string
}

// Stem reduces the given term to a discriminator string and returns it.
// Returns "" if the term cannot be indexed (length ≤ 2 or ≥ 30, or contains
// non-letter characters after normalization).
func (s *BrazilianStemmer) Stem(term string) string {
	s.createCT(term)

	if !isIndexable(s.ct) {
		return ""
	}
	if !isStemmable(s.ct) {
		return s.ct
	}

	s.r1 = getR1(s.ct)
	s.r2 = getR1(s.r1)
	s.rv = getRV(s.ct)
	s.term = term + ";" + s.ct

	altered := s.step1()
	if !altered {
		altered = s.step2()
	}

	if altered {
		s.step3()
	} else {
		s.step4()
	}

	s.step5()

	return s.ct
}

// isStemmable returns true if every character in term is a letter.
func isStemmable(term string) bool {
	for _, ch := range term {
		if !isLetter(ch) {
			return false
		}
	}
	return true
}

// isIndexable returns true if 2 < len(term) < 30.
func isIndexable(term string) bool {
	n := len(term)
	return n > 2 && n < 30
}

// isVowel reports whether value is a, e, i, o, or u.
func isVowel(value byte) bool {
	switch value {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// isLetter reports whether r is a Unicode letter.
func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		r == 'à' || r == 'á' || r == 'â' || r == 'ã' ||
		r == 'é' || r == 'ê' ||
		r == 'í' ||
		r == 'ó' || r == 'ô' || r == 'õ' ||
		r == 'ú' || r == 'ü' ||
		r == 'ç' || r == 'ñ'
}

// getR1 returns the region after the first non-vowel following a vowel, or ""
// if no such region exists.  The input is assumed to consist only of ASCII
// lowercase letters (post-changeTerm).
func getR1(value string) string {
	if value == "" {
		return ""
	}

	i := len(value) - 1 // last valid index

	// find first vowel
	j := 0
	for ; j < i; j++ {
		if isVowel(value[j]) {
			break
		}
	}
	if j >= i {
		return ""
	}

	// find first non-vowel after that vowel
	for ; j < i; j++ {
		if !isVowel(value[j]) {
			break
		}
	}
	if j >= i {
		return ""
	}

	return value[j+1:]
}

// getRV computes the RV region:
//   - If the second letter is a consonant, RV is the region after the next vowel.
//   - If the first two letters are vowels, RV is the region after the next consonant.
//   - Otherwise (consonant-vowel) RV is the region after the third letter.
//   - RV is "" if the positions cannot be found.
func getRV(value string) string {
	if value == "" {
		return ""
	}

	i := len(value) - 1

	// second letter is a consonant
	if i > 0 && !isVowel(value[1]) {
		for j := 2; j < i; j++ {
			if isVowel(value[j]) {
				return value[j+1:]
			}
		}
	}

	// first two letters are vowels
	if i > 1 && isVowel(value[0]) && isVowel(value[1]) {
		for j := 2; j < i; j++ {
			if !isVowel(value[j]) {
				return value[j+1:]
			}
		}
	}

	// consonant-vowel: region after third letter
	if i > 2 {
		return value[3:]
	}

	return ""
}

// changeTerm lowercases and removes diacritics from value.
func changeTerm(value string) string {
	if value == "" {
		return ""
	}
	value = strings.ToLower(value)
	var b strings.Builder
	b.Grow(len(value))
	for _, ch := range value {
		switch ch {
		case 'á', 'â', 'ã':
			b.WriteByte('a')
		case 'é', 'ê':
			b.WriteByte('e')
		case 'í':
			b.WriteByte('i')
		case 'ó', 'ô', 'õ':
			b.WriteByte('o')
		case 'ú', 'ü':
			b.WriteByte('u')
		case 'ç':
			b.WriteByte('c')
		case 'ñ':
			b.WriteByte('n')
		default:
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// createCT builds CT: change term, then strip leading/trailing punctuation.
func (s *BrazilianStemmer) createCT(term string) {
	s.ct = changeTerm(term)

	if len(s.ct) < 2 {
		return
	}

	// strip leading punctuation
	switch s.ct[0] {
	case '"', '\'', '-', ',', ';', '.', '?', '!':
		s.ct = s.ct[1:]
	}

	if len(s.ct) < 2 {
		return
	}

	// strip trailing punctuation
	last := s.ct[len(s.ct)-1]
	switch last {
	case '-', ',', ';', '.', '?', '!', '\'', '"':
		s.ct = s.ct[:len(s.ct)-1]
	}
}

// suffix reports whether value ends with suffix.
func sfx(value, suffix string) bool {
	if value == "" || suffix == "" {
		return false
	}
	return strings.HasSuffix(value, suffix)
}

// removeSuffix removes toRemove from the end of value.
func removeSuffix(value, toRemove string) string {
	if value == "" || toRemove == "" || !sfx(value, toRemove) {
		return value
	}
	return value[:len(value)-len(toRemove)]
}

// replaceSuffix replaces toReplace at the end of value with changeTo.
func replaceSuffix(value, toReplace, changeTo string) string {
	vv := removeSuffix(value, toReplace)
	if vv == value {
		return value
	}
	return vv + changeTo
}

// suffixPreceded reports whether value ends with suffix and the part before
// suffix ends with preceded.
func suffixPreceded(value, suffix, preceded string) bool {
	if value == "" || suffix == "" || preceded == "" || !sfx(value, suffix) {
		return false
	}
	return sfx(removeSuffix(value, suffix), preceded)
}

// step1 performs standard suffix removal. Returns true if a suffix was removed.
func (s *BrazilianStemmer) step1() bool {
	ct := s.ct

	// suffix length = 7
	if sfx(ct, "uciones") && sfx(s.r2, "uciones") {
		s.ct = replaceSuffix(ct, "uciones", "u")
		return true
	}

	// suffix length = 6
	if len(ct) >= 6 {
		if sfx(ct, "imentos") && sfx(s.r2, "imentos") {
			s.ct = removeSuffix(ct, "imentos")
			return true
		}
		if sfx(ct, "amentos") && sfx(s.r2, "amentos") {
			s.ct = removeSuffix(ct, "amentos")
			return true
		}
		if sfx(ct, "adores") && sfx(s.r2, "adores") {
			s.ct = removeSuffix(ct, "adores")
			return true
		}
		if sfx(ct, "adoras") && sfx(s.r2, "adoras") {
			s.ct = removeSuffix(ct, "adoras")
			return true
		}
		if sfx(ct, "logias") && sfx(s.r2, "logias") {
			// Note: Java source calls replaceSuffix but forgets to assign CT —
			// this is a Lucene bug; the assignment is absent. We faithfully
			// replicate the bug: CT is NOT modified.
			_ = replaceSuffix(ct, "logias", "log")
			return true
		}
		if sfx(ct, "encias") && sfx(s.r2, "encias") {
			s.ct = replaceSuffix(ct, "encias", "ente")
			return true
		}
		if sfx(ct, "amente") && sfx(s.r1, "amente") {
			s.ct = removeSuffix(ct, "amente")
			return true
		}
		if sfx(ct, "idades") && sfx(s.r2, "idades") {
			s.ct = removeSuffix(ct, "idades")
			return true
		}
	}

	// suffix length = 5
	if len(ct) >= 5 {
		if sfx(ct, "acoes") && sfx(s.r2, "acoes") {
			s.ct = removeSuffix(ct, "acoes")
			return true
		}
		if sfx(ct, "imento") && sfx(s.r2, "imento") {
			s.ct = removeSuffix(ct, "imento")
			return true
		}
		if sfx(ct, "amento") && sfx(s.r2, "amento") {
			s.ct = removeSuffix(ct, "amento")
			return true
		}
		if sfx(ct, "adora") && sfx(s.r2, "adora") {
			s.ct = removeSuffix(ct, "adora")
			return true
		}
		if sfx(ct, "ismos") && sfx(s.r2, "ismos") {
			s.ct = removeSuffix(ct, "ismos")
			return true
		}
		if sfx(ct, "istas") && sfx(s.r2, "istas") {
			s.ct = removeSuffix(ct, "istas")
			return true
		}
		if sfx(ct, "logia") && sfx(s.r2, "logia") {
			s.ct = replaceSuffix(ct, "logia", "log")
			return true
		}
		if sfx(ct, "ucion") && sfx(s.r2, "ucion") {
			s.ct = replaceSuffix(ct, "ucion", "u")
			return true
		}
		if sfx(ct, "encia") && sfx(s.r2, "encia") {
			s.ct = replaceSuffix(ct, "encia", "ente")
			return true
		}
		if sfx(ct, "mente") && sfx(s.r2, "mente") {
			s.ct = removeSuffix(ct, "mente")
			return true
		}
		if sfx(ct, "idade") && sfx(s.r2, "idade") {
			s.ct = removeSuffix(ct, "idade")
			return true
		}
	}

	// suffix length = 4
	if len(ct) >= 4 {
		if sfx(ct, "acao") && sfx(s.r2, "acao") {
			s.ct = removeSuffix(ct, "acao")
			return true
		}
		if sfx(ct, "ezas") && sfx(s.r2, "ezas") {
			s.ct = removeSuffix(ct, "ezas")
			return true
		}
		if sfx(ct, "icos") && sfx(s.r2, "icos") {
			s.ct = removeSuffix(ct, "icos")
			return true
		}
		if sfx(ct, "icas") && sfx(s.r2, "icas") {
			s.ct = removeSuffix(ct, "icas")
			return true
		}
		if sfx(ct, "ismo") && sfx(s.r2, "ismo") {
			s.ct = removeSuffix(ct, "ismo")
			return true
		}
		if sfx(ct, "avel") && sfx(s.r2, "avel") {
			s.ct = removeSuffix(ct, "avel")
			return true
		}
		if sfx(ct, "ivel") && sfx(s.r2, "ivel") {
			s.ct = removeSuffix(ct, "ivel")
			return true
		}
		if sfx(ct, "ista") && sfx(s.r2, "ista") {
			s.ct = removeSuffix(ct, "ista")
			return true
		}
		if sfx(ct, "osos") && sfx(s.r2, "osos") {
			s.ct = removeSuffix(ct, "osos")
			return true
		}
		if sfx(ct, "osas") && sfx(s.r2, "osas") {
			s.ct = removeSuffix(ct, "osas")
			return true
		}
		if sfx(ct, "ador") && sfx(s.r2, "ador") {
			s.ct = removeSuffix(ct, "ador")
			return true
		}
		if sfx(ct, "ivas") && sfx(s.r2, "ivas") {
			s.ct = removeSuffix(ct, "ivas")
			return true
		}
		if sfx(ct, "ivos") && sfx(s.r2, "ivos") {
			s.ct = removeSuffix(ct, "ivos")
			return true
		}
		if sfx(ct, "iras") && sfx(s.rv, "iras") && suffixPreceded(ct, "iras", "e") {
			s.ct = replaceSuffix(ct, "iras", "ir")
			return true
		}
	}

	// suffix length = 3
	if len(ct) >= 3 {
		if sfx(ct, "eza") && sfx(s.r2, "eza") {
			s.ct = removeSuffix(ct, "eza")
			return true
		}
		if sfx(ct, "ico") && sfx(s.r2, "ico") {
			s.ct = removeSuffix(ct, "ico")
			return true
		}
		if sfx(ct, "ica") && sfx(s.r2, "ica") {
			s.ct = removeSuffix(ct, "ica")
			return true
		}
		if sfx(ct, "oso") && sfx(s.r2, "oso") {
			s.ct = removeSuffix(ct, "oso")
			return true
		}
		if sfx(ct, "osa") && sfx(s.r2, "osa") {
			s.ct = removeSuffix(ct, "osa")
			return true
		}
		if sfx(ct, "iva") && sfx(s.r2, "iva") {
			s.ct = removeSuffix(ct, "iva")
			return true
		}
		if sfx(ct, "ivo") && sfx(s.r2, "ivo") {
			s.ct = removeSuffix(ct, "ivo")
			return true
		}
		if sfx(ct, "ira") && sfx(s.rv, "ira") && suffixPreceded(ct, "ira", "e") {
			s.ct = replaceSuffix(ct, "ira", "ir")
			return true
		}
	}

	return false
}

// step2 removes verb suffixes in RV.
func (s *BrazilianStemmer) step2() bool {
	if s.rv == "" {
		return false
	}
	rv := s.rv

	// suffix length = 7
	if len(rv) >= 7 {
		for _, suf := range []string{
			"issemos", "essemos", "assemos",
			"ariamos", "eriamos", "iriamos",
		} {
			if sfx(rv, suf) {
				s.ct = removeSuffix(s.ct, suf)
				return true
			}
		}
	}

	// suffix length = 6
	if len(rv) >= 6 {
		for _, suf := range []string{
			"iremos", "eremos", "aremos", "avamos",
			"iramos", "eramos", "aramos",
			"asseis", "esseis", "isseis",
			"arieis", "erieis", "irieis",
		} {
			if sfx(rv, suf) {
				s.ct = removeSuffix(s.ct, suf)
				return true
			}
		}
	}

	// suffix length = 5
	if len(rv) >= 5 {
		for _, suf := range []string{
			"irmos", "iamos", "armos", "ermos",
			"areis", "ereis", "ireis",
			"asses", "esses", "isses",
			"astes", "assem", "essem", "issem",
			"ardes", "erdes", "irdes",
			"ariam", "eriam", "iriam",
			"arias", "erias", "irias",
			"estes", "istes",
			"aveis",
		} {
			if sfx(rv, suf) {
				s.ct = removeSuffix(s.ct, suf)
				return true
			}
		}
		// duplicate "areis" in Java already handled above
	}

	// suffix length = 4
	if len(rv) >= 4 {
		for _, suf := range []string{
			"aria", "eria", "iria",
			"asse", "esse", "isse",
			"aste", "este", "iste",
			"arei", "erei", "irei",
			"aram", "eram", "iram", "avam",
			"arem", "erem", "irem",
			"ando", "endo", "indo",
			"arao", "erao", "irao",
			"adas", "idas",
			"aras", "eras", "iras", "avas",
			"ares", "eres", "ires",
			"ados", "idos",
			"amos", "emos", "imos",
			"ieis",
		} {
			if sfx(rv, suf) {
				s.ct = removeSuffix(s.ct, suf)
				return true
			}
		}
	}

	// suffix length = 3
	if len(rv) >= 3 {
		for _, suf := range []string{
			"ada", "ida", "ara", "era",
		} {
			if sfx(rv, suf) {
				s.ct = removeSuffix(s.ct, suf)
				return true
			}
		}
		// "ira" → remove "ava" (Lucene bug faithfully replicated)
		if sfx(rv, "ira") {
			s.ct = removeSuffix(s.ct, "ava")
			return true
		}
		for _, suf := range []string{
			"iam", "ado", "ido", "ias", "ais", "eis",
		} {
			if sfx(rv, suf) {
				s.ct = removeSuffix(s.ct, suf)
				return true
			}
		}
		// second "ira" check in Java (no-op if first already matched)
		if sfx(rv, "ira") {
			s.ct = removeSuffix(s.ct, "ira")
			return true
		}
		if sfx(rv, "ear") {
			s.ct = removeSuffix(s.ct, "ear")
			return true
		}
	}

	// suffix length = 2
	if len(rv) >= 2 {
		for _, suf := range []string{
			"ia", "ei", "am", "em",
			"ar", "er", "ir",
			"as", "es", "is",
			"eu", "iu", "ou",
		} {
			if sfx(rv, suf) {
				s.ct = removeSuffix(s.ct, suf)
				return true
			}
		}
	}

	return false
}

// step3 deletes suffix 'i' if in RV and preceded by 'c'.
func (s *BrazilianStemmer) step3() {
	if s.rv == "" {
		return
	}
	if sfx(s.rv, "i") && suffixPreceded(s.rv, "i", "c") {
		s.ct = removeSuffix(s.ct, "i")
	}
}

// step4 deletes residual suffixes in RV: os, a, i, o.
func (s *BrazilianStemmer) step4() {
	if s.rv == "" {
		return
	}
	for _, suf := range []string{"os", "a", "i", "o"} {
		if sfx(s.rv, suf) {
			s.ct = removeSuffix(s.ct, suf)
			return
		}
	}
}

// step5 removes trailing e (with gu/ci handling) or strips the cedilha.
func (s *BrazilianStemmer) step5() {
	if s.rv == "" {
		return
	}
	if sfx(s.rv, "e") {
		if suffixPreceded(s.rv, "e", "gu") {
			s.ct = removeSuffix(s.ct, "e")
			s.ct = removeSuffix(s.ct, "u")
			return
		}
		if suffixPreceded(s.rv, "e", "ci") {
			s.ct = removeSuffix(s.ct, "e")
			s.ct = removeSuffix(s.ct, "i")
			return
		}
		s.ct = removeSuffix(s.ct, "e")
	}
}
