// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import "fmt"

// SuggestionTimeoutError is returned (or panicked) when a Hunspell.Suggest
// call exceeds its time limit and TimeoutPolicyThrowException is used.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.SuggestionTimeoutException from
// Apache Lucene 10.4.0.
//
// Deviation: Java uses RuntimeException (an unchecked exception). Go uses a
// named error value; callers using TimeoutPolicyThrowException receive this
// error from the function return, not via panic.
type SuggestionTimeoutError struct {
	message       string
	partialResult []string
}

// NewSuggestionTimeoutError constructs a SuggestionTimeoutError.
func NewSuggestionTimeoutError(message string, partialResult []string) *SuggestionTimeoutError {
	var pr []string
	if partialResult != nil {
		cp := make([]string, len(partialResult))
		copy(cp, partialResult)
		pr = cp
	}
	return &SuggestionTimeoutError{message: message, partialResult: pr}
}

// Error implements the error interface.
func (e *SuggestionTimeoutError) Error() string {
	return fmt.Sprintf("suggestion timeout: %s", e.message)
}

// PartialResult returns the partial result accumulated before the timeout, or
// nil if none is available.
func (e *SuggestionTimeoutError) PartialResult() []string {
	if e.partialResult == nil {
		return nil
	}
	cp := make([]string, len(e.partialResult))
	copy(cp, e.partialResult)
	return cp
}
