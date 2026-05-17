// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/index"

// SegmentCacheable is implemented by objects whose results can be cached
// against a LeafReader.
//
// Mirrors org.apache.lucene.search.SegmentCacheable.
type SegmentCacheable interface {
	// IsCacheable returns true if this object is suitable for caching against
	// the given leaf context.
	IsCacheable(ctx *index.LeafReaderContext) bool
}
