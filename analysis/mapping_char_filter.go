// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bytes"
	"io"
)

// NormalizeCharMap provides character mappings for normalization.
// This is used by MappingCharFilter to replace characters.
type NormalizeCharMap struct {
	// mappings stores the character mappings
	mappings map[rune]rune
}

// NewNormalizeCharMap creates a new NormalizeCharMap.
func NewNormalizeCharMap() *NormalizeCharMap {
	return &NormalizeCharMap{
		mappings: make(map[rune]rune),
	}
}

// AddMapping adds a character mapping.
func (ncm *NormalizeCharMap) AddMapping(from, to rune) {
	ncm.mappings[from] = to
}

// AddMappingString adds a string mapping (first rune only).
func (ncm *NormalizeCharMap) AddMappingString(from, to string) {
	if len(from) > 0 && len(to) > 0 {
		ncm.mappings[[]rune(from)[0]] = []rune(to)[0]
	}
}

// GetMapping returns the mapped character for the given character.
func (ncm *NormalizeCharMap) GetMapping(r rune) (rune, bool) {
	if mapped, ok := ncm.mappings[r]; ok {
		return mapped, true
	}
	return r, false
}

// HasMapping returns true if a mapping exists for the given character.
func (ncm *NormalizeCharMap) HasMapping(r rune) bool {
	_, ok := ncm.mappings[r]
	return ok
}

// GetMappingCount returns the number of mappings.
func (ncm *NormalizeCharMap) GetMappingCount() int {
	return len(ncm.mappings)
}

// Clear clears all mappings.
func (ncm *NormalizeCharMap) Clear() {
	ncm.mappings = make(map[rune]rune)
}

// MappingCharFilter applies character mappings to the input.
// This is useful for normalizing characters, such as converting
// accented characters to their base forms.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.charfilter.MappingCharFilter.
type MappingCharFilter struct {
	*CharFilter
	charMap *NormalizeCharMap
	buffer  []rune
	position int
}

// NewMappingCharFilter creates a new MappingCharFilter.
func NewMappingCharFilter(charMap *NormalizeCharMap, input io.Reader) *MappingCharFilter {
	// Read all input
	data, err := io.ReadAll(input)
	if err != nil {
		data = []byte{}
	}

	// Convert to runes and apply mappings
	runes := []rune(string(data))
	mapped := make([]rune, len(runes))
	for i, r := range runes {
		if mappedR, ok := charMap.GetMapping(r); ok {
			mapped[i] = mappedR
		} else {
			mapped[i] = r
		}
	}

	return &MappingCharFilter{
		CharFilter: NewCharFilter(bytes.NewReader([]byte(string(mapped)))),
		charMap:    charMap,
		buffer:     mapped,
		position:   0,
	}
}

// Read reads characters into the provided buffer.
func (f *MappingCharFilter) Read(p []byte) (n int, err error) {
	if f.position >= len(f.buffer) {
		return 0, io.EOF
	}

	// Convert runes to bytes
	remaining := string(f.buffer[f.position:])
	n = copy(p, remaining)
	f.position += len([]rune(remaining[:n]))

	return n, nil
}

// GetCharMap returns the character map used by this filter.
func (f *MappingCharFilter) GetCharMap() *NormalizeCharMap {
	return f.charMap
}

// MappingCharFilterFactory creates MappingCharFilter instances.
type MappingCharFilterFactory struct {
	*BaseCharFilterFactory
	charMap *NormalizeCharMap
}

// NewMappingCharFilterFactory creates a new MappingCharFilterFactory.
func NewMappingCharFilterFactory(charMap *NormalizeCharMap) *MappingCharFilterFactory {
	return &MappingCharFilterFactory{
		BaseCharFilterFactory: NewBaseCharFilterFactory("mapping"),
		charMap:               charMap,
	}
}

// Create creates a new MappingCharFilter.
func (f *MappingCharFilterFactory) Create(input io.Reader) *MappingCharFilter {
	return NewMappingCharFilter(f.charMap, input)
}

// GetCharMap returns the character map used by this factory.
func (f *MappingCharFilterFactory) GetCharMap() *NormalizeCharMap {
	return f.charMap
}

// Common character mappings

// GetAccentedCharMap returns a map for removing accents from characters.
func GetAccentedCharMap() *NormalizeCharMap {
	m := NewNormalizeCharMap()

	// Latin accented characters
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

	for from, to := range accents {
		m.AddMapping(from, to)
	}

	return m
}

// GetCyrillicToLatinMap returns a map for transliterating Cyrillic to Latin.
// Note: This is a simplified mapping that only handles single character mappings.
// Multi-character transliterations (like 'zh', 'ch', 'sh') are not supported.
func GetCyrillicToLatinMap() *NormalizeCharMap {
	m := NewNormalizeCharMap()

	// Basic Cyrillic to Latin mapping (single characters only)
	cyrillic := map[rune]rune{
		'а': 'a', 'б': 'b', 'в': 'v', 'г': 'g', 'д': 'd', 'е': 'e',
		'з': 'z', 'и': 'i', 'й': 'j', 'к': 'k',
		'л': 'l', 'м': 'm', 'н': 'n', 'о': 'o', 'п': 'p', 'р': 'r',
		'с': 's', 'т': 't', 'у': 'u', 'ф': 'f', 'х': 'h',
		'ы': 'y', 'э': 'e',
	}

	for from, to := range cyrillic {
		m.AddMapping(from, to)
	}

	return m
}

// GetSmartQuotesMap returns a map for converting smart quotes to straight quotes.
func GetSmartQuotesMap() *NormalizeCharMap {
	m := NewNormalizeCharMap()

	// Smart quotes to straight quotes
	m.AddMapping('\u201c', '"') // Left double quotation mark
	m.AddMapping('\u201d', '"') // Right double quotation mark
	m.AddMapping('\u2018', '\'') // Left single quotation mark
	m.AddMapping('\u2019', '\'') // Right single quotation mark
	m.AddMapping('\u201a', ',')  // Single low-9 quotation mark
	m.AddMapping('\u201e', '"') // Double low-9 quotation mark

	return m
}

// GetDashesMap returns a map for converting dashes to hyphens.
func GetDashesMap() *NormalizeCharMap {
	m := NewNormalizeCharMap()

	// Various dashes to hyphen
	m.AddMapping('\u2010', '-') // Hyphen
	m.AddMapping('\u2011', '-') // Non-breaking hyphen
	m.AddMapping('\u2012', '-') // Figure dash
	m.AddMapping('\u2013', '-') // En dash
	m.AddMapping('\u2014', '-') // Em dash
	m.AddMapping('\u2015', '-') // Horizontal bar

	return m
}
