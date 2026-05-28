// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// HTMLStripCharFilter strips HTML tags from the input text.
// This is useful for indexing HTML content without the markup.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.charfilter.HTMLStripCharFilter.
type HTMLStripCharFilter struct {
	*CharFilter
	buffer      []byte
	position    int
	htmlRegex   *regexp.Regexp
	entityRegex *regexp.Regexp
	// readErr holds an error encountered while consuming the input (an I/O
	// failure, or ErrInputTooLarge when the input exceeds
	// MaxTokenizerInputSize). The constructor cannot return an error, so the
	// error is surfaced through the io.Reader contract on the first Read.
	readErr error
}

// htmlTagRegex matches HTML tags
var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

// htmlEntityRegex matches HTML entities
var htmlEntityRegex = regexp.MustCompile(`&(#?[a-zA-Z0-9]+);`)

// NewHTMLStripCharFilter creates a new HTMLStripCharFilter. The input is read
// in full, bounded by MaxTokenizerInputSize; if it is exceeded (or the input
// fails to read), the error is deferred and returned from the first call to
// Read, since the constructor itself cannot return one.
func NewHTMLStripCharFilter(input io.Reader) *HTMLStripCharFilter {
	// Read all input, bounded by MaxTokenizerInputSize.
	data, readErr := readAllLimited(input)
	if readErr != nil {
		// Do not strip/decode a partial buffer; surface the error instead.
		data = []byte{}
	}

	// Strip HTML tags
	stripped := htmlTagRegex.ReplaceAll(data, []byte{})

	// Decode HTML entities
	decoded := decodeHTMLEntities(stripped)

	return &HTMLStripCharFilter{
		CharFilter:  NewCharFilter(bytes.NewReader(decoded)),
		buffer:      decoded,
		position:    0,
		htmlRegex:   htmlTagRegex,
		entityRegex: htmlEntityRegex,
		readErr:     readErr,
	}
}

// Read reads characters into the provided buffer. If the input could not be
// consumed (an I/O error, or ErrInputTooLarge when the input exceeded
// MaxTokenizerInputSize), that error is returned here rather than silently
// yielding a truncated or empty stream.
func (f *HTMLStripCharFilter) Read(p []byte) (n int, err error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	if f.position >= len(f.buffer) {
		return 0, io.EOF
	}

	n = copy(p, f.buffer[f.position:])
	f.position += n

	return n, nil
}

// decodeHTMLEntities decodes common HTML entities.
func decodeHTMLEntities(data []byte) []byte {
	result := string(data)

	// Common HTML entities
	entities := map[string]string{
		"amp":  "&",
		"lt":   "<",
		"gt":   ">",
		"quot": `"`,
		"apos": "'",
		"nbsp": " ",
	}

	// Replace named entities
	for entity, replacement := range entities {
		result = strings.ReplaceAll(result, "&"+entity+";", replacement)
	}

	// Replace numeric entities (decimal)
	result = replaceNumericEntities(result)

	return []byte(result)
}

// replaceNumericEntities replaces numeric HTML entities.
func replaceNumericEntities(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '&' && s[i+1] == '#' {
			// Find the end of the entity
			end := i + 2
			for end < len(s) && s[end] != ';' {
				end++
			}
			if end < len(s) {
				// Extract the code
				codeStr := s[i+2 : end]
				var code int
				if len(codeStr) > 0 && (codeStr[0] == 'x' || codeStr[0] == 'X') {
					// Hexadecimal
					fmt.Sscanf(codeStr[1:], "%x", &code)
				} else {
					// Decimal
					fmt.Sscanf(codeStr, "%d", &code)
				}
				if code > 0 {
					result.WriteRune(rune(code))
				}
				i = end + 1
				continue
			}
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}

// HTMLStripCharFilterFactory creates HTMLStripCharFilter instances.
type HTMLStripCharFilterFactory struct {
	*BaseCharFilterFactory
}

// NewHTMLStripCharFilterFactory creates a new HTMLStripCharFilterFactory.
func NewHTMLStripCharFilterFactory() *HTMLStripCharFilterFactory {
	return &HTMLStripCharFilterFactory{
		BaseCharFilterFactory: NewBaseCharFilterFactory("htmlStrip"),
	}
}

// Create creates a new HTMLStripCharFilter.
func (f *HTMLStripCharFilterFactory) Create(input io.Reader) *HTMLStripCharFilter {
	return NewHTMLStripCharFilter(input)
}
