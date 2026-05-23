// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tokenattributes

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/dict"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// InflectionAttributeType is the reflect.Type of the InflectionAttribute interface.
var InflectionAttributeType = reflect.TypeOf((*InflectionAttribute)(nil)).Elem()

// InflectionAttribute exposes inflection type and form data for a Japanese token.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.tokenattributes.InflectionAttribute from
// Apache Lucene 10.4.0.
type InflectionAttribute interface {
	// InflectionType returns the inflection type of the current token, or
	// empty string when not applicable.
	InflectionType() string

	// InflectionForm returns the inflection form of the current token, or
	// empty string when not applicable.
	InflectionForm() string

	// SetToken associates the given token with this attribute.
	SetToken(token *dict.Token)
}

// InflectionAttributeImpl is the default implementation of InflectionAttribute.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.tokenattributes.InflectionAttributeImpl from
// Apache Lucene 10.4.0.
type InflectionAttributeImpl struct {
	token *dict.Token
}

var (
	_ util.AttributeImpl              = (*InflectionAttributeImpl)(nil)
	_ InflectionAttribute             = (*InflectionAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*InflectionAttributeImpl)(nil)
)

// NewInflectionAttributeImpl creates a new zero-value InflectionAttributeImpl.
func NewInflectionAttributeImpl() *InflectionAttributeImpl {
	return &InflectionAttributeImpl{}
}

// AttributeInterfaces satisfies util.AttributeInterfaceProvider.
func (a *InflectionAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{InflectionAttributeType}
}

// InflectionType returns the inflection type, or empty string.
func (a *InflectionAttributeImpl) InflectionType() string {
	if a.token == nil {
		return ""
	}
	return a.token.InflectionType()
}

// InflectionForm returns the inflection form, or empty string.
func (a *InflectionAttributeImpl) InflectionForm() string {
	if a.token == nil {
		return ""
	}
	return a.token.InflectionForm()
}

// SetToken sets the active token.
func (a *InflectionAttributeImpl) SetToken(token *dict.Token) { a.token = token }

// Clear resets the attribute to zero state.
func (a *InflectionAttributeImpl) Clear() { a.token = nil }

// End resets to end-of-field state (same as Clear for this attribute).
func (a *InflectionAttributeImpl) End() { a.Clear() }

// CopyTo copies this attribute state to target.
func (a *InflectionAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(InflectionAttribute)
	if !ok {
		panic("InflectionAttributeImpl.CopyTo: target must implement InflectionAttribute")
	}
	t.SetToken(a.token)
}

// CloneAttribute returns a deep copy of this impl.
func (a *InflectionAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &InflectionAttributeImpl{token: a.token}
}

// ReflectWith emits inflection type and form fields (with English translations)
// to reflector.
func (a *InflectionAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	inflType := a.InflectionType()
	var inflTypeEN string
	if inflType != "" {
		inflTypeEN = dict.GetInflectionTypeTranslation(inflType)
	}
	reflector(InflectionAttributeType, "inflectionType", inflType)
	reflector(InflectionAttributeType, "inflectionType (en)", inflTypeEN)

	form := a.InflectionForm()
	var formEN string
	if form != "" {
		formEN = dict.GetInflectedFormTranslation(form)
	}
	reflector(InflectionAttributeType, "inflectionForm", form)
	reflector(InflectionAttributeType, "inflectionForm (en)", formEN)
}
