// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

// WeightedTerm is a (term, weight) pair used by the highlighter to rank
// terms during scoring. Mirrors org.apache.lucene.search.highlight.WeightedTerm.
type WeightedTerm struct {
	Weight float32
	Term   string
}

// NewWeightedTerm builds a WeightedTerm.
func NewWeightedTerm(weight float32, term string) *WeightedTerm {
	return &WeightedTerm{Weight: weight, Term: term}
}

// GetTerm returns the term text.
func (w *WeightedTerm) GetTerm() string { return w.Term }

// GetWeight returns the weight.
func (w *WeightedTerm) GetWeight() float32 { return w.Weight }

// SetWeight overrides the weight.
func (w *WeightedTerm) SetWeight(v float32) { w.Weight = v }
