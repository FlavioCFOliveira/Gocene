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

// TooComplexToDeterminizeError carries the failed automaton, the optional
// RegExp that triggered the failure, and the work limit that was exceeded.
// Use errors.Is(err, ErrTooComplexToDeterminize) to detect the sentinel; use
// errors.As to extract this richer payload.
//
// This is the Go port of
// org.apache.lucene.util.automaton.TooComplexToDeterminizeException.
type TooComplexToDeterminizeError struct {
	// Automaton is the automaton that caused this exception, if any.
	Automaton *Automaton
	// RegExp is the RegExp that caused this exception, if any. Only set when
	// the failure originated while converting a RegExp to an automaton.
	RegExp *RegExp
	// DeterminizeWorkLimit is the maximum allowed determinize effort that was
	// exceeded.
	DeterminizeWorkLimit int
}

// Error implements the error interface, mirroring the message produced by the
// Java reference for both constructor variants.
func (e *TooComplexToDeterminizeError) Error() string {
	if e.RegExp != nil {
		return fmt.Sprintf("automaton: determinizing %s would require more than %d effort.",
			e.RegExp.GetOriginalString(), e.DeterminizeWorkLimit)
	}
	if e.Automaton == nil {
		return fmt.Sprintf("automaton: determinizing would require more than %d effort.", e.DeterminizeWorkLimit)
	}
	return fmt.Sprintf("automaton: determinizing automaton with %d states and %d transitions would require more than %d effort.",
		e.Automaton.NumStates(), e.Automaton.NumTransitions(), e.DeterminizeWorkLimit)
}

// Unwrap allows errors.Is to discover the sentinel ErrTooComplexToDeterminize.
func (e *TooComplexToDeterminizeError) Unwrap() error { return ErrTooComplexToDeterminize }

// GetAutomaton returns the automaton that caused this exception, if any.
// Provided for symmetry with the Java reference's getAutomaton().
func (e *TooComplexToDeterminizeError) GetAutomaton() *Automaton { return e.Automaton }

// GetRegExp returns the RegExp that caused this exception, if any. Provided
// for symmetry with the Java reference's getRegExp().
func (e *TooComplexToDeterminizeError) GetRegExp() *RegExp { return e.RegExp }

// GetDeterminizeWorkLimit returns the maximum allowed determinize effort.
// Provided for symmetry with the Java reference's getDeterminizeWorkLimit().
func (e *TooComplexToDeterminizeError) GetDeterminizeWorkLimit() int { return e.DeterminizeWorkLimit }

// NewTooComplexToDeterminizeError builds a TooComplexToDeterminizeError;
// this is what call-sites should return when they refuse to determinize a
// hairy automaton. Mirrors the Java
// TooComplexToDeterminizeException(Automaton, int) constructor.
func NewTooComplexToDeterminizeError(a *Automaton, workLimit int) error {
	return &TooComplexToDeterminizeError{Automaton: a, DeterminizeWorkLimit: workLimit}
}

// NewTooComplexToDeterminizeRegExpError wraps an underlying
// TooComplexToDeterminizeError with the RegExp that triggered it, preserving
// the original automaton and work limit. Mirrors the Java
// TooComplexToDeterminizeException(RegExp, TooComplexToDeterminizeException)
// constructor. If cause is nil this returns nil.
func NewTooComplexToDeterminizeRegExpError(r *RegExp, cause *TooComplexToDeterminizeError) error {
	if cause == nil {
		return nil
	}
	return &TooComplexToDeterminizeError{
		Automaton:            cause.Automaton,
		RegExp:               r,
		DeterminizeWorkLimit: cause.DeterminizeWorkLimit,
	}
}

// IsTooComplexToDeterminize is a convenience wrapper for errors.Is.
func IsTooComplexToDeterminize(err error) bool {
	return errors.Is(err, ErrTooComplexToDeterminize)
}
