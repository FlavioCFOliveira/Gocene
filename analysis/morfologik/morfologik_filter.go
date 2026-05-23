// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/morfologik/src/java/org/apache/lucene/analysis/morfologik/MorfologikFilter.java

package morfologik

import (
	"strings"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// WordData holds a single stemming result returned by [IStemmer.Lookup].
//
// This is the Go equivalent of morfologik.stemming.WordData. Each instance
// carries the surface-form stem and its morphosyntactic tag string.
type WordData struct {
	// Stem is the lemma (base form) for the input surface form.
	Stem string
	// Tag is the morphosyntactic annotation string, or "" when no tag is
	// available. Multiple annotation variants are joined with "+" or "|"
	// following the Morfologik convention.
	Tag string
}

// IStemmer is the Go equivalent of morfologik.stemming.IStemmer.
//
// Implementations perform dictionary-backed stem lookup for a single
// surface form and return all matching [WordData] entries (one per
// lemma candidate). Returning nil or an empty slice means the surface
// form has no dictionary entry.
type IStemmer interface {
	Lookup(token string) []WordData
}

// lemmaSplitter splits a Morfologik tag string on the "+" or "|" separator
// characters, matching the Java Pattern.compile("\\+|\\|") used in the
// reference implementation.
var lemmaSplitterChars = func(r rune) bool {
	return r == '+' || r == '|'
}

// MorfologikFilter transforms input tokens into their dictionary lemma(s) and
// provides morphosyntactic (POS) annotations via [MorphosyntacticTagsAttribute].
//
// For each input token the filter performs a dictionary lookup using the
// provided [IStemmer]. If the lookup succeeds, the filter emits one token per
// lemma candidate (at position increment 0 for candidates after the first).
// If the lookup fails it emits the original token with an empty tag list.
// Case-folded lookup is attempted as a fallback when the original form
// produces no results.
//
// This is the Go port of
// org.apache.lucene.analysis.morfologik.MorfologikFilter
// (Apache Lucene 10.4.0).
type MorfologikFilter struct {
	*analysis.BaseTokenFilter

	termAttr    analysis.CharTermAttribute
	tagsAttr    MorphosyntacticTagsAttribute
	posIncrAttr analysis.PositionIncrementAttribute
	keywordAttr analysis.KeywordAttribute

	stemmer IStemmer

	// lemmaList is the current batch of dictionary results.
	lemmaList []WordData
	// lemmaListIndex is the cursor into lemmaList.
	lemmaListIndex int
	// tagsList is a reusable slice of builders for tag parsing.
	tagsList []strings.Builder

	// current is the captured attribute state for multi-lemma expansion.
	current *util.AttributeState
}

// NewMorfologikFilter creates a filter using the provided [IStemmer].
//
// Parameters:
//   - in:  the upstream token stream.
//   - stemmer: an IStemmer that will be consulted for each token. Must not be nil.
func NewMorfologikFilter(in analysis.TokenStream, stemmer IStemmer) *MorfologikFilter {
	if stemmer == nil {
		panic("MorfologikFilter: stemmer must not be nil")
	}
	f := &MorfologikFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(in),
		stemmer:         stemmer,
		lemmaList:       nil,
	}

	// Register all required attributes with the shared AttributeSource.
	src := f.GetAttributeSource()

	termImpl := analysis.NewCharTermAttributeImpl()
	src.AddAttributeImpl(termImpl)
	if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
		f.termAttr = a.(analysis.CharTermAttribute)
	}

	tagsImpl := NewMorphosyntacticTagsAttributeImpl()
	src.AddAttributeImpl(tagsImpl)
	if a := src.GetAttribute(MorphosyntacticTagsAttributeType); a != nil {
		f.tagsAttr = a.(MorphosyntacticTagsAttribute)
	}

	posImpl := analysis.NewPositionIncrementAttributeImpl()
	src.AddAttributeImpl(posImpl)
	if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
		f.posIncrAttr = a.(analysis.PositionIncrementAttribute)
	}

	kwImpl := analysis.NewKeywordAttributeImpl()
	src.AddAttributeImpl(kwImpl)
	if a := src.GetAttribute(analysis.KeywordAttributeType); a != nil {
		f.keywordAttr = a.(analysis.KeywordAttribute)
	}

	return f
}

// popNextLemma advances lemmaListIndex and sets termAttr / tagsAttr to the
// values of the next lemma. Mirrors the private Java method of the same name.
func (f *MorfologikFilter) popNextLemma() {
	wd := f.lemmaList[f.lemmaListIndex]
	f.lemmaListIndex++

	if f.termAttr != nil {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(wd.Stem)
	}

	if f.tagsAttr == nil {
		return
	}

	if wd.Tag == "" {
		f.tagsAttr.SetTags([]strings.Builder{})
		return
	}

	parts := strings.FieldsFunc(wd.Tag, lemmaSplitterChars)
	// Grow tagsList to hold all parts, reusing existing builders.
	for len(f.tagsList) < len(parts) {
		f.tagsList = append(f.tagsList, strings.Builder{})
	}
	for i, p := range parts {
		f.tagsList[i].Reset()
		f.tagsList[i].WriteString(p)
	}
	f.tagsAttr.SetTags(f.tagsList[:len(parts)])
}

// lookupSurfaceForm queries the stemmer for token and updates lemmaList /
// lemmaListIndex. Returns true when at least one result was found.
func (f *MorfologikFilter) lookupSurfaceForm(token string) bool {
	f.lemmaList = f.stemmer.Lookup(token)
	f.lemmaListIndex = 0
	return len(f.lemmaList) > 0
}

// toLowercase returns the unicode-lowercased form of src.
func toLowercase(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRuneInString(src[i:])
		b.WriteRune([]rune(strings.ToLower(string(r)))[0])
		i += size
	}
	return b.String()
}

// IncrementToken advances to the next token. When the current lemma batch
// is not exhausted it re-emits the captured state with the next lemma.
// Otherwise it reads the next input token.
func (f *MorfologikFilter) IncrementToken() (bool, error) {
	if f.lemmaListIndex < len(f.lemmaList) {
		// Still draining the current lemma batch.
		f.GetAttributeSource().RestoreState(f.current)
		if f.posIncrAttr != nil {
			f.posIncrAttr.SetPositionIncrement(0)
		}
		f.popNextLemma()
		return true, nil
	}

	// Consume the next input token.
	ok, err := f.GetInput().IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	// If keyword or no lookup results, pass through unchanged.
	if f.keywordAttr != nil && f.keywordAttr.IsKeywordToken() {
		if f.tagsAttr != nil {
			f.tagsAttr.Clear()
		}
		return true, nil
	}

	term := ""
	if f.termAttr != nil {
		term = f.termAttr.String()
	}

	if f.lookupSurfaceForm(term) || f.lookupSurfaceForm(toLowercase(term)) {
		f.current = f.GetAttributeSource().CaptureState()
		f.popNextLemma()
	} else {
		if f.tagsAttr != nil {
			f.tagsAttr.Clear()
		}
	}
	return true, nil
}

// Reset clears the lemma accumulator state. Existing Gocene filters do not
// propagate Reset to the upstream tokenizer (the caller controls the full
// pipeline reset), so this method only resets MorfologikFilter's own fields.
func (f *MorfologikFilter) Reset() error {
	f.lemmaListIndex = 0
	f.lemmaList = nil
	f.tagsList = f.tagsList[:0]
	f.current = nil
	return nil
}

// Ensure MorfologikFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*MorfologikFilter)(nil)
