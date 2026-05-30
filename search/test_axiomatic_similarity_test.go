// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/similarities/TestAxiomaticSimilarity.java
//
// Deviation: all test methods skipped — the Java tests assert that constructors
// panic (IllegalArgumentException) on invalid s/k/ql parameters. Gocene's
// NewLuceneAxiomaticF2EXP/F3EXP constructors do not currently validate
// parameters; adding validation is an API change that is out of scope for this
// porting task. Tracking as backlog item.

package search

import "testing"

// TestAxiomaticSimilarity_IllegalS mirrors testIllegalS.
// Java: AxiomaticF2EXP(±Inf, 0.1), AxiomaticF2EXP(-1, 0.1) → IllegalArgumentException.
// Gocene deviation: NewLuceneAxiomaticF2EXP does not validate s; deferred.
func TestAxiomaticSimilarity_IllegalS(t *testing.T) {
	t.Fatal("NewLuceneAxiomaticF2EXP does not yet validate the s parameter; porting validation is out of scope (deferred)")
}

// TestAxiomaticSimilarity_IllegalK mirrors testIllegalK.
func TestAxiomaticSimilarity_IllegalK(t *testing.T) {
	t.Fatal("NewLuceneAxiomaticF2EXP does not yet validate the k parameter; porting validation is out of scope (deferred)")
}

// TestAxiomaticSimilarity_IllegalQL mirrors testIllegalQL.
func TestAxiomaticSimilarity_IllegalQL(t *testing.T) {
	t.Fatal("NewLuceneAxiomaticF3EXP does not yet validate the queryLen parameter; porting validation is out of scope (deferred)")
}
