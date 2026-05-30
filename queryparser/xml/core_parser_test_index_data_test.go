// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package xml_test

import "testing"

// TestCoreParserTestIndexData verifies that the index-data helper used by
// CoreParser XML tests can be constructed and closed without error.
//
// In Java, CoreParserTestIndexData reads reuters21578.txt from the test
// resources, builds an in-memory index with IndexWriter, and exposes an
// IndexSearcher for subsequent query tests.
//
// Execution is deferred because:
//   - The reuters21578.txt fixture has not been vendored into the Gocene tree
//   - DirectoryReader.Open() requires a complete IndexWriter round-trip that
//     is not yet supported end-to-end
//
// Port of: queryparser/src/test/.../xml/CoreParserTestIndexData.java
func TestCoreParserTestIndexData(t *testing.T) {
	t.Fatal("deferred: requires reuters21578.txt fixture and functional IndexWriter/DirectoryReader round-trip")
}
