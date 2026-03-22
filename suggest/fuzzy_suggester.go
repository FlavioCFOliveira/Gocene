// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

import (
	"context"
	"fmt"
	"sync"
)

// FuzzySuggester is a suggester that matches terms with fuzzy matching.
// It allows suggestions that are similar to the input prefix using edit distance.
type FuzzySuggester struct {
	*InMemorySuggester
	maxEdits int
	mu       sync.RWMutex
}

// NewFuzzySuggester creates a new FuzzySuggester.
func NewFuzzySuggester() *FuzzySuggester {
	return &FuzzySuggester{
		InMemorySuggester: NewInMemorySuggester(),
		maxEdits:          1, // Default to 1 edit
	}
}

// SetMaxEdits sets the maximum edit distance for fuzzy matching.
func (s *FuzzySuggester) SetMaxEdits(maxEdits int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxEdits = maxEdits
}

// Lookup suggests completions using fuzzy matching.
func (s *FuzzySuggester) Lookup(ctx context.Context, prefix string, num int) ([]*Suggestion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*Suggestion
	for term, suggestion := range s.suggestions {
		if s.fuzzyMatch(prefix, term, s.maxEdits) {
			results = append(results, suggestion)
		}
		if len(results) >= num {
			break
		}
	}

	return results, nil
}

// fuzzyMatch checks if target matches source within maxEdits edit distance.
func (s *FuzzySuggester) fuzzyMatch(source, target string, maxEdits int) bool {
	// Simple implementation of Levenshtein distance with early termination
	if len(target) < len(source)-maxEdits || len(target) > len(source)+maxEdits {
		return false
	}

	distance := s.levenshteinDistance(source, target)
	return distance <= maxEdits
}

// levenshteinDistance calculates the Levenshtein distance between two strings.
func (s *FuzzySuggester) levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Use dynamic programming
	previous := make([]int, len(s1)+1)
	current := make([]int, len(s1)+1)

	for i := 0; i <= len(s1); i++ {
		previous[i] = i
	}

	for j := 1; j <= len(s2); j++ {
		current[0] = j
		for i := 1; i <= len(s1); i++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			deletion := previous[i] + 1
			insertion := current[i-1] + 1
			substitution := previous[i-1] + cost
			min := deletion
			if insertion < min {
				min = insertion
			}
			if substitution < min {
				min = substitution
			}
			current[i] = min
		}
		previous, current = current, previous
	}

	return previous[len(s1)]
}

// String returns a string representation of this suggester.
func (s *FuzzySuggester) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fmt.Sprintf("FuzzySuggester{maxEdits=%d}", s.maxEdits)
}

// Ensure FuzzySuggester implements Suggester
var _ Suggester = (*FuzzySuggester)(nil)
