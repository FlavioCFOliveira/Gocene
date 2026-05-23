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

// Encoder is the interface for phonetic encoding algorithms.
//
// This mirrors the org.apache.commons.codec.Encoder interface used in
// Apache Commons Codec 1.17.2.
type Encoder interface {
	// Encode encodes the given string and returns the phonetic representation.
	Encode(value string) string
}

// PhoneticFilter creates tokens for phonetic matches using the specified encoder.
//
// This is the Go port of
// org.apache.lucene.analysis.phonetic.PhoneticFilter from Apache Lucene 10.4.0.
//
// When inject is true, the encoded token is added as a synonym with position
// increment 0. When inject is false, the original token is replaced by its
// encoded form.
type PhoneticFilter struct {
	*analysis.BaseTokenFilter

	inject  bool
	encoder Encoder
	// save holds the captured state of the original token when inject=true
	// and an encoding was produced.
	save *util.AttributeState

	termAttr   analysis.CharTermAttribute
	posIncAttr analysis.PositionIncrementAttribute
}

// NewPhoneticFilter creates a PhoneticFilter with the specified encoder and
// inject mode.
func NewPhoneticFilter(input analysis.TokenStream, encoder Encoder, inject bool) *PhoneticFilter {
	f := &PhoneticFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		encoder:         encoder,
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
func (f *PhoneticFilter) IncrementToken() (bool, error) {
	if f.save != nil {
		// Restore the buffered original token and emit it.
		f.GetAttributeSource().RestoreState(f.save)
		f.save = nil
		return true, nil
	}

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

	// Encode the term.
	phonetic := f.encoder.Encode(termStr)
	// If the encoding is empty or identical to the original, pass through.
	if phonetic == "" || phonetic == termStr {
		return true, nil
	}

	if !f.inject {
		// Replace the original token with the phonetic form.
		if f.termAttr != nil {
			f.termAttr.SetEmpty()
			f.termAttr.AppendString(phonetic)
		}
		return true, nil
	}

	// inject=true: return the phonetic code first (with the original position
	// increment), then return the original token (with posInc=0).
	//
	// Lucene Java PhoneticFilter emits the phonetic token first to avoid
	// capturing and restoring state for the common case where inject=true.
	origOffset := 0
	if f.posIncAttr != nil {
		origOffset = f.posIncAttr.GetPositionIncrement()
	}
	// Zero out posInc on the current state (which we'll save as the original).
	if f.posIncAttr != nil {
		f.posIncAttr.SetPositionIncrement(0)
	}
	f.save = f.GetAttributeSource().CaptureState()

	// Set posInc back to original for the phonetic token.
	if f.posIncAttr != nil {
		f.posIncAttr.SetPositionIncrement(origOffset)
	}
	// Set term to phonetic value.
	if f.termAttr != nil {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(phonetic)
	}
	return true, nil
}

// Reset resets the filter state.
func (f *PhoneticFilter) Reset() error {
	if r, ok := f.GetInput().(interface{ Reset() error }); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	f.save = nil
	return nil
}

// Ensure PhoneticFilter implements TokenFilter.
var _ analysis.TokenFilter = (*PhoneticFilter)(nil)

// PhoneticFilterFactory creates PhoneticFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.phonetic.PhoneticFilterFactory from
// Apache Lucene 10.4.0.
//
// Parameters:
//   - encoder (required): one of "DoubleMetaphone", "Metaphone", "Soundex",
//     "RefinedSoundex", "Caverphone" (v2.0), "ColognePhonetic", or "Nysiis"
//     (case-insensitive).
//   - inject (default=true): if true, add encoded tokens as synonyms.
//   - maxCodeLength: restricts the length of the phonetic code (only for
//     encoders that support it).
//
// Deviation from Lucene: the Java factory uses ResourceLoader and reflection
// to support arbitrary org.apache.commons.codec.Encoder implementations. The
// Go factory supports only the built-in encoders listed above.
type PhoneticFilterFactory struct {
	inject        bool
	encoderName   string
	maxCodeLength *int
}

// NewPhoneticFilterFactory creates a factory for the given encoder name with
// inject=true and no maxCodeLength restriction.
func NewPhoneticFilterFactory(encoderName string) *PhoneticFilterFactory {
	return &PhoneticFilterFactory{inject: true, encoderName: encoderName}
}

// NewPhoneticFilterFactoryWithArgs creates a factory from a string argument map.
func NewPhoneticFilterFactoryWithArgs(args map[string]string) (*PhoneticFilterFactory, error) {
	f := &PhoneticFilterFactory{inject: true}
	name, ok := args["encoder"]
	if !ok {
		return nil, fmt.Errorf("required parameter 'encoder' is missing")
	}
	f.encoderName = name
	delete(args, "encoder")

	if v, ok := args["inject"]; ok {
		f.inject = strings.EqualFold(v, "true")
		delete(args, "inject")
	}
	if v, ok := args["maxCodeLength"]; ok {
		var n int
		if _, err := fmt.Sscan(v, &n); err != nil {
			return nil, fmt.Errorf("invalid maxCodeLength %q: %w", v, err)
		}
		f.maxCodeLength = &n
		delete(args, "maxCodeLength")
	}
	if len(args) > 0 {
		return nil, fmt.Errorf("unknown parameters: %v", args)
	}
	return f, nil
}

// getEncoder builds and returns the configured Encoder.
func (f *PhoneticFilterFactory) getEncoder() (Encoder, error) {
	switch strings.ToUpper(f.encoderName) {
	case "DOUBLEMETAPHONE":
		enc := NewDoubleMetaphone()
		if f.maxCodeLength != nil {
			enc.MaxCodeLen = *f.maxCodeLength
		}
		return enc, nil
	case "METAPHONE":
		enc := NewMetaphone()
		if f.maxCodeLength != nil {
			enc.MaxCodeLen = *f.maxCodeLength
		}
		return enc, nil
	case "SOUNDEX":
		return NewSoundex(), nil
	case "REFINEDSOUNDEX":
		return NewRefinedSoundex(), nil
	case "CAVERPHONE":
		return NewCaverphone2(), nil
	case "COLOGNEPHONETIC":
		return NewColognePhonetic(), nil
	case "NYSIIS":
		return NewNysiis(), nil
	default:
		return nil, fmt.Errorf("unknown encoder %q", f.encoderName)
	}
}

// Create creates a PhoneticFilter wrapping input.
func (f *PhoneticFilterFactory) Create(input analysis.TokenStream) analysis.TokenFilter {
	enc, err := f.getEncoder()
	if err != nil {
		panic(err)
	}
	return NewPhoneticFilter(input, enc, f.inject)
}

// Ensure PhoneticFilterFactory implements TokenFilterFactory.
var _ analysis.TokenFilterFactory = (*PhoneticFilterFactory)(nil)
