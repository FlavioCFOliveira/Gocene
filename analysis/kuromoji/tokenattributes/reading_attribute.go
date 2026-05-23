// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tokenattributes

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/dict"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ReadingAttributeType is the reflect.Type of the ReadingAttribute interface.
var ReadingAttributeType = reflect.TypeOf((*ReadingAttribute)(nil)).Elem()

// ReadingAttribute exposes the reading and pronunciation data for a Japanese token.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.tokenattributes.ReadingAttribute from Apache
// Lucene 10.4.0.
type ReadingAttribute interface {
	// Reading returns the reading (katakana) of the current token, or empty
	// string when not available.
	Reading() string

	// Pronunciation returns the pronunciation of the current token, or empty
	// string when not available.
	Pronunciation() string

	// SetToken associates the given token with this attribute.
	SetToken(token *dict.Token)
}

// ReadingAttributeImpl is the default implementation of ReadingAttribute.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.tokenattributes.ReadingAttributeImpl from
// Apache Lucene 10.4.0.
type ReadingAttributeImpl struct {
	token *dict.Token
}

var (
	_ util.AttributeImpl              = (*ReadingAttributeImpl)(nil)
	_ ReadingAttribute                = (*ReadingAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*ReadingAttributeImpl)(nil)
)

// NewReadingAttributeImpl creates a new zero-value ReadingAttributeImpl.
func NewReadingAttributeImpl() *ReadingAttributeImpl {
	return &ReadingAttributeImpl{}
}

// AttributeInterfaces satisfies util.AttributeInterfaceProvider.
func (a *ReadingAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{ReadingAttributeType}
}

// Reading returns the reading of the current token.
func (a *ReadingAttributeImpl) Reading() string {
	if a.token == nil {
		return ""
	}
	return a.token.Reading()
}

// Pronunciation returns the pronunciation of the current token.
func (a *ReadingAttributeImpl) Pronunciation() string {
	if a.token == nil {
		return ""
	}
	return a.token.Pronunciation()
}

// SetToken sets the active token.
func (a *ReadingAttributeImpl) SetToken(token *dict.Token) { a.token = token }

// Clear resets the attribute to zero state.
func (a *ReadingAttributeImpl) Clear() { a.token = nil }

// End resets to end-of-field state (same as Clear for this attribute).
func (a *ReadingAttributeImpl) End() { a.Clear() }

// CopyTo copies this attribute state to target.
func (a *ReadingAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(ReadingAttribute)
	if !ok {
		panic("ReadingAttributeImpl.CopyTo: target must implement ReadingAttribute")
	}
	t.SetToken(a.token)
}

// CloneAttribute returns a deep copy of this impl.
func (a *ReadingAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &ReadingAttributeImpl{token: a.token}
}

// ReflectWith emits reading and pronunciation fields (with romaji translations)
// to reflector.
func (a *ReadingAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reading := a.Reading()
	var readingEN string
	if reading != "" {
		readingEN = dict.GetRomanization(reading)
	}
	reflector(ReadingAttributeType, "reading", reading)
	reflector(ReadingAttributeType, "reading (en)", readingEN)

	pronunciation := a.Pronunciation()
	var pronunciationEN string
	if pronunciation != "" {
		pronunciationEN = dict.GetRomanization(pronunciation)
	}
	reflector(ReadingAttributeType, "pronunciation", pronunciation)
	reflector(ReadingAttributeType, "pronunciation (en)", pronunciationEN)
}
