// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// Marker constants for ReverseStringFilter. The first three are
// useful prefixes for indexing reversed tokens that callers want to
// re-identify after retrieval; RTLDirectionMarker prepends a U+200F
// for right-to-left script reversal.
const (
	// StartOfHeadingMarker (U+0001) is the default marker.
	StartOfHeadingMarker = ''

	// InformationSeparatorMarker (U+001F) is an alternative marker.
	InformationSeparatorMarker = ''

	// PUAEC00Marker (U+EC00) is a Private Use Area marker.
	PUAEC00Marker = ''

	// RTLDirectionMarker (U+200F) is the right-to-left mark.
	RTLDirectionMarker = '‏'

	// noMarker is used internally to indicate "no prefix marker".
	noMarker = '￿'
)

// ReverseStringFilter reverses each token's text. When a marker rune
// is configured (non-zero, non-noMarker), it is prepended to the
// reversed token to allow downstream code to identify reversed terms
// and exclude them from non-reversed dictionaries.
//
// This is the Go port of
// org.apache.lucene.analysis.reverse.ReverseStringFilter from Apache
// Lucene 10.4.0.
//
// Deviation from Lucene: the reference reverses a char[] in place
// with explicit surrogate-pair handling so that non-BMP characters
// remain valid after reversal. The Go port works on a []rune
// intermediate; since []rune already enumerates whole code points,
// no surrogate-pair logic is required.
type ReverseStringFilter struct {
	*BaseTokenFilter

	marker   rune
	termAttr CharTermAttribute
}

// NewReverseStringFilter wraps input with a marker-less reverse
// filter.
func NewReverseStringFilter(input TokenStream) *ReverseStringFilter {
	return NewReverseStringFilterWithMarker(input, noMarker)
}

// NewReverseStringFilterWithMarker wraps input with a reverse filter
// that prepends marker to every reversed token. Pass noMarker (or
// any rune equal to 0xFFFF) to disable the prefix.
func NewReverseStringFilterWithMarker(input TokenStream, marker rune) *ReverseStringFilter {
	f := &ReverseStringFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		marker:          marker,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f
}

// IncrementToken reverses the current token's text and, when
// configured, prepends the marker.
func (f *ReverseStringFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if f.termAttr == nil {
		return true, nil
	}
	s := f.termAttr.String()
	reversed := ReverseRunes(s)
	f.termAttr.SetEmpty()
	if f.marker != noMarker {
		f.termAttr.AppendString(string(f.marker))
	}
	f.termAttr.AppendString(reversed)
	return true, nil
}

// ReverseRunes returns s with its rune sequence reversed.
func ReverseRunes(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// Ensure ReverseStringFilter implements TokenFilter.
var _ TokenFilter = (*ReverseStringFilter)(nil)

// ReverseStringFilterFactory creates ReverseStringFilter instances.
// When marker is noMarker (the default), reversed tokens are emitted
// without a prefix.
type ReverseStringFilterFactory struct {
	marker rune
}

// NewReverseStringFilterFactory returns a factory configured for
// marker-less reversal.
func NewReverseStringFilterFactory() *ReverseStringFilterFactory {
	return &ReverseStringFilterFactory{marker: noMarker}
}

// NewReverseStringFilterFactoryWithMarker returns a factory that
// produces reversed tokens prefixed with marker.
func NewReverseStringFilterFactoryWithMarker(marker rune) *ReverseStringFilterFactory {
	return &ReverseStringFilterFactory{marker: marker}
}

// Create returns a ReverseStringFilter wrapping input.
func (f *ReverseStringFilterFactory) Create(input TokenStream) TokenFilter {
	return NewReverseStringFilterWithMarker(input, f.marker)
}

// Ensure ReverseStringFilterFactory implements TokenFilterFactory.
var _ TokenFilterFactory = (*ReverseStringFilterFactory)(nil)
