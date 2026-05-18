package misc

// CollectorMemoryTracker accounts for the bytes a collector retains so the
// caller can enforce a budget. Mirrors
// org.apache.lucene.misc.CollectorMemoryTracker.
type CollectorMemoryTracker struct {
	MaxBytes int64
	used     int64
}

// NewCollectorMemoryTracker builds the tracker.
func NewCollectorMemoryTracker(maxBytes int64) *CollectorMemoryTracker {
	if maxBytes < 0 {
		maxBytes = 0
	}
	return &CollectorMemoryTracker{MaxBytes: maxBytes}
}

// Track records bytes used; returns false when the budget would be exceeded.
func (t *CollectorMemoryTracker) Track(bytes int64) bool {
	if t.MaxBytes > 0 && t.used+bytes > t.MaxBytes {
		return false
	}
	t.used += bytes
	return true
}

// Used returns the current allocation.
func (t *CollectorMemoryTracker) Used() int64 { return t.used }

// Release subtracts bytes from the budget.
func (t *CollectorMemoryTracker) Release(bytes int64) {
	t.used -= bytes
	if t.used < 0 {
		t.used = 0
	}
}
