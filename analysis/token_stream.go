// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis/api"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TokenStream is a type alias for api.TokenStream.
type TokenStream = api.TokenStream

// BaseTokenStream provides a base implementation for TokenStream.
//
// Embed this struct in concrete TokenStream implementations to inherit
// common functionality.
type BaseTokenStream struct {
	// attributes holds the attribute source for this token stream
	attributes *util.AttributeSource
}

// NewBaseTokenStream creates a new BaseTokenStream backed by a fresh
// [util.AttributeSource] using [util.DefaultAttributeFactoryInstance].
func NewBaseTokenStream() *BaseTokenStream {
	return NewBaseTokenStreamWithFactory(util.DefaultAttributeFactoryInstance)
}

// NewBaseTokenStreamWithFactory creates a new BaseTokenStream backed by a
// fresh [util.AttributeSource] using the supplied [util.AttributeFactory].
func NewBaseTokenStreamWithFactory(factory util.AttributeFactory) *BaseTokenStream {
	if factory == nil {
		panic("BaseTokenStream factory must not be nil")
	}
	return &BaseTokenStream{
		attributes: util.NewAttributeSourceWithFactory(factory),
	}
}

// GetAttributeSource returns the underlying [util.AttributeSource].
func (ts *BaseTokenStream) GetAttributeSource() *util.AttributeSource {
	return ts.attributes
}

// AddAttribute registers an [util.AttributeImpl] with the token
// stream's AttributeSource.
func (ts *BaseTokenStream) AddAttribute(attr util.AttributeImpl) {
	if attr == nil {
		return
	}
	ts.attributes.AddAttributeImpl(attr)
}

// GetAttribute is a legacy back-compat shim that resolves name through
// the [canonicalAttributeInterfaces] registry to a typed
// [util.AttributeSource.GetAttribute] lookup.
func (ts *BaseTokenStream) GetAttribute(name string) util.AttributeImpl {
	if t, ok := canonicalAttributeInterfaces[name]; ok {
		return ts.attributes.GetAttribute(t)
	}
	lower := strings.ToLower(name)
	for n, t := range canonicalAttributeInterfaces {
		if strings.ToLower(n) == lower {
			return ts.attributes.GetAttribute(t)
		}
	}
	return nil
}

// ClearAttributes clears all attributes.
func (ts *BaseTokenStream) ClearAttributes() {
	ts.attributes.ClearAttributes()
}

// IncrementToken advances to the next token.
// Subclasses must override this method.
func (ts *BaseTokenStream) IncrementToken() (bool, error) {
	return false, nil
}

// End performs end-of-stream operations.
func (ts *BaseTokenStream) End() error {
	return nil
}

// Close releases resources.
func (ts *BaseTokenStream) Close() error {
	return nil
}

// canonicalAttributeInterfaces maps the legacy string-keyed names used
// by [BaseTokenStream.GetAttribute] to the canonical reflect.Type of
// each Attribute interface declared in this package.
var canonicalAttributeInterfaces = map[string]reflect.Type{
	"CharTermAttribute":          CharTermAttributeType,
	"OffsetAttribute":            OffsetAttributeType,
	"tokenattributes.PositionIncrementAttribute": tokenattributes.PositionIncrementAttributeType,
	"TypeAttribute":              TypeAttributeType,
	"PayloadAttribute":           PayloadAttributeType,
	"FlagsAttribute":             FlagsAttributeType,
	"KeywordAttribute":           KeywordAttributeType,
	"PositionLengthAttribute":    PositionLengthAttributeType,
	"TermFrequencyAttribute":     TermFrequencyAttributeType,
	"SentenceAttribute":          SentenceAttributeType,
	"BytesTermAttribute":         BytesTermAttributeType,
	"TermToBytesRefAttribute":    TermToBytesRefAttributeType,
	"BoostAttribute":             BoostAttributeType,
}
