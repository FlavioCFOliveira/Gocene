// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TokenStream is an abstract base class for producing token streams.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.TokenStream.
//
// TokenStream is the fundamental abstraction for processing text in Lucene.
// It represents a stream of tokens that can be consumed incrementally.
// The workflow is:
//
// 1. Create TokenStream with a source (e.g., Tokenizer or another TokenStream)
// 2. Call IncrementToken() repeatedly until it returns false
// 3. Call End() to perform end-of-stream operations
// 4. Call Close() to release resources
//
// TokenStream uses an AttributeSource to manage attributes.
type TokenStream interface {
	// IncrementToken advances to the next token in the stream.
	// Returns true if a token is available, false if at end of stream.
	IncrementToken() (bool, error)

	// End performs end-of-stream operations.
	// Called after the last token has been consumed.
	End() error

	// Close releases resources held by this TokenStream.
	Close() error
}

// BaseTokenStream provides a base implementation for TokenStream.
//
// Embed this struct in concrete TokenStream implementations to inherit
// common functionality.
//
// Sprint 54 Phase 4 migrated the embedded [analysis.AttributeSource] to
// [util.AttributeSource], the Lucene-faithful 10.4.0-parity registry.
// The public surface ([BaseTokenStream.GetAttributeSource] now returns
// [util.AttributeSource]; legacy string-keyed [BaseTokenStream.GetAttribute]
// is retained as a back-compat shim that resolves via the
// [canonicalAttributeInterfaces] registry to a typed
// [util.AttributeSource.GetAttribute] lookup).
type BaseTokenStream struct {
	// attributes holds the attribute source for this token stream
	attributes *util.AttributeSource
}

// NewBaseTokenStream creates a new BaseTokenStream backed by a fresh
// [util.AttributeSource] using [util.DefaultAttributeFactoryInstance].
func NewBaseTokenStream() *BaseTokenStream {
	return &BaseTokenStream{
		attributes: util.NewAttributeSource(),
	}
}

// GetAttributeSource returns the underlying [util.AttributeSource].
// Sprint 54 Phase 4 promoted the return type from the legacy
// [analysis.AttributeSource] (now used by tests only) to the
// Lucene-faithful [util.AttributeSource].
func (ts *BaseTokenStream) GetAttributeSource() *util.AttributeSource {
	return ts.attributes
}

// AddAttribute registers an [AttributeImpl] with the token stream's
// AttributeSource. Routes through [util.AttributeSource.AddAttributeImpl]
// which delegates to the impl's [util.AttributeInterfaceProvider]
// declaration (Sprint 54 Phase 4 part A added that method to every
// analysis impl) to discover every Attribute interface the impl
// satisfies.
func (ts *BaseTokenStream) AddAttribute(attr AttributeImpl) {
	if attr == nil {
		return
	}
	ts.attributes.AddAttributeImpl(attr)
}

// GetAttribute is a legacy back-compat shim that resolves name through
// the [canonicalAttributeInterfaces] registry to a typed
// [util.AttributeSource.GetAttribute] lookup. New code should call
// [BaseTokenStream.GetAttributeSource] and use the typed
// [util.AttributeSource.GetAttribute] API directly.
func (ts *BaseTokenStream) GetAttribute(name string) AttributeImpl {
	if t, ok := canonicalAttributeInterfaces[name]; ok {
		return ts.attributes.GetAttribute(t)
	}
	// Case-insensitive fallback for the legacy "charTermAttribute" /
	// "Charterm..." variants exercised by older consumers.
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
// each Attribute interface declared in this package. The registry is
// initialised once from [registerCanonicalAttributeInterface] calls
// during package init to keep the map definition co-located with each
// interface's type variable declaration.
var canonicalAttributeInterfaces = map[string]reflect.Type{
	"CharTermAttribute":          CharTermAttributeType,
	"OffsetAttribute":            OffsetAttributeType,
	"PositionIncrementAttribute": PositionIncrementAttributeType,
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
