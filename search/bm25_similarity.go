// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// BM25Similarity is deprecated. Use LuceneBM25Similarity instead.
//
// This type is preserved for backwards compatibility only. It will be
// removed in a future version of Gocene.
//
// LuceneBM25Similarity is the canonical, Lucene-faithful BM25 implementation.
// It implements the complete Similarity interface and is production-ready.
//
// Deprecated: Use LuceneBM25Similarity().
type BM25Similarity = LuceneBM25Similarity
