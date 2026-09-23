// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/core/src/test/org/apache/lucene/index/TestIndexWriterOnError.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs it only when nightly tests are enabled.

package index_test

import "testing"

// TestIndexWriterOnErrorCheckpoint ports testCheckpoint().
func TestIndexWriterOnErrorCheckpoint(t *testing.T) {
	t.Fatal(indexWriterOnErrorMissing)
}
