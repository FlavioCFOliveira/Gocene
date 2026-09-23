// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestStressNRT.java
// (Apache Lucene 10.5.0).

package index_test

import "testing"

// TestStressNRT ports test(), which indexes into
// LuceneTestCase.newMaybeVirusCheckingDirectory().
func TestStressNRT(t *testing.T) {
	t.Fatal("org.apache.lucene.tests.util.LuceneTestCase#newMaybeVirusCheckingDirectory() " +
		"(org.apache.lucene.tests.mockfile.VirusCheckingFS) is not ported")
}
