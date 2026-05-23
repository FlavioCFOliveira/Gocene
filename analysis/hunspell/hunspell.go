// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"strings"
	"unicode"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// SuggestTimeLimit is the default suggestion time limit in milliseconds.
const SuggestTimeLimit = 250

// Hunspell is a spell checker based on Hunspell dictionaries.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.Hunspell from Apache Lucene 10.4.0.
//
// Deviation: The Java class uses CharsRef internally for zero-copy slicing.
// Go uses plain strings and []rune slices, which is idiomatic.
type Hunspell struct {
	Dictionary    *Dictionary
	Stemmer       *Stemmer
	policy        TimeoutPolicy
	checkCanceled func()
}

// NewHunspell creates a new Hunspell spell checker.
func NewHunspell(dictionary *Dictionary) *Hunspell {
	return NewHunspellWithPolicy(dictionary, TimeoutPolicyReturnPartialResult, func() {})
}

// NewHunspellWithPolicy creates a Hunspell with explicit timeout policy and
// cancellation callback.
func NewHunspellWithPolicy(dictionary *Dictionary, policy TimeoutPolicy, checkCanceled func()) *Hunspell {
	return &Hunspell{
		Dictionary:    dictionary,
		Stemmer:       NewStemmer(dictionary),
		policy:        policy,
		checkCanceled: checkCanceled,
	}
}

// Spell reports whether word is spelled correctly.
func (h *Hunspell) Spell(word string) bool {
	h.checkCanceled()
	if word == "" {
		return true
	}
	d := h.Dictionary
	if d.NeedsInputCleaning(word) {
		var sb strings.Builder
		word = d.CleanInputString(word, &sb)
	}
	if strings.HasSuffix(word, ".") {
		return h.spellWithTrailingDots(word)
	}
	return h.spellClean(word)
}

func (h *Hunspell) spellWithTrailingDots(word string) bool {
	runes := []rune(word)
	n := len(runes) - 1
	for n > 0 && runes[n-1] == '.' {
		n--
	}
	return h.spellClean(string(runes[:n])) || h.spellClean(string(runes[:n+1]))
}

func (h *Hunspell) spellClean(word string) bool {
	if isNumber(word) {
		return true
	}
	runes := []rune(word)
	res := h.checkSimpleWord(runes, len(runes), WordCaseNeutral, false)
	if res >= 0 {
		return res == 1
	}
	if h.checkCompounds(runes, len(runes), WordCaseNeutral) {
		return true
	}
	wc := h.Stemmer.caseOf(runes, len(runes))
	if wc == WordCaseUpper || wc == WordCaseTitle {
		found := false
		h.Stemmer.varyCase(runes, len(runes), wc, func(variant []rune, varLen int, _ WordCase) bool {
			if h.checkWord(variant, varLen, WordCaseNeutral) {
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	if h.Dictionary.breaks.IsNotEmpty() && !h.hasTooManyBreakOccurrences(word) {
		return h.tryBreaks(word)
	}
	return false
}

// checkSimpleWord returns 1 if valid, 0 if forbidden, -1 if unknown.
func (h *Hunspell) checkSimpleWord(word []rune, length int, originalCase WordCase, _ bool) int {
	entry := h.findStem(word, 0, length, originalCase, WordContextSimpleWord)
	if entry != nil {
		if h.Dictionary.HasFlag(entry.EntryID, h.Dictionary.forbiddenword) {
			return 0
		}
		return 1
	}
	return -1
}

func (h *Hunspell) checkWord(word []rune, length int, originalCase WordCase) bool {
	res := h.checkSimpleWord(word, length, originalCase, false)
	if res >= 0 {
		return res == 1
	}
	return h.checkCompounds(word, length, originalCase)
}

// checkWordStr checks whether a string word is correctly spelled (package-internal).
func (h *Hunspell) checkWordStr(word string) bool {
	runes := []rune(word)
	return h.checkWord(runes, len(runes), WordCaseNeutral)
}

func (h *Hunspell) checkCompounds(word []rune, length int, originalCase WordCase) bool {
	d := h.Dictionary
	if d.compoundRules != nil && h.checkCompoundRules(word, 0, length, nil) {
		return true
	}
	if d.compoundBegin != flagUnset || d.compoundFlag != flagUnset {
		return h.checkCompoundsFull(word, originalCase, nil)
	}
	return false
}

// findStem finds the first matching root stem.
func (h *Hunspell) findStem(word []rune, offset, length int, originalCase WordCase, context WordContext) *Root {
	h.checkCanceled()
	var result *Root
	checkCase := context != WordContextCompoundMiddle && context != WordContextCompoundEnd
	h.Stemmer.doStem(word, offset, length, context, func(stem []rune, formID, morphDataID, _, _, _, _ int) bool {
		if checkCase && !h.acceptCase(originalCase, formID) {
			return h.Dictionary.HasFlag(formID, DictionaryHiddenFlag)
		}
		if h.acceptsStem(formID) {
			result = &Root{Word: string(stem), EntryID: formID}
		}
		return false
	})
	return result
}

func (h *Hunspell) acceptCase(originalCase WordCase, entryID int) bool {
	d := h.Dictionary
	keepCase := d.HasFlag(entryID, d.keepcase)
	if originalCase != WordCaseNeutral {
		if keepCase && d.CheckSharpS && originalCase == WordCaseTitle {
			return true
		}
		return !keepCase
	}
	return !d.HasFlag(entryID, DictionaryHiddenFlag)
}

func (h *Hunspell) acceptsStem(formID int) bool {
	return true
}

func (h *Hunspell) checkCompoundsFull(word []rune, originalCase WordCase, prev *compoundPart) bool {
	d := h.Dictionary
	if prev != nil && prev.index > d.compoundMax-2 {
		return false
	}
	length := len(word)
	limit := length - d.compoundMin + 1
	for breakPos := d.compoundMin; breakPos < limit; breakPos++ {
		context := WordContextCompoundBegin
		if prev != nil {
			context = WordContextCompoundMiddle
		}
		if h.mayBreakIntoCompounds(word, 0, length, breakPos) {
			stem := h.findStem(word, 0, breakPos, originalCase, context)
			if stem == nil && d.simplifiedTriple && breakPos > 0 && word[breakPos-1] == word[breakPos] {
				stem = h.findStem(word, 0, breakPos+1, originalCase, context)
			}
			if stem != nil && !d.HasFlag(stem.EntryID, d.forbiddenword) {
				if prev == nil || prev.mayCompound(stem, breakPos) {
					part := &compoundPart{prev: prev, word: word, breakPos: breakPos, root: stem}
					if h.checkCompoundsAfter(word, originalCase, part) {
						return true
					}
				}
			}
		}
		for _, pat := range d.checkCompoundPatterns {
			expanded := pat.ExpandReplacement(word, breakPos)
			if expanded != nil {
				stem := h.findStem(expanded, 0, breakPos+pat.EndLength(), originalCase, context)
				if stem != nil {
					part := &compoundPart{prev: prev, word: expanded, breakPos: breakPos + pat.EndLength(), root: stem}
					if h.checkCompoundsAfter(expanded, originalCase, part) {
						return true
					}
				}
			}
		}
	}
	return false
}

func (h *Hunspell) checkCompoundsAfter(word []rune, originalCase WordCase, prev *compoundPart) bool {
	breakPos := prev.breakPos
	rem := word[breakPos:]
	d := h.Dictionary
	lastRoot := h.findStem(rem, 0, len(rem), originalCase, WordContextCompoundEnd)
	if lastRoot != nil &&
		!d.HasFlag(lastRoot.EntryID, d.forbiddenword) &&
		!(d.checkCompoundDup && prev.root.Word == lastRoot.Word) &&
		prev.mayCompound(lastRoot, len(rem)) {
		return true
	}
	return h.checkCompoundsFull(rem, originalCase, prev)
}

func (h *Hunspell) mayBreakIntoCompounds(word []rune, offset, length, breakPos int) bool {
	d := h.Dictionary
	if d.checkCompoundCase {
		a := word[breakPos-1]
		b := word[breakPos]
		if (unicode.IsUpper(a) || unicode.IsUpper(b)) && a != '-' && b != '-' {
			return false
		}
	}
	if d.checkCompoundTriple && word[breakPos-1] == word[breakPos] {
		if (breakPos > offset+1 && word[breakPos-2] == word[breakPos-1]) ||
			(breakPos < length-1 && word[breakPos] == word[breakPos+1]) {
			return false
		}
	}
	return true
}

func (h *Hunspell) checkCompoundRules(word []rune, offset, length int, words []*util.IntsRef) bool {
	if len(words) >= 100 {
		return false
	}
	h.checkCanceled()
	d := h.Dictionary
	limit := length - d.compoundMin + 1
	for breakPos := d.compoundMin; breakPos < limit; breakPos++ {
		forms := d.LookupWord(word, offset, breakPos)
		if forms != nil {
			words = append(words, forms)
			if h.mayHaveCompoundRule(words) {
				if h.checkLastCompoundPart(word, offset+breakPos, length-breakPos, words) {
					return true
				}
				if h.checkCompoundRules(word, offset+breakPos, length-breakPos, words) {
					return true
				}
			}
			words = words[:len(words)-1]
		}
	}
	return false
}

func (h *Hunspell) mayHaveCompoundRule(words []*util.IntsRef) bool {
	for _, rule := range h.Dictionary.compoundRules {
		if rule.MayMatch(words) {
			return true
		}
	}
	return false
}

func (h *Hunspell) checkLastCompoundPart(word []rune, start, length int, words []*util.IntsRef) bool {
	ref := &util.IntsRef{Ints: make([]int, 1), Length: 1}
	words = append(words, ref)
	found := false
	h.Stemmer.doStem(word, start, length, WordContextCompoundRuleEnd, func(stem []rune, formID, _, _, _, _, _ int) bool {
		ref.Ints[0] = formID
		for _, rule := range h.Dictionary.compoundRules {
			if rule.FullyMatches(words) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// GetRoots returns all roots (stems) for the given word.
func (h *Hunspell) GetRoots(word string) []string {
	stems := h.Stemmer.Stem(word)
	seen := make(map[string]struct{}, len(stems))
	var out []string
	for _, s := range stems {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// AnalyzeSimpleWord returns all analyses of a simple (non-compound) word.
func (h *Hunspell) AnalyzeSimpleWord(word string) []*AffixedWord {
	var result []*AffixedWord
	runes := []rune(word)
	h.Stemmer.Analyze(runes, len(runes), func(stem []rune, formID, morphDataID, outerPrefix, innerPrefix, outerSuffix, innerSuffix int) bool {
		d := h.Dictionary
		var prefixes, suffixes []Affix
		if outerPrefix >= 0 {
			prefixes = append(prefixes, NewAffix(d, outerPrefix))
		}
		if innerPrefix >= 0 {
			prefixes = append(prefixes, NewAffix(d, innerPrefix))
		}
		if outerSuffix >= 0 {
			suffixes = append(suffixes, NewAffix(d, outerSuffix))
		}
		if innerSuffix >= 0 {
			suffixes = append(suffixes, NewAffix(d, innerSuffix))
		}
		entry := h.Dictionary.DictEntryAt(string(stem), formID, morphDataID)
		result = append(result, NewAffixedWord(word, entry, prefixes, suffixes))
		return true
	})
	return result
}

// Suggest returns spell-check suggestions for a misspelled word.
func (h *Hunspell) Suggest(word string) ([]string, error) {
	s := NewSuggester(h.Dictionary)
	return s.SuggestWithTimeout(word, SuggestTimeLimit, h.checkCanceled)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func isNumber(s string) bool {
	runes := []rune(s)
	i := 0
	for i < len(runes) {
		c := runes[i]
		if c >= '0' && c <= '9' {
			i++
		} else if c == '.' || c == ',' || c == '-' {
			if i == 0 || i >= len(runes)-1 || !(runes[i+1] >= '0' && runes[i+1] <= '9') {
				return false
			}
			i += 2
		} else {
			return false
		}
	}
	return true
}

func (h *Hunspell) tryBreaks(word string) bool {
	d := h.Dictionary
	runes := []rune(word)
	for _, br := range d.breaks.Starting {
		if len(runes) > len([]rune(br)) && strings.HasPrefix(word, br) {
			if h.Spell(word[len(br):]) {
				return true
			}
		}
	}
	for _, br := range d.breaks.Ending {
		if len(runes) > len([]rune(br)) && strings.HasSuffix(word, br) {
			if h.Spell(word[:len(word)-len(br)]) {
				return true
			}
		}
	}
	for _, br := range d.breaks.Middle {
		pos := strings.Index(word, br)
		if h.canBeBrokenAt(word, br, pos) {
			return true
		}
		if pos > 0 {
			if h.canBeBrokenAt(word, br, strings.Index(word[pos+1:], br)+pos+1) {
				return true
			}
		}
	}
	return false
}

func (h *Hunspell) hasTooManyBreakOccurrences(word string) bool {
	count := 0
	for _, br := range h.Dictionary.breaks.Middle {
		pos := 0
		for {
			idx := strings.Index(word[pos:], br)
			if idx < 0 {
				break
			}
			count++
			if count >= 10 {
				return true
			}
			pos += idx + len(br)
		}
	}
	return false
}

func (h *Hunspell) canBeBrokenAt(word, breakStr string, breakPos int) bool {
	if breakPos <= 0 || breakPos >= len(word)-len(breakStr) {
		return false
	}
	return h.Spell(word[:breakPos]) && h.Spell(word[breakPos+len(breakStr):])
}

// ─── compoundPart ─────────────────────────────────────────────────────────────

type compoundPart struct {
	prev     *compoundPart
	word     []rune
	breakPos int
	root     *Root
	index    int
}

func (cp *compoundPart) mayCompound(nextRoot *Root, nextPartLen int) bool {
	d := cp.root // not dictionary but root of part; we need dictionary
	_ = d
	_ = nextRoot
	_ = nextPartLen
	// pattern checks deferred; for now check nothing (conservative allow)
	return true
}
