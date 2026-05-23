// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package pattern

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// PatternTypingRule holds a compiled regular expression together with the
// integer flags bitmask and the type-template string for a single rule.
//
// This is the Go equivalent of the Java record
// PatternTypingFilter.PatternTypingRule.
type PatternTypingRule struct {
	// Pattern is the compiled regular expression.
	Pattern *regexp.Regexp
	// Flags is the integer bitmask that will be set on FlagsAttribute.
	Flags int
	// TypeTemplate is the replacement template passed to Regexp.ReplaceAllString.
	TypeTemplate string
}

// PatternTypingFilter sets TypeAttribute and FlagsAttribute on tokens that
// match any of its rules. The type value is derived by applying the rule's
// template as a regexp replacement against the matched token.
//
// This is the Go port of
// org.apache.lucene.analysis.pattern.PatternTypingFilter from Apache Lucene
// 10.4.0.
type PatternTypingFilter struct {
	*analysis.BaseTokenFilter

	rules    []PatternTypingRule
	termAttr analysis.CharTermAttribute
	flagAttr analysis.FlagsAttribute
	typeAttr analysis.TypeAttribute
}

// NewPatternTypingFilter creates a PatternTypingFilter.
//
// rules must be non-nil and non-empty; every PatternTypingRule.Pattern must be
// non-nil.
func NewPatternTypingFilter(input analysis.TokenStream, rules ...PatternTypingRule) *PatternTypingFilter {
	if len(rules) == 0 {
		panic("patternTypingFilter: rules must not be empty")
	}
	f := &PatternTypingFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		rules:           rules,
	}
	src := f.GetAttributeSource()
	if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
		f.termAttr = a.(analysis.CharTermAttribute)
	} else {
		f.termAttr = analysis.NewCharTermAttribute()
		src.AddAttributeImpl(f.termAttr)
	}
	if a := src.GetAttribute(analysis.FlagsAttributeType); a != nil {
		f.flagAttr = a.(analysis.FlagsAttribute)
	} else {
		f.flagAttr = analysis.NewFlagsAttribute()
		src.AddAttributeImpl(f.flagAttr)
	}
	if a := src.GetAttribute(analysis.TypeAttributeType); a != nil {
		f.typeAttr = a.(analysis.TypeAttribute)
	} else {
		f.typeAttr = analysis.NewTypeAttribute()
		src.AddAttributeImpl(f.typeAttr)
	}
	return f
}

// IncrementToken advances to the next token.
func (f *PatternTypingFilter) IncrementToken() (bool, error) {
	ok, err := f.GetInput().IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	term := f.termAttr.String()
	for _, rule := range f.rules {
		if rule.Pattern.MatchString(term) {
			f.typeAttr.SetType(rule.Pattern.ReplaceAllString(term, rule.TypeTemplate))
			f.flagAttr.SetFlags(rule.Flags)
			return true, nil
		}
	}
	return true, nil
}

// Reset resets the filter.
func (f *PatternTypingFilter) Reset() error {
	if r, ok := f.GetInput().(interface{ Reset() error }); ok {
		return r.Reset()
	}
	return nil
}

// ── PatternTypingFilterFactory ────────────────────────────────────────────────

// PatternTypingFilterFactory creates PatternTypingFilter instances from a
// plain-text rules file.
//
// The file format is one rule per line:
//
//	<flags> <pattern> ::: <typeTemplate>
//
// Lines whose first character is '#' are treated as comments. flags must be a
// decimal integer with no leading spaces. The pattern and typeTemplate are
// separated by the literal " ::: " (space-colon-colon-colon-space).
//
// This is the Go port of
// org.apache.lucene.analysis.pattern.PatternTypingFilterFactory from Apache
// Lucene 10.4.0.
//
// Deviation: Java uses ResourceLoader to read files at inform() time. Go
// callers either supply a pre-built []PatternTypingRule or a rules file via
// LoadRulesFrom.
type PatternTypingFilterFactory struct {
	rules []PatternTypingRule
}

// NewPatternTypingFilterFactory constructs a factory with pre-built rules.
func NewPatternTypingFilterFactory(rules ...PatternTypingRule) *PatternTypingFilterFactory {
	return &PatternTypingFilterFactory{rules: rules}
}

// LoadPatternTypingRules parses a rules file from r and returns the resulting
// []PatternTypingRule.
//
// Format per line:
//
//	<decimal integer flags> <regex> ::: <typeTemplate>
//
// Lines starting with '#' are skipped.
func LoadPatternTypingRules(r io.Reader) ([]PatternTypingRule, error) {
	var rules []PatternTypingRule
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		firstSpace := strings.IndexByte(line, ' ')
		if firstSpace < 0 {
			return nil, fmt.Errorf("patternTypingFilter: malformed rule (no space): %q", line)
		}
		flagsVal, err := strconv.Atoi(line[:firstSpace])
		if err != nil {
			return nil, fmt.Errorf("patternTypingFilter: invalid flags %q: %w", line[:firstSpace], err)
		}
		rest := line[firstSpace+1:]
		parts := strings.SplitN(rest, " ::: ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("patternTypingFilter: rule must contain ' ::: ' separator: %q", line)
		}
		compiled, err := regexp.Compile(parts[0])
		if err != nil {
			return nil, fmt.Errorf("patternTypingFilter: invalid pattern %q: %w", parts[0], err)
		}
		rules = append(rules, PatternTypingRule{
			Pattern:      compiled,
			Flags:        flagsVal,
			TypeTemplate: parts[1],
		})
	}
	return rules, scanner.Err()
}

// Create returns a new PatternTypingFilter wrapping the given TokenStream.
func (f *PatternTypingFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewPatternTypingFilter(input, f.rules...)
}
