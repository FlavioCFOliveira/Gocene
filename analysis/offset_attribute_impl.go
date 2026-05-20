// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/OffsetAttributeImpl.java

package analysis

import (
	"fmt"
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// OffsetAttributeImpl is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.OffsetAttributeImpl.
//
// It is the exported concrete implementation of [OffsetAttribute].
// Start and end offsets default to 0, matching the Lucene default.
//
// Note: the package also contains the unexported [offsetAttribute]
// created by [NewOffsetAttribute]; OffsetAttributeImpl is the exported
// counterpart consumed by code that requires an exported concrete type.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/OffsetAttributeImpl.java
type OffsetAttributeImpl struct {
	startOffset int
	endOffset   int
}

// Compile-time assertions.
var (
	_ util.AttributeImpl              = (*OffsetAttributeImpl)(nil)
	_ OffsetAttribute                 = (*OffsetAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*OffsetAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (o *OffsetAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{OffsetAttributeType}
}

// NewOffsetAttributeImpl initialises this attribute with startOffset
// and endOffset of 0, matching the Lucene no-arg constructor.
func NewOffsetAttributeImpl() *OffsetAttributeImpl {
	return &OffsetAttributeImpl{}
}

// StartOffset returns the inclusive start character offset.
func (o *OffsetAttributeImpl) StartOffset() int { return o.startOffset }

// EndOffset returns the exclusive end character offset.
func (o *OffsetAttributeImpl) EndOffset() int { return o.endOffset }

// SetStartOffset sets the start offset individually (Gocene legacy
// compatibility; the Lucene reference exposes only the combined
// SetOffset setter).
func (o *OffsetAttributeImpl) SetStartOffset(offset int) { o.startOffset = offset }

// SetEndOffset sets the end offset individually.
func (o *OffsetAttributeImpl) SetEndOffset(offset int) { o.endOffset = offset }

// SetOffset is the Lucene-faithful combined setter. Panics when
// startOffset is negative or endOffset < startOffset, matching
// {@code OffsetAttributeImpl#setOffset(int, int)}'s
// IllegalArgumentException.
func (o *OffsetAttributeImpl) SetOffset(startOffset, endOffset int) {
	if startOffset < 0 || endOffset < startOffset {
		panic(fmt.Sprintf(
			"OffsetAttributeImpl.SetOffset: startOffset must be non-negative and endOffset must be >= startOffset; got startOffset=%d, endOffset=%d",
			startOffset, endOffset))
	}
	o.startOffset = startOffset
	o.endOffset = endOffset
}

// Clear resets both offsets to 0, matching
// {@code OffsetAttributeImpl#clear()}.
func (o *OffsetAttributeImpl) Clear() {
	o.startOffset = 0
	o.endOffset = 0
}

// End implements util.AttributeImpl.End. The Lucene base calls
// clear() from end().
func (o *OffsetAttributeImpl) End() { o.Clear() }

// Copy returns a deep clone of this impl as [util.AttributeImpl],
// satisfying the [OffsetAttribute] interface contract.
func (o *OffsetAttributeImpl) Copy() util.AttributeImpl {
	return &OffsetAttributeImpl{
		startOffset: o.startOffset,
		endOffset:   o.endOffset,
	}
}

// CloneAttribute returns a deep copy of this impl as [util.AttributeImpl].
func (o *OffsetAttributeImpl) CloneAttribute() util.AttributeImpl {
	return o.Copy()
}

// CopyTo copies the offsets onto target, which must implement
// [OffsetAttribute]; a panic is raised otherwise.
func (o *OffsetAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(OffsetAttribute)
	if !ok {
		panic("OffsetAttributeImpl.CopyTo: target must implement OffsetAttribute")
	}
	t.SetOffset(o.startOffset, o.endOffset)
}

// ReflectWith pushes the two (OffsetAttribute, "startOffset" /
// "endOffset", value) triples through reflector, matching the Lucene
// reference.
func (o *OffsetAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(OffsetAttributeType, "startOffset", o.startOffset)
	reflector(OffsetAttributeType, "endOffset", o.endOffset)
}

// Equals returns true if other is a [OffsetAttributeImpl] with the
// same start and end offsets, matching Lucene's {@code equals(Object)}.
func (o *OffsetAttributeImpl) Equals(other any) bool {
	if o == other {
		return true
	}
	oo, ok := other.(*OffsetAttributeImpl)
	if !ok {
		return false
	}
	return o.startOffset == oo.startOffset && o.endOffset == oo.endOffset
}

// HashCode mirrors {@code OffsetAttributeImpl#hashCode()}:
// code = startOffset; code = code*31 + endOffset.
func (o *OffsetAttributeImpl) HashCode() int {
	return o.startOffset*31 + o.endOffset
}
