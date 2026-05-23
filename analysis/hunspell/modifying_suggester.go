// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"strings"
	"unicode"
)

// maxCharDistance is the maximum character distance used in long-swap and move operations.
const maxCharDistance = 4

// modifyingSuggester modifies the misspelled word in various ways to obtain correct suggestions.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.ModifyingSuggester from Apache Lucene 10.4.0.
//
// Deviation: Java stores suggestions in a LinkedHashSet<Suggestion>; Go uses *[]rawSuggestion
// with a separate dedup map, both passed by pointer to avoid copies.
type modifyingSuggester struct {
	speller         *Hunspell
	word            string
	wordCase        WordCase
	fragmentChecker FragmentChecker
	proceedPastRep  bool
	tryChars        []rune
	tried           map[string]struct{}
}

// suggest populates suggestions and returns true if any "good" suggestions were found.
func (m *modifyingSuggester) suggest(suggestions *[]rawSuggestion) bool {
	m.tried = make(map[string]struct{})
	d := m.speller.Dictionary
	word := m.word

	low := word
	if m.wordCase != WordCaseLower {
		low = d.toLowerCase(word)
	}
	m.tryChars = []rune(d.tryChars)

	if m.wordCase == WordCaseUpper || m.wordCase == WordCaseMixed {
		m.trySuggestion(low, suggestions)
	}

	hasGood := m.tryVariationsOf(word, suggestions)

	switch m.wordCase {
	case WordCaseTitle:
		hasGood = m.tryVariationsOf(low, suggestions) || hasGood
	case WordCaseUpper:
		hasGood = m.tryVariationsOf(low, suggestions) || hasGood
		hasGood = m.tryVariationsOf(d.toTitleCase(word), suggestions) || hasGood
	case WordCaseMixed:
		dot := strings.IndexByte(word, '.')
		if dot > 0 && dot < len(word)-1 {
			afterDot := word[dot+1:]
			if CaseOfString(afterDot) == WordCaseTitle {
				*suggestions = append(*suggestions, rawSuggestion{raw: word[:dot+1] + " " + afterDot})
			}
		}

		runes := []rune(word)
		first := runes[0]
		capitalized := unicode.IsUpper(first)
		if capitalized {
			folded := string(append([]rune{d.caseFold(first)}, runes[1:]...))
			hasGood = m.tryVariationsOf(folded, suggestions) || hasGood
		}
		hasGood = m.tryVariationsOf(low, suggestions) || hasGood
		if capitalized {
			hasGood = m.tryVariationsOf(d.toTitleCase(low), suggestions) || hasGood
		}
		// reorder: put capitalized-after-space variants first
		reordered := make([]rawSuggestion, 0, len(*suggestions))
		front := make([]rawSuggestion, 0)
		for _, s := range *suggestions {
			if changed, ok := m.capitalizeAfterSpace(s.raw); ok {
				front = append(front, rawSuggestion{raw: changed})
			} else {
				reordered = append(reordered, s)
			}
		}
		*suggestions = append(front, reordered...)
	}
	return hasGood
}

// capitalizeAfterSpace handles "aNew" -> "a New" capitalisation.
// Returns the modified string and true if a capitalisation was applied.
func (m *modifyingSuggester) capitalizeAfterSpace(candidate string) (string, bool) {
	space := strings.IndexByte(candidate, ' ')
	if space <= 0 {
		return "", false
	}
	tail := len(candidate) - space - 1
	word := m.word
	if tail > 0 && !strings.HasSuffix(word, candidate[space+1:]) {
		cr := []rune(candidate)
		if space+1 < len(cr) {
			cr[space+1] = unicode.ToUpper(cr[space+1])
			return string(cr), true
		}
	}
	return "", false
}

type gradedSuggestions int

const (
	gradedNone   gradedSuggestions = iota
	gradedNormal gradedSuggestions = iota
	gradedBest   gradedSuggestions = iota
)

func (m *modifyingSuggester) tryVariationsOf(word string, suggestions *[]rawSuggestion) bool {
	hasGood := m.trySuggestion(strings.ToUpper(word), suggestions)

	repResult := m.tryRep(word, suggestions)
	if repResult == gradedBest && !m.proceedPastRep {
		return true
	}
	hasGood = hasGood || repResult != gradedNone

	d := m.speller.Dictionary
	if len(d.mapTable) > 0 {
		m.enumerateMapReplacements(word, "", 0, suggestions)
	}

	m.trySwappingChars(word, suggestions)
	m.tryLongSwap(word, suggestions)
	m.tryNeighborKeys(word, suggestions)
	m.tryRemovingChar(word, suggestions)
	m.tryAddingChar(word, suggestions)
	m.tryMovingChar(word, suggestions)
	m.tryReplacingChar(word, suggestions)
	m.tryTwoDuplicateChars(word, suggestions)

	goodSplit := m.checkDictionaryForSplitSuggestions(word)
	if len(goodSplit) > 0 {
		// put split suggestions first, then prior suggestions if hasGood
		var merged []rawSuggestion
		for _, s := range goodSplit {
			merged = append(merged, rawSuggestion{raw: s})
		}
		if hasGood {
			merged = append(merged, *suggestions...)
		}
		*suggestions = merged
		hasGood = true
	}

	if !hasGood && d.enableSplitSuggestions {
		m.trySplitting(word, suggestions)
	}
	return hasGood
}

func (m *modifyingSuggester) tryRep(word string, suggestions *[]rawSuggestion) gradedSuggestions {
	hasBest := false
	before := len(*suggestions)
	for _, entry := range m.speller.Dictionary.repTable {
		for _, candidate := range entry.Substitute(word) {
			candidate = strings.TrimSpace(candidate)
			if m.trySuggestion(candidate, suggestions) {
				hasBest = true
				continue
			}
			// multi-word: check each part
			if strings.Contains(candidate, " ") {
				parts := strings.Split(candidate, " ")
				allOK := true
				for _, p := range parts {
					if !m.checkSimpleWordStr(p) {
						allOK = false
						break
					}
				}
				if allOK {
					*suggestions = append(*suggestions, rawSuggestion{raw: candidate})
				}
			}
		}
	}
	if hasBest {
		return gradedBest
	}
	if len(*suggestions) > before {
		return gradedNormal
	}
	return gradedNone
}

func (m *modifyingSuggester) enumerateMapReplacements(word, accumulated string, offset int, suggestions *[]rawSuggestion) {
	if offset == len([]rune(word)) {
		m.trySuggestion(accumulated, suggestions)
		return
	}
	accLen := len([]rune(accumulated))
	wordRunes := []rune(word)
	for _, entries := range m.speller.Dictionary.mapTable {
		for _, entry := range entries {
			entryRunes := []rune(entry)
			if offset+len(entryRunes) > len(wordRunes) {
				continue
			}
			match := true
			for k, r := range entryRunes {
				if wordRunes[offset+k] != r {
					match = false
					break
				}
			}
			if match {
				for _, replacement := range entries {
					if entry == replacement {
						continue
					}
					next := accumulated + replacement
					end := accLen + len([]rune(replacement))
					if !m.fragmentChecker.HasImpossibleFragmentAround([]rune(next), accLen, end) {
						m.enumerateMapReplacements(word, next, offset+len(entryRunes), suggestions)
					}
				}
			}
		}
	}
	// advance by one rune
	next := accumulated + string(wordRunes[offset])
	if !m.fragmentChecker.HasImpossibleFragmentAround([]rune(next), accLen, accLen+1) {
		m.enumerateMapReplacements(word, next, offset+1, suggestions)
	}
}

func (m *modifyingSuggester) checkSimpleWordStr(part string) bool {
	runes := []rune(part)
	return m.speller.checkSimpleWord(runes, len(runes), WordCaseNeutral, false) == 1
}

func (m *modifyingSuggester) trySwappingChars(word string, suggestions *[]rawSuggestion) {
	runes := []rune(word)
	length := len(runes)
	for i := 0; i < length-1; i++ {
		candidate := make([]rune, length)
		copy(candidate, runes)
		candidate[i], candidate[i+1] = candidate[i+1], candidate[i]
		m.trySuggestion(string(candidate), suggestions)
	}
	if length == 4 || length == 5 {
		m.tryDoubleSwapForShortWords(runes, length, suggestions)
	}
}

func (m *modifyingSuggester) tryDoubleSwapForShortWords(runes []rune, length int, suggestions *[]rawSuggestion) {
	candidate := make([]rune, length)
	copy(candidate, runes)
	candidate[0], candidate[1] = runes[1], runes[0]
	candidate[length-1], candidate[length-2] = runes[length-2], runes[length-1]
	m.trySuggestion(string(candidate), suggestions)

	if length == 5 {
		candidate[0] = runes[0]
		candidate[1] = runes[2]
		candidate[2] = runes[1]
		m.trySuggestion(string(candidate), suggestions)
	}
}

func (m *modifyingSuggester) tryNeighborKeys(word string, suggestions *[]rawSuggestion) {
	runes := []rune(word)
	d := m.speller.Dictionary
	for i, c := range runes {
		up := unicode.ToUpper(c)
		if up != c {
			candidate := make([]rune, len(runes))
			copy(candidate, runes)
			candidate[i] = up
			m.trySuggestion(string(candidate), suggestions)
		}
		for _, group := range d.neighborKeyGroups {
			groupRunes := []rune(group)
			found := false
			for _, g := range groupRunes {
				if g == c {
					found = true
					break
				}
			}
			if found {
				for _, g := range groupRunes {
					if g != c {
						candidate := make([]rune, len(runes))
						copy(candidate, runes)
						candidate[i] = g
						m.tryModifiedSuggestion(i, string(candidate), suggestions)
					}
				}
			}
		}
	}
}

func (m *modifyingSuggester) tryModifiedSuggestion(modOffset int, candidate string, suggestions *[]rawSuggestion) {
	cr := []rune(candidate)
	if !m.fragmentChecker.HasImpossibleFragmentAround(cr, modOffset, modOffset+1) {
		m.trySuggestion(candidate, suggestions)
	}
}

func (m *modifyingSuggester) tryLongSwap(word string, suggestions *[]rawSuggestion) {
	runes := []rune(word)
	length := len(runes)
	for i := 0; i < length; i++ {
		for j := i + 2; j < length && j <= i+maxCharDistance; j++ {
			candidate := make([]rune, length)
			copy(candidate, runes)
			candidate[i], candidate[j] = runes[j], runes[i]
			m.trySuggestion(string(candidate), suggestions)
		}
	}
}

func (m *modifyingSuggester) tryRemovingChar(word string, suggestions *[]rawSuggestion) {
	runes := []rune(word)
	if len(runes) == 1 {
		return
	}
	for i := range runes {
		candidate := make([]rune, 0, len(runes)-1)
		candidate = append(candidate, runes[:i]...)
		candidate = append(candidate, runes[i+1:]...)
		m.trySuggestion(string(candidate), suggestions)
	}
}

func (m *modifyingSuggester) tryAddingChar(word string, suggestions *[]rawSuggestion) {
	runes := []rune(word)
	for i := 0; i <= len(runes); i++ {
		for _, toInsert := range m.tryChars {
			candidate := make([]rune, 0, len(runes)+1)
			candidate = append(candidate, runes[:i]...)
			candidate = append(candidate, toInsert)
			candidate = append(candidate, runes[i:]...)
			m.tryModifiedSuggestion(i, string(candidate), suggestions)
		}
	}
}

func (m *modifyingSuggester) tryMovingChar(word string, suggestions *[]rawSuggestion) {
	runes := []rune(word)
	length := len(runes)
	for i := 0; i < length; i++ {
		for j := i + 2; j < length && j <= i+maxCharDistance; j++ {
			// move runes[i] to position j
			candidate := make([]rune, length)
			copy(candidate, runes)
			copy(candidate[i:], runes[i+1:j+1])
			candidate[j] = runes[i]
			m.trySuggestion(string(candidate), suggestions)

			// move runes[j] to position i
			candidate2 := make([]rune, length)
			copy(candidate2, runes)
			copy(candidate2[i+1:], runes[i:j])
			candidate2[i] = runes[j]
			m.trySuggestion(string(candidate2), suggestions)
		}
		if i < length-1 {
			// move runes[i] to end of word
			candidate := make([]rune, length)
			copy(candidate[:i], runes[:i])
			copy(candidate[i:], runes[i+1:])
			candidate[length-1] = runes[i]
			m.trySuggestion(string(candidate), suggestions)
		}
	}
}

func (m *modifyingSuggester) tryReplacingChar(word string, suggestions *[]rawSuggestion) {
	runes := []rune(word)
	for i, c := range runes {
		for _, toInsert := range m.tryChars {
			if toInsert != c {
				candidate := make([]rune, len(runes))
				copy(candidate, runes)
				candidate[i] = toInsert
				m.tryModifiedSuggestion(i, string(candidate), suggestions)
			}
		}
	}
}

func (m *modifyingSuggester) tryTwoDuplicateChars(word string, suggestions *[]rawSuggestion) {
	runes := []rune(word)
	dupLen := 0
	for i := 2; i < len(runes); i++ {
		if runes[i] == runes[i-2] {
			dupLen++
			if dupLen == 3 || (dupLen == 2 && i >= 4) {
				candidate := make([]rune, 0, len(runes)-1)
				candidate = append(candidate, runes[:i-1]...)
				candidate = append(candidate, runes[i+1:]...)
				m.trySuggestion(string(candidate), suggestions)
				dupLen = 0
			}
		} else {
			dupLen = 0
		}
	}
}

func (m *modifyingSuggester) checkDictionaryForSplitSuggestions(word string) []string {
	runes := []rune(word)
	var result []string
	for i := 1; i < len(runes)-1; i++ {
		w1 := string(runes[:i])
		w2 := string(runes[i:])
		spaced := w1 + " " + w2
		if m.speller.checkWordStr(spaced) {
			result = append(result, spaced)
		}
		if m.shouldSplitByDash() {
			dashed := w1 + "-" + w2
			if m.speller.checkWordStr(dashed) {
				result = append(result, dashed)
			}
		}
	}
	return result
}

func (m *modifyingSuggester) trySplitting(word string, suggestions *[]rawSuggestion) {
	runes := []rune(word)
	for i := 1; i < len(runes); i++ {
		w1 := string(runes[:i])
		w2 := string(runes[i:])
		if m.checkSimpleWordStr(w1) && m.checkSimpleWordStr(w2) {
			*suggestions = append(*suggestions, rawSuggestion{raw: w1 + " " + w2})
			if len([]rune(w1)) > 1 && len([]rune(w2)) > 1 && m.shouldSplitByDash() {
				*suggestions = append(*suggestions, rawSuggestion{raw: w1 + "-" + w2})
			}
		}
	}
}

func (m *modifyingSuggester) shouldSplitByDash() bool {
	tc := m.speller.Dictionary.tryChars
	return strings.ContainsRune(tc, '-') || strings.ContainsRune(tc, 'a')
}

// trySuggestion adds candidate to suggestions if it passes spell-check and hasn't been tried.
// Returns true if it was a successful (spell-correct) suggestion.
func (m *modifyingSuggester) trySuggestion(candidate string, suggestions *[]rawSuggestion) bool {
	if _, already := m.tried[candidate]; already {
		return false
	}
	m.tried[candidate] = struct{}{}
	if m.speller.checkWordStr(candidate) {
		*suggestions = append(*suggestions, rawSuggestion{raw: candidate})
		return true
	}
	return false
}
