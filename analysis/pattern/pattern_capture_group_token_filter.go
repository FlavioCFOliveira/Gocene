// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package pattern

import (
	"regexp"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PatternCaptureGroupTokenFilter uses regex capture groups to emit multiple
// tokens from a single input token, one for each capture group match.
//
// For example, a pattern like "(https?://([\w.-]+))" matched against
// "http://www.foo.com/index" would return "http://www.foo.com" and
// "www.foo.com".
//
// If preserveOriginal is true, the original token is always emitted first. If
// no pattern matches and preserveOriginal is false, the original token is
// still emitted unchanged.
//
// This is the Go port of
// org.apache.lucene.analysis.pattern.PatternCaptureGroupTokenFilter from
// Apache Lucene 10.4.0.
//
// Deviation: Java uses CharsRefBuilder for zero-copy; Go uses a plain string
// snapshot of the current term. Java's charTermAttr.copyBuffer is replaced by
// setting the term directly on the attribute.
type PatternCaptureGroupTokenFilter struct {
	*analysis.BaseTokenFilter

	termAttr         analysis.CharTermAttribute
	posIncrAttr      analysis.PositionIncrementAttribute
	matchers         []*regexp.Regexp
	groupCounts      []int
	currentGroups    []int // -1 = not started, 0 = exhausted, ≥1 = next group index
	currentMatcher   int   // index into matchers; -1 = none
	savedState       *util.AttributeState
	spare            string // snapshot of the current input term
	preserveOriginal bool
}

// NewPatternCaptureGroupTokenFilter creates a filter that emits capture-group
// matches as additional tokens.
//
//   - input: upstream TokenStream
//   - preserveOriginal: if true, the original token is emitted first
//   - patterns: one or more compiled regular expressions
func NewPatternCaptureGroupTokenFilter(
	input analysis.TokenStream,
	preserveOriginal bool,
	patterns ...*regexp.Regexp,
) *PatternCaptureGroupTokenFilter {
	f := &PatternCaptureGroupTokenFilter{
		BaseTokenFilter:  analysis.NewBaseTokenFilter(input),
		matchers:         make([]*regexp.Regexp, len(patterns)),
		groupCounts:      make([]int, len(patterns)),
		currentGroups:    make([]int, len(patterns)),
		currentMatcher:   -1,
		preserveOriginal: preserveOriginal,
	}
	for i, p := range patterns {
		f.matchers[i] = p
		// NumSubexp returns the number of parenthesised subexpressions.
		f.groupCounts[i] = p.NumSubexp()
		f.currentGroups[i] = -1
	}
	src := f.GetAttributeSource()
	if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
		f.termAttr = a.(analysis.CharTermAttribute)
	} else {
		f.termAttr = analysis.NewCharTermAttribute()
		src.AddAttributeImpl(f.termAttr)
	}
	if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
		f.posIncrAttr = a.(analysis.PositionIncrementAttribute)
	} else {
		f.posIncrAttr = analysis.NewPositionIncrementAttribute()
		src.AddAttributeImpl(f.posIncrAttr)
	}
	return f
}

// nextCapture finds the next capture group to emit. It sets currentMatcher to
// the index of the matcher that has the leftmost next match, or -1 when none
// remain. Returns true when a capture was found.
func (f *PatternCaptureGroupTokenFilter) nextCapture() bool {
	minOffset := -1
	f.currentMatcher = -1

	for i, re := range f.matchers {
		if f.currentGroups[i] == -1 {
			// Lazily find first match for this pattern.
			locs := re.FindAllStringSubmatchIndex(f.spare, -1)
			if len(locs) == 0 {
				f.currentGroups[i] = 0 // exhausted
			} else {
				// Store all submatch locations; we iterate via groupCounts.
				// We encode the first pending group as the start index into a
				// flat submatch slice. For simplicity we store the match index
				// (1-based group number) and use re.FindStringSubmatchIndex for
				// the current iteration position. A single pass model is
				// sufficient here.
				f.currentGroups[i] = 1
			}
		}
		if f.currentGroups[i] == 0 {
			continue
		}
		// Advance through groups for this matcher.
		for f.currentGroups[i] <= f.groupCounts[i] {
			loc := f.matchLocForGroup(i, f.currentGroups[i])
			if loc == nil || loc[0] == loc[1] {
				f.currentGroups[i]++
				continue
			}
			// Skip if this group covers the entire spare and preserveOriginal
			// is active (the Java code skips start==0 && end==len(spare)).
			if f.preserveOriginal && loc[0] == 0 && loc[1] == len(f.spare) {
				f.currentGroups[i]++
				continue
			}
			start := loc[0]
			if minOffset == -1 || start < minOffset {
				minOffset = start
				f.currentMatcher = i
			}
			break
		}
		if f.currentGroups[i] > f.groupCounts[i] {
			f.currentGroups[i] = -1 // allow re-start on next input token
		}
	}
	return f.currentMatcher != -1
}

// matchLocForGroup returns the byte [start, end) for group g in the spare
// string using the stored matcher for matcher index idx.
func (f *PatternCaptureGroupTokenFilter) matchLocForGroup(idx, g int) []int {
	// We need to replay the match at the current iteration point.
	// Since we do not store all sub-match results upfront, we use FindAllStringSubmatchIndex.
	// For each call we re-run the regex; this is fine for correctness (and the
	// Java implementation resets/replays matchers each call anyway).
	all := f.matchers[idx].FindAllStringSubmatchIndex(f.spare, -1)
	// currentGroups[idx] tracks which match+group within the full match list.
	// We encode the position as: (matchIdx * (groupCounts+1)) + groupIdx.
	groups := f.groupCounts[idx] + 1
	flat := f.currentGroups[idx] - 1 // 0-based
	matchIdx := flat / groups
	groupIdx := flat % groups
	if matchIdx >= len(all) {
		return nil
	}
	sub := all[matchIdx]
	base := groupIdx * 2
	if base+1 >= len(sub) {
		return nil
	}
	if sub[base] < 0 {
		return nil
	}
	return sub[base : base+2]
}

// IncrementToken advances to the next token.
func (f *PatternCaptureGroupTokenFilter) IncrementToken() (bool, error) {
	src := f.GetAttributeSource()

	if f.currentMatcher != -1 && f.nextCapture() {
		src.RestoreState(f.savedState)
		loc := f.matchLocForGroup(f.currentMatcher, f.currentGroups[f.currentMatcher]-0)
		if loc != nil {
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(f.spare[loc[0]:loc[1]])
		}
		f.posIncrAttr.SetPositionIncrement(0)
		f.currentGroups[f.currentMatcher]++
		return true, nil
	}

	ok, err := f.GetInput().IncrementToken()
	if err != nil || !ok {
		return ok, err
	}

	f.spare = f.termAttr.String()
	f.savedState = src.CaptureState()

	for i := range f.matchers {
		f.currentGroups[i] = -1
	}

	if f.preserveOriginal {
		// Emit the original first; capture groups will follow.
		f.currentMatcher = 0
		return true, nil
	}

	if f.nextCapture() {
		loc := f.matchLocForGroup(f.currentMatcher, f.currentGroups[f.currentMatcher])
		if loc != nil {
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(f.spare[loc[0]:loc[1]])
		}
		f.posIncrAttr.SetPositionIncrement(1)
		f.currentGroups[f.currentMatcher]++
		return true, nil
	}
	// No matches — pass through original unchanged.
	return true, nil
}

// Reset resets the filter.
func (f *PatternCaptureGroupTokenFilter) Reset() error {
	f.currentMatcher = -1
	f.savedState = nil
	for i := range f.currentGroups {
		f.currentGroups[i] = -1
	}
	if r, ok := f.GetInput().(interface{ Reset() error }); ok {
		return r.Reset()
	}
	return nil
}
