// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// TermStats carries the postings-level statistics for a single term, namely
// the document frequency and the total term frequency. It is the Go port of
// org.apache.lucene.codecs.TermStats from Apache Lucene 10.4.0.
//
// TermStats is a passive value type, exchanged between postings consumers and
// producers when writing or reading term dictionaries. The Java field types
// (int and long) are preserved exactly:
//   - DocFreq:       number of documents containing the term
//   - TotalTermFreq: sum of term occurrences across all documents
//
// Note that org.apache.lucene.search.TermStatistics is a distinct,
// search-layer type and lives in the search/ package as TermStatistics.
type TermStats struct {
	// DocFreq is the number of documents containing the term.
	DocFreq int
	// TotalTermFreq is the total number of occurrences of the term across
	// all documents (sum of per-doc freq).
	TotalTermFreq int64
}

// NewTermStats constructs a TermStats with the given counts.
func NewTermStats(docFreq int, totalTermFreq int64) TermStats {
	return TermStats{
		DocFreq:       docFreq,
		TotalTermFreq: totalTermFreq,
	}
}
