// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"testing"
)

func TestSameThreadExecutor_RunsSynchronously(t *testing.T) {
	ex := NewSameThreadExecutorService()
	var ran bool
	if err := ex.Execute(func() { ran = true }); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !ran {
		t.Fatal("task did not run")
	}
}

func TestSameThreadExecutor_NilTaskIsNoOp(t *testing.T) {
	ex := NewSameThreadExecutorService()
	if err := ex.Execute(nil); err != nil {
		t.Fatalf("Execute(nil): %v", err)
	}
}

func TestSameThreadExecutor_ShutdownStateMachine(t *testing.T) {
	ex := NewSameThreadExecutorService()
	if ex.IsShutdown() {
		t.Fatal("fresh executor must not be shut down")
	}
	ex.Shutdown()
	if !ex.IsShutdown() || !ex.IsTerminated() {
		t.Fatal("after Shutdown both IsShutdown and IsTerminated must be true")
	}
	if !ex.AwaitTermination() {
		t.Fatal("AwaitTermination must return true for synchronous executor")
	}
}

func TestSameThreadExecutor_RejectsAfterShutdown(t *testing.T) {
	ex := NewSameThreadExecutorService()
	ex.Shutdown()
	called := false
	err := ex.Execute(func() { called = true })
	if !errors.Is(err, ErrExecutorShutdown) {
		t.Fatalf("got %v want ErrExecutorShutdown", err)
	}
	if called {
		t.Fatal("task must not run after shutdown")
	}
}

func TestSameThreadExecutor_ShutdownNowReturnsEmpty(t *testing.T) {
	ex := NewSameThreadExecutorService()
	if pending := ex.ShutdownNow(); pending != nil {
		t.Fatalf("ShutdownNow returned %v want nil", pending)
	}
	if !ex.IsShutdown() {
		t.Fatal("ShutdownNow must mark executor as shut down")
	}
}

func TestSameThreadExecutor_SatisfiesExecutorLike(t *testing.T) {
	var _ ExecutorLike = NewSameThreadExecutorService()
}
