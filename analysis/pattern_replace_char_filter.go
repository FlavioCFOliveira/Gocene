// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bytes"
	"io"
	"regexp"
)

// PatternReplaceCharFilter replaces patterns in the input text using regular expressions.
// This is useful for removing or replacing specific patterns before tokenization.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.charfilter.PatternReplaceCharFilter.
type PatternReplaceCharFilter struct {
	*CharFilter
	pattern     *regexp.Regexp
	replacement string
	buffer      []byte
	position    int
}

// NewPatternReplaceCharFilter creates a new PatternReplaceCharFilter.
func NewPatternReplaceCharFilter(pattern *regexp.Regexp, replacement string, input io.Reader) *PatternReplaceCharFilter {
	// Read all input
	data, err := io.ReadAll(input)
	if err != nil {
		data = []byte{}
	}

	// Apply pattern replacement
	replaced := pattern.ReplaceAll(data, []byte(replacement))

	return &PatternReplaceCharFilter{
		CharFilter:  NewCharFilter(bytes.NewReader(replaced)),
		pattern:     pattern,
		replacement: replacement,
		buffer:      replaced,
		position:    0,
	}
}

// NewPatternReplaceCharFilterWithString creates a new PatternReplaceCharFilter from a pattern string.
func NewPatternReplaceCharFilterWithString(pattern string, replacement string, input io.Reader) (*PatternReplaceCharFilter, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return NewPatternReplaceCharFilter(re, replacement, input), nil
}

// Read reads characters into the provided buffer.
func (f *PatternReplaceCharFilter) Read(p []byte) (n int, err error) {
	if f.position >= len(f.buffer) {
		return 0, io.EOF
	}

	n = copy(p, f.buffer[f.position:])
	f.position += n

	return n, nil
}

// GetPattern returns the pattern used by this filter.
func (f *PatternReplaceCharFilter) GetPattern() *regexp.Regexp {
	return f.pattern
}

// GetReplacement returns the replacement string.
func (f *PatternReplaceCharFilter) GetReplacement() string {
	return f.replacement
}

// PatternReplaceCharFilterFactory creates PatternReplaceCharFilter instances.
type PatternReplaceCharFilterFactory struct {
	*BaseCharFilterFactory
	pattern     *regexp.Regexp
	replacement string
}

// NewPatternReplaceCharFilterFactory creates a new PatternReplaceCharFilterFactory.
func NewPatternReplaceCharFilterFactory(pattern *regexp.Regexp, replacement string) *PatternReplaceCharFilterFactory {
	return &PatternReplaceCharFilterFactory{
		BaseCharFilterFactory: NewBaseCharFilterFactory("patternReplace"),
		pattern:               pattern,
		replacement:           replacement,
	}
}

// NewPatternReplaceCharFilterFactoryWithString creates a new factory from a pattern string.
func NewPatternReplaceCharFilterFactoryWithString(pattern string, replacement string) (*PatternReplaceCharFilterFactory, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return NewPatternReplaceCharFilterFactory(re, replacement), nil
}

// Create creates a new PatternReplaceCharFilter.
func (f *PatternReplaceCharFilterFactory) Create(input io.Reader) *PatternReplaceCharFilter {
	return NewPatternReplaceCharFilter(f.pattern, f.replacement, input)
}

// GetPattern returns the pattern used by this factory.
func (f *PatternReplaceCharFilterFactory) GetPattern() *regexp.Regexp {
	return f.pattern
}

// GetReplacement returns the replacement string.
func (f *PatternReplaceCharFilterFactory) GetReplacement() string {
	return f.replacement
}

// SetPattern sets the pattern.
func (f *PatternReplaceCharFilterFactory) SetPattern(pattern *regexp.Regexp) {
	f.pattern = pattern
}

// SetReplacement sets the replacement string.
func (f *PatternReplaceCharFilterFactory) SetReplacement(replacement string) {
	f.replacement = replacement
}

// Common pattern replacements

// GetEmailPattern returns a pattern for matching email addresses.
func GetEmailPattern() *regexp.Regexp {
	return regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
}

// GetURLPattern returns a pattern for matching URLs.
func GetURLPattern() *regexp.Regexp {
	return regexp.MustCompile(`https?://[^\s]+`)
}

// GetPhonePattern returns a pattern for matching phone numbers.
func GetPhonePattern() *regexp.Regexp {
	// Simple US phone number pattern
	return regexp.MustCompile(`\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`)
}

// GetNumberPattern returns a pattern for matching numbers.
func GetNumberPattern() *regexp.Regexp {
	return regexp.MustCompile(`\d+`)
}

// GetWhitespacePattern returns a pattern for matching multiple whitespace characters.
func GetWhitespacePattern() *regexp.Regexp {
	return regexp.MustCompile(`\s+`)
}

// GetNonWordPattern returns a pattern for matching non-word characters.
func GetNonWordPattern() *regexp.Regexp {
	return regexp.MustCompile(`\W+`)
}

// CreateEmailRemovalFilter creates a filter that removes email addresses.
func CreateEmailRemovalFilter(input io.Reader) *PatternReplaceCharFilter {
	return NewPatternReplaceCharFilter(GetEmailPattern(), "", input)
}

// CreateURLRemovalFilter creates a filter that removes URLs.
func CreateURLRemovalFilter(input io.Reader) *PatternReplaceCharFilter {
	return NewPatternReplaceCharFilter(GetURLPattern(), "", input)
}

// CreatePhoneNormalizationFilter creates a filter that normalizes phone numbers.
func CreatePhoneNormalizationFilter(input io.Reader) *PatternReplaceCharFilter {
	return NewPatternReplaceCharFilter(GetPhonePattern(), "PHONE", input)
}

// CreateNumberRemovalFilter creates a filter that removes numbers.
func CreateNumberRemovalFilter(input io.Reader) *PatternReplaceCharFilter {
	return NewPatternReplaceCharFilter(GetNumberPattern(), "", input)
}

// CreateWhitespaceNormalizationFilter creates a filter that normalizes whitespace.
func CreateWhitespaceNormalizationFilter(input io.Reader) *PatternReplaceCharFilter {
	return NewPatternReplaceCharFilter(GetWhitespacePattern(), " ", input)
}
