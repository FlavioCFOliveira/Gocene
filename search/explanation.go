// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

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
