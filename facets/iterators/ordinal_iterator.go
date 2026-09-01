// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package iterators

// OrdinalIterator is an iterator over ordinals.
// Mirrors org.apache.lucene.sandbox.facet.iterators.OrdinalIterator.
type OrdinalIterator interface {
	// NextOrd returns the next ordinal, or -1 when exhausted.
	NextOrd() int
}
