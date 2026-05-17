// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Scorable is allows access to the score of the current document being scored.
// This is the Go port of org.apache.lucene.search.Scorable.
//
// In Lucene, Scorer extends Scorable and Scorable provides the read-only API
// used by collectors. We mirror that shape here as a smaller interface.
type Scorable interface {
	// Score returns the score of the current document.
	Score() (float32, error)

	// SmoothingScore returns a smoothing score for the given document.
	// The default behavior returns 0.
	SmoothingScore(docID int) (float32, error)

	// SetMinCompetitiveScore allows a collector to hint at the minimum competitive
	// score that the scorer should produce. Implementations may use this to skip
	// non-competitive documents.
	SetMinCompetitiveScore(minScore float32) error

	// GetChildren returns the child scorables of this scorable, if any.
	GetChildren() ([]ChildScorable, error)
}

// ChildScorable mirrors Lucene's Scorable.ChildScorable record:
// a child Scorable plus its relationship label.
type ChildScorable struct {
	Child        Scorable
	Relationship string
}

// BaseScorable provides default no-op implementations of the optional
// Scorable methods so concrete scorers only need to implement Score.
type BaseScorable struct{}

// SmoothingScore returns 0 by default.
func (BaseScorable) SmoothingScore(docID int) (float32, error) { return 0, nil }

// SetMinCompetitiveScore is a no-op by default.
func (BaseScorable) SetMinCompetitiveScore(minScore float32) error { return nil }

// GetChildren returns no children by default.
func (BaseScorable) GetChildren() ([]ChildScorable, error) { return nil, nil }
