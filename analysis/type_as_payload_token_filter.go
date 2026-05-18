// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// TypeAsPayloadTokenFilter copies the value of the TypeAttribute into
// the PayloadAttribute, encoded as UTF-8. Tokens with an empty or nil
// type are passed through unchanged.
//
// This is the Go port of
// org.apache.lucene.analysis.payloads.TypeAsPayloadTokenFilter from
// Apache Lucene 10.4.0.
type TypeAsPayloadTokenFilter struct {
	*BaseTokenFilter

	typeAttr    TypeAttribute
	payloadAttr PayloadAttribute
}

// NewTypeAsPayloadTokenFilter wraps input with the type-to-payload
// filter.
func NewTypeAsPayloadTokenFilter(input TokenStream) *TypeAsPayloadTokenFilter {
	f := &TypeAsPayloadTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(TypeAttributeType); a != nil {
			f.typeAttr = a.(TypeAttribute)
		}
		if a := src.GetAttributeByType(PayloadAttributeType); a != nil {
			f.payloadAttr = a.(PayloadAttribute)
		}
	}
	return f
}

// IncrementToken advances the input and, when the TypeAttribute is
// non-empty, writes its UTF-8 bytes into the PayloadAttribute.
func (f *TypeAsPayloadTokenFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if f.typeAttr != nil && f.payloadAttr != nil {
		if t := f.typeAttr.GetType(); t != "" {
			f.payloadAttr.SetPayload([]byte(t))
		}
	}
	return true, nil
}

// Ensure TypeAsPayloadTokenFilter implements TokenFilter.
var _ TokenFilter = (*TypeAsPayloadTokenFilter)(nil)

// TypeAsPayloadTokenFilterFactory creates TypeAsPayloadTokenFilter
// instances.
type TypeAsPayloadTokenFilterFactory struct{}

// NewTypeAsPayloadTokenFilterFactory returns a fresh factory.
func NewTypeAsPayloadTokenFilterFactory() *TypeAsPayloadTokenFilterFactory {
	return &TypeAsPayloadTokenFilterFactory{}
}

// Create returns a TypeAsPayloadTokenFilter wrapping input.
func (f *TypeAsPayloadTokenFilterFactory) Create(input TokenStream) TokenFilter {
	return NewTypeAsPayloadTokenFilter(input)
}

// Ensure TypeAsPayloadTokenFilterFactory implements
// TokenFilterFactory.
var _ TokenFilterFactory = (*TypeAsPayloadTokenFilterFactory)(nil)
