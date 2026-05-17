// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"regexp"
	"strings"
)

// CSVParse parses a single CSV-encoded line, returning the unquoted
// fields. Returns an empty slice when the input has an odd number of
// quote characters (matching the Lucene reference's defensive
// fallback).
//
// This is the Go port of
// org.apache.lucene.analysis.util.CSVUtil.parse(String) from Apache
// Lucene 10.4.0.
//
// Deviation from Lucene: the Java reference returns String[]; the
// Go port returns []string. Behaviour is otherwise identical:
// commas inside quoted fields are preserved, doubled quotes ("")
// inside a quoted field are unescaped to a single quote, and the
// surrounding quotes are stripped from the returned values.
func CSVParse(line string) []string {
	if line == "" {
		return nil
	}
	// Quick parity check: even number of double quotes.
	if strings.Count(line, `"`)%2 != 0 {
		return []string{}
	}
	var fields []string
	var current strings.Builder
	insideQuote := false
	for _, r := range line {
		switch {
		case r == '"':
			insideQuote = !insideQuote
			current.WriteRune(r)
		case r == ',' && !insideQuote:
			fields = append(fields, csvUnQuoteUnEscape(current.String()))
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	fields = append(fields, csvUnQuoteUnEscape(current.String()))
	return fields
}

var csvUnQuoteRE = regexp.MustCompile(`^"([^"]+)"$`)

// csvUnQuoteUnEscape strips surrounding quotes and replaces doubled
// quotes with single quotes inside a quoted field.
func csvUnQuoteUnEscape(s string) string {
	out := s
	if m := csvUnQuoteRE.FindStringSubmatch(out); m != nil {
		out = m[1]
	}
	return strings.ReplaceAll(out, `""`, `"`)
}

// CSVQuoteEscape escapes a single field value: internal quotes are
// doubled, and the result is wrapped in quotes when it contains a
// comma.
//
// This is the Go port of
// org.apache.lucene.analysis.util.CSVUtil.quoteEscape(String).
func CSVQuoteEscape(original string) string {
	out := strings.ReplaceAll(original, `"`, `""`)
	if strings.ContainsRune(out, ',') {
		out = `"` + out + `"`
	}
	return out
}
