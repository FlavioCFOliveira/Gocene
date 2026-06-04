// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestKnnByteVectorQueryMMap.java
//
// TestKnnByteVectorQueryMMap extends TestKnnByteVectorQuery, overriding only
// newDirectoryForTest to use an MMapDirectory. The Go port reuses the byte
// fixture and overrides newIndex to open the integration index over an
// MMapDirectory rooted at a per-test temp directory, then runs the full
// inherited BaseKnnVectorQueryTestCase scenario set — proving the KNN byte
// vector flush/read path works over the memory-mapped store backend.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// mmapByteKnnFixture is byteKnnFixture backed by an MMapDirectory.
type mmapByteKnnFixture struct {
	byteKnnFixture
}

// newIndex opens the integration index over an MMapDirectory rooted at a fresh
// temp directory (the analogue of overriding newDirectoryForTest to return an
// MMapDirectory).
func (mmapByteKnnFixture) newIndex(t *testing.T) *integrationIndex {
	dir, err := store.NewMMapDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewMMapDirectory: %v", err)
	}
	return newIntegrationIndexWithDir(t, dir)
}

// TestKnnByteVectorQueryMMap runs the inherited byte scenario set over an
// MMapDirectory.
func TestKnnByteVectorQueryMMap(t *testing.T) {
	runKnnAllScenarios(t, mmapByteKnnFixture{})
}
