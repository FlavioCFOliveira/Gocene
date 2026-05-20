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
//   lucene/core/src/java/org/apache/lucene/search/MaxScoreAccumulator.java

import (
	"math"
	"sync/atomic"
)

// maxScoreAccumulator maintains the maximum (score, docID) pair across concurrent searchers.
// The pair is packed into a single int64 via docScoreEncoderEncode so that an atomic max
// suffices — no lock is required.
type maxScoreAccumulator struct {
	// acc holds the current maximum encoded (score, docID) value.
	// Initialized to math.MinInt64 so any real accumulation wins immediately.
	acc int64

	// modInterval is the bitmask used by callers to decide how often to call accumulate.
	// Default: 0x3ff (every 1024 documents).
	modInterval int64
}

// newMaxScoreAccumulator returns a new maxScoreAccumulator ready for use.
func newMaxScoreAccumulator() *maxScoreAccumulator {
	return &maxScoreAccumulator{
		acc:         math.MinInt64,
		modInterval: 0x3ff,
	}
}

// accumulate atomically updates the stored maximum with code using a lock-free CAS loop.
// code must have been produced by docScoreEncoderEncode.
func (m *maxScoreAccumulator) accumulate(code int64) {
	for {
		cur := atomic.LoadInt64(&m.acc)
		if code <= cur {
			return
		}
		if atomic.CompareAndSwapInt64(&m.acc, cur, code) {
			return
		}
	}
}

// getRaw returns the raw encoded int64 currently stored in the accumulator.
func (m *maxScoreAccumulator) getRaw() int64 {
	return atomic.LoadInt64(&m.acc)
}
