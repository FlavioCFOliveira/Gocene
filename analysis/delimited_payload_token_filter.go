// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	"reflect"
)

// DefaultPayloadDelimiter is the default delimiter byte separating the
// token from its payload in DelimitedPayloadTokenFilter input. The
// Lucene reference uses the character literal '|'.
const DefaultPayloadDelimiter byte = '|'

// DelimitedPayloadTokenFilter splits each incoming token at the first
// occurrence of the configured delimiter byte. Bytes before the
// delimiter remain as the token text; bytes after are passed through
// the configured PayloadEncoder and attached as the token's payload.
//
// This is the Go port of
// org.apache.lucene.analysis.payloads.DelimitedPayloadTokenFilter from
// Apache Lucene 10.4.0.
//
// Deviation from Lucene: the reference uses 'char' (UTF-16 code unit)
// as the delimiter type. Gocene's CharTermAttribute exposes its buffer
// as UTF-8 bytes, so the delimiter is a single byte. Callers should
// pick an ASCII delimiter (the Lucene default '|' is 0x7C, safely
// ASCII); multi-byte delimiters are out of scope and would require a
// boundary-aware scanner.
//
// Tokens that do not contain the delimiter are emitted with their
// payload cleared (set to nil), matching the Lucene contract.
type DelimitedPayloadTokenFilter struct {
	*BaseTokenFilter

	delimiter byte
	encoder   PayloadEncoder

	termAttr    CharTermAttribute
	payloadAttr PayloadAttribute
}

// NewDelimitedPayloadTokenFilter wraps input with a filter that splits
// tokens on delimiter and encodes the trailing bytes via encoder.
// Both delimiter and encoder are required; passing a nil encoder is
// invalid and panics on construction to match Lucene's
// Objects.requireNonNull(encoder).
func NewDelimitedPayloadTokenFilter(input TokenStream, delimiter byte, encoder PayloadEncoder) *DelimitedPayloadTokenFilter {
	if encoder == nil {
		panic(fmt.Sprintf("DelimitedPayloadTokenFilter: encoder must not be nil"))
	}
	f := &DelimitedPayloadTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		delimiter:       delimiter,
		encoder:         encoder,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
		if a := src.GetAttributeByType(PayloadAttributeType); a != nil {
			f.payloadAttr = a.(PayloadAttribute)
		}
	}
	return f
}

// IncrementToken pulls the next token from the input, splits at the
// first delimiter byte (if present), and updates the payload
// attribute accordingly.
func (f *DelimitedPayloadTokenFilter) IncrementToken() (bool, error) {
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
	buf := f.termAttr.Buffer()
	length := f.termAttr.Length()
	for i := 0; i < length; i++ {
		if buf[i] == f.delimiter {
			payload := f.encoder.EncodeSlice(buf, i+1, length-(i+1))
			if f.payloadAttr != nil {
				f.payloadAttr.SetPayload(payload)
			}
			f.termAttr.SetLength(i)
			return true, nil
		}
	}
	// No delimiter seen: clear any prior payload to match Lucene
	// behaviour (the reference filter sets the payload to null).
	if f.payloadAttr != nil {
		f.payloadAttr.SetPayload(nil)
	}
	return true, nil
}

// Ensure DelimitedPayloadTokenFilter implements TokenFilter.
var _ TokenFilter = (*DelimitedPayloadTokenFilter)(nil)

// DelimitedPayloadTokenFilterFactory creates
// DelimitedPayloadTokenFilter instances configured with a delimiter
// byte and a PayloadEncoder. The Lucene reference factory looks up
// encoders by class name from an SPI registry; Gocene has no analyser
// SPI yet, so the encoder is injected directly.
type DelimitedPayloadTokenFilterFactory struct {
	delimiter byte
	encoder   PayloadEncoder
}

// NewDelimitedPayloadTokenFilterFactory returns a factory configured
// with the default delimiter '|' and an IdentityPayloadEncoder.
func NewDelimitedPayloadTokenFilterFactory() *DelimitedPayloadTokenFilterFactory {
	return &DelimitedPayloadTokenFilterFactory{
		delimiter: DefaultPayloadDelimiter,
		encoder:   NewIdentityPayloadEncoder(),
	}
}

// NewDelimitedPayloadTokenFilterFactoryWithConfig returns a factory
// configured with the given delimiter and encoder. A nil encoder
// triggers the same panic as the filter constructor.
func NewDelimitedPayloadTokenFilterFactoryWithConfig(delimiter byte, encoder PayloadEncoder) *DelimitedPayloadTokenFilterFactory {
	if encoder == nil {
		panic("DelimitedPayloadTokenFilterFactory: encoder must not be nil")
	}
	return &DelimitedPayloadTokenFilterFactory{
		delimiter: delimiter,
		encoder:   encoder,
	}
}

// Create returns a DelimitedPayloadTokenFilter wrapping input.
func (f *DelimitedPayloadTokenFilterFactory) Create(input TokenStream) TokenFilter {
	return NewDelimitedPayloadTokenFilter(input, f.delimiter, f.encoder)
}

// Ensure DelimitedPayloadTokenFilterFactory implements
// TokenFilterFactory.
var _ TokenFilterFactory = (*DelimitedPayloadTokenFilterFactory)(nil)
