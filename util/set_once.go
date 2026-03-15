// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"sync/atomic"
)

// AlreadySetException is thrown when SetOnce.Set is called more than once.
type AlreadySetException struct {
	msg string
}

// Error returns the error message for AlreadySetException.
func (e *AlreadySetException) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return "The object cannot be set twice!"
}

// wrapper holds the object and marks that it was already set.
type wrapper[T any] struct {
	object T
}

// SetOnce is a semi-immutable object wrapper which allows one to set the value
// of an object exactly once, and retrieve it many times. If Set is called more
// than once, AlreadySetException is returned and the operation will fail.
type SetOnce[T any] struct {
	set atomic.Pointer[wrapper[T]]
}

// NewSetOnce creates a new SetOnce instance which does not set the internal object,
// and allows setting it by calling Set.
func NewSetOnce[T any]() *SetOnce[T] {
	return &SetOnce[T]{}
}

// NewSetOnceWithValue creates a new SetOnce instance with the internal object set to
// the given object. Note that any calls to Set afterwards will result in AlreadySetException.
func NewSetOnceWithValue[T any](obj T) *SetOnce[T] {
	s := &SetOnce[T]{}
	s.set.Store(&wrapper[T]{object: obj})
	return s
}

// Set sets the given object. If the object has already been set, an error is returned.
func (s *SetOnce[T]) Set(obj T) error {
	if !s.TrySet(obj) {
		return &AlreadySetException{}
	}
	return nil
}

// TrySet sets the given object if none was set before.
// Returns true if object was set successfully, false otherwise.
func (s *SetOnce[T]) TrySet(obj T) bool {
	return s.set.CompareAndSwap(nil, &wrapper[T]{object: obj})
}

// Get returns the object set by Set, or the zero value if not set.
func (s *SetOnce[T]) Get() T {
	var zero T
	w := s.set.Load()
	if w == nil {
		return zero
	}
	return w.object
}

// IsSet returns true if the value has been set, false otherwise.
func (s *SetOnce[T]) IsSet() bool {
	return s.set.Load() != nil
}
