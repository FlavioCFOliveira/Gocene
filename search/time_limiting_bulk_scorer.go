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
//   lucene/core/src/java/org/apache/lucene/search/TimeLimitingBulkScorer.java

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ErrTimeExceeded is returned by TimeLimitingBulkScorer.Score when the
// configured QueryTimeout signals that the query should stop.
//
// Mirrors org.apache.lucene.search.TimeLimitingBulkScorer.TimeExceededException.
// Java throws an unchecked RuntimeException; Go surfaces it as a sentinel error
// so callers can distinguish it with errors.Is.
var ErrTimeExceeded = errors.New("TimeLimit Exceeded")

// Interval is the number of document intervals scored per timeout check.
// After each interval the scorer consults the QueryTimeout. The interval
// grows by 50% on each call to amortise the check cost, mirroring the Java
// implementation.
//
// Mirrors TimeLimitingBulkScorer.INTERVAL.
const Interval = 100

// TimeLimitingBulkScorer wraps a BulkScorer and aborts scoring when the
// provided QueryTimeout signals that the time limit has been exceeded.
//
// Mirrors org.apache.lucene.search.TimeLimitingBulkScorer (Lucene 10.4.0).
//
// Deviations from Java:
//   - TimeExceededException (RuntimeException in Java) becomes ErrTimeExceeded
//     (a sentinel error) to fit Go's error-return convention.
type TimeLimitingBulkScorer struct {
	in           BulkScorer
	queryTimeout index.QueryTimeout
}

// NewTimeLimitingBulkScorer wraps bulkScorer with a timeout guard driven by
// queryTimeout.
//
// Mirrors TimeLimitingBulkScorer(BulkScorer, QueryTimeout).
func NewTimeLimitingBulkScorer(bulkScorer BulkScorer, queryTimeout index.QueryTimeout) *TimeLimitingBulkScorer {
	if bulkScorer == nil {
		panic("TimeLimitingBulkScorer: bulkScorer must not be nil")
	}
	if queryTimeout == nil {
		panic("TimeLimitingBulkScorer: queryTimeout must not be nil")
	}
	return &TimeLimitingBulkScorer{in: bulkScorer, queryTimeout: queryTimeout}
}

// Score scores [min, max) in exponentially-growing sub-ranges, consulting the
// QueryTimeout before each sub-range and returning ErrTimeExceeded if the time
// limit has been exceeded. It returns the next matching document on or after
// max otherwise.
//
// Faithful port of TimeLimitingBulkScorer.score(LeafCollector, Bits, int, int).
func (s *TimeLimitingBulkScorer) Score(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	interval := Interval
	for min < max {
		newMax := int(util.MathUnsignedMin(int32(min+interval), int32(max)))
		// Increase the interval by 50% on each iteration, with an overflow guard.
		newInterval := interval + (interval >> 1)
		if interval < newInterval {
			interval = newInterval
		}
		if s.queryTimeout.ShouldExit() {
			return 0, ErrTimeExceeded
		}
		var err error
		min, err = s.in.Score(collector, acceptDocs, min, newMax)
		if err != nil {
			return 0, err
		}
	}
	return min, nil
}

// Cost delegates to the inner scorer's cost.
//
// Mirrors TimeLimitingBulkScorer.cost().
func (s *TimeLimitingBulkScorer) Cost() int64 {
	return s.in.Cost()
}

var _ BulkScorer = (*TimeLimitingBulkScorer)(nil)
