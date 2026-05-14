// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// Accountable is the Go port of org.apache.lucene.util.Accountable.
//
// An object whose RAM usage can be computed. Implementations should report
// the number of bytes used by the object itself plus the bytes used by any
// owned data structures (recursively, where applicable).
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/Accountable.java
type Accountable interface {
	// RamBytesUsed returns the total amount of RAM, in bytes, consumed by
	// this object and any sub-objects it owns. Implementations should return
	// a stable value that does not include transient buffers.
	RamBytesUsed() int64
}

// AccountableWithChildren is implemented by Accountables that also expose
// their owned child Accountables, mirroring Lucene's
// Accountable.getChildResources() default method.
//
// Returning an empty slice is equivalent to "no children", matching the
// Java default of Collections.emptyList().
type AccountableWithChildren interface {
	Accountable

	// GetChildResources returns the child Accountables owned by this
	// object. The returned slice must not be retained or mutated by the
	// caller; implementations may return a freshly allocated slice or
	// reuse an internal one.
	GetChildResources() []Accountable
}

// GetChildResources returns the child Accountables of a, or an empty
// slice when a does not expose any. This mirrors the Java default
// behaviour: callers do not have to type-assert on every Accountable.
func GetChildResources(a Accountable) []Accountable {
	if awc, ok := a.(AccountableWithChildren); ok {
		return awc.GetChildResources()
	}
	return nil
}
