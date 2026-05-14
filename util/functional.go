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
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

// This file groups the Go ports of Lucene's small @FunctionalInterface
// types declared in org.apache.lucene.util. Each Java interface with a
// single abstract method is represented in Go as a plain func type so
// that any compatible function literal can satisfy it directly. The
// originals are simple by design (mostly checked-exception variants of
// java.util.function.*); the Go ports preserve their names and shapes
// for ease of cross-referencing.

// FloatToFloatFunction is the Go port of
// org.apache.lucene.util.FloatToFloatFunction.
//
// Java declares it as a @FunctionalInterface with a single
// {@code float applyAsFloat(float value)} method. The Go counterpart is
// a function type with the same shape; callers invoke it as f(x).
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/FloatToFloatFunction.java
type FloatToFloatFunction func(value float32) float32

// IOBooleanSupplier is the Go port of
// org.apache.lucene.util.IOBooleanSupplier.
//
// Java declares it as a @FunctionalInterface whose single method
// {@code boolean get() throws IOException} produces a boolean and may
// raise an IOException. In Go the checked exception becomes an error
// return.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/IOBooleanSupplier.java
type IOBooleanSupplier func() (bool, error)
