// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"regexp"
)

// PatternReplaceFilter applies a regular expression pattern to each token
// and replaces matches with a replacement string.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.pattern.PatternReplaceFilter.
//
// This filter is useful for:
// - Normalizing token text (e.g., removing punctuation, standardizing formats)
// - Extracting parts of tokens using capture groups
// - Replacing patterns with fixed strings
//
// The replacement string can contain capture group references using $1, $2, etc.
// to include matched groups in the replacement.
//
// Example usage:
//
//	pattern := regexp.MustCompile(`[^a-zA-Z0-9]`)
//	filter := NewPatternReplaceFilter(input, pattern, "", true)
//
// This would remove all non-alphanumeric characters from tokens.
type PatternReplaceFilter struct {
	*BaseTokenFilter

	// pattern is the regular expression to match
	pattern *regexp.Regexp

	// replacement is the replacement string (can contain $1, $2, etc.)
	replacement string

	// termAttr holds the CharTermAttribute from the shared attribute source
	termAttr CharTermAttribute

	// replaceAll determines whether to replace all matches or just the first
	replaceAll bool
}

// NewPatternReplaceFilter creates a new PatternReplaceFilter.
//
// Parameters:
//   - input: the input TokenStream
//   - pattern: the regular expression pattern to match
//   - replacement: the replacement string (can use $1, $2 for capture groups)
//   - replaceAll: if true, replaces all matches; if false, replaces only the first match
//
// Example:
//
//	pattern := regexp.MustCompile(`\d+`)
//	filter := NewPatternReplaceFilter(input, pattern, "NUM", true)
func NewPatternReplaceFilter(input TokenStream, pattern *regexp.Regexp, replacement string, replaceAll bool) *PatternReplaceFilter {
	filter := &PatternReplaceFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		pattern:         pattern,
		replacement:     replacement,
		replaceAll:      replaceAll,
	}

	// Get the CharTermAttribute from the shared AttributeSource
	attrSrc := filter.GetAttributeSource()
	if attrSrc != nil {
		attr := attrSrc.GetAttributeByType(reflect.TypeOf(&charTermAttribute{}))
		if attr != nil {
			filter.termAttr = attr.(CharTermAttribute)
		}
	}

	return filter
}

// NewPatternReplaceFilterFirstMatch creates a new PatternReplaceFilter that only
// replaces the first match of the pattern.
//
// This is a convenience constructor for the common case of replacing only the first match.
func NewPatternReplaceFilterFirstMatch(input TokenStream, pattern *regexp.Regexp, replacement string) *PatternReplaceFilter {
	return NewPatternReplaceFilter(input, pattern, replacement, false)
}

// NewPatternReplaceFilterAllMatches creates a new PatternReplaceFilter that replaces
// all matches of the pattern.
//
// This is a convenience constructor for the common case of replacing all matches.
func NewPatternReplaceFilterAllMatches(input TokenStream, pattern *regexp.Regexp, replacement string) *PatternReplaceFilter {
	return NewPatternReplaceFilter(input, pattern, replacement, true)
}

// IncrementToken advances to the next token and applies the pattern replacement.
func (f *PatternReplaceFilter) IncrementToken() (bool, error) {
	hasToken, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !hasToken {
		return false, nil
	}

	// Apply pattern replacement to the token
	if f.termAttr != nil {
		text := f.termAttr.String()
		var replaced string
		if f.replaceAll {
			replaced = f.pattern.ReplaceAllString(text, f.replacement)
		} else {
			replaced = f.pattern.ReplaceAllString(text, f.replacement)
			// For first match only, we need to use ReplaceAllStringFunc with a counter
			// Actually, Go's ReplaceAllString replaces all matches, so for first match only
			// we need a different approach
			if !f.replaceAll {
				// Find the first match and replace only that
				loc := f.pattern.FindStringIndex(text)
				if loc != nil {
					// Get the matched substring
					matched := text[loc[0]:loc[1]]
					// Expand the replacement (handle $1, $2, etc.)
					expanded := f.pattern.ReplaceAllString(matched, f.replacement)
					// Build the result: before + replacement + after
					replaced = text[:loc[0]] + expanded + text[loc[1]:]
				} else {
					replaced = text
				}
			}
		}
		f.termAttr.SetValue(replaced)
	}

	return true, nil
}

// GetPattern returns the pattern used by this filter.
func (f *PatternReplaceFilter) GetPattern() *regexp.Regexp {
	return f.pattern
}

// GetReplacement returns the replacement string.
func (f *PatternReplaceFilter) GetReplacement() string {
	return f.replacement
}

// IsReplaceAll returns true if this filter replaces all matches.
func (f *PatternReplaceFilter) IsReplaceAll() bool {
	return f.replaceAll
}

// Ensure PatternReplaceFilter implements TokenFilter
var _ TokenFilter = (*PatternReplaceFilter)(nil)

// PatternReplaceFilterFactory creates PatternReplaceFilter instances.
type PatternReplaceFilterFactory struct {
	pattern     *regexp.Regexp
	replacement string
	replaceAll  bool
}

// NewPatternReplaceFilterFactory creates a new PatternReplaceFilterFactory.
//
// Parameters:
//   - pattern: the regular expression pattern to match
//   - replacement: the replacement string
//   - replaceAll: if true, replaces all matches; if false, replaces only the first
func NewPatternReplaceFilterFactory(pattern *regexp.Regexp, replacement string, replaceAll bool) *PatternReplaceFilterFactory {
	return &PatternReplaceFilterFactory{
		pattern:     pattern,
		replacement: replacement,
		replaceAll:  replaceAll,
	}
}

// NewPatternReplaceFilterFactoryWithString creates a new factory from a pattern string.
func NewPatternReplaceFilterFactoryWithString(pattern string, replacement string, replaceAll bool) (*PatternReplaceFilterFactory, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return NewPatternReplaceFilterFactory(re, replacement, replaceAll), nil
}

// Create creates a new PatternReplaceFilter wrapping the given input.
func (f *PatternReplaceFilterFactory) Create(input TokenStream) TokenFilter {
	return NewPatternReplaceFilter(input, f.pattern, f.replacement, f.replaceAll)
}

// GetPattern returns the pattern used by this factory.
func (f *PatternReplaceFilterFactory) GetPattern() *regexp.Regexp {
	return f.pattern
}

// GetReplacement returns the replacement string.
func (f *PatternReplaceFilterFactory) GetReplacement() string {
	return f.replacement
}

// IsReplaceAll returns true if this factory creates filters that replace all matches.
func (f *PatternReplaceFilterFactory) IsReplaceAll() bool {
	return f.replaceAll
}

// SetPattern sets the pattern.
func (f *PatternReplaceFilterFactory) SetPattern(pattern *regexp.Regexp) {
	f.pattern = pattern
}

// SetReplacement sets the replacement string.
func (f *PatternReplaceFilterFactory) SetReplacement(replacement string) {
	f.replacement = replacement
}

// SetReplaceAll sets whether to replace all matches or just the first.
func (f *PatternReplaceFilterFactory) SetReplaceAll(replaceAll bool) {
	f.replaceAll = replaceAll
}

// Ensure PatternReplaceFilterFactory implements TokenFilterFactory
var _ TokenFilterFactory = (*PatternReplaceFilterFactory)(nil)

// Common pattern-based filters

// CreateDigitRemovalFilter creates a filter that removes all digits from tokens.
func CreateDigitRemovalFilter(input TokenStream) *PatternReplaceFilter {
	return NewPatternReplaceFilterAllMatches(input, regexp.MustCompile(`\d+`), "")
}

// CreatePunctuationRemovalFilter creates a filter that removes all punctuation from tokens.
func CreatePunctuationRemovalFilter(input TokenStream) *PatternReplaceFilter {
	return NewPatternReplaceFilterAllMatches(input, regexp.MustCompile(`[^\w\s]`), "")
}

// CreateTokenWhitespaceNormalizationFilter creates a filter that normalizes whitespace in tokens.
func CreateTokenWhitespaceNormalizationFilter(input TokenStream) *PatternReplaceFilter {
	return NewPatternReplaceFilterAllMatches(input, regexp.MustCompile(`\s+`), " ")
}

// CreateEmailNormalizationFilter creates a filter that normalizes email addresses in tokens.
func CreateEmailNormalizationFilter(input TokenStream) *PatternReplaceFilter {
	return NewPatternReplaceFilterAllMatches(input, regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`), "EMAIL")
}

// CreateURLNormalizationFilter creates a filter that normalizes URLs in tokens.
func CreateURLNormalizationFilter(input TokenStream) *PatternReplaceFilter {
	return NewPatternReplaceFilterAllMatches(input, regexp.MustCompile(`https?://[^\s]+`), "URL")
}

// CreateTokenPhoneNormalizationFilter creates a filter that normalizes phone numbers in tokens.
func CreateTokenPhoneNormalizationFilter(input TokenStream) *PatternReplaceFilter {
	return NewPatternReplaceFilterAllMatches(input, regexp.MustCompile(`\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`), "PHONE")
}

// CreateCamelCaseSplitFilter creates a filter that inserts spaces in camelCase tokens.
// Note: This is a simplified version that inserts spaces before uppercase letters.
func CreateCamelCaseSplitFilter(input TokenStream) *PatternReplaceFilter {
	return NewPatternReplaceFilterAllMatches(input, regexp.MustCompile(`([a-z])([A-Z])`), "$1 $2")
}

// CreateSnakeCaseToSpaceFilter creates a filter that converts underscores to spaces.
func CreateSnakeCaseToSpaceFilter(input TokenStream) *PatternReplaceFilter {
	return NewPatternReplaceFilterAllMatches(input, regexp.MustCompile(`_`), " ")
}

// CreateHyphenToSpaceFilter creates a filter that converts hyphens to spaces.
func CreateHyphenToSpaceFilter(input TokenStream) *PatternReplaceFilter {
	return NewPatternReplaceFilterAllMatches(input, regexp.MustCompile(`-`), " ")
}
