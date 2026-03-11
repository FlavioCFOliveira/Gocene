// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// IndexOptions controls how much information is stored in the postings lists.
// This determines what information is available for scoring and highlighting.
type IndexOptions int

const (
	// IndexOptionsNone means the field is not indexed at all.
	IndexOptionsNone IndexOptions = iota

	// IndexOptionsDocs means only the document IDs are indexed.
	// This is useful for fields that need to be searchable but don't need
	// scoring information (e.g., filter fields).
	IndexOptionsDocs

	// IndexOptionsDocsAndFreqs means document IDs and term frequencies are indexed.
	// This allows for scoring based on term frequency but doesn't store
	// positional information.
	IndexOptionsDocsAndFreqs

	// IndexOptionsDocsAndFreqsAndPositions means document IDs, term frequencies,
	// and term positions are indexed. This enables phrase queries and positional
	// queries but uses more space.
	IndexOptionsDocsAndFreqsAndPositions

	// IndexOptionsDocsAndFreqsAndPositionsAndOffsets means all information is indexed:
	// document IDs, term frequencies, positions, and character offsets.
	// This enables highlighting and span queries.
	IndexOptionsDocsAndFreqsAndPositionsAndOffsets
)

// String returns the string representation of the IndexOptions.
func (io IndexOptions) String() string {
	switch io {
	case IndexOptionsNone:
		return "NONE"
	case IndexOptionsDocs:
		return "DOCS"
	case IndexOptionsDocsAndFreqs:
		return "DOCS_AND_FREQS"
	case IndexOptionsDocsAndFreqsAndPositions:
		return "DOCS_AND_FREQS_AND_POSITIONS"
	case IndexOptionsDocsAndFreqsAndPositionsAndOffsets:
		return "DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", io)
	}
}

// IsIndexed returns true if the field is indexed (has any index options other than NONE).
func (io IndexOptions) IsIndexed() bool {
	return io != IndexOptionsNone
}

// HasFreqs returns true if term frequencies are stored.
func (io IndexOptions) HasFreqs() bool {
	return io >= IndexOptionsDocsAndFreqs
}

// HasPositions returns true if term positions are stored.
func (io IndexOptions) HasPositions() bool {
	return io >= IndexOptionsDocsAndFreqsAndPositions
}

// HasOffsets returns true if term offsets are stored.
func (io IndexOptions) HasOffsets() bool {
	return io >= IndexOptionsDocsAndFreqsAndPositionsAndOffsets
}
