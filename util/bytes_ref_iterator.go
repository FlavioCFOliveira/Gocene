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

// BytesRefIterator is a simple iterator interface for BytesRef iteration.
//
// Port of org.apache.lucene.util.BytesRefIterator from Apache Lucene 10.4.0.
// Java's checked IOException becomes an idiomatic Go error return; nil
// values for both BytesRef and error signal end of iteration.
//
// The returned BytesRef may be re-used across calls to Next. After Next
// returns (nil, nil) the iterator MUST NOT be called again; results are
// undefined.
type BytesRefIterator interface {
	// Next advances the iteration and returns the next BytesRef, or
	// (nil, nil) when the end of the iteration is reached. A non-nil
	// error indicates a low-level I/O failure.
	Next() (*BytesRef, error)
}

// BytesRefIteratorFunc adapts a plain function to the BytesRefIterator
// interface, mirroring Java's single-method functional interface use
// (e.g. the EMPTY singleton declared as a lambda).
type BytesRefIteratorFunc func() (*BytesRef, error)

// Next satisfies BytesRefIterator by invoking the underlying function.
func (f BytesRefIteratorFunc) Next() (*BytesRef, error) { return f() }

// EmptyBytesRefIterator is the singleton iterator that yields zero
// BytesRefs. Mirrors BytesRefIterator.EMPTY in Java.
//
//nolint:gochecknoglobals // mirrors the singleton in Lucene.
var EmptyBytesRefIterator BytesRefIterator = BytesRefIteratorFunc(func() (*BytesRef, error) {
	return nil, nil
})
