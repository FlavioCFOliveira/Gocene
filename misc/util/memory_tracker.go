// Package util implements org.apache.lucene.misc.util.
package util

// MemoryTracker reports the bytes a component holds. Mirrors
// org.apache.lucene.misc.util.MemoryTracker. The Go port uses a single
// interface plus a basic in-memory implementation; concrete subclasses can
// embed or wrap the helper.
type MemoryTracker interface {
	Bytes() int64
	Update(delta int64)
}

// CounterMemoryTracker is the simplest MemoryTracker — just a counter.
type CounterMemoryTracker struct {
	count int64
}

// NewCounterMemoryTracker builds the tracker.
func NewCounterMemoryTracker() *CounterMemoryTracker { return &CounterMemoryTracker{} }

// Bytes returns the current count.
func (t *CounterMemoryTracker) Bytes() int64 { return t.count }

// Update increments by delta (which may be negative).
func (t *CounterMemoryTracker) Update(delta int64) { t.count += delta }

var _ MemoryTracker = (*CounterMemoryTracker)(nil)
