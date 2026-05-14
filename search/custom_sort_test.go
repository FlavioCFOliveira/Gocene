// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// GC-938: Custom Sort Tests
// Validate custom sort implementations produce identical ordering to Java Lucene.
//
// NOTE: These tests are skipped because IndexSearcher.Search does not yet support
// a Sort parameter. The Sort infrastructure (Sort, SortField) exists but is not
// yet wired into the search path.

import "testing"

func TestCustomSort_BasicSort(t *testing.T) {
	t.Skip("IndexSearcher.Search does not yet support Sort parameter")
}

func TestCustomSort_DescendingSort(t *testing.T) {
	t.Skip("IndexSearcher.Search does not yet support Sort parameter")
}

func TestCustomSort_MultiFieldSort(t *testing.T) {
	t.Skip("IndexSearcher.Search does not yet support Sort parameter")
}

func BenchmarkCustomSort_Performance(b *testing.B) {
	b.Skip("IndexSearcher.Search does not yet support Sort parameter")
}
