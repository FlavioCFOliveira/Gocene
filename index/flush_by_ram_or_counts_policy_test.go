// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestFlushByRamOrCountsPolicy.java
// (Apache Lucene 10.5.0).
//
// Every test method of the Java class indexes documents drawn from the
// class-level LineFileDocs (created in @BeforeClass) through IndexThread.
// org.apache.lucene.tests.util.LineFileDocs is not ported to Gocene, so each
// port fails naming the missing test-framework class.

package index

import "testing"

const flushByRamOrCountsPolicyMissing = "org.apache.lucene.tests.util.LineFileDocs is not ported: " +
	"TestFlushByRamOrCountsPolicy feeds every IndexThread from the class-level LineFileDocs"

func TestFlushByRamOrCountsPolicyFlushByRam(t *testing.T) {
	t.Fatal(flushByRamOrCountsPolicyMissing)
}

func TestFlushByRamOrCountsPolicyFlushByRamLargeBuffer(t *testing.T) {
	t.Fatal(flushByRamOrCountsPolicyMissing)
}

func TestFlushByRamOrCountsPolicyFlushDocCount(t *testing.T) {
	t.Fatal(flushByRamOrCountsPolicyMissing)
}

func TestFlushByRamOrCountsPolicyRandom(t *testing.T) {
	t.Fatal(flushByRamOrCountsPolicyMissing)
}

func TestFlushByRamOrCountsPolicyStallControl(t *testing.T) {
	t.Fatal(flushByRamOrCountsPolicyMissing)
}
