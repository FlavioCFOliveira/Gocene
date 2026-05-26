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

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// NewLatLonPointBoxQuery creates a bounding-box query for indexed
// [document.LatLonPoint] fields.
//
// It is the Go port of Lucene 10.4.0's static factory
// org.apache.lucene.document.LatLonPoint#newBoxQuery (lucene/core/src/
// java/org/apache/lucene/document/LatLonPoint.java).
//
// The query matches all indexed points whose (latitude, longitude) falls
// within [minLatitude, maxLatitude] × [minLongitude, maxLongitude].
// Dateline-crossing boxes (maxLongitude < minLongitude) are handled by
// splitting the range into two [PointRangeQuery] leaves joined by a
// SHOULD [BooleanQuery] wrapped in [ConstantScoreQuery], matching the
// Java reference exactly.
//
// Special cases (matching the Java reference):
//   - minLatitude == 90.0 → [MatchNoDocsQuery] (90° cannot be encoded)
//   - minLongitude == maxLongitude == 180.0 → [MatchNoDocsQuery]
//     (180° cannot be encoded)
//   - minLongitude == 180.0 && maxLongitude < minLongitude →
//     minLongitude is corrected to -180.0 before encoding (dateline ceil)
//
// # Package placement
//
// The Java class places this factory on LatLonPoint in the document
// package. In Gocene the query family lives in search/ to avoid the
// document→search import cycle (see [NewLatLonPointDistanceQuery] and
// [NewLatLonPointQuery] for the same rationale).
func NewLatLonPointBoxQuery(
	field string,
	minLatitude, maxLatitude float64,
	minLongitude, maxLongitude float64,
) (Query, error) {
	// exact double values of lat=90.0D and lon=180.0D must be treated
	// specially as they cannot be represented in the encoding and would
	// drag in extra bogus junk — matching the Java reference comment.
	if minLatitude == 90.0 {
		return &MatchNoDocsQuery{reason: "LatLonPointBoxQuery with minLatitude=90.0"}, nil
	}
	if minLongitude == 180.0 {
		if maxLongitude == 180.0 {
			return &MatchNoDocsQuery{reason: "LatLonPointBoxQuery with minLongitude=maxLongitude=180.0"}, nil
		}
		if maxLongitude < minLongitude {
			// encodeCeil() with dateline wrapping: correct the min lon to
			// -180 so the left-open range starts from the global minimum.
			minLongitude = -180.0
		}
	}

	lower := document.EncodeLatLonCeil(minLatitude, minLongitude)
	upper := document.EncodeLatLon(maxLatitude, maxLongitude)

	// Crosses dateline: rewrite into SHOULD(left, right) with the
	// longitude dimension left open in each sub-range, then wrap in
	// ConstantScoreQuery to disable coord boosting (a multi-valued doc
	// could match both rects and get unfairly boosted otherwise).
	if maxLongitude < minLongitude {
		const bytesPerDim = 4 // LatLonPoint.BYTES

		// Left sub-range: lon in [INT_MIN, maxLon].
		leftOpen := make([]byte, 2*bytesPerDim)
		copy(leftOpen, lower)
		// Leave longitude dimension open: set lon bytes to INT_MIN.
		util.IntToSortableBytes(math.MinInt32, leftOpen, bytesPerDim)
		left, err := NewPointRangeQueryMultiDim(field, leftOpen, upper, 2)
		if err != nil {
			return nil, err
		}

		// Right sub-range: lon in [minLon, INT_MAX].
		rightOpen := make([]byte, 2*bytesPerDim)
		copy(rightOpen, upper)
		// Leave longitude dimension open: set lon bytes to INT_MAX.
		util.IntToSortableBytes(math.MaxInt32, rightOpen, bytesPerDim)
		right, err := NewPointRangeQueryMultiDim(field, lower, rightOpen, 2)
		if err != nil {
			return nil, err
		}

		bq := NewBooleanQuery()
		bq.Add(left, SHOULD)
		bq.Add(right, SHOULD)
		return NewConstantScoreQuery(bq), nil
	}

	return NewPointRangeQueryMultiDim(field, lower, upper, 2)
}
