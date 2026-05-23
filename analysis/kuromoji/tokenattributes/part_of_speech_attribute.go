// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tokenattributes

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/dict"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PartOfSpeechAttributeType is the reflect.Type of the PartOfSpeechAttribute
// interface.
var PartOfSpeechAttributeType = reflect.TypeOf((*PartOfSpeechAttribute)(nil)).Elem()

// PartOfSpeechAttribute exposes the part-of-speech tag for a Japanese token.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.tokenattributes.PartOfSpeechAttribute from
// Apache Lucene 10.4.0.
type PartOfSpeechAttribute interface {
	// PartOfSpeech returns the part-of-speech tag of the current token.
	PartOfSpeech() string

	// SetToken associates the given token with this attribute.
	SetToken(token *dict.Token)
}

// PartOfSpeechAttributeImpl is the default implementation of
// PartOfSpeechAttribute.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.tokenattributes.PartOfSpeechAttributeImpl from
// Apache Lucene 10.4.0.
type PartOfSpeechAttributeImpl struct {
	token *dict.Token
}

var (
	_ util.AttributeImpl              = (*PartOfSpeechAttributeImpl)(nil)
	_ PartOfSpeechAttribute           = (*PartOfSpeechAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*PartOfSpeechAttributeImpl)(nil)
)

// NewPartOfSpeechAttributeImpl creates a new zero-value
// PartOfSpeechAttributeImpl.
func NewPartOfSpeechAttributeImpl() *PartOfSpeechAttributeImpl {
	return &PartOfSpeechAttributeImpl{}
}

// AttributeInterfaces satisfies util.AttributeInterfaceProvider.
func (a *PartOfSpeechAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{PartOfSpeechAttributeType}
}

// PartOfSpeech returns the POS tag of the current token.
func (a *PartOfSpeechAttributeImpl) PartOfSpeech() string {
	if a.token == nil {
		return ""
	}
	return a.token.PartOfSpeech()
}

// SetToken sets the active token.
func (a *PartOfSpeechAttributeImpl) SetToken(token *dict.Token) { a.token = token }

// Clear resets the attribute to zero state.
func (a *PartOfSpeechAttributeImpl) Clear() { a.token = nil }

// End resets to end-of-field state (same as Clear for this attribute).
func (a *PartOfSpeechAttributeImpl) End() { a.Clear() }

// CopyTo copies this attribute state to target.
func (a *PartOfSpeechAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(PartOfSpeechAttribute)
	if !ok {
		panic("PartOfSpeechAttributeImpl.CopyTo: target must implement PartOfSpeechAttribute")
	}
	t.SetToken(a.token)
}

// CloneAttribute returns a deep copy of this impl.
func (a *PartOfSpeechAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &PartOfSpeechAttributeImpl{token: a.token}
}

// ReflectWith emits POS tag and its English translation to reflector.
func (a *PartOfSpeechAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	pos := a.PartOfSpeech()
	var posEN string
	if pos != "" {
		posEN = dict.GetPOSTranslation(pos)
	}
	reflector(PartOfSpeechAttributeType, "partOfSpeech", pos)
	reflector(PartOfSpeechAttributeType, "partOfSpeech (en)", posEN)
}
