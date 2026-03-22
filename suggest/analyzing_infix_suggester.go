// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

import (
	"context"
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// AnalyzingInfixSuggester is a suggester that matches terms by infix (substring).
// It analyzes input text and matches against indexed terms.
type AnalyzingInfixSuggester struct {
	*InMemorySuggester
	analyzer analysis.Analyzer
	index    map[string][]string // maps tokens to original terms
	mu       sync.RWMutex
}

// NewAnalyzingInfixSuggester creates a new AnalyzingInfixSuggester.
func NewAnalyzingInfixSuggester(analyzer analysis.Analyzer) *AnalyzingInfixSuggester {
	return &AnalyzingInfixSuggester{
		InMemorySuggester: NewInMemorySuggester(),
		analyzer:          analyzer,
		index:             make(map[string][]string),
	}
}

// AddSuggestion adds a suggestion with analysis.
func (s *AnalyzingInfixSuggester) AddSuggestion(term string, weight int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.InMemorySuggester.AddSuggestion(term, weight)

	// Index tokens for infix matching
	// For simplicity, we just split into n-grams
	for i := 0; i < len(term); i++ {
		for j := i + 1; j <= len(term) && j <= i+10; j++ {
			token := term[i:j]
			s.index[token] = append(s.index[token], term)
		}
	}
}

// Lookup suggests completions for the given prefix.
func (s *AnalyzingInfixSuggester) Lookup(ctx context.Context, prefix string, num int) ([]*Suggestion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check if we have any matches for this prefix
	terms := s.index[prefix]
	if len(terms) == 0 {
		return []*Suggestion{}, nil
	}

	// Get unique suggestions
	seen := make(map[string]bool)
	var results []*Suggestion
	for _, term := range terms {
		if seen[term] {
			continue
		}
		seen[term] = true
		if suggestion, ok := s.suggestions[term]; ok {
			results = append(results, suggestion)
		}
		if len(results) >= num {
			break
		}
	}

	return results, nil
}

// String returns a string representation of this suggester.
func (s *AnalyzingInfixSuggester) String() string {
	return fmt.Sprintf("AnalyzingInfixSuggester{indexSize=%d}", len(s.index))
}

// Ensure AnalyzingInfixSuggester implements Suggester
var _ Suggester = (*AnalyzingInfixSuggester)(nil)
