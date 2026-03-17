// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package sortedset provides facet counting implementations using SortedSetDocValues.
//
// This package contains implementations that use SortedSetDocValues for efficient
// facet counting, including:
//
// - SortedSetDocValuesFacetCounts: Facet counting using SortedSetDocValues ordinals
//
// These implementations are part of Lucene's facet module and provide
// efficient counting for facets stored as SortedSetDocValues.
//
// This is the Go port of Lucene's org.apache.lucene.facet.sortedset package.
package sortedset
