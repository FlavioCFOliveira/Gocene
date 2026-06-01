// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "math"

// Explanation describes how the score for a document was computed.
type Explanation interface {
	// IsMatch returns true if this is a match.
	IsMatch() bool

	// GetValue returns the value of this explanation node.
	GetValue() float32

	// GetDescription returns the description of this explanation node.
	GetDescription() string

	// GetDetails returns the sub-explanations of this explanation node.
	GetDetails() []Explanation
}

// BaseExplanation provides a basic implementation of Explanation.
type BaseExplanation struct {
	match       bool
	value       float32
	description string
	details     []Explanation
}

// NewExplanation creates a new Explanation.
func NewExplanation(match bool, value float32, description string) *BaseExplanation {
	return &BaseExplanation{
		match:       match,
		value:       value,
		description: description,
		details:     make([]Explanation, 0),
	}
}

// IsMatch returns true if this is a match.
func (e *BaseExplanation) IsMatch() bool {
	return e.match
}

// GetValue returns the value of this explanation node.
func (e *BaseExplanation) GetValue() float32 {
	return e.value
}

// GetDescription returns the description of this explanation node.
func (e *BaseExplanation) GetDescription() string {
	return e.description
}

// GetDetails returns the sub-explanations of this explanation node.
func (e *BaseExplanation) GetDetails() []Explanation {
	return e.details
}

// AddDetail adds a sub-explanation.
func (e *BaseExplanation) AddDetail(detail Explanation) {
	e.details = append(e.details, detail)
}

// MatchExplanation creates a match explanation with the given value and description.
// Mirrors Explanation.match in Lucene.
func MatchExplanation(value float32, description string) *BaseExplanation {
	return NewExplanation(true, value, description)
}

// NoMatchExplanation creates a non-match explanation with the given description.
// Mirrors Explanation.noMatch in Lucene.
func NoMatchExplanation(description string) *BaseExplanation {
	return NewExplanation(false, 0, description)
}

// MatchExplanationWithDetails creates a match explanation with the given value,
// description and sub-explanations. Mirrors the variadic
// Explanation.match(value, description, details...) overload in Lucene.
func MatchExplanationWithDetails(value float32, description string, details ...Explanation) *BaseExplanation {
	e := NewExplanation(true, value, description)
	e.details = append(e.details, details...)
	return e
}

// NoMatchExplanationWithDetails creates a non-match explanation with the given
// description and sub-explanations. Mirrors the variadic
// Explanation.noMatch(description, details...) overload in Lucene.
func NoMatchExplanationWithDetails(description string, details ...Explanation) *BaseExplanation {
	e := NewExplanation(false, 0, description)
	e.details = append(e.details, details...)
	return e
}

// Equals reports whether e and other model the same explanation. It is a
// faithful port of Explanation.equals: the match flag, value, description and
// the ordered list of sub-explanations must all be equal (the latter compared
// recursively). Mirroring Lucene, only *BaseExplanation values are comparable;
// any other Explanation implementation is unequal.
func (e *BaseExplanation) Equals(other Explanation) bool {
	if e == nil || other == nil {
		return e == nil && other == nil
	}
	that, ok := other.(*BaseExplanation)
	if !ok {
		return false
	}
	if e == that {
		return true
	}
	if e.match != that.match || e.value != that.value || e.description != that.description {
		return false
	}
	if len(e.details) != len(that.details) {
		return false
	}
	for i := range e.details {
		left, ok := e.details[i].(*BaseExplanation)
		if !ok {
			// Fall back to a structural comparison for foreign implementations.
			if !explanationsEqual(e.details[i], that.details[i]) {
				return false
			}
			continue
		}
		if !left.Equals(that.details[i]) {
			return false
		}
	}
	return true
}

// HashCode returns a hash consistent with Equals, mirroring
// Explanation.hashCode (Objects.hash over match, value, description, details).
func (e *BaseExplanation) HashCode() int {
	if e == nil {
		return 0
	}
	h := 1
	if e.match {
		h = 31*h + 1231
	} else {
		h = 31*h + 1237
	}
	h = 31*h + int(math.Float32bits(e.value))
	h = 31*h + stringHash(e.description)
	detailHash := 1
	for _, d := range e.details {
		dh := 0
		if be, ok := d.(*BaseExplanation); ok {
			dh = be.HashCode()
		}
		detailHash = 31*detailHash + dh
	}
	h = 31*h + detailHash
	return h
}

// explanationsEqual compares two Explanation values structurally without
// requiring the concrete *BaseExplanation type, used as a fallback for foreign
// Explanation implementations encountered while comparing sub-details.
func explanationsEqual(a, b Explanation) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.IsMatch() != b.IsMatch() || a.GetValue() != b.GetValue() || a.GetDescription() != b.GetDescription() {
		return false
	}
	ad, bd := a.GetDetails(), b.GetDetails()
	if len(ad) != len(bd) {
		return false
	}
	for i := range ad {
		if !explanationsEqual(ad[i], bd[i]) {
			return false
		}
	}
	return true
}
