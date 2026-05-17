// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// Impact is the score contribution of a single document, expressed as the term
// frequency (Freq) and the encoded norm (Norm) at the time the impact was
// observed. It is the Go port of org.apache.lucene.codecs.Impact from
// Apache Lucene 10.4.0.
//
// Impact is value-typed by design (matching the Java record-like contract):
// callers compare and copy instances directly. Norm is a signed int64 so that
// the full Java long range — including negative encoded norms — round-trips
// without truncation.
//
// The (Freq, Norm) pair forms a partial order used to prune dominated impacts.
// Higher Freq combined with smaller unsigned-norm-distance contributes a
// higher BM25 score for any plausible parameter set; CompetitiveImpactAccumulator
// uses that relationship to keep only the impacts that can lead the score in a
// block.
type Impact struct {
	// Freq is the term frequency contributed by the document.
	Freq int
	// Norm is the encoded length norm (raw byte sign-extended to int64).
	Norm int64
}

// NewImpact constructs an Impact with the given frequency and norm. Provided
// for parity with the Java constructor; struct-literal construction is also
// supported.
func NewImpact(freq int, norm int64) Impact {
	return Impact{Freq: freq, Norm: norm}
}
