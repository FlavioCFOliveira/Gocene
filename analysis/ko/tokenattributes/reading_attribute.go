// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tokenattributes

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ReadingAttributeType is the reflect.Type of the ReadingAttribute interface.
var ReadingAttributeType = reflect.TypeOf((*ReadingAttribute)(nil)).Elem()

// ReadingAttribute exposes reading (Hanja → Hangul) data for Korean tokens.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.tokenattributes.ReadingAttribute from Apache
// Lucene 10.4.0.
type ReadingAttribute interface {
	// GetReading returns the reading of the current token, or empty string.
	GetReading() string

	// SetToken sets the token that drives this attribute.
	SetToken(token KoreanToken)
}

// ReadingAttributeImpl is the concrete implementation of ReadingAttribute.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.tokenattributes.ReadingAttributeImpl from
// Apache Lucene 10.4.0.
type ReadingAttributeImpl struct {
	token KoreanToken
}

var (
	_ util.AttributeImpl              = (*ReadingAttributeImpl)(nil)
	_ ReadingAttribute                = (*ReadingAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*ReadingAttributeImpl)(nil)
)

// NewReadingAttributeImpl creates a new ReadingAttributeImpl.
func NewReadingAttributeImpl() *ReadingAttributeImpl {
	return &ReadingAttributeImpl{}
}

// AttributeInterfaces satisfies util.AttributeInterfaceProvider.
func (a *ReadingAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{ReadingAttributeType}
}

// GetReading returns the reading of the current token, or empty string.
func (a *ReadingAttributeImpl) GetReading() string {
	if a.token == nil {
		return ""
	}
	return a.token.GetReading()
}

// SetToken sets the token that drives this attribute.
func (a *ReadingAttributeImpl) SetToken(token KoreanToken) { a.token = token }

// Clear resets the attribute to zero state.
func (a *ReadingAttributeImpl) Clear() { a.token = nil }

// End resets to end-of-field state (same as Clear for this attribute).
func (a *ReadingAttributeImpl) End() { a.Clear() }

// CopyTo copies this attribute's state to target.
func (a *ReadingAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(ReadingAttribute)
	if !ok {
		panic("ReadingAttributeImpl.CopyTo: target must implement ReadingAttribute")
	}
	t.SetToken(a.token)
}

// CloneAttribute returns a deep clone of this impl.
func (a *ReadingAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &ReadingAttributeImpl{token: a.token}
}

// ReflectWith reports attribute values via the provided reflector.
func (a *ReadingAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(ReadingAttributeType, "reading", a.GetReading())
}
