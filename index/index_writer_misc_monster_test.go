// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// Package index_test contains additional @Nightly / JVM-fork test stubs ported
// from Apache Lucene's index test suite.
//
// Reference tag: releases/lucene/10.4.0 (commit 9983b7c)
//
// These tests are excluded from the standard suite because Lucene annotates
// them @Nightly or because they depend on JVM-specific fork/crash harnesses.
// They are kept as documented t.Fatal stubs so the test mapping remains 1:1.
package index_test

import "testing"

// ---------------------------------------------------------------------------
// TestIndexWriterReader.testDuringAddIndexes (@Nightly)
// ---------------------------------------------------------------------------

// TestIndexWriterReader_DuringAddIndexes ports testDuringAddIndexes() (a @Nightly
// stress test).
func TestIndexWriterReader_DuringAddIndexes(t *testing.T) {
	// NRT openIfChanged is now available; MockDirectoryWrapper fault injection
	// is tracked by rmp #250 (T105.2.4).
	t.Fatal("nightly stress test; needs MockDirectoryWrapper fault injection; NRT openIfChanged is now available")
}

// ---------------------------------------------------------------------------
// TestIndexWriterThreadsToSegments.testDocsStuckInRAMForever (@Nightly)
// ---------------------------------------------------------------------------

// TestIndexWriterThreadsToSegments_DocsStuckInRAMForever ports
// testDocsStuckInRAMForever (a @Nightly test). Skipped: requires
// SegmentInfoFormat.read, SegmentReader.docFreq and core readers, which are
// not yet wired (see SegmentReader core-readers gap).
func TestIndexWriterThreadsToSegments_DocsStuckInRAMForever(t *testing.T) {
	t.Fatal("nightly; requires SegmentInfoFormat.read + SegmentReader.docFreq (Sprint 55 option c)")
}

// ---------------------------------------------------------------------------
// TestMixedDocValuesUpdates.testTonsOfUpdates (@Nightly)
// ---------------------------------------------------------------------------

// TestMixedDocValuesUpdates_TonsOfUpdates mirrors testTonsOfUpdates (@Nightly,
// LUCENE-5248): a large index with many binary fields and update terms, RAM
// buffer tuned to flush frequently, verifying RAM is bounded and values stay
// consistent.
func TestMixedDocValuesUpdates_TonsOfUpdates(t *testing.T) {
	t.Fatal("GOC-4202: nightly stress case not ported")
}

// ---------------------------------------------------------------------------
// TestIndexWriterOnJRECrash.testNRTThreads (@Nightly, JVM-fork-specific)
// ---------------------------------------------------------------------------

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
	t.Fatal("JVM-fork-specific: re-forks the JVM and crashes the child mid-write; no direct Go equivalent")
}
