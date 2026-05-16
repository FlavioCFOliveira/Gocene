// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// BytesTermAttributeImpl is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.BytesTermAttributeImpl.
//
// It backs [BytesTermAttribute] (and therefore
// [TermToBytesRefAttribute]) with a single mutable BytesRef pointer.
// CopyTo deep-copies the source BytesRef (Lucene parity) so the
// destination owns its own buffer.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/BytesTermAttributeImpl.java
type BytesTermAttributeImpl struct {
	bytes *util.BytesRef
}

// Compile-time assertions to lock in the contract this impl
// participates in.
var (
	_ AttributeImpl           = (*BytesTermAttributeImpl)(nil)
	_ BytesTermAttribute      = (*BytesTermAttributeImpl)(nil)
	_ TermToBytesRefAttribute = (*BytesTermAttributeImpl)(nil)
	_ AttributeReflectable    = (*BytesTermAttributeImpl)(nil)
)

// NewBytesTermAttributeImpl initialises this attribute with no bytes,
// matching the Lucene no-arg constructor.
func NewBytesTermAttributeImpl() *BytesTermAttributeImpl {
	return &BytesTermAttributeImpl{}
}

// GetBytesRef returns the current BytesRef, or nil if no bytes have
// been set since the last Clear.
func (b *BytesTermAttributeImpl) GetBytesRef() *util.BytesRef {
	return b.bytes
}

// SetBytesRef sets the current BytesRef.
func (b *BytesTermAttributeImpl) SetBytesRef(bytes *util.BytesRef) {
	b.bytes = bytes
}

// Clear resets this impl to its default state (no bytes), matching
// {@code BytesTermAttributeImpl#clear()}.
func (b *BytesTermAttributeImpl) Clear() {
	b.bytes = nil
}

// CopyTo deep-copies this impl's BytesRef onto target. The target must
// be a [BytesTermAttributeImpl], matching the Lucene cast contract; a
// panic with an explanatory message is raised otherwise.
func (b *BytesTermAttributeImpl) CopyTo(target AttributeImpl) {
	other, ok := target.(*BytesTermAttributeImpl)
	if !ok {
		panic("BytesTermAttributeImpl.CopyTo: target must be *BytesTermAttributeImpl")
	}
	if b.bytes == nil {
		other.bytes = nil
		return
	}
	other.bytes = util.BytesRefDeepCopyOf(b.bytes)
}

// Copy returns a deep clone of this impl, matching the result of
// {@code BytesTermAttributeImpl#clone()}.
func (b *BytesTermAttributeImpl) Copy() AttributeImpl {
	clone := NewBytesTermAttributeImpl()
	b.CopyTo(clone)
	return clone
}

// ReflectWith pushes the single (TermToBytesRefAttribute, "bytes",
// bytes) triple through reflector, matching the Lucene reference
// exactly.
func (b *BytesTermAttributeImpl) ReflectWith(reflector AttributeReflector) {
	reflector(reflect.TypeOf((*TermToBytesRefAttribute)(nil)).Elem(), "bytes", b.bytes)
}

// Equals returns true if other is a [BytesTermAttributeImpl] whose
// BytesRef compares equal, matching Lucene's {@code equals(Object)}.
// Two nil BytesRefs compare equal.
func (b *BytesTermAttributeImpl) Equals(other any) bool {
	if b == other {
		return true
	}
	o, ok := other.(*BytesTermAttributeImpl)
	if !ok {
		return false
	}
	if b.bytes == nil || o.bytes == nil {
		return b.bytes == nil && o.bytes == nil
	}
	return util.BytesRefEquals(b.bytes, o.bytes)
}

// HashCode returns the hash of the underlying BytesRef, or 0 when the
// BytesRef is nil, matching {@code Objects.hash(bytes)} on a nil
// pointer in Lucene.
func (b *BytesTermAttributeImpl) HashCode() int {
	if b.bytes == nil {
		return 0
	}
	return b.bytes.HashCode()
}
