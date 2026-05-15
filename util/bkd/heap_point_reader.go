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

package bkd

// HeapPointReader reads buffered points from an in-heap index-based
// source. Port of org.apache.lucene.util.bkd.HeapPointReader
// (Lucene 10.4.0). The source is exposed as a Go function that
// returns the PointValue at a given index, mirroring Java's
// IntFunction<PointValue>.
type HeapPointReader struct {
	curRead int
	end     int
	points  func(int) PointValue
}

// NewHeapPointReader builds a reader iterating points in the
// half-open range [start, end) of the supplied index-based source.
// The first Next() call advances to start; pointValue() returns the
// point at that index.
func NewHeapPointReader(points func(int) PointValue, start, end int) *HeapPointReader {
	return &HeapPointReader{
		curRead: start - 1,
		end:     end,
		points:  points,
	}
}

// Next advances to the next point. Returns false once iteration is
// done, else true. Never returns an error in this implementation.
func (r *HeapPointReader) Next() (bool, error) {
	r.curRead++
	return r.curRead < r.end, nil
}

// PointValue returns the current point, looked up from the source
// function. Calling PointValue() before the first successful Next()
// produces an out-of-range index for the underlying source.
func (r *HeapPointReader) PointValue() PointValue {
	return r.points(r.curRead)
}

// Close is a no-op; the heap reader holds no system resources.
func (r *HeapPointReader) Close() error { return nil }
