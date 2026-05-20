// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterOnJRECrash
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterOnJRECrash.java
//
// GOC-4171: Port TestIndexWriterOnJRECrash from Apache Lucene to Go.
package index_test

import "testing"

// TestIndexWriterOnJRECrash is intentionally a skipped stub.
//
// The upstream Lucene test is JVM-fork-specific and has no direct Go
// equivalent. It re-forks the JVM as a child process via ProcessBuilder
// (setting -Dtests.crashmode=true), runs TestNRTThreads in that child,
// kills the process mid-write after a randomized delay, then walks the
// temp directory running CheckIndex to assert the index is not corrupt.
//
// Go has no equivalent to the JVM fork/impersonation harness this test
// relies on (getJvmForkArguments, JUnitCore re-invocation, the parent
// JVM crashing its own clone). Reproducing it would require a bespoke
// out-of-process crash harness unrelated to the Gocene codec/index code
// under test. It is therefore recorded here as a documented skip.
func TestIndexWriterOnJRECrash(t *testing.T) {
	t.Skip("JVM-fork-specific: re-forks the JVM and crashes the child mid-write; no direct Go equivalent")
}
