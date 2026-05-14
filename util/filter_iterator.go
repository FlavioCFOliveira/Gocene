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

// FilterIterator is the Go port of org.apache.lucene.util.FilterIterator.
//
// It wraps another Iterator and filters its elements through an Accept
// predicate. Only elements for which Accept returns true are yielded by
// HasNext / Next. The underlying Iterator is consumed lazily: each call
// to HasNext advances the wrapped iterator until either an accepted
// element is found or the wrapped iterator is exhausted.
//
// Removal of elements is not supported, matching the Java
// UnsupportedOperationException behavior; Remove panics.
//
// The wrapped iterator and the predicate must not be nil; passing nil
// values yields a FilterIterator whose Next will panic on first use,
// mirroring the NullPointerException semantics of the Java reference.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/FilterIterator.java
type FilterIterator[T any] struct {
	iterator Iterator[T]
	accept   func(T) bool
	next     T
	hasNext  bool
}

// The Iterator interface used by FilterIterator is defined in
// merged_iterator.go and matches Java's java.util.Iterator contract:
// HasNext() reports whether more elements remain; Next() returns the next
// element (or the zero value of T when no element is available, with
// HasNext() returning false).

// NewFilterIterator constructs a FilterIterator that wraps baseIterator
// and filters its elements through the accept predicate. The wrapped
// iterator and predicate must be non-nil; passing nil values would cause
// subsequent calls to panic, mirroring the NullPointerException semantics
// of the Java reference.
func NewFilterIterator[T any](baseIterator Iterator[T], accept func(T) bool) *FilterIterator[T] {
	f := &FilterIterator[T]{iterator: baseIterator, accept: accept}
	f.setNext()
	return f
}

// HasNext reports whether the FilterIterator has additional accepted
// elements. The look-ahead is primed by the constructor and refreshed by
// every successful call to Next.
func (f *FilterIterator[T]) HasNext() bool {
	return f.hasNext
}

// Next returns the next accepted element from the wrapped iterator. When
// the iterator is exhausted Next panics with NoSuchElementException, mirroring
// the Java reference behavior; callers must always check HasNext first.
func (f *FilterIterator[T]) Next() T {
	if !f.hasNext {
		panic("NoSuchElementException")
	}
	current := f.next
	f.setNext()
	return current
}

// Remove is not supported and always panics, mirroring the
// UnsupportedOperationException raised by the Java reference.
func (f *FilterIterator[T]) Remove() {
	panic("UnsupportedOperationException: remove is not supported")
}

// setNext advances the wrapped iterator until the next accepted element is
// found or the wrapped iterator is exhausted.
func (f *FilterIterator[T]) setNext() {
	for f.iterator.HasNext() {
		v := f.iterator.Next()
		if f.accept(v) {
			f.next = v
			f.hasNext = true
			return
		}
	}
	var zero T
	f.next = zero
	f.hasNext = false
}
