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

// DoubleMetaphoneFilter produces tokens for phonetic matches using the
// Double Metaphone algorithm. It supports secondary (alternate) codes and
// can operate in inject mode (adding encoded forms as synonyms) or replace
// mode (replacing the original token with its encoded forms).
//
// This is the Go port of
// org.apache.lucene.analysis.phonetic.DoubleMetaphoneFilter from
// Apache Lucene 10.4.0.
type DoubleMetaphoneFilter struct {
	*analysis.BaseTokenFilter

	encoder         *DoubleMetaphone
	inject          bool
	remainingTokens []*util.AttributeState

	termAttr   analysis.CharTermAttribute
	posIncAttr analysis.PositionIncrementAttribute
}

// NewDoubleMetaphoneFilter creates a DoubleMetaphoneFilter with the specified
// maximum code length and inject mode.
func NewDoubleMetaphoneFilter(input analysis.TokenStream, maxCodeLength int, inject bool) *DoubleMetaphoneFilter {
	if maxCodeLength < 1 {
		panic(fmt.Sprintf("maxCodeLength must be >=1, got %d", maxCodeLength))
	}
	f := &DoubleMetaphoneFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		encoder:         &DoubleMetaphone{MaxCodeLen: maxCodeLength},
		inject:          inject,
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
func (f *DoubleMetaphoneFilter) IncrementToken() (bool, error) {
	for {
		// Emit any buffered tokens from the previous input token.
		if len(f.remainingTokens) > 0 {
			f.GetAttributeSource().RestoreState(f.remainingTokens[0])
			f.remainingTokens = f.remainingTokens[1:]
			return true, nil
		}

		// Advance input.
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

		var termStr string
		if f.termAttr != nil {
			termStr = f.termAttr.String()
		}

		firstAlternativeIncrement := 0
		if f.inject {
			firstAlternativeIncrement = 0
		} else {
			if f.posIncAttr != nil {
				firstAlternativeIncrement = f.posIncAttr.GetPositionIncrement()
			}
		}

		primary, alternate := f.encoder.DoubleMetaphoneValue(termStr)

		// Determine which codes to emit.
		saveState := f.inject

		// Emit primary code if different from the original term.
		if primary != "" && primary != termStr {
			if saveState {
				f.remainingTokens = append(f.remainingTokens, f.GetAttributeSource().CaptureState())
			}
			if f.posIncAttr != nil {
				f.posIncAttr.SetPositionIncrement(firstAlternativeIncrement)
			}
			firstAlternativeIncrement = 0
			if f.termAttr != nil {
				f.termAttr.SetEmpty()
				f.termAttr.AppendString(primary)
			}
			saveState = true
		}

		// Emit alternate code if different from primary and original.
		if alternate != "" && alternate != primary && primary != termStr {
			if saveState {
				f.remainingTokens = append(f.remainingTokens, f.GetAttributeSource().CaptureState())
				saveState = false // consumed
			}
			if f.posIncAttr != nil {
				f.posIncAttr.SetPositionIncrement(firstAlternativeIncrement)
			}
			if f.termAttr != nil {
				f.termAttr.SetEmpty()
				f.termAttr.AppendString(alternate)
			}
			saveState = true
		}

		// Just one token to return (no buffered alternatives).
		if len(f.remainingTokens) == 0 {
			return true, nil
		}

		if saveState {
			f.remainingTokens = append(f.remainingTokens, f.GetAttributeSource().CaptureState())
		}
	}
}

// Reset resets the filter state.
func (f *DoubleMetaphoneFilter) Reset() error {
	if r, ok := f.GetInput().(interface{ Reset() error }); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	f.remainingTokens = f.remainingTokens[:0]
	return nil
}

// Ensure DoubleMetaphoneFilter implements TokenFilter.
var _ analysis.TokenFilter = (*DoubleMetaphoneFilter)(nil)

// DoubleMetaphoneFilterFactory creates DoubleMetaphoneFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.phonetic.DoubleMetaphoneFilterFactory from
// Apache Lucene 10.4.0.
//
// Parameters:
//   - inject (default=true): if true, add encoded tokens as synonyms.
//   - maxCodeLength (default=4): restricts the length of the phonetic code.
type DoubleMetaphoneFilterFactory struct {
	inject        bool
	maxCodeLength int
}

// DefaultMaxCodeLength is the default maximum code length for DoubleMetaphone.
const DefaultMaxCodeLength = 4

// NewDoubleMetaphoneFilterFactory creates a factory with default settings
// (inject=true, maxCodeLength=4).
func NewDoubleMetaphoneFilterFactory() *DoubleMetaphoneFilterFactory {
	return &DoubleMetaphoneFilterFactory{inject: true, maxCodeLength: DefaultMaxCodeLength}
}

// NewDoubleMetaphoneFilterFactoryWithArgs creates a factory from a string argument map.
func NewDoubleMetaphoneFilterFactoryWithArgs(args map[string]string) (*DoubleMetaphoneFilterFactory, error) {
	f := &DoubleMetaphoneFilterFactory{inject: true, maxCodeLength: DefaultMaxCodeLength}
	if v, ok := args["inject"]; ok {
		f.inject = strings.EqualFold(v, "true")
		delete(args, "inject")
	}
	if v, ok := args["maxCodeLength"]; ok {
		var n int
		if _, err := fmt.Sscan(v, &n); err != nil {
			return nil, fmt.Errorf("invalid maxCodeLength %q: %w", v, err)
		}
		f.maxCodeLength = n
		delete(args, "maxCodeLength")
	}
	if len(args) > 0 {
		return nil, fmt.Errorf("unknown parameters: %v", args)
	}
	return f, nil
}

// Create creates a DoubleMetaphoneFilter wrapping input.
func (f *DoubleMetaphoneFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	return NewDoubleMetaphoneFilter(input, f.maxCodeLength, f.inject)
}

// Ensure DoubleMetaphoneFilterFactory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*DoubleMetaphoneFilterFactory)(nil)
