// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package knn

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// nonOptimisticManager satisfies KnnCollectorManager but not
// OptimisticKnnCollectorManager. Used to assert the Java-default
// fallback path of the helpers.
type nonOptimisticManager struct{}

func (nonOptimisticManager) NewCollector(_ int, _ KnnSearchStrategy, _ *index.LeafReaderContext) (hnsw.KnnCollector, error) {
	return nil, errors.New("not used in this test")
}

// optimisticManager satisfies OptimisticKnnCollectorManager and tracks
// which path was exercised.
type optimisticManager struct {
	newCollectorCalls           int
	newOptimisticCollectorCalls int
	isOptimistic                bool
}

func (m *optimisticManager) NewCollector(_ int, _ KnnSearchStrategy, _ *index.LeafReaderContext) (hnsw.KnnCollector, error) {
	m.newCollectorCalls++
	return nil, nil
}

func (m *optimisticManager) NewOptimisticCollector(_ int, _ KnnSearchStrategy, _ *index.LeafReaderContext, _ int) (hnsw.KnnCollector, error) {
	m.newOptimisticCollectorCalls++
	return nil, nil
}

func (m *optimisticManager) IsOptimistic() bool { return m.isOptimistic }

// TestNonOptimisticFallback asserts that helpers behave as if the
// Java defaults were in force: NewOptimisticCollector returns
// (nil, nil) and IsOptimistic returns false.
func TestNonOptimisticFallback(t *testing.T) {
	var m nonOptimisticManager
	c, err := NewOptimisticCollector(m, 10, nil, nil, 5)
	if err != nil {
		t.Fatalf("NewOptimisticCollector err = %v, want nil", err)
	}
	if c != nil {
		t.Errorf("NewOptimisticCollector = %v, want nil", c)
	}
	if IsOptimistic(m) {
		t.Errorf("IsOptimistic = true, want false on non-optimistic manager")
	}
}

// TestOptimisticDispatch asserts the helpers route through the
// optional interface when the manager implements it.
func TestOptimisticDispatch(t *testing.T) {
	m := &optimisticManager{isOptimistic: true}
	if _, err := NewOptimisticCollector(m, 10, nil, nil, 5); err != nil {
		t.Fatalf("NewOptimisticCollector err = %v, want nil", err)
	}
	if m.newOptimisticCollectorCalls != 1 || m.newCollectorCalls != 0 {
		t.Errorf("calls = (newCollector=%d, newOptimistic=%d), want (0,1)", m.newCollectorCalls, m.newOptimisticCollectorCalls)
	}
	if !IsOptimistic(m) {
		t.Errorf("IsOptimistic = false, want true")
	}
}
