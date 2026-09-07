// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// Impacts conveys information about upcoming impacts (i.e. (freq, norm)
// pairs that may trigger non-zero scores) within a postings list. Mirrors
// org.apache.lucene.index.Impacts from Apache Lucene 10.4.0.
type Impacts interface {
	// NumLevels returns the number of levels of impact summary information.
	// Always > 0 and may differ across positions in the same postings list.
	NumLevels() int

	// GetDocIDUpTo returns the maximum inclusive doc ID up to which the
	// impacts returned by GetImpacts(level) are valid. Non-decreasing in level.
	GetDocIDUpTo(level int) int

	// GetImpacts returns the (freq, norm) impacts for the given level. The
	// returned buffer is never empty and is only guaranteed to be valid until
	// the iterator advances.
	GetImpacts(level int) *FreqAndNormBuffer
}
