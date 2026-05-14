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

import "errors"

// ErrThreadInterrupted is the Go analogue of
// org.apache.lucene.util.ThreadInterruptedException. Java throws an unchecked
// RuntimeException when Thread.interrupt() is observed; Go's concurrency model
// has no equivalent primitive, so callers receive this sentinel error wrapped
// in whatever context is appropriate. Use [errors.Is] to detect it.
//
// Lucene-divergence note: the Java type carries an InterruptedException cause;
// Go consumers typically wrap a [context.Canceled] / [context.DeadlineExceeded]
// or a domain-specific error in addition to this sentinel.
var ErrThreadInterrupted = errors.New("thread interrupted")

// ThreadInterruptedError wraps an underlying cause (typically
// [context.Canceled] or a goroutine-cancellation error) and reports it as a
// thread-interrupted condition. It satisfies the error interface and unwraps
// to both the cause and to [ErrThreadInterrupted], so the following all hold:
//
//	errors.Is(err, ErrThreadInterrupted)         // true for any *ThreadInterruptedError
//	errors.Is(err, context.Canceled)             // true when cause is context.Canceled
//	errors.Unwrap(err)                            // returns the cause, if any
type ThreadInterruptedError struct {
	// Cause is the underlying error that triggered the interruption, or nil.
	Cause error
}

// NewThreadInterruptedError wraps the provided cause. A nil cause yields an
// error whose message is identical to [ErrThreadInterrupted].
func NewThreadInterruptedError(cause error) *ThreadInterruptedError {
	return &ThreadInterruptedError{Cause: cause}
}

// Error implements the error interface. The message mirrors Lucene's
// RuntimeException output when wrapping an InterruptedException: the cause's
// message follows the sentinel, separated by ": ".
func (e *ThreadInterruptedError) Error() string {
	if e == nil || e.Cause == nil {
		return ErrThreadInterrupted.Error()
	}
	return ErrThreadInterrupted.Error() + ": " + e.Cause.Error()
}

// Unwrap returns the cause so [errors.Is] / [errors.As] can walk the chain.
func (e *ThreadInterruptedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Is reports whether the target matches this error. It returns true when the
// target is [ErrThreadInterrupted], enabling errors.Is(err, ErrThreadInterrupted)
// to detect a thread-interrupted condition even when a non-nil cause is set.
func (e *ThreadInterruptedError) Is(target error) bool {
	return target == ErrThreadInterrupted
}
