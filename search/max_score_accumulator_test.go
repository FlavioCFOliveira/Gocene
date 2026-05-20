// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestMaxScoreAccumulator.java

package search

import "testing"

func TestMaxScoreAccumulator_Simple(t *testing.T) {
	acc := newMaxScoreAccumulator()

	acc.accumulate(docScoreEncoderEncode(0, 0))
	if got := docScoreEncoderToScore(acc.getRaw()); got != 0 {
		t.Fatalf("score: want 0, got %v", got)
	}
	if got := docScoreEncoderDocID(acc.getRaw()); got != 0 {
		t.Fatalf("docID: want 0, got %v", got)
	}

	// Same score, higher docID — lower docID (0) must win.
	acc.accumulate(docScoreEncoderEncode(10, 0))
	if got := docScoreEncoderToScore(acc.getRaw()); got != 0 {
		t.Fatalf("score: want 0, got %v", got)
	}
	if got := docScoreEncoderDocID(acc.getRaw()); got != 0 {
		t.Fatalf("docID: want 0, got %v", got)
	}

	// Higher score — must replace.
	acc.accumulate(docScoreEncoderEncode(100, 1000))
	if got := docScoreEncoderToScore(acc.getRaw()); got != 1000 {
		t.Fatalf("score: want 1000, got %v", got)
	}
	if got := docScoreEncoderDocID(acc.getRaw()); got != 100 {
		t.Fatalf("docID: want 100, got %v", got)
	}

	// Lower score — must not replace.
	acc.accumulate(docScoreEncoderEncode(1000, 5))
	if got := docScoreEncoderToScore(acc.getRaw()); got != 1000 {
		t.Fatalf("score: want 1000, got %v", got)
	}
	if got := docScoreEncoderDocID(acc.getRaw()); got != 100 {
		t.Fatalf("docID: want 100, got %v", got)
	}

	// Same score, lower docID — must replace.
	acc.accumulate(docScoreEncoderEncode(99, 1000))
	if got := docScoreEncoderToScore(acc.getRaw()); got != 1000 {
		t.Fatalf("score: want 1000, got %v", got)
	}
	if got := docScoreEncoderDocID(acc.getRaw()); got != 99 {
		t.Fatalf("docID: want 99, got %v", got)
	}

	// Higher score, higher docID — must replace (score wins).
	acc.accumulate(docScoreEncoderEncode(1000, 1001))
	if got := docScoreEncoderToScore(acc.getRaw()); got != 1001 {
		t.Fatalf("score: want 1001, got %v", got)
	}
	if got := docScoreEncoderDocID(acc.getRaw()); got != 1000 {
		t.Fatalf("docID: want 1000, got %v", got)
	}

	// Same score, lower docID — must replace.
	acc.accumulate(docScoreEncoderEncode(10, 1001))
	if got := docScoreEncoderToScore(acc.getRaw()); got != 1001 {
		t.Fatalf("score: want 1001, got %v", got)
	}
	if got := docScoreEncoderDocID(acc.getRaw()); got != 10 {
		t.Fatalf("docID: want 10, got %v", got)
	}

	// Same score, higher docID — must not replace.
	acc.accumulate(docScoreEncoderEncode(100, 1001))
	if got := docScoreEncoderToScore(acc.getRaw()); got != 1001 {
		t.Fatalf("score: want 1001, got %v", got)
	}
	if got := docScoreEncoderDocID(acc.getRaw()); got != 10 {
		t.Fatalf("docID: want 10, got %v", got)
	}
}
