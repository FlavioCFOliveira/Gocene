// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// SorterDocMap maps documents from the source order to the sorted order
// during an index sort. Mirrors org.apache.lucene.index.Sorter.DocMap from
// Apache Lucene 10.4.0.
//
// SorterDocMap was lifted into the SPI by rmp #4707 because the wide
// KnnVectorsWriter.Flush signature takes a Sorter.DocMap parameter — the
// SPI cannot reference index.SorterDocMap without creating a cycle, so the
// interface lives here as the canonical declaration. Package index keeps a
// type alias for source-level compatibility.
type SorterDocMap interface {
	// OldToNew returns the new doc ID for the given old doc ID, or -1 if
	// the source document was discarded.
	OldToNew(oldDocID int) int

	// NewToOld returns the old doc ID for the given new doc ID, or -1 if
	// the target slot is empty.
	NewToOld(newDocID int) int

	// Size returns the number of documents in the sorted view.
	Size() int
}
