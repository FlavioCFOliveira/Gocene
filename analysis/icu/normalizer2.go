// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// NormalizerMode selects the composition or decomposition form.
//
// Go port of com.ibm.icu.text.Normalizer2.Mode.
type NormalizerMode int

const (
	// NormalizerModeCompose applies NFC/NFKC composition.
	NormalizerModeCompose NormalizerMode = iota
	// NormalizerModeDecompose applies NFD/NFKD decomposition.
	NormalizerModeDecompose
)

// Normalizer2 normalizes Unicode text according to a specified form.
//
// This is the Go equivalent of com.ibm.icu.text.Normalizer2. Callers
// may supply a concrete implementation or use one of the constructors
// below to create a standard normalizer.
//
// Deviation: The Java Normalizer2 is an abstract class with a rich API
// (quickCheck, spanQuickCheckYes, hasBoundaryBefore, hasBoundaryAfter,
// filteredNormalize, etc.). The Go interface exposes only the methods
// required by ICUNormalizer2CharFilter and ICUNormalizer2Filter; the
// full ICU API requires the ICU4J CGO binding.
type Normalizer2 interface {
	// Normalize returns the normalised form of s.
	Normalize(s string) string

	// QuickCheck returns Normalizer2QuickCheckYes if s is already in
	// the normalised form, Normalizer2QuickCheckMaybe or
	// Normalizer2QuickCheckNo otherwise.
	QuickCheck(s string) Normalizer2QuickCheck

	// SpanQuickCheckYes returns the length prefix of s that is
	// confirmed normalised by a quick check. The remainder needs
	// full normalisation.
	SpanQuickCheckYes(s string) int

	// HasBoundaryBefore reports whether the code point at byte offset
	// pos in s is a normalization boundary (i.e. normalization can
	// restart from there).
	HasBoundaryBefore(s string, pos int) bool

	// HasBoundaryAfter is the same but checks after pos.
	HasBoundaryAfter(s string, pos int) bool
}

// Normalizer2QuickCheck encodes the result of Normalizer2.QuickCheck.
type Normalizer2QuickCheck int

const (
	// Normalizer2QuickCheckNo — the string is definitely not normalised.
	Normalizer2QuickCheckNo Normalizer2QuickCheck = iota
	// Normalizer2QuickCheckMaybe — a full normalisation pass is needed.
	Normalizer2QuickCheckMaybe
	// Normalizer2QuickCheckYes — the string is already normalised.
	Normalizer2QuickCheckYes
)

// goTextNormalizer implements Normalizer2 using golang.org/x/text/unicode/norm.
//
// It supports NFKC_CF (NFKC + case fold) and the four standard forms
// (NFC, NFD, NFKC, NFKD).
//
// Deviation: golang.org/x/text does not expose SpanQuickCheckYes or
// boundary predicates at the same granularity as ICU4J's Normalizer2.
// SpanQuickCheckYes is approximated via norm.Form.QuickSpanString; the
// boundary predicates use next-boundary logic from the norm package.
type goTextNormalizer struct {
	form       norm.Form
	caseFold   bool
	caseFolding cases.Caser
}

// NewNFKCCaseFoldNormalizer returns a Normalizer2 that applies NFKC
// normalisation followed by Unicode case-folding, equivalent to ICU4J's
// Normalizer2.getInstance(null, "nfkc_cf", Normalizer2.Mode.COMPOSE).
func NewNFKCCaseFoldNormalizer() Normalizer2 {
	return &goTextNormalizer{
		form:       norm.NFKC,
		caseFold:   true,
		caseFolding: cases.Fold(),
	}
}

// NewNormalizer2 returns a Normalizer2 for the named form and mode.
// Recognised form names: "nfc", "nfd", "nfkc", "nfkd", "nfkc_cf", "nfkc_scf".
func NewNormalizer2(form string, mode NormalizerMode) Normalizer2 {
	switch form {
	case "nfkc_cf", "nfkc_scf":
		// Both nfkc_cf and nfkc_scf include case folding.
		return NewNFKCCaseFoldNormalizer()
	default:
		var f norm.Form
		switch form {
		case "nfd":
			f = norm.NFD
		case "nfkc":
			f = norm.NFKC
		case "nfkd":
			f = norm.NFKD
		default: // "nfc" and unknown
			f = norm.NFC
		}
		if mode == NormalizerModeDecompose {
			switch f {
			case norm.NFC:
				f = norm.NFD
			case norm.NFKC:
				f = norm.NFKD
			}
		}
		return &goTextNormalizer{form: f, caseFold: false}
	}
}

func (n *goTextNormalizer) Normalize(s string) string {
	result := n.form.String(s)
	if n.caseFold {
		result = n.caseFolding.String(result)
	}
	return result
}

func (n *goTextNormalizer) QuickCheck(s string) Normalizer2QuickCheck {
	if n.caseFold {
		// Case-fold normalizers cannot quick-check without doing the work.
		return Normalizer2QuickCheckMaybe
	}
	if n.form.IsNormalString(s) {
		return Normalizer2QuickCheckYes
	}
	return Normalizer2QuickCheckNo
}

func (n *goTextNormalizer) SpanQuickCheckYes(s string) int {
	if n.caseFold {
		// With case folding we cannot confirm any span without applying it.
		return 0
	}
	return n.form.QuickSpanString(s)
}

func (n *goTextNormalizer) HasBoundaryBefore(s string, pos int) bool {
	if pos >= len(s) {
		return true
	}
	// A position is a boundary when the norm package considers the next
	// boundary at or before pos (i.e. the character at pos starts a new
	// normalisation segment).
	next := n.form.NextBoundaryInString(s[pos:], true)
	return next == 0
}

func (n *goTextNormalizer) HasBoundaryAfter(s string, pos int) bool {
	if pos >= len(s) {
		return true
	}
	// After pos: the boundary is at the end of the character at pos.
	sub := s[:pos]
	last := n.form.LastBoundary([]byte(sub))
	return last == len(sub)
}

// FilteredNormalizer2 wraps a Normalizer2 with a Unicode set filter so that
// code points outside the set are passed through unchanged.
//
// Go port of com.ibm.icu.text.FilteredNormalizer2 (Apache Lucene 10.4.0).
//
// Deviation: The Java FilteredNormalizer2 processes the string character by
// character, alternating normalised and unmodified runs. This Go
// implementation provides the same behaviour using a rune-based scan.
type FilteredNormalizer2 struct {
	inner  Normalizer2
	filter UnicodeSet
}

// NewFilteredNormalizer2 wraps inner so that only code points in filter are
// normalised.
func NewFilteredNormalizer2(inner Normalizer2, filter UnicodeSet) *FilteredNormalizer2 {
	return &FilteredNormalizer2{inner: inner, filter: filter}
}

// Normalize applies the inner normalizer only to code points contained in
// the filter; other code points are copied unchanged.
func (f *FilteredNormalizer2) Normalize(s string) string {
	var buf []rune
	var pending []rune
	for _, r := range s {
		if f.filter.ContainsRune(r) {
			pending = append(pending, r)
		} else {
			if len(pending) > 0 {
				buf = append(buf, []rune(f.inner.Normalize(string(pending)))...)
				pending = pending[:0]
			}
			buf = append(buf, r)
		}
	}
	if len(pending) > 0 {
		buf = append(buf, []rune(f.inner.Normalize(string(pending)))...)
	}
	return string(buf)
}

// QuickCheck delegates to the inner normalizer.
func (f *FilteredNormalizer2) QuickCheck(s string) Normalizer2QuickCheck {
	return f.inner.QuickCheck(s)
}

// SpanQuickCheckYes returns 0 for filtered normalizers to force full
// normalisation through Normalize.
func (f *FilteredNormalizer2) SpanQuickCheckYes(_ string) int { return 0 }

// HasBoundaryBefore delegates to the inner normalizer.
func (f *FilteredNormalizer2) HasBoundaryBefore(s string, pos int) bool {
	return f.inner.HasBoundaryBefore(s, pos)
}

// HasBoundaryAfter delegates to the inner normalizer.
func (f *FilteredNormalizer2) HasBoundaryAfter(s string, pos int) bool {
	return f.inner.HasBoundaryAfter(s, pos)
}

// UnicodeSet represents a set of Unicode code points.
//
// Go port of com.ibm.icu.text.UnicodeSet (minimal interface).
//
// Deviation: The Java UnicodeSet is a rich pattern-language capable of
// expressing arbitrary character sets (e.g. "[:Lowercase:]"). This Go
// interface exposes only ContainsRune, which is what FilteredNormalizer2
// needs at runtime.
type UnicodeSet interface {
	// ContainsRune reports whether r is a member of this set.
	ContainsRune(r rune) bool
}

// Ensure compile-time interface satisfaction.
var _ Normalizer2 = (*goTextNormalizer)(nil)
var _ Normalizer2 = (*FilteredNormalizer2)(nil)
