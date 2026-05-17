// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// SimpleFieldComparator is a base implementation used by FieldComparator
// subclasses that can use the same instance as their LeafFieldComparator
// across all leaves.
//
// Mirrors org.apache.lucene.search.SimpleFieldComparator. The hook
// DoSetNextReader is invoked once per leaf so subclasses can prepare
// reader-specific state without re-implementing GetLeafComparator.
type SimpleFieldComparator struct {
	// DoSetNextReader is invoked before each leaf is processed. Optional.
	DoSetNextReader func(ctx *index.LeafReaderContext) error
	// SetScorerHook is invoked once the scorer becomes available. Optional.
	SetScorerHook func(scorer Scorer) error
}

// GetLeafComparator invokes the optional reader-setup hook and returns the
// receiver as its own LeafFieldComparator-like view.
func (c *SimpleFieldComparator) GetLeafComparator(ctx *index.LeafReaderContext) (*SimpleFieldComparator, error) {
	if c.DoSetNextReader != nil {
		if err := c.DoSetNextReader(ctx); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// SetScorer delegates to the optional hook (no-op by default).
func (c *SimpleFieldComparator) SetScorer(scorer Scorer) error {
	if c.SetScorerHook != nil {
		return c.SetScorerHook(scorer)
	}
	return nil
}
