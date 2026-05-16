// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// SentenceAttribute is the Go port of Lucene's
// org.apache.lucene.analysis.tokenattributes.SentenceAttribute.
//
// SentenceAttribute tracks the sentence index a given token belongs to
// (and may carry other sentence-specific data in the future).
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/analysis/tokenattributes/SentenceAttribute.java
type SentenceAttribute interface {
	Attribute

	// GetSentenceIndex returns the sentence index for the current
	// token.
	GetSentenceIndex() int

	// SetSentenceIndex sets the sentence index for the current token.
	SetSentenceIndex(sentenceIndex int)
}
