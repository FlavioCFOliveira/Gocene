// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestFilterDirectoryReader.java
// (Apache Lucene 10.5.0).
//
// Every test method subclasses org.apache.lucene.index.FilterDirectoryReader
// through its constructor FilterDirectoryReader(DirectoryReader,
// SubReaderWrapper), overriding doWrapDirectoryReader and getReaderCacheHelper;
// testDelegatingCacheHelper also uses the nested DelegatingCacheHelper. Gocene's
// FilterDirectoryReader declares none of these members (it only wraps a
// DirectoryReader), so each port fails naming the missing members.

package index

import "testing"

const filterDirectoryReaderMissing = "org.apache.lucene.index.FilterDirectoryReader is not ported: " +
	"FilterDirectoryReader(DirectoryReader, SubReaderWrapper), the nested SubReaderWrapper and " +
	"DelegatingCacheHelper classes, doWrapDirectoryReader(DirectoryReader) and unwrap(DirectoryReader) " +
	"are missing"

func TestFilterDirectoryReaderDoubleClose(t *testing.T) {
	t.Fatal(filterDirectoryReaderMissing)
}

func TestFilterDirectoryReaderFilterDirectoryReaderNumDocsIsLazy(t *testing.T) {
	t.Fatal(filterDirectoryReaderMissing)
}

func TestFilterDirectoryReaderDelegatingCacheHelper(t *testing.T) {
	t.Fatal(filterDirectoryReaderMissing)
}
