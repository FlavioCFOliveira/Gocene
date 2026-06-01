// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// RegexpQueryHandler is a CustomQueryHandler that matches Regexp queries by
// indexing them via their longest static substring and generating ngrams from
// document tokens to match.
//
// Port of org.apache.lucene.monitor.RegexpQueryHandler.
//
// Deviation: Gocene does not yet have a RegexpQuery type; handleQuery returns
// nil for any query that is not a recognized regexp.  Full regexp support is
// deferred to backlog #2693.
type RegexpQueryHandler struct {
	ngramSuffix        string
	maxTokenSize       int
	wildcardToken      string
	wildcardTokenBytes *util.BytesRef
	excludedFields     map[string]struct{}
}

const (
	// DefaultNGramSuffix is the default suffix appended to extracted ngrams.
	DefaultNGramSuffix = "XX"
	// DefaultMaxTokenSize is the maximum input-token length before a wildcard is emitted.
	DefaultMaxTokenSize = 30
	// DefaultWildcardToken is emitted when an input token exceeds MaxTokenSize.
	DefaultWildcardToken = "__WILDCARD__"
)

// NewRegexpQueryHandler creates a RegexpQueryHandler with default settings.
func NewRegexpQueryHandler() *RegexpQueryHandler {
	return NewRegexpQueryHandlerFull(DefaultNGramSuffix, DefaultMaxTokenSize, DefaultWildcardToken, nil)
}

// NewRegexpQueryHandlerMaxToken creates a RegexpQueryHandler with a custom max token size.
func NewRegexpQueryHandlerMaxToken(maxTokenSize int) *RegexpQueryHandler {
	return NewRegexpQueryHandlerFull(DefaultNGramSuffix, maxTokenSize, DefaultWildcardToken, nil)
}

// NewRegexpQueryHandlerFull creates a RegexpQueryHandler with all options.
func NewRegexpQueryHandlerFull(
	ngramSuffix string,
	maxTokenSize int,
	wildcardToken string,
	excludedFields []string,
) *RegexpQueryHandler {
	excl := make(map[string]struct{}, len(excludedFields))
	for _, f := range excludedFields {
		excl[f] = struct{}{}
	}
	return &RegexpQueryHandler{
		ngramSuffix:        ngramSuffix,
		maxTokenSize:       maxTokenSize,
		wildcardToken:      wildcardToken,
		wildcardTokenBytes: util.NewBytesRef([]byte(wildcardToken)),
		excludedFields:     excl,
	}
}

// WrapTermStream adds SuffixingNGramTokenFilter to the stream for non-excluded fields.
func (h *RegexpQueryHandler) WrapTermStream(field string, in analysis.TokenStream) analysis.TokenStream {
	if _, excluded := h.excludedFields[field]; excluded {
		return in
	}
	return NewSuffixingNGramTokenFilter(in, h.ngramSuffix, h.wildcardToken, h.maxTokenSize)
}

// HandleQuery builds a QueryTree for a regexp query by extracting the
// longest static substring from the regexp pattern text.  Returns nil for
// non-regexp queries (caller falls back to default term extraction).
//
// Deviation: Gocene does not yet have a RegexpQuery type, so detection
// relies on the query's string representation (field:/pattern/ format)
// obtained via String(). Once the RegexpQuery type is ported, switch to a
// type assertion on the concrete RegexpQuery type.
func (h *RegexpQueryHandler) HandleQuery(q search.Query, weightor TermWeightor) QueryTree {
	qs := queryString(q)
	if qs == "" {
		return nil
	}
	pattern := parseOutRegexp(qs)
	if pattern == "" {
		return nil
	}
	longest := selectLongestSubstring(pattern)
	if longest == "" {
		return nil
	}
	field, term := splitFieldAndTerm(qs, longest)
	return NewTermQueryTree(field, util.NewBytesRef([]byte(term)), weightor.ApplyAsDouble(index.NewTerm(field, term)))
}

// queryString attempts to obtain a string representation of the query.
// It tries Stringer first, then falls back to the query's ToString method
// if the concrete type implements it.
func queryString(q search.Query) string {
	if s, ok := q.(interface{ String() string }); ok {
		return s.String()
	}
	// Many query types implement ToString(field string).
	if s, ok := q.(interface{ ToString(string) string }); ok {
		return s.ToString("")
	}
	return ""
}

// splitFieldAndTerm splits the query string representation into a field
// and term. If the representation includes a field prefix (field:term),
// the field is extracted; otherwise the field is empty.
func splitFieldAndTerm(rep, term string) (string, string) {
	fieldSep := strings.Index(rep, ":")
	if fieldSep < 0 {
		return "", term
	}
	return rep[:fieldSep], term
}

// parseOutRegexp extracts the regexp pattern from a toString representation.
func parseOutRegexp(rep string) string {
	fieldSep := strings.Index(rep, ":")
	if fieldSep < 0 {
		return rep
	}
	firstSlash := strings.Index(rep[fieldSep:], "/")
	if firstSlash < 0 {
		return rep
	}
	firstSlash += fieldSep
	lastSlash := strings.LastIndex(rep, "/")
	if lastSlash <= firstSlash {
		return rep
	}
	return rep[firstSlash+1 : lastSlash]
}

// selectLongestSubstring finds the longest static substring in a regexp pattern.
func selectLongestSubstring(regexp string) string {
	selected := ""
	for _, part := range splitRegexp(regexp) {
		if len(part) > len(selected) {
			selected = part
		}
	}
	return selected
}

func splitRegexp(s string) []string {
	var parts []string
	start := 0
	for i, ch := range s {
		switch ch {
		case '.', '*', '?':
			if i > start {
				parts = append(parts, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}
