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
//   lucene/core/src/java/org/apache/lucene/search/DocScoreEncoder.java

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// docScoreEncoderLeastCompetitiveCode is the encoded value for (Integer.MAX_VALUE, Float.NEGATIVE_INFINITY).
// Any real (docID, score) pair will encode to a value greater than or equal to this.
const docScoreEncoderLeastCompetitiveCode = int64(math.MinInt64)

// docScoreEncoderEncode packs a (docID, score) pair into a single int64 value such that
// comparing two encoded values by int64 ordering yields the best pair first:
//   - higher score → larger encoded value
//   - equal score and lower docID → larger encoded value
//
// Upper 32 bits: FloatToSortableInt(score) — sortable encoding maps higher scores to higher ints.
// Lower 32 bits: math.MaxInt32 - docID — inverted so lower docIDs win on a tie.
func docScoreEncoderEncode(docID int32, score float32) int64 {
	sortableScore := int64(util.FloatToSortableInt(score))
	return (sortableScore << 32) | int64(math.MaxInt32-docID)
}

// docScoreEncoderToScore extracts the score from an encoded value.
func docScoreEncoderToScore(value int64) float32 {
	return util.SortableIntToFloat(int32(value >> 32))
}

// docScoreEncoderDocID extracts the document ID from an encoded value.
func docScoreEncoderDocID(value int64) int32 {
	return math.MaxInt32 - int32(value)
}
