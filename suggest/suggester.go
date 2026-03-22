// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package suggest provides search suggestion capabilities.
// This package implements various suggesters for auto-completion
// and search-as-you-type functionality.
package suggest

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Suggester is the base interface for all suggesters.
// It provides methods for lookup and building suggestions.
type Suggester interface {
	// Lookup suggests completions for the given prefix.
	Lookup(ctx context.Context, prefix string, num int) ([]*Suggestion, error)

	// Build builds the suggester from a data source.
	Build(ctx context.Context) error
}

// Suggestion represents a single suggestion result.
type Suggestion struct {
	// Term is the suggested term
	Term string

	// Score is the relevance score
	Score float32

	// Payload is optional additional data
	Payload string

	// Weight is the weight of this suggestion
	Weight int64
}

// NewSuggestion creates a new Suggestion.
func NewSuggestion(term string, weight int64) *Suggestion {
	return &Suggestion{
		Term:   term,
		Weight: weight,
		Score:  float32(weight),
	}
}

// String returns a string representation of this suggestion.
func (s *Suggestion) String() string {
	return fmt.Sprintf("Suggestion{term=%s, weight=%d}", s.Term, s.Weight)
}

// Ensure Suggestion implements Stringer
var _ fmt.Stringer = (*Suggestion)(nil)

// InMemorySuggester is a simple in-memory suggester.
type InMemorySuggester struct {
	suggestions map[string]*Suggestion
	mu          sync.RWMutex
}

// NewInMemorySuggester creates a new InMemorySuggester.
func NewInMemorySuggester() *InMemorySuggester {
	return &InMemorySuggester{
		suggestions: make(map[string]*Suggestion),
	}
}

// AddSuggestion adds a suggestion to the suggester.
func (s *InMemorySuggester) AddSuggestion(term string, weight int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.suggestions[term] = NewSuggestion(term, weight)
}

// Lookup suggests completions for the given prefix.
func (s *InMemorySuggester) Lookup(ctx context.Context, prefix string, num int) ([]*Suggestion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Suggestion
	for term, suggestion := range s.suggestions {
		if strings.HasPrefix(term, prefix) {
			results = append(results, suggestion)
		}
		if len(results) >= num {
			break
		}
	}

	return results, nil
}

// Build builds the suggester from a data source.
func (s *InMemorySuggester) Build(ctx context.Context) error {
	// Nothing to do for in-memory suggester
	return nil
}

// Ensure InMemorySuggester implements Suggester
var _ Suggester = (*InMemorySuggester)(nil)
