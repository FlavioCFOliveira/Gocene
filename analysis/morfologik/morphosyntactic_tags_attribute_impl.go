// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/morfologik/src/java/org/apache/lucene/analysis/morfologik/MorphosyntacticTagsAttributeImpl.java

package morfologik

import (
	"reflect"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// MorphosyntacticTagsAttributeImpl is the concrete implementation of
// [MorphosyntacticTagsAttribute]. It holds a list of potential tag variants
// for the current token.
//
// Callers that need a stable copy of the tags must clone the slice and each
// [strings.Builder] individually, because the implementation reuses builders
// across tokens to avoid allocations (matching the Lucene contract).
//
// This is the Go port of
// org.apache.lucene.analysis.morfologik.MorphosyntacticTagsAttributeImpl
// (Apache Lucene 10.4.0).
type MorphosyntacticTagsAttributeImpl struct {
	util.BaseAttributeImpl

	// tags holds the current morphosyntactic tag variants. nil means
	// no-value (same as Java's null initial state).
	tags []strings.Builder
}

// Compile-time assertions.
var (
	_ util.AttributeImpl              = (*MorphosyntacticTagsAttributeImpl)(nil)
	_ MorphosyntacticTagsAttribute    = (*MorphosyntacticTagsAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*MorphosyntacticTagsAttributeImpl)(nil)
)

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (m *MorphosyntacticTagsAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{MorphosyntacticTagsAttributeType}
}

// NewMorphosyntacticTagsAttributeImpl initialises this attribute with no
// tags, matching the Lucene no-arg constructor.
func NewMorphosyntacticTagsAttributeImpl() *MorphosyntacticTagsAttributeImpl {
	return &MorphosyntacticTagsAttributeImpl{}
}

// GetTags returns the current POS tag list. Returns nil when no tags have
// been set since the last Clear.
func (m *MorphosyntacticTagsAttributeImpl) GetTags() []strings.Builder {
	return m.tags
}

// SetTags replaces the internal tag list with tags. The slice is NOT copied;
// ownership transfers to this impl (matching the Lucene "reference is stored
// directly" contract).
func (m *MorphosyntacticTagsAttributeImpl) SetTags(tags []strings.Builder) {
	m.tags = tags
}

// Clear resets the attribute to its no-value state (nil tags).
func (m *MorphosyntacticTagsAttributeImpl) Clear() {
	m.tags = nil
}

// End delegates to Clear, matching the Lucene base-class default of
// calling clear() from end().
func (m *MorphosyntacticTagsAttributeImpl) End() {
	m.Clear()
}

// Equals returns true when other is a [MorphosyntacticTagsAttribute] whose
// tag list is equal to this one. Two nil tag lists compare equal.
func (m *MorphosyntacticTagsAttributeImpl) Equals(other any) bool {
	if m == other {
		return true
	}
	o, ok := other.(MorphosyntacticTagsAttribute)
	if !ok {
		return false
	}
	return equalBuilderSlices(m.tags, o.GetTags())
}

// equalBuilderSlices reports whether two []strings.Builder slices have the
// same length and the same string content at every position.
func equalBuilderSlices(a, b []strings.Builder) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].String() != b[i].String() {
			return false
		}
	}
	return true
}

// HashCode mirrors MorphosyntacticTagsAttributeImpl#hashCode(): 0 when
// tags is nil; otherwise the sum of each builder's string hash (matching
// Java's List.hashCode contract, which sums element hash codes with a
// polynomial, but the Lucene implementation delegates to List.hashCode
// whose contract is 1 + sum(31*acc + elem.hashCode())).
func (m *MorphosyntacticTagsAttributeImpl) HashCode() int {
	if m.tags == nil {
		return 0
	}
	// Reproduce Java's AbstractList.hashCode: result = 1; for each e: result = 31*result + hashCode(e)
	result := 1
	for _, b := range m.tags {
		s := b.String()
		h := 0
		for _, r := range s {
			h = 31*h + int(r)
		}
		result = 31*result + h
	}
	return result
}

// CopyTo deep-copies this impl's tag list onto target, which must implement
// [MorphosyntacticTagsAttribute]; panics otherwise.
func (m *MorphosyntacticTagsAttributeImpl) CopyTo(target util.AttributeImpl) {
	t, ok := target.(MorphosyntacticTagsAttribute)
	if !ok {
		panic("MorphosyntacticTagsAttributeImpl.CopyTo: target must implement MorphosyntacticTagsAttribute")
	}
	if m.tags == nil {
		t.SetTags(nil)
		return
	}
	cloned := make([]strings.Builder, len(m.tags))
	for i, b := range m.tags {
		cloned[i].WriteString(b.String())
	}
	t.SetTags(cloned)
}

// CloneAttribute returns a deep clone of this impl.
func (m *MorphosyntacticTagsAttributeImpl) CloneAttribute() util.AttributeImpl {
	cloned := NewMorphosyntacticTagsAttributeImpl()
	m.CopyTo(cloned)
	return cloned
}

// ReflectWith pushes the (MorphosyntacticTagsAttribute, "tags", value)
// triple through reflector, matching the Lucene reference exactly.
func (m *MorphosyntacticTagsAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(MorphosyntacticTagsAttributeType, "tags", m.tags)
}
