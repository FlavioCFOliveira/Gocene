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

package util

import "fmt"

// SelectorInterface is the dynamic-dispatch counterpart of Lucene's
// abstract Selector. Concrete selection algorithms (e.g. IntroSelector)
// embed Selector and require callers to provide an impl satisfying this
// interface.
type SelectorInterface interface {
	// Swap exchanges the values at slots i and j.
	Swap(i, j int)
	// Select reorders the range [from, to) so that the element at slot k
	// is the same as if all elements were sorted, with elements <= k to its
	// left and elements >= k to its right.
	Select(from, to, k int)
}

// Selector is the base type for selection algorithms, providing the shared
// argument checking that mirrors Lucene's Selector.checkArgs.
type Selector struct{}

// CheckArgs validates that k is inside the half-open range [from, to).
// The error semantics match the Java IllegalArgumentException messages.
func (Selector) CheckArgs(from, to, k int) {
	if k < from {
		panic(fmt.Sprintf("k must be >= from; k=%d from=%d", k, from))
	}
	if k >= to {
		panic(fmt.Sprintf("k must be < to; k=%d to=%d", k, to))
	}
}
