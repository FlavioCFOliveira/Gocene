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

// FSTReader abstracts the byte storage backing an FST. Implementations
// own a contiguous region of bytes (on-heap, off-heap, etc.) and
// expose a reverse-direction BytesReader plus the ability to copy
// the bytes back out to a DataOutput.
//
// Mirrors org.apache.lucene.util.fst.FSTReader. The Accountable
// contract from Lucene is expressed via the RAMBytesUsed method.
type FSTReader interface {
	// GetReverseBytesReader returns a BytesReader that reads the FST
	// bytes in reverse order, which is how the FST traversal code
	// consumes them.
	GetReverseBytesReader() BytesReader

	// WriteTo serialises the entire byte region back to a DataOutput.
	// Used when copying an FST between stores.
	WriteTo(out store.DataOutput) error

	// RAMBytesUsed reports the approximate heap footprint of this
	// reader.
	RAMBytesUsed() int64
}
