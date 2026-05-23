// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// CompoundRule encodes a COMPOUNDRULE pattern from the affix file.
//
// The rule string may contain flags (possibly grouped with parentheses) and
// wildcard characters '?' (zero-or-one) and '*' (zero-or-more).
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.CompoundRule from Apache Lucene 10.4.0.
type CompoundRule struct {
	data       []rune
	dictionary *Dictionary
}

// NewCompoundRule parses a COMPOUNDRULE pattern and returns a CompoundRule.
func NewCompoundRule(rule string, fps FlagParsingStrategy, d *Dictionary) (*CompoundRule, error) {
	var parsed strings.Builder
	pos := 0
	for pos < len(rule) {
		lParen := strings.Index(rule[pos:], "(")
		if lParen < 0 {
			// No more groups — parse the remaining chars as flags.
			for _, r := range fps.ParseFlags(rule[pos:]) {
				parsed.WriteRune(r)
			}
			break
		}
		lParen += pos
		// Parse the chars before the '(' as flags.
		for _, r := range fps.ParseFlags(rule[pos:lParen]) {
			parsed.WriteRune(r)
		}
		rParen := strings.Index(rule[lParen+1:], ")")
		if rParen < 0 {
			return nil, fmt.Errorf("hunspell: unmatched parentheses in compound rule: %q", rule)
		}
		rParen += lParen + 1
		for _, r := range fps.ParseFlags(rule[lParen+1 : rParen]) {
			parsed.WriteRune(r)
		}
		pos = rParen + 1
		if pos < len(rule) && (rule[pos] == '?' || rule[pos] == '*') {
			parsed.WriteByte(rule[pos])
			pos++
		}
	}
	return &CompoundRule{data: []rune(parsed.String()), dictionary: d}, nil
}

// MayMatch reports whether the given (potentially incomplete) word list may
// match this compound rule.
func (cr *CompoundRule) MayMatch(words []*util.IntsRef) bool {
	return cr.match(words, 0, 0, false)
}

// FullyMatches reports whether the given word list exactly matches this rule.
func (cr *CompoundRule) FullyMatches(words []*util.IntsRef) bool {
	return cr.match(words, 0, 0, true)
}

func (cr *CompoundRule) match(words []*util.IntsRef, patIdx, wordIdx int, fully bool) bool {
	if patIdx >= len(cr.data) {
		return wordIdx >= len(words)
	}
	if wordIdx >= len(words) && !fully {
		return true
	}

	flag := cr.data[patIdx]

	if patIdx < len(cr.data)-1 && cr.data[patIdx+1] == '*' {
		startWI := wordIdx
		for wordIdx < len(words) && cr.dictionary.HasFlagInForms(words[wordIdx], flag) {
			wordIdx++
		}
		for wordIdx >= startWI {
			if cr.match(words, patIdx+2, wordIdx, fully) {
				return true
			}
			wordIdx--
		}
		return false
	}

	currentMatches := wordIdx < len(words) && cr.dictionary.HasFlagInForms(words[wordIdx], flag)

	if patIdx < len(cr.data)-1 && cr.data[patIdx+1] == '?' {
		if currentMatches && cr.match(words, patIdx+2, wordIdx+1, fully) {
			return true
		}
		return cr.match(words, patIdx+2, wordIdx, fully)
	}

	return currentMatches && cr.match(words, patIdx+1, wordIdx+1, fully)
}

func (cr *CompoundRule) String() string { return string(cr.data) }
