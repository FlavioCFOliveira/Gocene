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
}
