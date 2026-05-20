// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/TypeAttributeImpl.java

package analysis

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TypeAttributeImpl is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.TypeAttributeImpl.
//
// It is the exported concrete implementation of [TypeAttribute].
// The default type is [DefaultTypeAttributeValue] ("word"), matching
// the Lucene default.
//
// Note: the package also contains the unexported [typeAttributeImpl]
// created by [NewTypeAttribute]; TypeAttributeImpl is the exported
// counterpart consumed by code that requires an exported concrete type.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/TypeAttributeImpl.java
type TypeAttributeImpl struct {
	tokenType string
}

// Compile-time assertions.
var (
	_ util.AttributeImpl              = (*TypeAttributeImpl)(nil)
	_ TypeAttribute                   = (*TypeAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*TypeAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (ty *TypeAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{TypeAttributeType}
}

// NewTypeAttributeImpl initialises this attribute with the default
// type "word", matching the Lucene no-arg constructor.
func NewTypeAttributeImpl() *TypeAttributeImpl {
	return &TypeAttributeImpl{tokenType: DefaultTypeAttributeValue}
}

// NewTypeAttributeImplWithType initialises this attribute with the
// given type string, matching the Lucene single-arg constructor.
func NewTypeAttributeImplWithType(tokenType string) *TypeAttributeImpl {
	return &TypeAttributeImpl{tokenType: tokenType}
}

// GetType returns the current token type.
func (ty *TypeAttributeImpl) GetType() string { return ty.tokenType }

// SetType replaces the token type.
func (ty *TypeAttributeImpl) SetType(tokenType string) { ty.tokenType = tokenType }

// Clear resets the type to the Lucene default ("word"), matching
// {@code TypeAttributeImpl#clear()}.
func (ty *TypeAttributeImpl) Clear() { ty.tokenType = DefaultTypeAttributeValue }

// End implements util.AttributeImpl.End. The Lucene base calls
// clear() from end().
func (ty *TypeAttributeImpl) End() { ty.Clear() }

// CloneAttribute returns a deep copy of this impl as [util.AttributeImpl].
func (ty *TypeAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &TypeAttributeImpl{tokenType: ty.tokenType}
}

// CopyTo copies the type string onto target, which must implement
// [TypeAttribute]; a panic is raised otherwise.
func (ty *TypeAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(TypeAttribute)
	if !ok {
		panic("TypeAttributeImpl.CopyTo: target must implement TypeAttribute")
	}
	t.SetType(ty.tokenType)
}

// ReflectWith pushes the (TypeAttribute, "type", value) triple through
// reflector, matching the Lucene reference.
func (ty *TypeAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(TypeAttributeType, "type", ty.tokenType)
}

// Equals returns true if other is a [TypeAttributeImpl] with the same
// type string, matching Lucene's {@code equals(Object)}.
func (ty *TypeAttributeImpl) Equals(other any) bool {
	if ty == other {
		return true
	}
	o, ok := other.(*TypeAttributeImpl)
	if !ok {
		return false
	}
	return ty.tokenType == o.tokenType
}

// HashCode returns the Java-style hash of the type string (or 0 when
// the string is empty), matching Lucene's {@code hashCode()}.
func (ty *TypeAttributeImpl) HashCode() int {
	return javaStringHash(ty.tokenType)
}
