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

import "io"

// PointWriter appends many points and, at the end, provides a
// PointReader to iterate those points. Abstracts whether the
// implementation writes to disk or uses heap arrays. Port of
// org.apache.lucene.util.bkd.PointWriter (Lucene 10.4.0).
//
// In Java, both append overloads (byte[]+docID and PointValue) share
// the same name; Go does not allow overloading, so they are split
// into Append and AppendPointValue.
type PointWriter interface {
	io.Closer

	// Append adds a new point from the packed value and docID.
	Append(packedValue []byte, docID int) error

	// AppendPointValue adds a new point from a PointValue.
	AppendPointValue(pointValue PointValue) error

	// GetReader returns a PointReader iterator stepping through all
	// previously added points starting at startPoint with the given
	// length.
	GetReader(startPoint, length int64) (PointReader, error)

	// Count returns the number of points in this writer.
	Count() int64

	// Destroy removes any temp files backing this writer.
	Destroy() error
}
