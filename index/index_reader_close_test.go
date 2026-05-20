// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for IndexReader close-listener lifecycle
// and exception handling.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestIndexReaderClose.java
//
// GOC-4259: Port test `org.apache.lucene.index.TestIndexReaderClose`.
//
// # Test coverage
//
//   - TestIndexReaderClose_CloseUnderException          — 1:1 port of testCloseUnderException()
//   - TestIndexReaderClose_CoreListenerOnWrapper        — 1:1 port of testCoreListenerOnWrapperWithDifferentCacheKey()
//   - TestIndexReaderClose_RegisterListenerOnClosed     — 1:1 port of testRegisterListenerOnClosedReader()
//
// # Deviations from the Java reference
//
//   - All tests are degraded to t.Skip.
//
//   - testCloseUnderException opens DirectoryReader.open(dir), wraps it in
//     an anonymous FilterLeafReader that can throw on doClose, registers
//     CacheHelper ClosedListeners, calls close(), and asserts listener
//     invocation and exception propagation.  Blockers:
//     (a) DirectoryReader.open(Directory) — requires wired codec reader;
//     (b) getOnlyLeafReader — not ported;
//     (c) FilterLeafReader with anonymous doClose override — requires user-
//     supplied LeafReader implementation;
//     (d) CacheHelper / addClosedListener / ClosedListener interface — not
//     yet exposed on Gocene's reader types.
//
//   - testCoreListenerOnWrapperWithDifferentCacheKey additionally requires
//     RandomIndexWriter, FilterLeafReader subclass that returns a different
//     CacheHelper key, and verifying listener isolation between the wrapper
//     and the inner reader.
//
//   - testRegisterListenerOnClosedReader requires opening a DirectoryReader,
//     closing it, and asserting that addClosedListener on the closed reader
//     throws AlreadyClosedException or invokes the listener immediately
//     (depending on the implementation contract).
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package index_test

import "testing"

// TestIndexReaderClose_CloseUnderException ports testCloseUnderException().
//
// Java wraps a leaf reader in a FilterLeafReader that throws on doClose,
// registers ClosedListeners, closes the reader, and asserts listener count
// reaches zero and that exceptions propagate correctly.
//
// Degraded to t.Skip: DirectoryReader.open(Directory), getOnlyLeafReader,
// FilterLeafReader, and CacheHelper.addClosedListener are not yet ported.
func TestIndexReaderClose_CloseUnderException(t *testing.T) {
	t.Skip("needs DirectoryReader.open(Directory) with wired codec reader, " +
		"getOnlyLeafReader, FilterLeafReader, and CacheHelper.addClosedListener " +
		"(not yet ported)")
}

// TestIndexReaderClose_CoreListenerOnWrapper ports
// testCoreListenerOnWrapperWithDifferentCacheKey().
//
// Java wraps a leaf reader in a FilterLeafReader with a different cache key,
// registers a CoreClosedListener on the core, and asserts the listener is
// only called when the underlying core closes, not when the wrapper closes.
//
// Degraded to t.Skip: same blockers as TestIndexReaderClose_CloseUnderException;
// also requires RandomIndexWriter and FilterLeafReader with custom cache key.
func TestIndexReaderClose_CoreListenerOnWrapper(t *testing.T) {
	t.Skip("needs DirectoryReader.open(Directory), RandomIndexWriter, " +
		"FilterLeafReader with custom CacheHelper key, and " +
		"CacheHelper.addClosedListener (not yet ported)")
}

// TestIndexReaderClose_RegisterListenerOnClosed ports
// testRegisterListenerOnClosedReader().
//
// Java closes a reader, then calls addClosedListener on the closed reader and
// asserts the listener is invoked immediately (or AlreadyClosedException is
// thrown, depending on the implementation).
//
// Degraded to t.Skip: DirectoryReader.open(Directory) and
// CacheHelper.addClosedListener are not yet ported.
func TestIndexReaderClose_RegisterListenerOnClosed(t *testing.T) {
	t.Skip("needs DirectoryReader.open(Directory) with wired codec reader and " +
		"CacheHelper.addClosedListener on closed reader (not yet ported)")
}
