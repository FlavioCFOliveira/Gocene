// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tokenattributes

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/dict"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BaseFormAttributeType is the reflect.Type of the BaseFormAttribute interface.
var BaseFormAttributeType = reflect.TypeOf((*BaseFormAttribute)(nil)).Elem()

// BaseFormAttribute exposes the dictionary base form of a Japanese token.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.tokenattributes.BaseFormAttribute from
// Apache Lucene 10.4.0.
type BaseFormAttribute interface {
	// BaseForm returns the base (dictionary) form of the current token, or
	// empty string when not applicable.
	BaseForm() string

	// SetToken associates the given token with this attribute.
	SetToken(token *dict.Token)
}

// BaseFormAttributeImpl is the default implementation of BaseFormAttribute.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.tokenattributes.BaseFormAttributeImpl from
// Apache Lucene 10.4.0.
type BaseFormAttributeImpl struct {
	token *dict.Token
}

var (
	_ util.AttributeImpl              = (*BaseFormAttributeImpl)(nil)
	_ BaseFormAttribute               = (*BaseFormAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*BaseFormAttributeImpl)(nil)
)

// NewBaseFormAttributeImpl creates a new zero-value BaseFormAttributeImpl.
func NewBaseFormAttributeImpl() *BaseFormAttributeImpl {
	return &BaseFormAttributeImpl{}
}

// AttributeInterfaces satisfies util.AttributeInterfaceProvider.
func (a *BaseFormAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{BaseFormAttributeType}
}

// BaseForm returns the base form of the current token, or empty string.
func (a *BaseFormAttributeImpl) BaseForm() string {
	if a.token == nil {
		return ""
	}
	return a.token.BaseForm()
}

// SetToken sets the active token.
func (a *BaseFormAttributeImpl) SetToken(token *dict.Token) { a.token = token }

// Clear resets the attribute to zero state.
func (a *BaseFormAttributeImpl) Clear() { a.token = nil }

// End resets to end-of-field state (same as Clear for this attribute).
func (a *BaseFormAttributeImpl) End() { a.Clear() }

// CopyTo copies this attribute state to target.
func (a *BaseFormAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(BaseFormAttribute)
	if !ok {
		panic("BaseFormAttributeImpl.CopyTo: target must implement BaseFormAttribute")
	}
	t.SetToken(a.token)
}

// CloneAttribute returns a deep copy of this impl.
func (a *BaseFormAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &BaseFormAttributeImpl{token: a.token}
}

// ReflectWith emits (BaseFormAttributeType, "baseForm", value) to reflector.
func (a *BaseFormAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(BaseFormAttributeType, "baseForm", a.BaseForm())
}
