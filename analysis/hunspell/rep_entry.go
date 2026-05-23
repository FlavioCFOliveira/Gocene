// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import "strings"

// RepEntry represents one entry in the REP (replacement) table of a Hunspell
// affix file.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.RepEntry from Apache Lucene 10.4.0.
type RepEntry struct {
	pattern     string
	replacement string
	mustStart   bool
	mustEnd     bool
	patternLen  int
}

// NewRepEntry constructs a RepEntry from the raw (possibly anchored) pattern
// and replacement strings as they appear in the *.aff file.
func NewRepEntry(rawPattern, rawReplacement string) *RepEntry {
	mustStart := strings.HasPrefix(rawPattern, "^")
	mustEnd := strings.HasSuffix(rawPattern, "$")

	start := 0
	if mustStart {
		start = 1
	}
	end := len(rawPattern)
	if mustEnd {
		end--
	}
	pattern := rawPattern[start:end]
	replacement := strings.ReplaceAll(rawReplacement, "_", " ")

	return &RepEntry{
		pattern:     pattern,
		replacement: replacement,
		mustStart:   mustStart,
		mustEnd:     mustEnd,
		patternLen:  len(pattern),
	}
}

// IsMiddle reports whether this entry has no start/end anchors.
func (r *RepEntry) IsMiddle() bool {
	return !r.mustStart && !r.mustEnd
}

// Substitute applies the replacement to word and returns all resulting strings.
// Returns an empty slice when the pattern does not match.
func (r *RepEntry) Substitute(word string) []string {
	if r.mustStart {
		var matches bool
		if r.mustEnd {
			matches = word == r.pattern
		} else {
			matches = strings.HasPrefix(word, r.pattern)
		}
		if !matches {
			return nil
		}
		return []string{r.replacement + word[r.patternLen:]}
	}

	if r.mustEnd {
		if !strings.HasSuffix(word, r.pattern) {
			return nil
		}
		return []string{word[:len(word)-r.patternLen] + r.replacement}
	}

	pos := strings.Index(word, r.pattern)
	if pos < 0 {
		return nil
	}

	var result []string
	for pos >= 0 {
		result = append(result, word[:pos]+r.replacement+word[pos+r.patternLen:])
		pos = strings.Index(word[pos+1:], r.pattern)
		if pos >= 0 {
			pos += len(word) - (len(word[pos+1:]) + r.patternLen - (len(word[pos+1:]) - len(word[pos+1:])))
			// re-compute from original word
			break
		}
	}

	// redo simply to avoid the offset confusion above
	result = result[:0]
	for i := 0; i <= len(word)-r.patternLen; {
		j := strings.Index(word[i:], r.pattern)
		if j < 0 {
			break
		}
		abs := i + j
		result = append(result, word[:abs]+r.replacement+word[abs+r.patternLen:])
		i = abs + 1
	}
	return result
}

func (r *RepEntry) String() string {
	start := ""
	if r.mustStart {
		start = "^"
	}
	end := ""
	if r.mustEnd {
		end = "$"
	}
	return start + r.pattern + end + "->" + r.replacement
}
