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
//
// This file completes the port of org.apache.lucene.util.Counter.
// The thread-safe variant (*Counter, atomic.AddInt64/LoadInt64) is
// already defined in util/byte_block_pool.go; this file adds the
// abstract Counter interface plus the non-thread-safe SerialCounter
// variant so the full Java API surface (addAndGet, get, newCounter,
// newCounter(boolean)) is covered.

package util

// CounterAPI captures the abstract surface of
// org.apache.lucene.util.Counter: a 64-bit accumulator with AddAndGet
// and Get. Both *Counter (atomic) and *SerialCounter (single-thread)
// satisfy this interface.
type CounterAPI interface {
	// AddAndGet adds delta to the counter and returns the new value.
	AddAndGet(delta int64) int64

	// Get returns the current value.
	Get() int64
}

// SerialCounter is the non-thread-safe sibling of *Counter. Mirrors
// Java's private SerialCounter inner class. Use it when the caller is
// already serialised (e.g. behind a per-document buffer) and wants to
// avoid the cost of an atomic operation.
//
// Zero value is ready to use.
type SerialCounter struct {
	value int64
}

// NewSerialCounter returns a new, zero-valued, non-thread-safe counter.
func NewSerialCounter() *SerialCounter {
	return &SerialCounter{}
}

// AddAndGet adds delta to the counter and returns the new value.
// Not safe for concurrent use.
func (s *SerialCounter) AddAndGet(delta int64) int64 {
	s.value += delta
	return s.value
}

// Get returns the current value. Not safe for concurrent use; callers
// that may race a writer should use the atomic *Counter instead.
func (s *SerialCounter) Get() int64 {
	return s.value
}

// NewCounterThreadSafe is an alias of NewCounter that matches the
// Lucene convention {@code Counter.newCounter(true)}. It returns the
// atomic *Counter variant. Provided so callers porting literally from
// Java retain a readable migration trail.
func NewCounterThreadSafe() CounterAPI {
	return NewCounter()
}

// NewCounterOf returns the appropriate Counter variant for the
// supplied thread-safety preference. Equivalent to Lucene's
// {@code Counter.newCounter(boolean threadSafe)}.
func NewCounterOf(threadSafe bool) CounterAPI {
	if threadSafe {
		return NewCounter()
	}
	return NewSerialCounter()
}
