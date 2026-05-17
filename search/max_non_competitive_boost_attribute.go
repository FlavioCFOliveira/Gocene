// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// MaxNonCompetitiveBoostAttribute is used by MultiTermQuery rewrite methods
// to communicate the highest boost of a term that is not competitive yet.
//
// Mirrors org.apache.lucene.search.MaxNonCompetitiveBoostAttribute.
type MaxNonCompetitiveBoostAttribute interface {
	// SetMaxNonCompetitiveBoost records the new highest non-competitive boost.
	SetMaxNonCompetitiveBoost(maxNonCompetitiveBoost float32)
	// GetMaxNonCompetitiveBoost returns the highest non-competitive boost,
	// defaulting to negative infinity.
	GetMaxNonCompetitiveBoost() float32
	// SetCompetitiveTerm records the term that triggered the change.
	SetCompetitiveTerm(competitiveTerm []byte)
	// GetCompetitiveTerm returns the term that triggered the change, or nil.
	GetCompetitiveTerm() []byte
}

// MaxNonCompetitiveBoostAttributeImpl is the default MaxNonCompetitiveBoostAttribute
// implementation.
//
// Mirrors org.apache.lucene.search.MaxNonCompetitiveBoostAttributeImpl.
type MaxNonCompetitiveBoostAttributeImpl struct {
	maxNonCompetitiveBoost float32
	competitiveTerm        []byte
}

// NewMaxNonCompetitiveBoostAttributeImpl creates a new instance with the
// canonical defaults (NEGATIVE_INFINITY boost, nil term).
func NewMaxNonCompetitiveBoostAttributeImpl() *MaxNonCompetitiveBoostAttributeImpl {
	return &MaxNonCompetitiveBoostAttributeImpl{
		maxNonCompetitiveBoost: float32(math.Inf(-1)),
	}
}

// SetMaxNonCompetitiveBoost updates the boost threshold.
func (a *MaxNonCompetitiveBoostAttributeImpl) SetMaxNonCompetitiveBoost(v float32) {
	a.maxNonCompetitiveBoost = v
}

// GetMaxNonCompetitiveBoost returns the current threshold.
func (a *MaxNonCompetitiveBoostAttributeImpl) GetMaxNonCompetitiveBoost() float32 {
	return a.maxNonCompetitiveBoost
}

// SetCompetitiveTerm updates the recorded triggering term.
func (a *MaxNonCompetitiveBoostAttributeImpl) SetCompetitiveTerm(term []byte) {
	a.competitiveTerm = term
}

// GetCompetitiveTerm returns the triggering term.
func (a *MaxNonCompetitiveBoostAttributeImpl) GetCompetitiveTerm() []byte {
	return a.competitiveTerm
}

// Clear resets the attribute to its initial state.
func (a *MaxNonCompetitiveBoostAttributeImpl) Clear() {
	a.maxNonCompetitiveBoost = float32(math.Inf(-1))
	a.competitiveTerm = nil
}

// CopyTo copies this attribute's state into target.
func (a *MaxNonCompetitiveBoostAttributeImpl) CopyTo(target *MaxNonCompetitiveBoostAttributeImpl) {
	target.maxNonCompetitiveBoost = a.maxNonCompetitiveBoost
	target.competitiveTerm = a.competitiveTerm
}

// Compile-time assertion of interface conformance.
var _ MaxNonCompetitiveBoostAttribute = (*MaxNonCompetitiveBoostAttributeImpl)(nil)
