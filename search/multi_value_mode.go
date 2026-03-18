// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// MultiValueMode determines how to select a value from a multi-valued field.
// This is the Go port of Lucene's org.apache.lucene.search.MultiValueMode.
type MultiValueMode int

const (
	// MultiValueMin selects the minimum value
	MultiValueMin MultiValueMode = iota
	// MultiValueMax selects the maximum value
	MultiValueMax
	// MultiValueSum selects the sum of all values
	MultiValueSum
	// MultiValueAvg selects the average of all values
	MultiValueAvg
)

// String returns a string representation of the mode.
func (m MultiValueMode) String() string {
	switch m {
	case MultiValueMin:
		return "MIN"
	case MultiValueMax:
		return "MAX"
	case MultiValueSum:
		return "SUM"
	case MultiValueAvg:
		return "AVG"
	default:
		return "UNKNOWN"
	}
}
