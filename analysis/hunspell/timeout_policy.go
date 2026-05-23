// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

// TimeoutPolicy determines what to do when Hunspell API calls take too long.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.TimeoutPolicy from Apache Lucene 10.4.0.
type TimeoutPolicy int

const (
	// TimeoutPolicyNoTimeout lets the computation complete regardless of time.
	TimeoutPolicyNoTimeout TimeoutPolicy = iota
	// TimeoutPolicyReturnPartialResult stops calculation and returns whatever
	// has been computed so far.
	TimeoutPolicyReturnPartialResult
	// TimeoutPolicyThrowException signals a timeout via SuggestionTimeoutError.
	TimeoutPolicyThrowException
)

func (p TimeoutPolicy) String() string {
	switch p {
	case TimeoutPolicyNoTimeout:
		return "NO_TIMEOUT"
	case TimeoutPolicyReturnPartialResult:
		return "RETURN_PARTIAL_RESULT"
	case TimeoutPolicyThrowException:
		return "THROW_EXCEPTION"
	default:
		return "UNKNOWN"
	}
}
