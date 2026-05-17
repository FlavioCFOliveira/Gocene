// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// NumericPayloadTokenFilter attaches a pre-encoded float payload to
// every token whose TypeAttribute equals the configured type string.
//
// This is the Go port of
// org.apache.lucene.analysis.payloads.NumericPayloadTokenFilter from
// Apache Lucene 10.4.0.
//
// The payload is encoded once at construction time via
// EncodeFloatPayload and re-used for every matching token; the
// reference implementation does the same to avoid per-token
// allocation.
type NumericPayloadTokenFilter struct {
	*BaseTokenFilter

	typeMatch string
	payload   []byte

	payloadAttr *PayloadAttribute
	typeAttr    *TypeAttribute
}

// NewNumericPayloadTokenFilter wraps input with a filter that attaches
// the encoded payload to every token of type typeMatch.
func NewNumericPayloadTokenFilter(input TokenStream, payload float32, typeMatch string) *NumericPayloadTokenFilter {
	f := &NumericPayloadTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		typeMatch:       typeMatch,
		payload:         EncodeFloatPayload(payload),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&PayloadAttribute{})); a != nil {
			f.payloadAttr = a.(*PayloadAttribute)
		}
		if a := src.GetAttributeByType(reflect.TypeOf(&TypeAttribute{})); a != nil {
			f.typeAttr = a.(*TypeAttribute)
		}
	}
	return f
}

// IncrementToken pulls the next token and, when its TypeAttribute
// equals typeMatch, copies the cached payload into the
// PayloadAttribute.
func (f *NumericPayloadTokenFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if f.typeAttr != nil && f.payloadAttr != nil && f.typeAttr.Type == f.typeMatch {
		f.payloadAttr.SetPayload(f.payload)
	}
	return true, nil
}

// Ensure NumericPayloadTokenFilter implements TokenFilter.
var _ TokenFilter = (*NumericPayloadTokenFilter)(nil)

// NumericPayloadTokenFilterFactory creates NumericPayloadTokenFilter
// instances with the configured payload value and type match string.
type NumericPayloadTokenFilterFactory struct {
	payload   float32
	typeMatch string
}

// NewNumericPayloadTokenFilterFactory returns a factory configured
// with the given payload value and type match.
func NewNumericPayloadTokenFilterFactory(payload float32, typeMatch string) *NumericPayloadTokenFilterFactory {
	return &NumericPayloadTokenFilterFactory{
		payload:   payload,
		typeMatch: typeMatch,
	}
}

// Create returns a NumericPayloadTokenFilter wrapping input.
func (f *NumericPayloadTokenFilterFactory) Create(input TokenStream) TokenFilter {
	return NewNumericPayloadTokenFilter(input, f.payload, f.typeMatch)
}

// Ensure NumericPayloadTokenFilterFactory implements
// TokenFilterFactory.
var _ TokenFilterFactory = (*NumericPayloadTokenFilterFactory)(nil)
