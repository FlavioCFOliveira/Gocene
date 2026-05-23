// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package phonetic

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// bmTokenPattern matches individual phonetic tokens from the BM output.
// BM output format: "ab|ac|..." or "(ab|ac)-(da|db)" for compound names.
// This pattern extracts individual phoneme codes, skipping separators.
var bmTokenPattern = regexp.MustCompile(`([^()|-]+)`)

// BeiderMorseFilter is a TokenFilter for Beider-Morse phonetic encoding.
// It encodes each token into one or more Beider-Morse phonetic codes,
// emitting each code as a separate token at position increment 0.
//
// This is the Go port of
// org.apache.lucene.analysis.phonetic.BeiderMorseFilter from
// Apache Lucene 10.4.0.
type BeiderMorseFilter struct {
	*analysis.BaseTokenFilter

	engine    *PhoneticEngine
	languages *LanguageSet

	// encoded holds the current BM output for the current input token.
	encoded string
	// matches holds the individual phoneme tokens extracted from encoded.
	matches []string
	// matchIdx is the index of the next match to emit.
	matchIdx int
	// state is the captured attribute state of the current input token.
	state *util.AttributeState

	termAttr   analysis.CharTermAttribute
	posIncAttr analysis.PositionIncrementAttribute
}

// NewBeiderMorseFilter creates a BeiderMorseFilter that uses the given engine
// with automatic language detection.
func NewBeiderMorseFilter(input analysis.TokenStream, engine *PhoneticEngine) *BeiderMorseFilter {
	return NewBeiderMorseFilterWithLanguages(input, engine, nil)
}

// NewBeiderMorseFilterWithLanguages creates a BeiderMorseFilter with a specific
// language set. If languages is nil, automatic language detection is used.
func NewBeiderMorseFilterWithLanguages(input analysis.TokenStream, engine *PhoneticEngine, languages *LanguageSet) *BeiderMorseFilter {
	f := &BeiderMorseFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		engine:          engine,
		languages:       languages,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
			f.posIncAttr = a.(analysis.PositionIncrementAttribute)
		}
	}
	return f
}

// IncrementToken advances to the next token.
func (f *BeiderMorseFilter) IncrementToken() (bool, error) {
	// Emit remaining buffered phoneme tokens for the current input token.
	if f.matchIdx < len(f.matches) {
		f.GetAttributeSource().RestoreState(f.state)
		if f.termAttr != nil {
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(f.matches[f.matchIdx])
		}
		if f.posIncAttr != nil {
			f.posIncAttr.SetPositionIncrement(0)
		}
		f.matchIdx++
		return true, nil
	}

	// Advance to the next input token.
	ok, err := f.GetInput().IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	var termStr string
	if f.termAttr != nil {
		termStr = f.termAttr.String()
	}

	// Encode the current token.
	var encoded string
	if f.languages != nil {
		encoded = f.engine.EncodeWithLanguages(termStr, f.languages)
	} else {
		encoded = f.engine.Encode(termStr)
	}

	// Capture the current state for restoring on subsequent matched tokens.
	f.state = f.GetAttributeSource().CaptureState()
	f.matches = f.matches[:0]
	f.matchIdx = 0

	if encoded != "" {
		// Extract individual phoneme codes from the BM output.
		// BM output may contain '|', '(', ')' and '-' as structural characters.
		for _, sub := range bmTokenPattern.FindAllString(encoded, -1) {
			if sub != "" {
				f.matches = append(f.matches, sub)
			}
		}
	}

	// If we have matches, emit the first one.
	if len(f.matches) > 0 {
		// Restore state first, then override term.
		f.GetAttributeSource().RestoreState(f.state)
		if f.termAttr != nil {
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(f.matches[0])
		}
		f.matchIdx = 1
		return true, nil
	}

	// No encoding produced: pass through the original token.
	return true, nil
}

// Reset resets the filter state.
func (f *BeiderMorseFilter) Reset() error {
	if r, ok := f.GetInput().(interface{ Reset() error }); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	f.encoded = ""
	f.state = nil
	f.matches = f.matches[:0]
	f.matchIdx = 0
	return nil
}

// Ensure BeiderMorseFilter implements TokenFilter.
var _ analysis.TokenFilter = (*BeiderMorseFilter)(nil)

// BeiderMorseFilterFactory creates BeiderMorseFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.phonetic.BeiderMorseFilterFactory from
// Apache Lucene 10.4.0.
//
// Parameters:
//   - nameType (default="GENERIC"): one of "GENERIC", "SEPHARDIC", "ASHKENAZI".
//   - ruleType (default="APPROX"): one of "APPROX", "EXACT".
//   - concat (default=true): if true, use concatenation for compound names.
//   - languageSet (default="auto"): comma-separated language names, or "auto"
//     for automatic language detection.
type BeiderMorseFilterFactory struct {
	engine      *PhoneticEngine
	languageSet *LanguageSet
}

// NewBeiderMorseFilterFactory creates a factory with default settings.
func NewBeiderMorseFilterFactory() *BeiderMorseFilterFactory {
	return &BeiderMorseFilterFactory{
		engine: NewPhoneticEngine(NameTypeGeneric, RuleTypeApprox, true),
	}
}

// NewBeiderMorseFilterFactoryWithArgs creates a factory from a string argument map.
func NewBeiderMorseFilterFactoryWithArgs(args map[string]string) (*BeiderMorseFilterFactory, error) {
	nameType := NameTypeGeneric
	ruleType := RuleTypeApprox
	concat := true
	var langSet *LanguageSet

	if v, ok := args["nameType"]; ok {
		nameType = ParseNameType(v)
		delete(args, "nameType")
	}
	if v, ok := args["ruleType"]; ok {
		ruleType = ParseRuleType(v)
		delete(args, "ruleType")
	}
	if v, ok := args["concat"]; ok {
		concat = strings.EqualFold(v, "true")
		delete(args, "concat")
	}
	if v, ok := args["languageSet"]; ok {
		if v != "" && !strings.EqualFold(v, "auto") {
			langs := strings.Split(v, ",")
			for i, l := range langs {
				langs[i] = strings.TrimSpace(l)
			}
			langSet = NewLanguageSet(langs)
		}
		delete(args, "languageSet")
	}

	if len(args) > 0 {
		return nil, fmt.Errorf("unknown parameters: %v", args)
	}

	return &BeiderMorseFilterFactory{
		engine:      NewPhoneticEngine(nameType, ruleType, concat),
		languageSet: langSet,
	}, nil
}

// Create creates a BeiderMorseFilter wrapping input.
func (f *BeiderMorseFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewBeiderMorseFilterWithLanguages(input, f.engine, f.languageSet)
}

// Ensure BeiderMorseFilterFactory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*BeiderMorseFilterFactory)(nil)
