// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uhighlight

import "strings"

// PassageFormatter renders a set of top passages into a human-readable
// snippet string. Mirrors org.apache.lucene.search.uhighlight.PassageFormatter.
type PassageFormatter interface {
	// Format renders passages (sorted in document order) against the
	// original field content. Returns the rendered snippet string.
	Format(passages []*Passage, content string) string
}

// DefaultPassageFormatter renders matches with <b>...</b> markup and
// joins disjoint passages with an ellipsis. Mirrors
// org.apache.lucene.search.uhighlight.DefaultPassageFormatter.
type DefaultPassageFormatter struct {
	preTag   string
	postTag  string
	ellipsis string
	escape   bool
}

// NewDefaultPassageFormatter returns the Lucene defaults
// (preTag=<b>, postTag=</b>, ellipsis="... ", escape=false).
func NewDefaultPassageFormatter() *DefaultPassageFormatter {
	return NewDefaultPassageFormatterWith("<b>", "</b>", "... ", false)
}

// NewDefaultPassageFormatterWith returns a formatter with custom tags.
// preTag, postTag and ellipsis must not be empty (a zero-length value is
// allowed; nil is not since strings are value types in Go).
func NewDefaultPassageFormatterWith(preTag, postTag, ellipsis string, escape bool) *DefaultPassageFormatter {
	return &DefaultPassageFormatter{preTag: preTag, postTag: postTag, ellipsis: ellipsis, escape: escape}
}

// Format implements PassageFormatter.
func (f *DefaultPassageFormatter) Format(passages []*Passage, content string) string {
	var sb strings.Builder
	pos := 0
	for _, passage := range passages {
		// Don't add ellipsis if it's the first one or if it's connected.
		if sb.Len() > 0 && passage.StartOffset() != pos {
			sb.WriteString(f.ellipsis)
		}
		pos = passage.StartOffset()
		matchStarts := passage.MatchStarts()
		matchEnds := passage.MatchEnds()
		numMatches := passage.NumMatches()
		for i := 0; i < numMatches; i++ {
			start := matchStarts[i]
			// Defensive: skip matches that fall outside the passage window.
			if start < pos || start >= passage.EndOffset() {
				continue
			}
			// Append content before this start.
			f.appendContent(&sb, content, pos, start)

			end := matchEnds[i]
			// Look ahead to merge overlapping matches into a single tag pair.
			for i+1 < numMatches && matchStarts[i+1] < end {
				if matchEnds[i+1] > end {
					end = matchEnds[i+1]
				}
				i++
			}
			if end > passage.EndOffset() {
				end = passage.EndOffset()
			}
			sb.WriteString(f.preTag)
			f.appendContent(&sb, content, start, end)
			sb.WriteString(f.postTag)
			pos = end
		}
		// Trailing tail of the passage. A "term" from the analyser could
		// straddle a sentence boundary; the max-with-pos guard mirrors the
		// Lucene reference.
		tail := passage.EndOffset()
		if tail < pos {
			tail = pos
		}
		f.appendContent(&sb, content, pos, tail)
		pos = passage.EndOffset()
	}
	return sb.String()
}

// appendContent appends content[start:end] to dest, applying OWASP-style
// HTML escaping when escape is true.
func (f *DefaultPassageFormatter) appendContent(dest *strings.Builder, content string, start, end int) {
	if start < 0 {
		start = 0
	}
	if end > len(content) {
		end = len(content)
	}
	if start >= end {
		return
	}
	if !f.escape {
		dest.WriteString(content[start:end])
		return
	}
	for i := start; i < end; i++ {
		ch := content[i]
		switch ch {
		case '&':
			dest.WriteString("&amp;")
		case '<':
			dest.WriteString("&lt;")
		case '>':
			dest.WriteString("&gt;")
		case '"':
			dest.WriteString("&quot;")
		case '\'':
			dest.WriteString("&#x27;")
		case '/':
			dest.WriteString("&#x2F;")
		default:
			dest.WriteByte(ch)
		}
	}
}

var _ PassageFormatter = (*DefaultPassageFormatter)(nil)
