// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package taxonomy provides taxonomy-based facet counting implementations.
//
// This package contains implementations that use a taxonomy index for efficient
// hierarchical facet counting, including:
//
// - FastTaxonomyFacetCounts: Optimized facet counting using taxonomy ordinals
//
// The taxonomy-based approach is efficient for hierarchical facets where
// child counts need to be aggregated up to parent categories.
//
// This is the Go port of Lucene's org.apache.lucene.facet.taxonomy package.
package taxonomy
