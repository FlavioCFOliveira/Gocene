// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"fmt"
	"strings"
)

// CheckCompoundPattern checks a CHECKCOMPOUNDPATTERN rule that prohibits
// certain component combinations in compound words.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.CheckCompoundPattern from Apache Lucene 10.4.0.
type CheckCompoundPattern struct {
	endChars    string
	beginChars  string
	replacement string
	endFlags    []rune
	beginFlags  []rune
	dictionary  *Dictionary
}

// NewCheckCompoundPattern parses a CHECKCOMPOUNDPATTERN line and returns a
// CheckCompoundPattern.
func NewCheckCompoundPattern(unparsed string, fps FlagParsingStrategy, d *Dictionary) (*CheckCompoundPattern, error) {
	parts := strings.Fields(unparsed)
	if len(parts) < 3 {
		return nil, fmt.Errorf("hunspell: invalid CHECKCOMPOUNDPATTERN: %q", unparsed)
	}

	p := &CheckCompoundPattern{dictionary: d}

	flagSep := strings.IndexByte(parts[1], '/')
	if flagSep < 0 {
		p.endChars = parts[1]
	} else {
		p.endChars = parts[1][:flagSep]
		p.endFlags = fps.ParseFlags(parts[1][flagSep+1:])
	}

	flagSep = strings.IndexByte(parts[2], '/')
	if flagSep < 0 {
		p.beginChars = parts[2]
	} else {
		p.beginChars = parts[2][:flagSep]
		p.beginFlags = fps.ParseFlags(parts[2][flagSep+1:])
	}

	if len(parts) >= 4 {
		p.replacement = parts[3]
	}

	return p, nil
}

// ProhibitsCompounding reports whether this pattern prohibits a compound split
// at breakPos in word.  rootBefore and rootAfter are the roots on each side.
func (p *CheckCompoundPattern) ProhibitsCompounding(word []rune, breakPos int, rootBefore, rootAfter *Root) bool {
	if isNonAffixedPattern(p.endChars) {
		if !runesMatch(word, breakPos-len([]rune(rootBefore.Word)), []rune(rootBefore.Word)) {
			return false
		}
	} else if !runesMatch(word, breakPos-len([]rune(p.endChars)), []rune(p.endChars)) {
		return false
	}

	if isNonAffixedPattern(p.beginChars) {
		if !runesMatch(word, breakPos, []rune(rootAfter.Word)) {
			return false
		}
	} else if !runesMatch(word, breakPos, []rune(p.beginChars)) {
		return false
	}

	if len(p.endFlags) > 0 && !p.hasAllFlags(rootBefore, p.endFlags) {
		return false
	}
	if len(p.beginFlags) > 0 && !p.hasAllFlags(rootAfter, p.beginFlags) {
		return false
	}
	return true
}

// EndLength returns the length of the end-chars component.
func (p *CheckCompoundPattern) EndLength() int { return len([]rune(p.endChars)) }

// ExpandReplacement returns a new word slice if the replacement rule applies,
// otherwise nil.
func (p *CheckCompoundPattern) ExpandReplacement(word []rune, breakPos int) []rune {
	if p.replacement != "" && runesMatch(word, breakPos, []rune(p.replacement)) {
		repRunes := []rune(p.replacement)
		endRunes := []rune(p.endChars)
		beginRunes := []rune(p.beginChars)

		result := make([]rune, 0, len(word))
		result = append(result, word[:breakPos]...)
		result = append(result, endRunes...)
		result = append(result, beginRunes...)
		result = append(result, word[breakPos+len(repRunes):]...)
		return result
	}
	return nil
}

func (p *CheckCompoundPattern) String() string {
	s := p.endChars + " " + p.beginChars
	if p.replacement != "" {
		s += " -> " + p.replacement
	}
	return s
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func isNonAffixedPattern(pattern string) bool {
	return len(pattern) == 1 && pattern[0] == '0'
}

func (p *CheckCompoundPattern) hasAllFlags(root *Root, flags []rune) bool {
	for _, f := range flags {
		if !p.dictionary.HasFlag(root.EntryID, f) {
			return false
		}
	}
	return true
}

func runesMatch(word []rune, offset int, pattern []rune) bool {
	n := len(pattern)
	if offset < 0 || offset+n > len(word) {
		return false
	}
	for i, r := range pattern {
		if word[offset+i] != r {
			return false
		}
	}
	return true
}
