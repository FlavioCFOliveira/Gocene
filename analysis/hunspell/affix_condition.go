// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"regexp"
	"strings"
)

// alwaysTrueKey is the deduplication key for trivially-true affix conditions.
// Mirrors AffixCondition.ALWAYS_TRUE_KEY in Apache Lucene 10.4.0.
const alwaysTrueKey = ".*"

// AffixCondition checks the "condition" part of a PFX/SFX rule.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.AffixCondition from Apache Lucene 10.4.0.
//
// Deviation: Java uses an interface with lambda-style default methods; Go uses
// a struct with a function field for the compiled predicate so the zero-value
// (nil) corresponds to ALWAYS_TRUE.
type AffixCondition struct {
	// accepts is nil only for ALWAYS_TRUE; ALWAYS_FALSE uses a func that
	// always returns false.
	accepts func(word []rune, offset, length int) bool
}

var (
	// alwaysTrueCond is the condition that accepts every stem.
	alwaysTrueCond = &AffixCondition{accepts: nil}
	// alwaysFalseCond rejects every stem.
	alwaysFalseCond = &AffixCondition{accepts: func(_ []rune, _, _ int) bool { return false }}
)

// AcceptsStem reports whether the given stem (word[offset:offset+length]) is
// accepted by this condition.
func (ac *AffixCondition) AcceptsStem(word []rune, offset, length int) bool {
	if ac == nil || ac.accepts == nil {
		return true
	}
	return ac.accepts(word, offset, length)
}

// AcceptsStemString is a convenience wrapper for string stems.
func (ac *AffixCondition) AcceptsStemString(stem string) bool {
	runes := []rune(stem)
	return ac.AcceptsStem(runes, 0, len(runes))
}

// AffixConditionUniqueKey returns the deduplication key for an affix condition
// triple (kind, strip, condition).  For trivially-true conditions it returns
// alwaysTrueKey.
//
// Mirrors AffixCondition.uniqueKey in Apache Lucene 10.4.0.
func AffixConditionUniqueKey(kind AffixKind, strip, condition string) string {
	if condition == "." {
		return alwaysTrueKey
	}
	if kind == AffixKindPrefix && strings.HasPrefix(strip, condition) {
		return alwaysTrueKey
	}
	if kind == AffixKindSuffix && strings.HasSuffix(strip, condition) && !isRegexpCondition(condition) {
		return alwaysTrueKey
	}
	return condition + " " + kindName(kind) + " " + strip
}

// CompileAffixCondition analyses the affix kind, strip, and condition string
// and returns an AffixCondition optimised for that combination.
//
// Mirrors AffixCondition.compile in Apache Lucene 10.4.0.
func CompileAffixCondition(kind AffixKind, strip, condition, _ string) *AffixCondition {
	if !isRegexpCondition(condition) {
		// plain substring match
		if kind == AffixKindSuffix && strings.HasSuffix(condition, strip) {
			return substringCondition(kind, condition[:len(condition)-len(strip)])
		}
		if kind == AffixKindPrefix && strings.HasPrefix(condition, strip) {
			return substringCondition(kind, condition[len(strip):])
		}
		return alwaysFalseCond
	}

	// Tolerate unclosed '[' as Hunspell does.
	last := strings.LastIndex(condition, "[")
	if last >= 0 && strings.Index(condition[last+1:], "]") < 0 {
		condition = condition + "]"
	}

	condRunes := conditionRunes(condition)
	stripRunes := []rune(strip)

	if len(condRunes) <= len(stripRunes) {
		var regexpSrc string
		if kind == AffixKindPrefix {
			regexpSrc = "(?s).*" + condition
		} else {
			regexpSrc = "(?s)" + condition + ".*"
		}
		re, err := regexp.Compile(regexpSrc)
		if err != nil {
			return alwaysFalseCond
		}
		if re.MatchString(strip) {
			return alwaysTrueCond
		}
		return alwaysFalseCond
	}

	if kind == AffixKindPrefix {
		split := skipN(condition, len(stripRunes))
		stripPart := condition[:split]
		re, err := regexp.Compile("(?s)" + stripPart + ".*")
		if err != nil {
			return alwaysFalseCond
		}
		if !re.MatchString(strip) {
			return alwaysFalseCond
		}
		return regexpCondition(kind, condition[split:], len(condRunes)-len(stripRunes))
	}

	split := skipN(condition, len(condRunes)-len(stripRunes))
	stripPart := condition[split:]
	re, err := regexp.Compile("(?s)" + stripPart + ".*")
	if err != nil {
		return alwaysFalseCond
	}
	if !re.MatchString(strip) {
		return alwaysFalseCond
	}
	return regexpCondition(kind, condition[:split], len(condRunes)-len(stripRunes))
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func kindName(kind AffixKind) string {
	if kind == AffixKindPrefix {
		return "PREFIX"
	}
	return "SUFFIX"
}

func isRegexpCondition(condition string) bool {
	return strings.ContainsAny(condition, "[.-")
}

// conditionRunes counts the number of "char patterns" in a condition string.
// A char pattern is either a single character or a [...] bracket expression.
func conditionRunes(condition string) []string {
	var patterns []string
	i := 0
	for i < len(condition) {
		if condition[i] == '[' {
			j := strings.Index(condition[i+1:], "]")
			if j < 0 {
				// unterminated bracket — treat as single char
				patterns = append(patterns, string(condition[i]))
				i++
			} else {
				patterns = append(patterns, condition[i:i+j+2])
				i += j + 2
			}
		} else {
			patterns = append(patterns, string(condition[i]))
			i++
		}
	}
	return patterns
}

// skipN returns the byte index after the first n char-patterns in condition.
func skipN(condition string, n int) int {
	i := 0
	for k := 0; k < n && i < len(condition); k++ {
		if condition[i] == '[' {
			j := strings.Index(condition[i+1:], "]")
			if j < 0 {
				i++
			} else {
				i += j + 2
			}
		} else {
			i++
		}
	}
	return i
}

func substringCondition(kind AffixKind, stemCondition string) *AffixCondition {
	forSuffix := kind == AffixKindSuffix
	condRunes := []rune(stemCondition)
	condLen := len(condRunes)
	return &AffixCondition{
		accepts: func(word []rune, offset, length int) bool {
			if length < condLen {
				return false
			}
			matchStart := offset
			if forSuffix {
				matchStart = offset + length - condLen
			}
			for i, r := range condRunes {
				if r != word[matchStart+i] {
					return false
				}
			}
			return true
		},
	}
}

func regexpCondition(kind AffixKind, condition string, charCount int) *AffixCondition {
	forSuffix := kind == AffixKindSuffix
	src := "(?s)" + escapeDash(condition)
	re, err := regexp.Compile(src)
	if err != nil {
		return alwaysFalseCond
	}
	return &AffixCondition{
		accepts: func(word []rune, offset, length int) bool {
			if length < charCount {
				return false
			}
			var start int
			if forSuffix {
				start = offset + length - charCount
			} else {
				start = offset
			}
			segment := string(word[start : start+charCount])
			return re.MatchString(segment)
		},
	}
}

func escapeDash(re string) string {
	if !strings.Contains(re, "-") {
		return re
	}
	var b strings.Builder
	b.Grow(len(re) + 4)
	for i := 0; i < len(re); i++ {
		c := re[i]
		if c == '-' {
			b.WriteString(`\-`)
		} else {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(re) {
				i++
				b.WriteByte(re[i])
			}
		}
	}
	return b.String()
}
