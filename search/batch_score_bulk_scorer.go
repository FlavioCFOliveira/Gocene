// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/BatchScoreBulkScorer.java

import "github.com/FlavioCFOliveira/Gocene/util"

// BatchScoreBulkScorer is a BulkScorer used when ScoreMode.needsScores()
// is true and the scorer's nextDocsAndScores has optimizations to run
// faster than one-by-one iteration.
//
// Mirrors org.apache.lucene.search.BatchScoreBulkScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java calls scorer.nextDocsAndScores(max, acceptDocs, buffer) to
//     fill a DocAndFloatFeatureBuffer in batches; this method is not on
//     Gocene's Scorer interface.  The implementation degrades to
//     one-by-one iteration via NextDoc, equivalent to the Java fallback
//     path (Weight.DefaultBulkScorer) that is taken when the collector
//     has a competitiveIterator.
//   - scorer.setMinCompetitiveScore and SimpleScorable feedback are not
//     available on Gocene's interfaces; these paths are omitted.
type BatchScoreBulkScorer struct {
	scorer Scorer
}

// NewBatchScoreBulkScorer constructs a BatchScoreBulkScorer wrapping scorer.
//
// Mirrors BatchScoreBulkScorer(Scorer).
func NewBatchScoreBulkScorer(scorer Scorer) *BatchScoreBulkScorer {
	return &BatchScoreBulkScorer{scorer: scorer}
}

// Score iterates over matching documents in [min, max), applying acceptDocs
// filtering, and collects each matching document with its score. It returns
// the first document the scorer advanced to at or beyond max.
//
// Mirrors BatchScoreBulkScorer.score(LeafCollector, Bits, int, int), degraded
// to one-by-one iteration (the nextDocsAndScores batch path is not available on
// Gocene's Scorer interface), which is equivalent to the DefaultBulkScorer
// fallback Lucene takes when the collector has a competitiveIterator.
func (bs *BatchScoreBulkScorer) Score(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	return NewDefaultBulkScorer(bs.scorer).Score(collector, acceptDocs, min, max)
}

// Cost returns the estimated number of documents this scorer will visit.
//
// Mirrors BatchScoreBulkScorer.cost().
func (bs *BatchScoreBulkScorer) Cost() int64 {
	return bs.scorer.Cost()
}

var _ BulkScorer = (*BatchScoreBulkScorer)(nil)
