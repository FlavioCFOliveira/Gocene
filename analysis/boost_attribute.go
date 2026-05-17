// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// DefaultBoost is the boost value returned by a BoostAttribute that
// has never been explicitly set.
const DefaultBoost float32 = 1.0

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
type BoostAttribute struct {
	Boost float32
}

// NewBoostAttribute returns an attribute initialised to DefaultBoost.
func NewBoostAttribute() *BoostAttribute {
	return &BoostAttribute{Boost: DefaultBoost}
}

// SetBoost sets the boost.
func (b *BoostAttribute) SetBoost(boost float32) {
	b.Boost = boost
}

// GetBoost returns the boost.
func (b *BoostAttribute) GetBoost() float32 {
	return b.Boost
}

// Clear resets the boost to DefaultBoost.
func (b *BoostAttribute) Clear() {
	b.Boost = DefaultBoost
}

// Copy returns a deep copy of this attribute.
func (b *BoostAttribute) Copy() AttributeImpl {
	return &BoostAttribute{Boost: b.Boost}
}

// CopyTo copies this attribute to another implementation.
func (b *BoostAttribute) CopyTo(target AttributeImpl) {
	if t, ok := target.(*BoostAttribute); ok {
		t.Boost = b.Boost
	}
}

// Ensure BoostAttribute implements AttributeImpl.
var _ AttributeImpl = (*BoostAttribute)(nil)
