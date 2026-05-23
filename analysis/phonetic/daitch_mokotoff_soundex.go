// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package phonetic provides token filters for phonetic matching.
// This is the Go port of org.apache.lucene.analysis.phonetic from
// Apache Lucene 10.4.0.
package phonetic

import (
	"strings"
	"unicode"
)

// DaitchMokotoffSoundex encodes names using the Daitch–Mokotoff Soundex algorithm.
// It returns a '|'-separated string of all possible 6-digit code branches, as
// produced by Apache Commons Codec 1.17.2 DaitchMokotoffSoundex.soundex().
//
// This is a Go port of
// org.apache.commons.codec.language.DaitchMokotoffSoundex from
// Apache Commons Codec 1.17.2.
type DaitchMokotoffSoundex struct{}

// Soundex encodes the name and returns a pipe-separated list of 6-digit codes.
func (e *DaitchMokotoffSoundex) Soundex(name string) string {
	return dmsEncode(name)
}

// dmRule represents one entry in the DM Soundex coding table.
type dmRule struct {
	pattern string
	// alternatives[i] = [code-at-start, code-after-vowel, code-other]
	// Multiple alternate codings produce multiple branches.
	alternatives [][3]string
}

// dmVowelSet holds the vowel characters used in DM Soundex position detection.
const dmVowelSet = "AEIOUY"

// dmsTableByChar maps the first character of a pattern to its rules,
// sorted longest-pattern-first within each bucket.
var dmsTableByChar map[rune][]dmRule

func init() {
	dmsTableByChar = buildDMSTableByChar()
}

// dmRawData encodes the DM Soundex coding table from Apache Commons Codec 1.17.2.
// Each entry: pattern, codesAtStart, codesAfterVowel, codesOther.
// Alternate codings within a single rule are separated by '|' within each field.
// Empty string ("") means no code is appended for that position type.
var dmRawData = []struct{ p, s, a, o string }{
	// Vowel sequences that produce codes
	{"AI", "0", "1", ""},
	{"AJ", "0", "1", ""},
	{"AY", "0", "1", ""},
	{"AU", "0", "7", ""},
	// Single vowels: only produce code at start of name
	{"A", "0", "", ""},
	{"B", "7", "7", "7"},
	{"CHS", "5", "54", "54"},
	{"CH", "5|4", "5|4", "5|4"},
	{"CK", "5|45", "5|45", "5|45"},
	{"CZ", "4", "4", "4"},
	{"CS", "4", "4", "4"},
	{"C", "5|4", "5|4", "5|4"},
	{"DRZ", "4", "4", "4"},
	{"DRS", "4", "4", "4"},
	{"DSH", "4", "4", "4"},
	{"DSZ", "4", "4", "4"},
	{"DZH", "4", "4", "4"},
	{"DZS", "4", "4", "4"},
	{"DZ", "4", "4", "4"},
	{"DS", "4", "4", "4"},
	{"D", "3", "3", "3"},
	{"EI", "0", "1", ""},
	{"EJ", "0", "1", ""},
	{"EY", "0", "1", ""},
	{"EU", "1", "1", ""},
	// Single vowels: only produce code at start of name
	{"E", "0", "", ""},
	{"FB", "7", "7", "7"},
	{"F", "7", "7", "7"},
	{"G", "5", "5", "5"},
	{"H", "5", "5", ""},
	{"IA", "1", "1", ""},
	{"IE", "1", "1", ""},
	{"IO", "1", "1", ""},
	{"IU", "1", "1", ""},
	// Single vowels: only produce code at start of name
	{"I", "0", "", ""},
	{"J", "1|4", "1|4", "1|4"},
	{"KS", "5", "54", "54"},
	{"KH", "5", "5", "5"},
	{"K", "5", "5", "5"},
	{"L", "8", "8", "8"},
	{"MN", "", "66", "66"},
	{"M", "6", "6", "6"},
	{"NM", "", "66", "66"},
	{"N", "6", "6", "6"},
	{"OI", "0", "1", ""},
	{"OJ", "0", "1", ""},
	{"OY", "0", "1", ""},
	// Single vowels: only produce code at start of name
	{"O", "0", "", ""},
	{"PH", "7", "7", "7"},
	{"P", "7", "7", "7"},
	{"Q", "5", "5", "5"},
	{"RZ", "94|4", "94|4", "94|4"},
	{"RS", "94|4", "94|4", "94|4"},
	{"R", "9", "9", "9"},
	{"SCHTSCH", "2", "4", "4"},
	{"SCHTSH", "2", "4", "4"},
	{"SCHTCH", "2", "4", "4"},
	{"SHTCH", "2", "4", "4"},
	{"SHTSH", "2", "4", "4"},
	{"STCH", "2", "4", "4"},
	{"STSCH", "2", "4", "4"},
	{"SCHD", "2", "43", "43"},
	{"SCHT", "2", "43", "43"},
	{"SHCH", "2", "4", "4"},
	{"SCH", "4", "4", "4"},
	{"SHT", "2", "43", "43"},
	{"SHD", "2", "43", "43"},
	{"SH", "4", "4", "4"},
	{"SC", "2", "4", "4"},
	{"ST", "2", "43", "43"},
	{"SD", "2", "43", "43"},
	{"SZ", "4", "4", "4"},
	{"S", "4", "4", "4"},
	{"TTSCH", "4", "4", "4"},
	{"TSCH", "4", "4", "4"},
	{"TTCH", "4", "4", "4"},
	{"TCH", "4", "4", "4"},
	{"TH", "3", "3", "3"},
	{"TRZ", "4", "4", "4"},
	{"TRS", "4", "4", "4"},
	{"TSH", "4", "4", "4"},
	{"TSZ", "4", "4", "4"},
	{"TS", "4", "4", "4"},
	{"TZ", "4", "4", "4"},
	{"TC", "4", "4", "4"},
	{"T", "3", "3", "3"},
	{"UI", "0", "1", ""},
	{"UJ", "0", "1", ""},
	{"UY", "0", "1", ""},
	{"UE", "0", "1", ""},
	// Single vowels: only produce code at start of name
	{"U", "0", "", ""},
	{"V", "7", "7", "7"},
	{"W", "7", "7", "7"},
	{"X", "5", "54", "54"},
	// Y: only at start of name
	{"Y", "1", "", ""},
	{"ZSCH", "4", "4", "4"},
	{"ZSH", "4", "4", "4"},
	{"ZHDZH", "2", "4", "4"},
	{"ZDZH", "2", "4", "4"},
	{"ZDZ", "2", "4", "4"},
	{"ZHD", "2", "43", "43"},
	{"ZD", "2", "43", "43"},
	{"ZH", "4", "4", "4"},
	{"ZS", "4", "44", "44"},
	{"Z", "4", "4", "4"},
}

func buildDMSTableByChar() map[rune][]dmRule {
	type key struct{ p, s, a, o string }
	seen := make(map[key]bool)

	byChar := make(map[rune][]dmRule)
	for _, raw := range dmRawData {
		k := key{raw.p, raw.s, raw.a, raw.o}
		if seen[k] {
			continue
		}
		seen[k] = true

		starts := strings.Split(raw.s, "|")
		afters := strings.Split(raw.a, "|")
		others := strings.Split(raw.o, "|")
		n := maxOf3(len(starts), len(afters), len(others))
		alts := make([][3]string, n)
		for i := 0; i < n; i++ {
			alts[i] = [3]string{
				safeIndex(starts, i),
				safeIndex(afters, i),
				safeIndex(others, i),
			}
		}

		ch := rune(raw.p[0])
		byChar[ch] = append(byChar[ch], dmRule{pattern: raw.p, alternatives: alts})
	}

	// Sort each bucket longest-pattern-first to prefer the longest match.
	for ch := range byChar {
		rules := byChar[ch]
		// insertion sort (small N)
		for i := 1; i < len(rules); i++ {
			for j := i; j > 0 && len([]rune(rules[j].pattern)) > len([]rune(rules[j-1].pattern)); j-- {
				rules[j], rules[j-1] = rules[j-1], rules[j]
			}
		}
		byChar[ch] = rules
	}
	return byChar
}

func maxOf3(a, b, c int) int {
	if a >= b && a >= c {
		return a
	}
	if b >= c {
		return b
	}
	return c
}

func safeIndex(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return s[len(s)-1]
}

// dmBranch is one encoding path through the input.
type dmBranch struct {
	// digits written so far (max 6)
	digits [6]byte
	// number of digits written
	n int
	// last digit string appended (for duplicate suppression)
	last string
}

// appendCode adds each rune of code to the branch, suppressing consecutive
// identical single-digit codes per the DM Soundex rules.
func (b *dmBranch) appendCode(code string) {
	for _, ch := range code {
		if b.n >= 6 {
			break
		}
		d := string(ch)
		// suppress identical consecutive single-digit codes
		if d != b.last {
			b.digits[b.n] = byte(ch)
			b.n++
			b.last = d
		}
	}
}

// result pads the branch to 6 digits and returns the code string.
func (b dmBranch) result() string {
	cp := b
	for cp.n < 6 {
		cp.digits[cp.n] = '0'
		cp.n++
	}
	return string(cp.digits[:6])
}

// dmsEncode applies the DM Soundex algorithm to name, returning a '|'-separated
// string of all 6-digit code branches, in the order they are first produced.
func dmsEncode(name string) string {
	if name == "" {
		return ""
	}

	// Normalise: upper-case, strip non-letters.
	upper := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) {
			return unicode.ToUpper(r)
		}
		return -1
	}, name)
	if upper == "" {
		return ""
	}

	runes := []rune(upper)
	total := len(runes)

	branches := []dmBranch{{}}

	for i := 0; i < total; {
		ch := runes[i]

		rules, exists := dmsTableByChar[ch]
		if !exists {
			i++
			continue
		}

		// Find the longest matching rule.
		var match *dmRule
		for idx := range rules {
			r := &rules[idx]
			pat := []rune(r.pattern)
			if i+len(pat) > total {
				continue
			}
			ok := true
			for k, pr := range pat {
				if runes[i+k] != pr {
					ok = false
					break
				}
			}
			if ok {
				match = r
				break // rules are sorted longest-first
			}
		}

		if match == nil {
			i++
			continue
		}

		// Determine position type: 0=start, 1=after-vowel, 2=other.
		posType := 2
		if i == 0 {
			posType = 0
		} else if strings.ContainsRune(dmVowelSet, runes[i-1]) {
			posType = 1
		}

		// Expand branches for each alternate coding of the matched rule.
		newBranches := make([]dmBranch, 0, len(branches)*len(match.alternatives))
		for _, b := range branches {
			for _, alt := range match.alternatives {
				nb := b
				nb.appendCode(alt[posType])
				newBranches = append(newBranches, nb)
			}
		}
		branches = newBranches

		i += len([]rune(match.pattern))
	}

	// Collect unique results.
	seen := make(map[string]bool, len(branches))
	results := make([]string, 0, len(branches))
	for _, b := range branches {
		s := b.result()
		if !seen[s] {
			seen[s] = true
			results = append(results, s)
		}
	}
	if len(results) == 0 {
		return ""
	}
	return strings.Join(results, "|")
}
