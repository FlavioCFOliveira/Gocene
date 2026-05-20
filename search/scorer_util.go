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

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/ScorerUtil.java
//   (costWithMinShouldMatch only)

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/ScorerUtil.java

// CostWithMinShouldMatch estimates the iteration cost of a BooleanQuery
// with minShouldMatch clauses.
//
// The idea: a query c1,c2,...,cn with minShouldMatch=m can be rewritten as
//
//	(c1 AND (c2..cn|m-1)) OR (!c1 AND (c2..cn|m))
//
// Assuming clauses are sorted by ascending cost the recursion shows the
// cost equals the sum of the (numScorers - minShouldMatch + 1) least-costly
// scorers.
//
// Mirrors ScorerUtil.costWithMinShouldMatch (Lucene 10.4.0).
func CostWithMinShouldMatch(costs []int64, numScorers, minShouldMatch int) int64 {
	keep := numScorers - minShouldMatch + 1
	if keep <= 0 {
		return 0
	}
	// Max-heap capped at `keep` — we accumulate the keep smallest values.
	heap := make([]int64, 0, keep)
	heapSize := 0

	for _, c := range costs {
		if heapSize < keep {
			heap = append(heap, c)
			heapSize++
			// sift up
			i := heapSize - 1
			for i > 0 {
				p := (i - 1) / 2
				if heap[p] >= heap[i] {
					break
				}
				heap[p], heap[i] = heap[i], heap[p]
				i = p
			}
		} else if heapSize > 0 && c < heap[0] {
			// replace root (max) with smaller value
			heap[0] = c
			// sift down
			i := 0
			for {
				l := 2*i + 1
				r := 2*i + 2
				largest := i
				if l < heapSize && heap[l] > heap[largest] {
					largest = l
				}
				if r < heapSize && heap[r] > heap[largest] {
					largest = r
				}
				if largest == i {
					break
				}
				heap[i], heap[largest] = heap[largest], heap[i]
				i = largest
			}
		}
	}

	var sum int64
	for _, v := range heap[:heapSize] {
		sum += v
	}
	return sum
}

// MinRequiredScore computes a minimum required score such that
//
//	float32(util.MathSumUpperBound(minRequiredScore + maxRemainingScore, numScorers)) <= minCompetitiveScore.
//
// The computed value may not be the greatest value satisfying the condition;
// documents near the boundary may still be evaluated and filtered later.
//
// Mirrors ScorerUtil.minRequiredScore(double, float, int) (Lucene 10.4.0).
func MinRequiredScore(maxRemainingScore float64, minCompetitiveScore float32, numScorers int) float64 {
	minRequiredScore := float64(minCompetitiveScore) - maxRemainingScore
	// Use the float32 ulp (not float64 ulp) to converge faster, matching Java.
	subtraction := float64(math.Float32frombits(math.Float32bits(minCompetitiveScore)+1) - minCompetitiveScore)
	if subtraction <= 0 {
		// Fallback for edge cases (e.g. minCompetitiveScore == 0).
		subtraction = math.SmallestNonzeroFloat64
	}
	for minRequiredScore > 0 &&
		float32(util.MathSumUpperBound(minRequiredScore+maxRemainingScore, numScorers)) >= minCompetitiveScore {
		minRequiredScore -= subtraction
	}
	return minRequiredScore
}
