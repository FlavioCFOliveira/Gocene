// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

import (
	"context"
	"fmt"
	"sync"
)

// BlendedInfixSuggester blends infix suggestions from multiple sources.
// It combines suggestions with weights from different fields or sources.
type BlendedInfixSuggester struct {
	suggesters []Suggester
	mu         sync.RWMutex
}

// NewBlendedInfixSuggester creates a new BlendedInfixSuggester.
func NewBlendedInfixSuggester() *BlendedInfixSuggester {
	return &BlendedInfixSuggester{
		suggesters: make([]Suggester, 0),
	}
}

// AddSuggester adds a suggester to the blend.
func (s *BlendedInfixSuggester) AddSuggester(suggester Suggester) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.suggesters = append(s.suggesters, suggester)
}

// Lookup suggests completions by blending results from all suggesters.
func (s *BlendedInfixSuggester) Lookup(ctx context.Context, prefix string, num int) ([]*Suggestion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect suggestions from all suggesters
	allSuggestions := make(map[string]*Suggestion)
	for _, suggester := range s.suggesters {
		suggestions, err := suggester.Lookup(ctx, prefix, num)
		if err != nil {
			return nil, err
		}
		for _, sug := range suggestions {
			if existing, ok := allSuggestions[sug.Term]; ok {
				// Blend scores
				existing.Score = (existing.Score + sug.Score) / 2
				if sug.Weight > existing.Weight {
					existing.Weight = sug.Weight
				}
			} else {
				// Make a copy
				allSuggestions[sug.Term] = &Suggestion{
					Term:    sug.Term,
					Score:   sug.Score,
					Payload: sug.Payload,
					Weight:  sug.Weight,
				}
			}
		}
	}

	// Convert to slice
	var results []*Suggestion
	for _, sug := range allSuggestions {
		results = append(results, sug)
		if len(results) >= num {
			break
		}
	}

	return results, nil
}

// Build builds all underlying suggesters.
func (s *BlendedInfixSuggester) Build(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, suggester := range s.suggesters {
		if err := suggester.Build(ctx); err != nil {
			return err
		}
	}
	return nil
}

// String returns a string representation of this suggester.
func (s *BlendedInfixSuggester) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fmt.Sprintf("BlendedInfixSuggester{suggesters=%d}", len(s.suggesters))
}

// Ensure BlendedInfixSuggester implements Suggester
var _ Suggester = (*BlendedInfixSuggester)(nil)
