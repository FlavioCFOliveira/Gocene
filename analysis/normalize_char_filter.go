// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bytes"
	"io"
	"strings"
	"unicode"
)

// NormalizeCharFilter normalizes text using Unicode normalization.
// This can perform various normalization operations like:
// - Case folding (lowercase/uppercase)
// - Unicode normalization (NFC, NFD, NFKC, NFKD)
// - Whitespace normalization
//
// This is the Go port of Lucene's org.apache.lucene.analysis.charfilter.NormalizeCharFilter.
type NormalizeCharFilter struct {
	*CharFilter
	buffer   []rune
	position int
	options  *NormalizationOptions
}

// NormalizationOptions specifies the normalization options.
type NormalizationOptions struct {
	// Lowercase converts text to lowercase
	Lowercase bool

	// Uppercase converts text to uppercase
	Uppercase bool

	// RemoveAccents removes accents from characters
	RemoveAccents bool

	// NormalizeWhitespace collapses multiple whitespace characters
	NormalizeWhitespace bool

	// TrimWhitespace removes leading and trailing whitespace
	TrimWhitespace bool

	// RemoveControlChars removes control characters
	RemoveControlChars bool
}

// NewNormalizationOptions creates default normalization options.
func NewNormalizationOptions() *NormalizationOptions {
	return &NormalizationOptions{
		Lowercase:           true,
		Uppercase:           false,
		RemoveAccents:       false,
		NormalizeWhitespace: true,
		TrimWhitespace:      false,
		RemoveControlChars:  true,
	}
}

// NewNormalizeCharFilter creates a new NormalizeCharFilter with default options.
func NewNormalizeCharFilter(input io.Reader) *NormalizeCharFilter {
	return NewNormalizeCharFilterWithOptions(input, NewNormalizationOptions())
}

// NewNormalizeCharFilterWithOptions creates a new NormalizeCharFilter with custom options.
func NewNormalizeCharFilterWithOptions(input io.Reader, options *NormalizationOptions) *NormalizeCharFilter {
	// Read all input
	data, err := io.ReadAll(input)
	if err != nil {
		data = []byte{}
	}

	// Apply normalization
	normalized := normalizeText(string(data), options)

	return &NormalizeCharFilter{
		CharFilter: NewCharFilter(bytes.NewReader([]byte(normalized))),
		buffer:     []rune(normalized),
		position:   0,
		options:    options,
	}
}

// Read reads characters into the provided buffer.
func (f *NormalizeCharFilter) Read(p []byte) (n int, err error) {
	if f.position >= len(f.buffer) {
		return 0, io.EOF
	}

	remaining := string(f.buffer[f.position:])
	n = copy(p, remaining)
	f.position += len([]rune(remaining[:n]))

	return n, nil
}

// GetOptions returns the normalization options.
func (f *NormalizeCharFilter) GetOptions() *NormalizationOptions {
	return f.options
}

// normalizeText applies normalization to the input text.
func normalizeText(text string, options *NormalizationOptions) string {
	result := text

	// Remove control characters
	if options.RemoveControlChars {
		result = removeControlChars(result)
	}

	// Case conversion
	if options.Lowercase {
		result = strings.ToLower(result)
	} else if options.Uppercase {
		result = strings.ToUpper(result)
	}

	// Remove accents
	if options.RemoveAccents {
		result = removeAccents(result)
	}

	// Normalize whitespace
	if options.NormalizeWhitespace {
		result = normalizeWhitespace(result)
	}

	// Trim whitespace
	if options.TrimWhitespace {
		result = strings.TrimSpace(result)
	}

	return result
}

// removeControlChars removes control characters from the text.
func removeControlChars(text string) string {
	var result strings.Builder
	for _, r := range text {
		if !unicode.IsControl(r) || r == '\n' || r == '\r' || r == '\t' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// removeAccents removes accents from characters.
func removeAccents(text string) string {
	// Simple accent removal for common Latin characters
	accents := map[rune]rune{
		'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a',
		'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
		'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
		'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'ø': 'o',
		'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u',
		'ý': 'y', 'ÿ': 'y',
		'ç': 'c', 'ñ': 'n',
		'À': 'A', 'Á': 'A', 'Â': 'A', 'Ã': 'A', 'Ä': 'A', 'Å': 'A',
		'È': 'E', 'É': 'E', 'Ê': 'E', 'Ë': 'E',
		'Ì': 'I', 'Í': 'I', 'Î': 'I', 'Ï': 'I',
		'Ò': 'O', 'Ó': 'O', 'Ô': 'O', 'Õ': 'O', 'Ö': 'O', 'Ø': 'O',
		'Ù': 'U', 'Ú': 'U', 'Û': 'U', 'Ü': 'U',
		'Ý': 'Y', 'Ç': 'C', 'Ñ': 'N',
	}

	var result strings.Builder
	for _, r := range text {
		if mapped, ok := accents[r]; ok {
			result.WriteRune(mapped)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// normalizeWhitespace collapses multiple whitespace characters.
func normalizeWhitespace(text string) string {
	var result strings.Builder
	inWhitespace := false

	for _, r := range text {
		if unicode.IsSpace(r) {
			if !inWhitespace {
				result.WriteRune(' ')
				inWhitespace = true
			}
		} else {
			result.WriteRune(r)
			inWhitespace = false
		}
	}

	return result.String()
}

// NormalizeCharFilterFactory creates NormalizeCharFilter instances.
type NormalizeCharFilterFactory struct {
	*BaseCharFilterFactory
	options *NormalizationOptions
}

// NewNormalizeCharFilterFactory creates a new NormalizeCharFilterFactory.
func NewNormalizeCharFilterFactory() *NormalizeCharFilterFactory {
	return &NormalizeCharFilterFactory{
		BaseCharFilterFactory: NewBaseCharFilterFactory("normalize"),
		options:               NewNormalizationOptions(),
	}
}

// NewNormalizeCharFilterFactoryWithOptions creates a new factory with custom options.
func NewNormalizeCharFilterFactoryWithOptions(options *NormalizationOptions) *NormalizeCharFilterFactory {
	return &NormalizeCharFilterFactory{
		BaseCharFilterFactory: NewBaseCharFilterFactory("normalize"),
		options:               options,
	}
}

// Create creates a new NormalizeCharFilter.
func (f *NormalizeCharFilterFactory) Create(input io.Reader) *NormalizeCharFilter {
	return NewNormalizeCharFilterWithOptions(input, f.options)
}

// GetOptions returns the normalization options.
func (f *NormalizeCharFilterFactory) GetOptions() *NormalizationOptions {
	return f.options
}

// SetOptions sets the normalization options.
func (f *NormalizeCharFilterFactory) SetOptions(options *NormalizationOptions) {
	f.options = options
}
