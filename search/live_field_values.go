// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "sync"

// LiveFieldValues tracks recently-added field values between NRT (near
// real-time) reader refreshes. Lookups consult an in-memory current map,
// then a frozen "old" map from the previous refresh cycle, and finally fall
// back to a user-provided searcher lookup function once the value has had a
// chance to be promoted to the index.
//
// Mirrors org.apache.lucene.search.LiveFieldValues. The Lucene contract notes
// that callers must serialize updates for the same id from different threads.
type LiveFieldValues[T any] struct {
	missingValue T
	lookup       func(id string) (T, bool, error)

	mu      sync.RWMutex
	current map[string]T
	old     map[string]T
}

// NewLiveFieldValues creates a LiveFieldValues backed by lookup. lookup is
// invoked when neither the current nor the old map has a value for id; if
// lookup returns ok=false, the value is treated as absent.
func NewLiveFieldValues[T any](missingValue T, lookup func(id string) (T, bool, error)) *LiveFieldValues[T] {
	return &LiveFieldValues[T]{
		missingValue: missingValue,
		lookup:       lookup,
		current:      make(map[string]T),
	}
}

// Add records a new value for id.
func (l *LiveFieldValues[T]) Add(id string, value T) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.current[id] = value
}

// Delete records id as deleted, by mapping it to the missing-value sentinel.
func (l *LiveFieldValues[T]) Delete(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.current[id] = l.missingValue
}

// Get returns the latest known value for id. It consults current → old →
// lookup. The second return reports whether a value was found.
func (l *LiveFieldValues[T]) Get(id string) (T, bool, error) {
	l.mu.RLock()
	if v, ok := l.current[id]; ok {
		l.mu.RUnlock()
		return v, true, nil
	}
	if v, ok := l.old[id]; ok {
		l.mu.RUnlock()
		return v, true, nil
	}
	l.mu.RUnlock()
	if l.lookup == nil {
		var zero T
		return zero, false, nil
	}
	return l.lookup(id)
}

// Size returns the number of buffered entries (current + old).
func (l *LiveFieldValues[T]) Size() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.current) + len(l.old)
}

// BeforeRefresh moves current → old in preparation for a refresh cycle.
func (l *LiveFieldValues[T]) BeforeRefresh() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.old = l.current
	l.current = make(map[string]T)
}

// AfterRefresh clears the old map after the new searcher is visible.
func (l *LiveFieldValues[T]) AfterRefresh(refreshed bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.old = nil
}
