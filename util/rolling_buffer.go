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

// Resettable is the contract implemented by elements held in a
// [RollingBuffer]. Reset returns the receiver to a pristine state so
// the buffer can reuse the instance for a new position.
//
// Mirrors org.apache.lucene.util.RollingBuffer.Resettable.
type Resettable interface {
	Reset()
}

// RollingBuffer behaves like a forever-growing T slice but internally
// uses a circular buffer to reuse instances of T. The buffer is
// indexed by an abstract, monotonically-increasing "position".
//
// T must satisfy [Resettable] so the buffer can recycle slots when
// [RollingBuffer.FreeBefore] advances past them.
//
// Port of org.apache.lucene.util.RollingBuffer (the Java class is
// abstract over newInstance; the Go port takes the factory as a
// constructor argument).
//
// Not safe for concurrent use.
type RollingBuffer[T Resettable] struct {
	buffer    []T
	newFn     func() T
	nextWrite int
	nextPos   int
	count     int
}

// NewRollingBuffer returns a RollingBuffer pre-populated with eight
// instances produced by newFn. newFn must be non-nil and is invoked
// every time the buffer grows.
func NewRollingBuffer[T Resettable](newFn func() T) *RollingBuffer[T] {
	if newFn == nil {
		panic("util.NewRollingBuffer: newFn must not be nil")
	}
	rb := &RollingBuffer[T]{
		buffer: make([]T, 8),
		newFn:  newFn,
	}
	for i := range rb.buffer {
		rb.buffer[i] = newFn()
	}
	return rb
}

// Reset reverts the buffer to its initial state. All previously-issued
// instances are reset and remain owned by the buffer.
func (r *RollingBuffer[T]) Reset() {
	r.nextWrite--
	for r.count > 0 {
		if r.nextWrite == -1 {
			r.nextWrite = len(r.buffer) - 1
		}
		r.buffer[r.nextWrite].Reset()
		r.nextWrite--
		r.count--
	}
	r.nextWrite = 0
	r.nextPos = 0
	r.count = 0
}

// Get returns the T instance for the absolute position pos. pos may be
// arbitrarily far in the future, but must not be earlier than the
// last [RollingBuffer.FreeBefore] threshold. Out-of-bounds access
// panics, mirroring Lucene's assertion semantics.
func (r *RollingBuffer[T]) Get(pos int) T {
	for pos >= r.nextPos {
		if r.count == len(r.buffer) {
			newLen := Oversize(1+r.count, NumBytesObjectRef)
			newBuffer := make([]T, newLen)
			copy(newBuffer, r.buffer[r.nextWrite:])
			copy(newBuffer[len(r.buffer)-r.nextWrite:], r.buffer[:r.nextWrite])
			for i := len(r.buffer); i < newLen; i++ {
				newBuffer[i] = r.newFn()
			}
			r.nextWrite = len(r.buffer)
			r.buffer = newBuffer
		}
		if r.nextWrite == len(r.buffer) {
			r.nextWrite = 0
		}
		r.nextWrite++
		r.nextPos++
		r.count++
	}
	if !r.inBounds(pos) {
		panic("util.RollingBuffer.Get: position out of bounds")
	}
	return r.buffer[r.indexOf(pos)]
}

// MaxPos returns the highest position ever looked up, or -1 if no
// position has been looked up since the last reset.
func (r *RollingBuffer[T]) MaxPos() int { return r.nextPos - 1 }

// BufferSize returns the number of active positions currently
// retained in the ring.
func (r *RollingBuffer[T]) BufferSize() int { return r.count }

// FreeBefore resets every slot strictly older than pos, returning
// them to the recycle pool.
func (r *RollingBuffer[T]) FreeBefore(pos int) {
	toFree := r.count - (r.nextPos - pos)
	if toFree < 0 || toFree > r.count {
		panic("util.RollingBuffer.FreeBefore: pos out of valid range")
	}
	index := r.nextWrite - r.count
	if index < 0 {
		index += len(r.buffer)
	}
	for i := 0; i < toFree; i++ {
		if index == len(r.buffer) {
			index = 0
		}
		r.buffer[index].Reset()
		index++
	}
	r.count -= toFree
}

func (r *RollingBuffer[T]) inBounds(pos int) bool {
	return pos < r.nextPos && pos >= r.nextPos-r.count
}

func (r *RollingBuffer[T]) indexOf(pos int) int {
	idx := r.nextWrite - (r.nextPos - pos)
	if idx < 0 {
		idx += len(r.buffer)
	}
	return idx
}
