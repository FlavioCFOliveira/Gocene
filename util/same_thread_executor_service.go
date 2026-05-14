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
//
// -----------------------------------------------------------------------------
// PORT NOTE (intentional divergence from Java):
//
// Java's SameThreadExecutorService extends AbstractExecutorService and
// runs Runnables on the calling thread. Go's standard library has no
// ExecutorService analogue, so this port introduces a minimal
// [ExecutorLike] interface and a [SameThreadExecutorService] that
// satisfies it. Submit() / Execute() simply call the function on the
// caller's goroutine. Concurrency callers in Gocene that need the
// full Java surface (Future, invokeAll, ...) should use higher-level
// helpers built on top of this primitive.
// -----------------------------------------------------------------------------

package util

import (
	"errors"
	"sync/atomic"
)

// ErrExecutorShutdown is returned by [SameThreadExecutorService.Execute]
// after [SameThreadExecutorService.Shutdown]. Mirrors Java's
// RejectedExecutionException.
var ErrExecutorShutdown = errors.New("executor is shut down")

// ExecutorLike is a minimal Go-side analogue of Java's ExecutorService.
// Only the operations actually used by Lucene's same-thread executor
// are exposed; richer concurrency abstractions live in their own
// packages.
type ExecutorLike interface {
	// Execute submits a task for immediate or eventual execution.
	// Implementations may run the task synchronously or on a worker
	// goroutine; semantics are documented per implementation.
	Execute(task func()) error

	// Shutdown initiates an orderly termination. Tasks already
	// accepted are allowed to complete (synchronous executors have no
	// in-flight tasks). Submissions after Shutdown must return
	// [ErrExecutorShutdown].
	Shutdown()

	// IsShutdown reports whether Shutdown has been invoked.
	IsShutdown() bool

	// IsTerminated reports whether the executor has finished all
	// accepted tasks after a Shutdown. For the synchronous executor
	// this is equivalent to IsShutdown.
	IsTerminated() bool
}

// SameThreadExecutorService runs every submitted task synchronously
// on the goroutine that invoked [SameThreadExecutorService.Execute].
// Useful as a stand-in when callers parametrise APIs over an
// [ExecutorLike] but want to opt out of concurrency.
//
// The zero value is ready to use. Methods are safe for concurrent use.
type SameThreadExecutorService struct {
	shutdown atomic.Bool
}

// NewSameThreadExecutorService returns an executor that runs tasks on
// the calling goroutine. The Java type is final; in Go we still
// expose a constructor for clarity at call sites.
func NewSameThreadExecutorService() *SameThreadExecutorService {
	return &SameThreadExecutorService{}
}

// Execute runs task immediately on the calling goroutine. After
// [SameThreadExecutorService.Shutdown] it returns [ErrExecutorShutdown]
// without invoking task.
func (s *SameThreadExecutorService) Execute(task func()) error {
	if s.shutdown.Load() {
		return ErrExecutorShutdown
	}
	if task == nil {
		return nil
	}
	task()
	return nil
}

// Shutdown flips the executor into the shut-down state. Already-running
// tasks (necessarily on the caller's stack) are unaffected.
func (s *SameThreadExecutorService) Shutdown() {
	s.shutdown.Store(true)
}

// ShutdownNow is the Java parity wrapper around Shutdown; it never
// returns pending tasks because the executor never queues any.
func (s *SameThreadExecutorService) ShutdownNow() []func() {
	s.Shutdown()
	return nil
}

// IsShutdown reports whether Shutdown has been called.
func (s *SameThreadExecutorService) IsShutdown() bool { return s.shutdown.Load() }

// IsTerminated reports whether the executor has finished all accepted
// tasks. For a synchronous executor this is equivalent to IsShutdown.
func (s *SameThreadExecutorService) IsTerminated() bool { return s.shutdown.Load() }

// AwaitTermination is a Java-parity no-op that always returns true:
// there is nothing to wait for in a synchronous executor.
func (s *SameThreadExecutorService) AwaitTermination() bool { return true }

var _ ExecutorLike = (*SameThreadExecutorService)(nil)
