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

// LongValues is an abstraction over an indexed sequence of int64 values.
// It is the Go analogue of Lucene's abstract class
// org.apache.lucene.util.LongValues. Implementations may be backed by a
// dense slice, a sparse mapping, a function, or any other source; the
// only contract is that Get(i) is deterministic for a given i.
//
// The Java abstract method is named "long get(long index)"; in Go we
// preserve the name and the int64 indexing.
type LongValues interface {
	// Get returns the value at the given index. The valid range of
	// indices is implementation-defined.
	Get(index int64) int64
}

// IdentityLongValues is the LongValues instance that returns its index.
// Mirrors LongValues.IDENTITY in Lucene.
//
//nolint:gochecknoglobals // mirrors the public static IDENTITY field in Lucene.
var IdentityLongValues LongValues = identityLongValues{}

// ZeroLongValues is the LongValues instance that always returns zero.
// Mirrors LongValues.ZEROES in Lucene.
//
//nolint:gochecknoglobals // mirrors the public static ZEROES field in Lucene.
var ZeroLongValues LongValues = zeroLongValues{}

type identityLongValues struct{}

func (identityLongValues) Get(index int64) int64 { return index }

type zeroLongValues struct{}

func (zeroLongValues) Get(index int64) int64 { return 0 }

// LongValuesFunc adapts a plain func(int64) int64 to the LongValues
// interface. This is the most common Go idiom for one-off "lambda"
// implementations of single-method interfaces and avoids forcing every
// caller to declare a named type just to expose a Get method.
type LongValuesFunc func(index int64) int64

// Get satisfies the LongValues interface.
func (f LongValuesFunc) Get(index int64) int64 { return f(index) }
