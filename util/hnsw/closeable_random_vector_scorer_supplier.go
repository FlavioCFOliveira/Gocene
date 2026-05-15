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

package hnsw

import "io"

// CloseableRandomVectorScorerSupplier is a RandomVectorScorerSupplier
// that owns external resources and must be closed by the caller.
// Port of org.apache.lucene.util.hnsw.CloseableRandomVectorScorerSupplier
// (Lucene 10.4.0).
//
// NOTE: the RandomVectorScorerSupplier returned by Copy() is not
// necessarily closeable; only the original supplier is guaranteed
// to be.
type CloseableRandomVectorScorerSupplier interface {
	io.Closer
	RandomVectorScorerSupplier

	// TotalVectorCount returns the total number of vectors known to
	// this supplier.
	TotalVectorCount() int
}
