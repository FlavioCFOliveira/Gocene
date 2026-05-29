// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"strings"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FacetQuery is a TermQuery on the encoded drill-down term for a specific
// (dimension, path) pair. Mirrors org.apache.lucene.facet.FacetQuery.
type FacetQuery struct {
	*search.TermQuery
	dim  string
	path []string
}

// NewFacetQuery constructs a FacetQuery for the supplied dimension and path.
// The field used for the underlying TermQuery is derived from FacetsConfig
// via DrillDownFieldName.
func NewFacetQuery(config *FacetsConfig, dim string, path ...string) *FacetQuery {
	field := DrillDownFieldName(config, dim)
	encoded := PathToString(dim, path)
	return &FacetQuery{
		TermQuery: search.NewTermQuery(index.NewTerm(field, encoded)),
		dim:       dim,
		path:      append([]string(nil), path...),
	}
}

// GetDim returns the dimension being queried.
func (q *FacetQuery) GetDim() string { return q.dim }

// GetPath returns the hierarchical path.
func (q *FacetQuery) GetPath() []string { return q.path }

// DrillDownFieldName returns the index field name used for drill-down terms
// on the supplied dimension. When config is nil the default field "$facets"
// is used.
func DrillDownFieldName(config *FacetsConfig, dim string) string {
	if config == nil {
		return "$facets"
	}
	dc := config.GetDimConfig(dim)
	if dc != nil && dc.IndexFieldName != "" {
		return dc.IndexFieldName
	}
	return "$facets"
}

// PathToString encodes a (dim, path...) tuple into the single string used as
// the indexed drill-down term value. It mirrors, byte-for-byte,
// org.apache.lucene.facet.FacetsConfig.pathToString(String[], int): components
// are joined with DelimChar (U+001F), and any DelimChar or escapeChar inside a
// component is prefixed with escapeChar (U+001E) so that arbitrary labels
// (including those containing '/') round-trip through StringToPath.
//
// Each component must be non-empty; an empty component yields the empty term
// that Lucene also produces for a zero-length path.
func PathToString(dim string, path []string) string {
	full := make([]string, 1+len(path))
	full[0] = dim
	copy(full[1:], path)
	return pathComponentsToString(full)
}

// pathComponentsToString encodes a full path (dim followed by its components)
// using the Lucene DELIM_CHAR/ESCAPE_CHAR scheme. Empty input yields "".
func pathComponentsToString(path []string) string {
	if len(path) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, s := range path {
		for _, ch := range s {
			if ch == DelimChar || ch == escapeChar {
				sb.WriteRune(escapeChar)
			}
			sb.WriteRune(ch)
		}
		sb.WriteRune(DelimChar)
	}
	// Trim off the trailing DelimChar.
	encoded := sb.String()
	return encoded[:len(encoded)-utf8.RuneLen(DelimChar)]
}

// StringToPath turns an encoded indexed facet term (produced by PathToString or
// FacetsConfig.pathToString) back into its component strings, reversing the
// DelimChar/escapeChar escaping. It mirrors
// org.apache.lucene.facet.FacetsConfig.stringToPath. An empty string yields an
// empty slice.
func StringToPath(s string) []string {
	if len(s) == 0 {
		return []string{}
	}
	parts := make([]string, 0, 4)
	var buf strings.Builder
	lastEscape := false
	for _, ch := range s {
		switch {
		case lastEscape:
			buf.WriteRune(ch)
			lastEscape = false
		case ch == escapeChar:
			lastEscape = true
		case ch == DelimChar:
			parts = append(parts, buf.String())
			buf.Reset()
		default:
			buf.WriteRune(ch)
		}
	}
	parts = append(parts, buf.String())
	return parts
}
