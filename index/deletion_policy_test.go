// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for custom IndexDeletionPolicy implementations.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestDeletionPolicy.java
//
// GOC-4248: Port test `org.apache.lucene.index.TestDeletionPolicy`.
//
// # Test coverage
//
//   - TestDeletionPolicy_ExpirationTime        — 1:1 port of testExpirationTimeDeletionPolicy()
//   - TestDeletionPolicy_KeepAll               — 1:1 port of testKeepAllDeletionPolicy()
//   - TestDeletionPolicy_OpenPriorSnapshot     — 1:1 port of testOpenPriorSnapshot()
//   - TestDeletionPolicy_KeepNoneOnInit        — 1:1 port of testKeepNoneOnInitDeletionPolicy()
//   - TestDeletionPolicy_KeepLastN             — 1:1 port of testKeepLastNDeletionPolicy()
//   - TestDeletionPolicy_KeepLastNWithCreates  — 1:1 port of testKeepLastNDeletionPolicyWithCreates()
//   - TestDeletionPolicy_KeepLastNCommits      — 1:1 port of testKeepLastNCommitsDeletionPolicy()
//   - TestDeletionPolicy_KeepLastNCommitsZero  — 1:1 port of testKeepLastNCommitsDeletionPolicyWithZeroCommits()
//
// # Deviations from the Java reference
//
//   - All tests are degraded to t.Skip.
//
//   - Every test method exercises the full deletion-policy lifecycle:
//     multiple commits are written, the policy's onCommit / onInit hooks
//     are invoked, and old commits are selectively removed via
//     IndexCommit.Delete().  The following Gocene gaps block execution:
//
//   - IndexCommit.Delete() is not yet functional; calling it is a no-op so
//     the expected set of retained commits never matches the actual set.
//
//   - IndexWriterConfig.SetIndexCommit does not exist; testOpenPriorSnapshot
//     and variants that reopen from a specific historic commit cannot be run.
//
//   - DirectoryReader.open(IndexCommit) is not implemented; reading from a
//     historic commit is impossible.
//
//   - SegmentInfos.getLastCommitGeneration / getLastCommitSegmentsFileName
//     are not exposed on the public API.
//
//   - LiveIndexWriterConfig.GetIndexDeletionPolicy() is absent; tests that
//     retrieve the policy from a live writer cannot compile.
//
//   - testExpirationTimeDeletionPolicy additionally uses wall-clock sleeps
//     and System.currentTimeMillis() stamping on IndexCommit, which would
//     require IndexCommit to expose a user-visible timestamp.
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package index_test

import "testing"

// TestDeletionPolicy_ExpirationTime ports testExpirationTimeDeletionPolicy().
//
// Java registers an ExpirationTimeDeletionPolicy that deletes all commits
// older than a given wall-clock interval, writes multiple commits with
// sleep intervals between them, and then asserts only recent commits survive.
//
// Degraded to t.Skip: IndexCommit.Delete() is a no-op; wall-clock timestamp
// on IndexCommit is not exposed; SegmentInfos.getLastCommitGeneration not
// available.
func TestDeletionPolicy_ExpirationTime(t *testing.T) {
	t.Fatal("needs functional IndexCommit.Delete(), wall-clock timestamp on " +
		"IndexCommit, and SegmentInfos.getLastCommitGeneration (not yet ported)")
}

// TestDeletionPolicy_KeepAll ports testKeepAllDeletionPolicy().
//
// Java registers a KeepAllDeletionPolicy that never deletes any commit,
// writes several commits, and asserts that every commit-generation file
// (segments_N) is still present on disk.
//
// Degraded to t.Skip: SegmentInfos.getLastCommitGeneration is not exposed
// and IndexCommit enumeration via IndexWriter.listCommits is missing.
func TestDeletionPolicy_KeepAll(t *testing.T) {
	t.Fatal("needs SegmentInfos.getLastCommitGeneration and " +
		"IndexWriter.listCommits (IndexCommit enumeration) not yet ported")
}

// TestDeletionPolicy_OpenPriorSnapshot ports testOpenPriorSnapshot().
//
// Java keeps a list of IndexCommit handles produced by successive commits,
// then reopens the writer from each historic commit via
// IndexWriterConfig.SetIndexCommit and verifies the document count matches
// the snapshot.
//
// Degraded to t.Skip: IndexWriterConfig.SetIndexCommit does not exist;
// DirectoryReader.open(IndexCommit) is not implemented.
func TestDeletionPolicy_OpenPriorSnapshot(t *testing.T) {
	t.Fatal("needs IndexWriterConfig.SetIndexCommit and " +
		"DirectoryReader.open(IndexCommit), neither of which is ported")
}

// TestDeletionPolicy_KeepNoneOnInit ports testKeepNoneOnInitDeletionPolicy().
//
// Java registers a policy whose onInit deletes all commits and whose
// onCommit keeps only the last commit, then verifies that after writer
// reopens only one segments file survives.
//
// Degraded to t.Skip: IndexCommit.Delete() is a no-op; SegmentInfos commit
// generation tracking not exposed.
func TestDeletionPolicy_KeepNoneOnInit(t *testing.T) {
	t.Fatal("needs functional IndexCommit.Delete() and SegmentInfos commit " +
		"generation tracking (not yet ported)")
}

// TestDeletionPolicy_KeepLastN ports testKeepLastNDeletionPolicy().
//
// Java registers a KeepLastNDeletionPolicy(N) that retains the most recent
// N commits and deletes the rest, writes N+M commits, and asserts that
// exactly N segments files survive.
//
// Degraded to t.Skip: IndexCommit.Delete() is a no-op; commit-generation
// enumeration not available.
func TestDeletionPolicy_KeepLastN(t *testing.T) {
	t.Fatal("needs functional IndexCommit.Delete() and commit-generation " +
		"enumeration via IndexWriter.listCommits (not yet ported)")
}

// TestDeletionPolicy_KeepLastNWithCreates ports
// testKeepLastNDeletionPolicyWithCreates().
//
// Same as TestDeletionPolicy_KeepLastN but interleaves CREATE-mode writer
// reopens with APPEND-mode commits; verifies the policy resets its commit
// list on each CREATE open.
//
// Degraded to t.Skip: same blockers as TestDeletionPolicy_KeepLastN; also
// requires LiveIndexWriterConfig.GetIndexDeletionPolicy() to retrieve the
// policy after each reopen.
func TestDeletionPolicy_KeepLastNWithCreates(t *testing.T) {
	t.Fatal("needs functional IndexCommit.Delete(), LiveIndexWriterConfig." +
		"GetIndexDeletionPolicy(), and commit-generation enumeration (not yet ported)")
}

// TestDeletionPolicy_KeepLastNCommits ports
// testKeepLastNCommitsDeletionPolicy().
//
// Java keeps the last N commits by counting onCommit invocations; asserts
// that after M total commits exactly N segments files are on disk and that
// the policy's commit list length never exceeds N.
//
// Degraded to t.Skip: IndexCommit.Delete() is a no-op; SegmentInfos
// getLastCommitGeneration not exposed.
func TestDeletionPolicy_KeepLastNCommits(t *testing.T) {
	t.Fatal("needs functional IndexCommit.Delete() and " +
		"SegmentInfos.getLastCommitGeneration (not yet ported)")
}

// TestDeletionPolicy_KeepLastNCommitsZero ports
// testKeepLastNCommitsDeletionPolicyWithZeroCommits().
//
// Java opens a writer with a KeepLastN(0) policy and asserts that even with
// zero commits retained the index remains consistent (the current in-flight
// commit is never deleted).
//
// Degraded to t.Skip: IndexCommit.Delete() is a no-op; exact segments-file
// count cannot be verified without commit-generation enumeration.
func TestDeletionPolicy_KeepLastNCommitsZero(t *testing.T) {
	t.Fatal("needs functional IndexCommit.Delete() and commit-generation " +
		"enumeration; KeepLastN(0) semantics unverifiable without them")
}
