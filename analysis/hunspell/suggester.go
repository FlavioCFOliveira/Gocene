// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"strings"
	"time"
	"unicode"
)

// Suggester generates spelling correction suggestions for misspelled words.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.Suggester from Apache Lucene 10.4.0.
type Suggester struct {
	dictionary       *Dictionary
	suggestibleCache *SuggestibleEntryCache
	fragmentChecker  FragmentChecker
	proceedPastRep   bool
}

// NewSuggester constructs a new Suggester.
func NewSuggester(dictionary *Dictionary) *Suggester {
	return &Suggester{
		dictionary:      dictionary,
		fragmentChecker: EverythingPossible,
	}
}

// WithSuggestibleEntryCache returns a copy with a pre-built entry cache.
func (s *Suggester) WithSuggestibleEntryCache() *Suggester {
	cache := BuildSuggestibleEntryCache(s.dictionary.words)
	return &Suggester{
		dictionary:       s.dictionary,
		suggestibleCache: cache,
		fragmentChecker:  s.fragmentChecker,
		proceedPastRep:   s.proceedPastRep,
	}
}

// WithFragmentChecker returns a copy with the given FragmentChecker.
func (s *Suggester) WithFragmentChecker(fc FragmentChecker) *Suggester {
	return &Suggester{
		dictionary:       s.dictionary,
		suggestibleCache: s.suggestibleCache,
		fragmentChecker:  fc,
		proceedPastRep:   s.proceedPastRep,
	}
}

// ProceedPastRep returns a copy that does not stop after REP-rule suggestions.
func (s *Suggester) ProceedPastRep() *Suggester {
	return &Suggester{
		dictionary:       s.dictionary,
		suggestibleCache: s.suggestibleCache,
		fragmentChecker:  s.fragmentChecker,
		proceedPastRep:   true,
	}
}

// SuggestNoTimeout computes suggestions without a time limit.
func (s *Suggester) SuggestNoTimeout(word string, checkCanceled func()) ([]string, error) {
	return s.suggest(word, checkCanceled)
}

// SuggestWithTimeout computes suggestions with a time limit in milliseconds.
func (s *Suggester) SuggestWithTimeout(word string, timeLimitMs int64, checkCanceled func()) ([]string, error) {
	deadline := time.Now().Add(time.Duration(timeLimitMs) * time.Millisecond)
	count := 0
	wrapped := func() {
		checkCanceled()
		count++
		if count%100 == 0 && time.Now().After(deadline) {
			panic(&SuggestionTimeoutError{message: "suggestion time limit exceeded"})
		}
	}

	var result []string
	var retErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				if te, ok := r.(*SuggestionTimeoutError); ok {
					retErr = te
				} else {
					panic(r)
				}
			}
		}()
		result, _ = s.suggest(word, wrapped)
	}()
	return result, retErr
}

func (s *Suggester) suggest(word string, checkCanceled func()) ([]string, error) {
	checkCanceled()
	if len([]rune(word)) >= 100 {
		return nil, nil
	}
	d := s.dictionary
	if d.NeedsInputCleaning(word) {
		var sb strings.Builder
		word = d.CleanInputString(word, &sb)
	}

	// Speller for suggestion checking.
	speller := NewHunspellWithPolicy(d, TimeoutPolicyNoTimeout, checkCanceled)

	wordCase := CaseOfString(word)

	if d.forceUCase != flagUnset && wordCase == WordCaseLower {
		runes := []rune(word)
		title := string(append([]rune{unicode.ToTitle(runes[0])}, runes[1:]...))
		if speller.Spell(title) {
			return []string{title}, nil
		}
	}

	var suggestions []rawSuggestion

	modSug := &modifyingSuggester{
		speller:         speller,
		word:            word,
		wordCase:        wordCase,
		fragmentChecker: s.fragmentChecker,
		proceedPastRep:  s.proceedPastRep,
	}
	hasGood := modSug.suggest(&suggestions)

	if !hasGood && d.maxNGramSuggestions > 0 {
		genSug := &generatingSuggester{
			dictionary: d,
			speller:    speller,
			entryCache: s.suggestibleCache,
		}
		lower := d.toLowerCase(word)
		generated := genSug.suggest(lower, wordCase, suggestions)
		for _, raw := range generated {
			suggestions = append(suggestions, rawSuggestion{raw: raw})
		}
	}

	if strings.Contains(word, "-") && !anySuggestionContainsDash(suggestions) {
		for _, raw := range s.modifyChunksBetweenDashes(word, speller, checkCanceled) {
			suggestions = append(suggestions, rawSuggestion{raw: raw})
		}
	}

	return postprocessSuggestions(d, speller, suggestions, word, wordCase), nil
}

func anySuggestionContainsDash(suggestions []rawSuggestion) bool {
	for _, s := range suggestions {
		if strings.Contains(s.raw, "-") {
			return true
		}
	}
	return false
}

func postprocessSuggestions(d *Dictionary, speller *Hunspell, suggestions []rawSuggestion, misspelled string, wordCase WordCase) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, s := range suggestions {
		for _, r := range adjustAndCleanSuggestion(d, speller, s.raw, misspelled, wordCase) {
			if _, ok := seen[r]; !ok {
				seen[r] = struct{}{}
				result = append(result, r)
			}
		}
	}
	return result
}

func adjustAndCleanSuggestion(d *Dictionary, _ *Hunspell, raw, misspelled string, originalCase WordCase) []string {
	var adjusted string
	switch originalCase {
	case WordCaseUpper:
		adjusted = strings.ToUpper(raw)
	default:
		misspelledRunes := []rune(misspelled)
		if len(misspelledRunes) > 0 && unicode.IsUpper(misspelledRunes[0]) {
			rawRunes := []rune(raw)
			if len(rawRunes) > 0 {
				adjusted = string(append([]rune{unicode.ToTitle(rawRunes[0])}, rawRunes[1:]...))
			} else {
				adjusted = raw
			}
		} else {
			adjusted = raw
		}
	}
	cleaned := cleanSuggestOutput(d, adjusted)
	seen := make(map[string]struct{})
	var out []string
	if _, ok := seen[cleaned]; !ok {
		seen[cleaned] = struct{}{}
		out = append(out, cleaned)
	}
	if cleaned != adjusted {
		if _, ok := seen[adjusted]; !ok {
			seen[adjusted] = struct{}{}
			out = append(out, adjusted)
		}
	}
	if originalCase == WordCaseUpper && d.CheckSharpS && strings.Contains(raw, "ß") {
		cr := cleanSuggestOutput(d, raw)
		if _, ok := seen[cr]; !ok {
			seen[cr] = struct{}{}
			out = append(out, cr)
		}
	}
	return out
}

func cleanSuggestOutput(d *Dictionary, s string) string {
	if d.oconv == nil {
		return s
	}
	var sb strings.Builder
	sb.WriteString(s)
	d.oconv.ApplyMappings(&sb)
	return sb.String()
}

func (s *Suggester) modifyChunksBetweenDashes(word string, speller *Hunspell, checkCanceled func()) []string {
	var result []string
	chunkStart := 0
	for chunkStart < len(word) {
		chunkEnd := strings.Index(word[chunkStart:], "-")
		if chunkEnd < 0 {
			chunkEnd = len(word)
		} else {
			chunkEnd += chunkStart
		}
		if chunkEnd > chunkStart {
			chunk := word[chunkStart:chunkEnd]
			if !speller.Spell(chunk) {
				subs, _ := s.SuggestNoTimeout(chunk, checkCanceled)
				for _, chunkSug := range subs {
					replaced := word[:chunkStart] + chunkSug + word[chunkEnd:]
					if speller.Spell(replaced) {
						result = append(result, replaced)
					}
				}
			}
		}
		chunkStart = chunkEnd + 1
	}
	return result
}

type rawSuggestion struct {
	raw string
}
