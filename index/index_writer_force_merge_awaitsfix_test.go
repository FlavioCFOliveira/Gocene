// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_awaitsfix

// The @AwaitsFix(bugUrl = "https://github.com/apache/lucene/issues/13478")
// test port of
// lucene/core/src/test/org/apache/lucene/index/TestIndexWriterForceMerge.java
// (Apache Lucene 10.5.0). Lucene runs @AwaitsFix tests only when
// tests.awaitsfix is enabled; the gocene_awaitsfix build tag renders that
// switch.

package index_test

import "testing"

func TestIndexWriterForceMergeMergePerField(t *testing.T) {
	t.Fatal("overriding org.apache.lucene.index.ConcurrentMergeScheduler#getIntraMergeExecutor(MergePolicy.OneMerge) " +
		"to return the protected intraMergeExecutor is not ported")
}
