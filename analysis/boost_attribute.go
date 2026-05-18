// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// DefaultBoost is the boost value returned by a BoostAttribute that
// has never been explicitly set.
const DefaultBoost float32 = 1.0

// BoostAttributeType is the reflect.Type of the [BoostAttribute]
// interface, used as the lookup key for an [util.AttributeSource].
// Sprint 54 Phase 4 promoted BoostAttribute from a bare struct to a
// Lucene-faithful interface+impl pair; the variable now holds the
// interface type rather than the concrete pointer type.
var BoostAttributeType = reflect.TypeOf((*BoostAttribute)(nil)).Elem()

// BoostAttribute carries a per-token boost factor. Token filters
// (notably DelimitedBoostTokenFilter) populate this attribute so
// downstream query construction can read the boost without going
// through the term text.
//
// This is the Go port of
// org.apache.lucene.search.BoostAttribute from Apache Lucene 10.4.0.
// It lives in the analysis package because BoostAttribute is owned
// by AttributeSource at the token-stream level; Gocene's
// search-package equivalent re-exports BoostAttribute as needed.
//
// Sprint 54 Phase 4 promoted this attribute from a bare struct to a
// Lucene-faithful interface+impl pair to match the conventions of
// every other Sprint 12 / Sprint 54 attribute. The concrete impl is
// [boostAttributeImpl].
type BoostAttribute interface {
	util.AttributeImpl

	// GetBoost returns the boost. Defaults to [DefaultBoost] until
	// SetBoost is called.
	GetBoost() float32

	// SetBoost replaces the boost.
	SetBoost(boost float32)
}

// boostAttributeImpl is the default [BoostAttribute] implementation.
type boostAttributeImpl struct {
	util.BaseAttributeImpl
	boost float32
}

// Compile-time assertions to lock in the contracts this impl
// participates in.
var (
	_ BoostAttribute                  = (*boostAttributeImpl)(nil)
	_ util.AttributeImpl              = (*boostAttributeImpl)(nil)
	_ util.AttributeInterfaceProvider = (*boostAttributeImpl)(nil)
)

// NewBoostAttribute returns an attribute initialised to [DefaultBoost].
func NewBoostAttribute() BoostAttribute {
	return &boostAttributeImpl{boost: DefaultBoost}
}

// SetBoost sets the boost.
func (b *boostAttributeImpl) SetBoost(boost float32) {
	b.boost = boost
}

// GetBoost returns the boost.
func (b *boostAttributeImpl) GetBoost() float32 {
	return b.boost
}

// Clear resets the boost to [DefaultBoost].
func (b *boostAttributeImpl) Clear() {
	b.boost = DefaultBoost
}

// End mirrors Lucene's default end() = clear() for impls without a
// distinct end-of-field state. [util.BaseAttributeImpl.End] is a no-op
// (the Go embedding cannot virtually dispatch to Clear) so an explicit
// override is required.
func (b *boostAttributeImpl) End() { b.Clear() }

// ReflectWith pushes the single (BoostAttribute, "boost", value) triple
// through reflector.
func (b *boostAttributeImpl) ReflectWith(reflector util.AttributeReflector) {
	reflector(BoostAttributeType, "boost", b.boost)
}

// CloneAttribute returns a deep clone of this impl.
func (b *boostAttributeImpl) CloneAttribute() util.AttributeImpl {
	return &boostAttributeImpl{boost: b.boost}
}

// CopyTo copies this attribute to target. Any target satisfying
// [BoostAttribute] is supported; mismatched targets are silently
// ignored, mirroring the Lucene fallback.
func (b *boostAttributeImpl) CopyTo(target util.AttributeImpl) {
	if t, ok := target.(BoostAttribute); ok {
		t.SetBoost(b.boost)
	}
}

// AttributeInterfaces satisfies [util.AttributeInterfaceProvider].
func (b *boostAttributeImpl) AttributeInterfaces() []reflect.Type {
	return []reflect.Type{BoostAttributeType}
}
