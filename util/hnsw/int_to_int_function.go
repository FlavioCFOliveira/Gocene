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

// IntToIntFunction is the native int-to-int function. Port of
// org.apache.lucene.util.hnsw.IntToIntFunction (Lucene 10.4.0).
//
// Lucene exposes a single-method functional interface. In Go we
// expose it as both an interface (for receiver types that want to
// implement the contract explicitly) and as a free function type
// (IntToIntFunc) for lightweight closure-style use.
type IntToIntFunction interface {
	Apply(v int) int
}

// IntToIntFunc adapts an ordinary Go function to the
// IntToIntFunction interface, the equivalent of a Java lambda.
type IntToIntFunc func(v int) int

// Apply implements IntToIntFunction.
func (f IntToIntFunc) Apply(v int) int { return f(v) }
