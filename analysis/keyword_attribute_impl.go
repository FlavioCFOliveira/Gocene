// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/KeywordAttributeImpl.java

package analysis

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// KeywordAttributeImpl is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.KeywordAttributeImpl.
//
// It is the exported concrete implementation of [KeywordAttribute].
// The default keyword flag is false, matching the Lucene default.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/KeywordAttributeImpl.java
type KeywordAttributeImpl struct {
	keyword bool
}

// Compile-time assertions.
var (
	_ util.AttributeImpl              = (*KeywordAttributeImpl)(nil)
	_ KeywordAttribute                = (*KeywordAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*KeywordAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (k *KeywordAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{KeywordAttributeType}
}

// NewKeywordAttributeImpl initialises this attribute with the keyword
// flag set to false, matching the Lucene no-arg constructor.
func NewKeywordAttributeImpl() *KeywordAttributeImpl {
	return &KeywordAttributeImpl{}
}

// IsKeywordToken returns true if the current token is a keyword.
func (k *KeywordAttributeImpl) IsKeywordToken() bool { return k.keyword }

// SetKeyword toggles the keyword flag.
func (k *KeywordAttributeImpl) SetKeyword(isKeyword bool) { k.keyword = isKeyword }

// Clear resets the keyword flag to false, matching
// {@code KeywordAttributeImpl#clear()}.
func (k *KeywordAttributeImpl) Clear() { k.keyword = false }

// End implements util.AttributeImpl.End. The Lucene base calls
// clear() from end().
func (k *KeywordAttributeImpl) End() { k.Clear() }

// CloneAttribute returns a deep copy of this impl as [util.AttributeImpl].
func (k *KeywordAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &KeywordAttributeImpl{keyword: k.keyword}
}

// CopyTo copies the keyword flag onto target, which must implement
// [KeywordAttribute]; a panic is raised otherwise.
func (k *KeywordAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(KeywordAttribute)
	if !ok {
		panic("KeywordAttributeImpl.CopyTo: target must implement KeywordAttribute")
	}
	t.SetKeyword(k.keyword)
}

// ReflectWith pushes the (KeywordAttribute, "keyword", value) triple
// through reflector, matching the Lucene reference.
func (k *KeywordAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(KeywordAttributeType, "keyword", k.keyword)
}

// Equals returns true if other is a [KeywordAttributeImpl] with the
// same keyword flag, matching Lucene's {@code equals(Object)}.
// Lucene's KeywordAttributeImpl.equals uses getClass() check, so only
// [*KeywordAttributeImpl] matches.
func (k *KeywordAttributeImpl) Equals(other any) bool {
	if k == other {
		return true
	}
	o, ok := other.(*KeywordAttributeImpl)
	if !ok {
		return false
	}
	return k.keyword == o.keyword
}

// HashCode mirrors {@code KeywordAttributeImpl#hashCode()}: 31 when
// keyword=true, 37 otherwise.
func (k *KeywordAttributeImpl) HashCode() int {
	if k.keyword {
		return 31
	}
	return 37
}
