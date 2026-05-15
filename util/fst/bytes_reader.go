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

package fst

import "github.com/FlavioCFOliveira/Gocene/store"

// BytesReader is the Go counterpart of Lucene's
// org.apache.lucene.util.fst.FST.BytesReader — an extended DataInput
// (with VInt/VLong support) whose absolute position can be queried
// and reset. The FST format stores arcs and node tables at known
// offsets within the backing byte stream; readers may seek backward
// (reverse readers) or forward depending on the storage layout.
type BytesReader interface {
	store.DataInput
	store.VariableLengthInput

	// GetPosition returns the current read position.
	GetPosition() int64

	// SetPosition seeks the reader to the given position.
	SetPosition(pos int64)

	// SkipBytes advances the reader position by n bytes in the
	// reader's natural direction. For reverse readers (which iterate
	// "backward" through the underlying stream) a positive n moves
	// the underlying position backward — that is, "forward" in the
	// reverse iteration order — so internally r.pos -= n. Negative
	// values are allowed and skip in the opposite direction; this
	// matches Lucene's contract (BitTableUtil.previousBitSet calls
	// reader.skipBytes(-2)).
	SkipBytes(n int64) error
}
