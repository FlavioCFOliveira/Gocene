// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// TokenOffsetPayloadTokenFilter encodes the current token's
// start/end offsets into an 8-byte payload: bytes [0:4) are the start
// offset and bytes [4:8) are the end offset, each big-endian int32.
//
// This is the Go port of
// org.apache.lucene.analysis.payloads.TokenOffsetPayloadTokenFilter
// from Apache Lucene 10.4.0.
type TokenOffsetPayloadTokenFilter struct {
	*BaseTokenFilter

	offsetAttr  OffsetAttribute
	payloadAttr PayloadAttribute
}

// NewTokenOffsetPayloadTokenFilter wraps input with the offset-to-payload
// filter.
func NewTokenOffsetPayloadTokenFilter(input TokenStream) *TokenOffsetPayloadTokenFilter {
	f := &TokenOffsetPayloadTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(OffsetAttributeType); a != nil {
			f.offsetAttr = a.(OffsetAttribute)
		}
		if a := src.GetAttribute(PayloadAttributeType); a != nil {
			f.payloadAttr = a.(PayloadAttribute)
		}
	}
	return f
}

// IncrementToken advances the input and overwrites the PayloadAttribute
// with the 8-byte offset payload.
func (f *TokenOffsetPayloadTokenFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if f.offsetAttr != nil && f.payloadAttr != nil {
		data := make([]byte, 8)
		EncodeIntPayloadInto(int32(f.offsetAttr.StartOffset()), data, 0)
		EncodeIntPayloadInto(int32(f.offsetAttr.EndOffset()), data, 4)
		f.payloadAttr.SetPayload(data)
	}
	return true, nil
}

// Ensure TokenOffsetPayloadTokenFilter implements TokenFilter.
var _ TokenFilter = (*TokenOffsetPayloadTokenFilter)(nil)

// TokenOffsetPayloadTokenFilterFactory creates
// TokenOffsetPayloadTokenFilter instances.
type TokenOffsetPayloadTokenFilterFactory struct{}

// NewTokenOffsetPayloadTokenFilterFactory returns a fresh factory.
func NewTokenOffsetPayloadTokenFilterFactory() *TokenOffsetPayloadTokenFilterFactory {
	return &TokenOffsetPayloadTokenFilterFactory{}
}

// Create returns a TokenOffsetPayloadTokenFilter wrapping input.
func (f *TokenOffsetPayloadTokenFilterFactory) Create(input TokenStream) TokenFilter {
	return NewTokenOffsetPayloadTokenFilter(input)
}

// Ensure TokenOffsetPayloadTokenFilterFactory implements
// TokenFilterFactory.
var _ TokenFilterFactory = (*TokenOffsetPayloadTokenFilterFactory)(nil)
