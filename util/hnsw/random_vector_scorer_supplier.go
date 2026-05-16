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

// RandomVectorScorerSupplier supplies UpdateableRandomVectorScorer
// instances from a vector ordinal. Port of
// org.apache.lucene.util.hnsw.RandomVectorScorerSupplier
// (Lucene 10.4.0).
type RandomVectorScorerSupplier interface {
	// Scorer creates an UpdateableRandomVectorScorer for scoring
	// random nodes in batches against an ordinal.
	Scorer() (UpdateableRandomVectorScorer, error)

	// Copy returns a copy of the supplier whose underlying
	// vectorValues are independent so the copy is safe to be used
	// from another goroutine.
	Copy() (RandomVectorScorerSupplier, error)
}
