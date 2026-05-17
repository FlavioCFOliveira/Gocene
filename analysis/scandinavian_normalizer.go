// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// ScandinavianFolding identifies a single folding rule consumed by
// ScandinavianNormalizer. The Lucene reference exposes the same set
// as a Java EnumSet.
type ScandinavianFolding int

// Scandinavian folding rules.
const (
	ScandinavianFoldingAA ScandinavianFolding = iota
	ScandinavianFoldingAO
	ScandinavianFoldingAE
	ScandinavianFoldingOE
	ScandinavianFoldingOO
)

// AllScandinavianFoldings is the set of every folding rule.
var AllScandinavianFoldings = []ScandinavianFolding{
	ScandinavianFoldingAA, ScandinavianFoldingAO,
	ScandinavianFoldingAE, ScandinavianFoldingOE, ScandinavianFoldingOO,
}

// Scandinavian Unicode constants used by the normaliser. The names
// (and lower/upper pairs) match the Lucene reference.
const (
	scAA   = 'Å'
	scaa   = 'å'
	scAE   = 'Æ'
	scae   = 'æ'
	scAEse = 'Ä'
	scaese = 'ä'
	scOE   = 'Ø'
	scoe   = 'ø'
	scOEse = 'Ö'
	scoese = 'ö'
)

// ScandinavianNormalizer normalises interchangeable Scandinavian
// characters (æ Æ ä Ä ö Ö ø Ø) and folded variants (aa, ao, ae, oe,
// oo) to the canonical å Å æ Æ ø Ø.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.ScandinavianNormalizer
// from Apache Lucene 10.4.0. The Foldings argument controls which
// double-letter fold rules are active; the reference uses Java
// EnumSet, here we use a slice of ScandinavianFolding for simplicity.
type ScandinavianNormalizer struct {
	foldings map[ScandinavianFolding]struct{}
}

// NewScandinavianNormalizer returns a normaliser configured with the
// given foldings.
func NewScandinavianNormalizer(foldings []ScandinavianFolding) *ScandinavianNormalizer {
	set := make(map[ScandinavianFolding]struct{}, len(foldings))
	for _, f := range foldings {
		set[f] = struct{}{}
	}
	return &ScandinavianNormalizer{foldings: set}
}

func (n *ScandinavianNormalizer) has(f ScandinavianFolding) bool {
	_, ok := n.foldings[f]
	return ok
}

// ProcessToken applies the normalisation rules to runes[:length] and
// returns the new length. The buffer is modified in place where
// possible; double-letter fold deletions shrink the buffer (the
// caller slices it down to the returned value).
func (n *ScandinavianNormalizer) ProcessToken(runes []rune, length int) int {
	for i := 0; i < length; i++ {
		switch runes[i] {
		case scaese:
			runes[i] = scae
		case scAEse:
			runes[i] = scAE
		case scoese:
			runes[i] = scoe
		case scOEse:
			runes[i] = scOE
		}
		if length-1 > i {
			cur := runes[i]
			next := runes[i+1]
			switch cur {
			case 'a':
				if (n.has(ScandinavianFoldingAA) && (next == 'a' || next == 'A')) ||
					(n.has(ScandinavianFoldingAO) && (next == 'o' || next == 'O')) {
					length = runeDelete(runes, i+1, length)
					runes[i] = scaa
				} else if n.has(ScandinavianFoldingAE) && (next == 'e' || next == 'E') {
					length = runeDelete(runes, i+1, length)
					runes[i] = scae
				}
			case 'A':
				if (n.has(ScandinavianFoldingAA) && (next == 'a' || next == 'A')) ||
					(n.has(ScandinavianFoldingAO) && (next == 'o' || next == 'O')) {
					length = runeDelete(runes, i+1, length)
					runes[i] = scAA
				} else if n.has(ScandinavianFoldingAE) && (next == 'e' || next == 'E') {
					length = runeDelete(runes, i+1, length)
					runes[i] = scAE
				}
			case 'o':
				if (n.has(ScandinavianFoldingOE) && (next == 'e' || next == 'E')) ||
					(n.has(ScandinavianFoldingOO) && (next == 'o' || next == 'O')) {
					length = runeDelete(runes, i+1, length)
					runes[i] = scoe
				}
			case 'O':
				if (n.has(ScandinavianFoldingOE) && (next == 'e' || next == 'E')) ||
					(n.has(ScandinavianFoldingOO) && (next == 'o' || next == 'O')) {
					length = runeDelete(runes, i+1, length)
					runes[i] = scOE
				}
			}
		}
	}
	return length
}

// runeDelete shifts runes[pos+1:length] one position left, returning
// the new length. Equivalent to Lucene's StemmerUtil.delete on a
// char[].
func runeDelete(runes []rune, pos, length int) int {
	if pos < 0 || pos >= length {
		return length
	}
	copy(runes[pos:length-1], runes[pos+1:length])
	return length - 1
}

// ScandinavianNormalizationFilter applies ScandinavianNormalizer
// (configured with the full folding set) to every token.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.ScandinavianNormalizationFilter.
type ScandinavianNormalizationFilter struct {
	*BaseTokenFilter

	normalizer *ScandinavianNormalizer
	termAttr   CharTermAttribute
}

// NewScandinavianNormalizationFilter wraps input with the full-fold
// normaliser.
func NewScandinavianNormalizationFilter(input TokenStream) *ScandinavianNormalizationFilter {
	return NewScandinavianNormalizationFilterWithFoldings(input, AllScandinavianFoldings)
}

// NewScandinavianNormalizationFilterWithFoldings wraps input with a
// normaliser configured for the given foldings.
func NewScandinavianNormalizationFilterWithFoldings(input TokenStream, foldings []ScandinavianFolding) *ScandinavianNormalizationFilter {
	f := &ScandinavianNormalizationFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		normalizer:      NewScandinavianNormalizer(foldings),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f
}

// IncrementToken normalises the current token.
func (f *ScandinavianNormalizationFilter) IncrementToken() (bool, error) {
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
	runes := []rune(f.termAttr.String())
	n := f.normalizer.ProcessToken(runes, len(runes))
	res := string(runes[:n])
	if res != f.termAttr.String() {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(res)
	}
	return true, nil
}

// Ensure ScandinavianNormalizationFilter implements TokenFilter.
var _ TokenFilter = (*ScandinavianNormalizationFilter)(nil)

// ScandinavianNormalizationFilterFactory creates instances.
type ScandinavianNormalizationFilterFactory struct{}

// NewScandinavianNormalizationFilterFactory returns a fresh factory.
func NewScandinavianNormalizationFilterFactory() *ScandinavianNormalizationFilterFactory {
	return &ScandinavianNormalizationFilterFactory{}
}

// Create wraps input with the filter.
func (f *ScandinavianNormalizationFilterFactory) Create(input TokenStream) TokenFilter {
	return NewScandinavianNormalizationFilter(input)
}

// Ensure factory satisfies TokenFilterFactory.
var _ TokenFilterFactory = (*ScandinavianNormalizationFilterFactory)(nil)

// ScandinavianFoldingFilter applies the harder normalisation that
// also folds å→a, ä→a, æ→a, ö→o, ø→o and double-letter sequences.
// It is implemented in Lucene as a separate class with a specialised
// IncrementToken; Gocene re-uses ScandinavianNormalizer for the
// double-letter part and adds a small post-pass for the single-letter
// foldings.
//
// This is the Go port of
// org.apache.lucene.analysis.miscellaneous.ScandinavianFoldingFilter.
type ScandinavianFoldingFilter struct {
	*BaseTokenFilter

	normalizer *ScandinavianNormalizer
	termAttr   CharTermAttribute
}

// NewScandinavianFoldingFilter wraps input.
func NewScandinavianFoldingFilter(input TokenStream) *ScandinavianFoldingFilter {
	f := &ScandinavianFoldingFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		normalizer:      NewScandinavianNormalizer(AllScandinavianFoldings),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f
}

// IncrementToken applies double-letter normalisation followed by the
// single-letter aggressive folding.
func (f *ScandinavianFoldingFilter) IncrementToken() (bool, error) {
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
	runes := []rune(f.termAttr.String())
	n := f.normalizer.ProcessToken(runes, len(runes))
	// Fold single-letter variants.
	for i := 0; i < n; i++ {
		switch runes[i] {
		case scaa, scae, scaese:
			runes[i] = 'a'
		case scAA, scAE, scAEse:
			runes[i] = 'A'
		case scoe, scoese:
			runes[i] = 'o'
		case scOE, scOEse:
			runes[i] = 'O'
		}
	}
	res := string(runes[:n])
	if res != f.termAttr.String() {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(res)
	}
	return true, nil
}

// Ensure ScandinavianFoldingFilter implements TokenFilter.
var _ TokenFilter = (*ScandinavianFoldingFilter)(nil)

// ScandinavianFoldingFilterFactory creates instances.
type ScandinavianFoldingFilterFactory struct{}

// NewScandinavianFoldingFilterFactory returns a fresh factory.
func NewScandinavianFoldingFilterFactory() *ScandinavianFoldingFilterFactory {
	return &ScandinavianFoldingFilterFactory{}
}

// Create wraps input with ScandinavianFoldingFilter.
func (f *ScandinavianFoldingFilterFactory) Create(input TokenStream) TokenFilter {
	return NewScandinavianFoldingFilter(input)
}

// Ensure factory satisfies TokenFilterFactory.
var _ TokenFilterFactory = (*ScandinavianFoldingFilterFactory)(nil)

// NorwegianNormalizationFilter applies a Norwegian-specific subset of
// the Scandinavian foldings (AE, OE, AA) on top of the
// single-character mappings.
//
// This is the Go port of
// org.apache.lucene.analysis.no.NorwegianNormalizationFilter.
type NorwegianNormalizationFilter struct {
	*BaseTokenFilter

	normalizer *ScandinavianNormalizer
	termAttr   CharTermAttribute
}

// NewNorwegianNormalizationFilter wraps input.
func NewNorwegianNormalizationFilter(input TokenStream) *NorwegianNormalizationFilter {
	f := &NorwegianNormalizationFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		normalizer: NewScandinavianNormalizer([]ScandinavianFolding{
			ScandinavianFoldingAE, ScandinavianFoldingOE, ScandinavianFoldingAA,
		}),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f
}

// IncrementToken normalises the current token.
func (f *NorwegianNormalizationFilter) IncrementToken() (bool, error) {
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
	runes := []rune(f.termAttr.String())
	n := f.normalizer.ProcessToken(runes, len(runes))
	res := string(runes[:n])
	if res != f.termAttr.String() {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(res)
	}
	return true, nil
}

// Ensure NorwegianNormalizationFilter implements TokenFilter.
var _ TokenFilter = (*NorwegianNormalizationFilter)(nil)

// NorwegianNormalizationFilterFactory creates instances.
type NorwegianNormalizationFilterFactory struct{}

// NewNorwegianNormalizationFilterFactory returns a fresh factory.
func NewNorwegianNormalizationFilterFactory() *NorwegianNormalizationFilterFactory {
	return &NorwegianNormalizationFilterFactory{}
}

// Create wraps input.
func (f *NorwegianNormalizationFilterFactory) Create(input TokenStream) TokenFilter {
	return NewNorwegianNormalizationFilter(input)
}
