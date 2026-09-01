package index

import (
	"testing"
	"time"
)

func TestMergeObserver_Basic(t *testing.T) {
	spec := &MergeSpecification{}
	m1 := NewOneMerge([]SegmentCommitInfo{})
	m2 := NewOneMerge([]SegmentCommitInfo{})
	spec.Add(m1)
	spec.Add(m2)

	observer := NewMergeObserver(spec)

	if observer.NumMerges() != 2 {
		t.Errorf("expected 2 merges, got %d", observer.NumMerges())
	}

	if observer.NumCompletedMerges() != 0 {
		t.Errorf("expected 0 completed merges, got %d", observer.NumCompletedMerges())
	}

	// Complete first merge
	m1.completed.Store(true)
	m1.MergeCompleted <- true

	if observer.NumCompletedMerges() != 1 {
		t.Errorf("expected 1 completed merge, got %d", observer.NumCompletedMerges())
	}

	// Complete second merge
	m2.completed.Store(true)
	m2.MergeCompleted <- true

	if observer.NumCompletedMerges() != 2 {
		t.Errorf("expected 2 completed merges, got %d", observer.NumCompletedMerges())
	}
}

func TestMergeObserver_Await(t *testing.T) {
	spec := &MergeSpecification{}
	m1 := NewOneMerge([]SegmentCommitInfo{})
	spec.Add(m1)
	observer := NewMergeObserver(spec)

	go func() {
		time.Sleep(10 * time.Millisecond)
		m1.completed.Store(true)
		m1.MergeCompleted <- true
	}()

	if !observer.Await() {
		t.Error("expected Await to return true")
	}
}

func TestMergeObserver_AwaitFailure(t *testing.T) {
	spec := &MergeSpecification{}
	m1 := NewOneMerge([]SegmentCommitInfo{})
	spec.Add(m1)
	observer := NewMergeObserver(spec)

	go func() {
		time.Sleep(10 * time.Millisecond)
		m1.completed.Store(true)
		m1.MergeCompleted <- false
	}()

	if observer.Await() {
		t.Error("expected Await to return false on merge failure")
	}
}

func TestMergeObserver_AwaitWithTimeout(t *testing.T) {
	spec := &MergeSpecification{}
	m1 := NewOneMerge([]SegmentCommitInfo{})
	spec.Add(m1)
	observer := NewMergeObserver(spec)

	// Test timeout
	if observer.AwaitWithTimeout(5 * time.Millisecond) {
		t.Error("expected AwaitWithTimeout to return false on timeout")
	}

	// Test success within timeout
	go func() {
		time.Sleep(10 * time.Millisecond)
		m1.completed.Store(true)
		m1.MergeCompleted <- true
	}()

	if !observer.AwaitWithTimeout(50 * time.Millisecond) {
		t.Error("expected AwaitWithTimeout to return true on success")
	}
}

func TestMergeObserver_AwaitAsync(t *testing.T) {
	spec := &MergeSpecification{}
	m1 := NewOneMerge([]SegmentCommitInfo{})
	spec.Add(m1)
	observer := NewMergeObserver(spec)

	asyncChan := observer.AwaitAsync()

	go func() {
		time.Sleep(10 * time.Millisecond)
		m1.completed.Store(true)
		m1.MergeCompleted <- true
	}()

	select {
	case <-asyncChan:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("expected AwaitAsync channel to close")
	}
}

func TestMergeObserver_NoSpec(t *testing.T) {
	observer := NewMergeObserver(nil)

	if observer.NumMerges() != 0 {
		t.Errorf("expected 0 merges, got %d", observer.NumMerges())
	}
	if observer.NumCompletedMerges() != 0 {
		t.Errorf("expected 0 completed, got %d", observer.NumCompletedMerges())
	}
	if !observer.Await() {
		t.Error("expected Await to return true for nil spec")
	}
	if !observer.AwaitWithTimeout(time.Second) {
		t.Error("expected AwaitWithTimeout to return true for nil spec")
	}
	
	select {
	case <-observer.AwaitAsync():
		// Success
	case <-time.After(10 * time.Millisecond):
		t.Error("expected AwaitAsync to return immediately for nil spec")
	}
}
