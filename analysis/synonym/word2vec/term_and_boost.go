// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package word2vec

import "github.com/FlavioCFOliveira/Gocene/util"

// TermAndBoost wraps a term and a boost value produced by the Word2Vec synonym
// search. The term bytes are deep-copied at construction to match the Java
// record's compact-constructor behaviour.
//
// This is the Go port of
// org.apache.lucene.analysis.synonym.word2vec.TermAndBoost from
// Apache Lucene 10.4.0.
type TermAndBoost struct {
	// Term is a deep copy of the matched term's bytes.
	Term *util.BytesRef

	// Boost is the cosine similarity score for this synonym.
	Boost float32
}

// NewTermAndBoost creates a TermAndBoost, deep-copying term.
func NewTermAndBoost(term *util.BytesRef, boost float32) *TermAndBoost {
	return &TermAndBoost{
		Term:  term.Clone(),
		Boost: boost,
	}
}
