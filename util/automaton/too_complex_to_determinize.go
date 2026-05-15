// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.TooComplexToDeterminizeException
// from Apache Lucene 10.4.0 (Apache License 2.0).

package automaton

import (
	"errors"
	"fmt"
)

// TooComplexToDeterminizeError carries the failed automaton along with the
// work limit that was exceeded. Use errors.Is(err, ErrTooComplexToDeterminize)
// to detect the sentinel; use errors.As to extract this richer payload.
type TooComplexToDeterminizeError struct {
	Automaton            *Automaton
	DeterminizeWorkLimit int
}

// Error implements the error interface.
func (e *TooComplexToDeterminizeError) Error() string {
	if e.Automaton == nil {
		return fmt.Sprintf("automaton: determinizing would require more than %d effort.", e.DeterminizeWorkLimit)
	}
	return fmt.Sprintf("automaton: determinizing automaton with %d states and %d transitions would require more than %d effort.",
		e.Automaton.NumStates(), e.Automaton.NumTransitions(), e.DeterminizeWorkLimit)
}

// Unwrap allows errors.Is to discover the sentinel ErrTooComplexToDeterminize.
func (e *TooComplexToDeterminizeError) Unwrap() error { return ErrTooComplexToDeterminize }

// NewTooComplexToDeterminizeError builds a TooComplexToDeterminizeError; this
// is what call-sites should return when they refuse to determinize a hairy
// automaton.
func NewTooComplexToDeterminizeError(a *Automaton, workLimit int) error {
	return &TooComplexToDeterminizeError{Automaton: a, DeterminizeWorkLimit: workLimit}
}

// IsTooComplexToDeterminize is a convenience wrapper for errors.Is.
func IsTooComplexToDeterminize(err error) bool {
	return errors.Is(err, ErrTooComplexToDeterminize)
}
