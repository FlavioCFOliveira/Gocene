// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package phonetic

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DaitchMokotoffSoundexFilter creates tokens for phonetic matches based on
// Daitch–Mokotoff Soundex.
//
// This is the Go port of
// org.apache.lucene.analysis.phonetic.DaitchMokotoffSoundexFilter from
// Apache Lucene 10.4.0.
//
// When inject is true, the encoded token(s) are added as synonyms alongside
// the original token. When inject is false, the original token is replaced
// by the encoded tokens.
type DaitchMokotoffSoundexFilter struct {
	*analysis.BaseTokenFilter

	inject  bool
	encoder *DaitchMokotoffSoundex

	// encoded holds the current pipe-separated codes string.
	encoded string
	// state holds the captured attribute state for the current input token.
	state *util.AttributeState
	// matches is the list of code strings yet to emit for the current token.
	matches []string
	// matchIdx is the index of the next match to emit.
	matchIdx int

	termAttr   analysis.CharTermAttribute
	posIncAttr analysis.PositionIncrementAttribute
}

// NewDaitchMokotoffSoundexFilter creates a DaitchMokotoffSoundexFilter.
// When inject is true, encoded forms are added as synonyms; otherwise they
// replace the original token.
func NewDaitchMokotoffSoundexFilter(input analysis.TokenStream, inject bool) *DaitchMokotoffSoundexFilter {
	f := &DaitchMokotoffSoundexFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		inject:          inject,
		encoder:         &DaitchMokotoffSoundex{},
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
func (f *DaitchMokotoffSoundexFilter) IncrementToken() (bool, error) {
	// Emit remaining buffered codes for the current token.
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

	// Pass through zero-length terms.
	if f.termAttr != nil && f.termAttr.Length() == 0 {
		return true, nil
	}

	// Encode the current term.
	var termStr string
	if f.termAttr != nil {
		termStr = f.termAttr.String()
	}
	f.encoded = f.encoder.Soundex(termStr)
	f.state = f.GetAttributeSource().CaptureState()
	f.matches = f.matches[:0]
	f.matchIdx = 0

	if f.encoded != "" {
		codes := splitDMSCodes(f.encoded)
		f.matches = append(f.matches, codes...)
	}

	if !f.inject {
		// Replace the original token with the first encoded form.
		if len(f.matches) > 0 {
			first := f.matches[0]
			if f.termAttr != nil {
				f.termAttr.SetEmpty()
				f.termAttr.AppendString(first)
			}
			f.matches = f.matches[1:]
		}
		return true, nil
	}

	// inject=true: keep the original token, emit encoded forms afterwards.
	// Re-capture state after the original term is set.
	if len(f.matches) > 0 {
		f.state = f.GetAttributeSource().CaptureState()
	}
	return true, nil
}

// splitDMSCodes splits a pipe-separated DM Soundex code string into individual codes.
func splitDMSCodes(encoded string) []string {
	parts := strings.Split(encoded, "|")
	var result []string
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// Reset resets the filter state.
func (f *DaitchMokotoffSoundexFilter) Reset() error {
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

// Ensure DaitchMokotoffSoundexFilter implements TokenFilter.
var _ analysis.TokenFilter = (*DaitchMokotoffSoundexFilter)(nil)

// DaitchMokotoffSoundexFilterFactory creates DaitchMokotoffSoundexFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.phonetic.DaitchMokotoffSoundexFilterFactory from
// Apache Lucene 10.4.0.
//
// Parameters:
//   - inject (default=true): if true, add encoded tokens as synonyms alongside
//     the original token; if false, replace the original token.
type DaitchMokotoffSoundexFilterFactory struct {
	inject bool
}

// NewDaitchMokotoffSoundexFilterFactory creates a factory with inject=true.
func NewDaitchMokotoffSoundexFilterFactory() *DaitchMokotoffSoundexFilterFactory {
	return &DaitchMokotoffSoundexFilterFactory{inject: true}
}

// NewDaitchMokotoffSoundexFilterFactoryWithArgs creates a factory from a string
// argument map (matching Java's Map<String, String> args convention).
func NewDaitchMokotoffSoundexFilterFactoryWithArgs(args map[string]string) (*DaitchMokotoffSoundexFilterFactory, error) {
	inject := true
	if v, ok := args["inject"]; ok {
		inject = strings.EqualFold(v, "true")
		delete(args, "inject")
	}
	if len(args) > 0 {
		return nil, fmt.Errorf("unknown parameters: %v", args)
	}
	return &DaitchMokotoffSoundexFilterFactory{inject: inject}, nil
}

// Inject returns the inject configuration.
func (f *DaitchMokotoffSoundexFilterFactory) Inject() bool { return f.inject }

// Create creates a DaitchMokotoffSoundexFilter wrapping input.
func (f *DaitchMokotoffSoundexFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewDaitchMokotoffSoundexFilter(input, f.inject)
}

// Ensure DaitchMokotoffSoundexFilterFactory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*DaitchMokotoffSoundexFilterFactory)(nil)
