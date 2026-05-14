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

import (
	"errors"
	"fmt"
	"sync/atomic"
)

// ErrRefCountOverRelease is returned by DecRef when the reference
// count is decremented below 1. Mirrors Java's IllegalStateException
// "too many decRef calls".
var ErrRefCountOverRelease = errors.New("ref count over-release")

// RefCount is the Go port of org.apache.lucene.util.RefCount: an
// atomic reference counter that calls a release function when the
// count hits zero. The counter starts at 1 — the constructor
// represents the initial reference.
//
// RefCount is generic over the held value type T. The release
// callback is supplied at construction; pass nil for a no-op release.
//
// Safe for concurrent use.
type RefCount[T any] struct {
	count    atomic.Int32
	object   T
	releaseF func(T) error
}

// NewRefCount constructs a RefCount holding object with initial count
// 1. release is invoked once the count transitions to zero; pass nil
// for a no-op (the Java parent class provides an empty release()).
func NewRefCount[T any](object T, release func(T) error) *RefCount[T] {
	r := &RefCount[T]{object: object, releaseF: release}
	r.count.Store(1)
	return r
}

// Get returns the held object. The returned value remains valid
// until release is invoked.
func (r *RefCount[T]) Get() T { return r.object }

// GetRefCount returns the current reference count.
func (r *RefCount[T]) GetRefCount() int32 { return r.count.Load() }

// IncRef increments the reference count. Must be paired with a later
// DecRef call.
func (r *RefCount[T]) IncRef() { r.count.Add(1) }

// DecRef decrements the reference count and invokes release on the
// zero transition. Returns an error wrapping [ErrRefCountOverRelease]
// when the count goes negative, or the error returned by release.
//
// On release failure the count is restored so callers can retry.
func (r *RefCount[T]) DecRef() error {
	rc := r.count.Add(-1)
	switch {
	case rc == 0:
		if r.releaseF == nil {
			return nil
		}
		if err := r.releaseF(r.object); err != nil {
			// Restore the reference so callers can retry release.
			r.count.Add(1)
			return fmt.Errorf("release: %w", err)
		}
		return nil
	case rc < 0:
		return fmt.Errorf("%w: refCount=%d after decrement", ErrRefCountOverRelease, rc)
	default:
		return nil
	}
}

// Close is a convenience wrapper that calls DecRef and discards the
// returned error. Implementations that need the error should call
// DecRef directly.
func (r *RefCount[T]) Close() error { return r.DecRef() }
