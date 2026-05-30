// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_memory_compat_test.go is the explicit landing pad for the
// memory audit rows that Sprint 114 T25 (rmp 4633) acknowledged but did
// NOT fully cover. Each entry below cites its audit row verbatim from
// docs/compat-coverage.tsv with the reason it remains deferred.
//
// Every deferral runs as a t.Skip subtest so the row appears in the
// `go test -v` output (evidence the row was considered) without failing
// the build.
package memory

import "testing"

// TestMemoryAudit_DeferredRows iterates every memory-side leg that T25
// recognised but could not complete with the current state of the
// Gocene memory port. The body of each subtest is a t.Skip with the
// row's audit citation.
//
// auditGapNotes is reproduced VERBATIM from docs/compat-coverage.tsv:
//
//	memory MemoryIndex transient in-memory postings
//	  org.apache.lucene.index.memory.MemoryIndex
//	  memory/memory_index.go
//	  yes:memory/memory_index_test.go
//	  yes:memory/memory_index_against_directory_test.go no
//	  No persisted binary artefact; gap is the absence of byte-for-byte
//	  parity tests vs Lucene MemoryIndex internal layout (where applicable
//	  to merges).
func TestMemoryAudit_DeferredRows(t *testing.T) {
	const auditGap = "No persisted binary artefact; gap is the absence of byte-for-byte parity tests vs Lucene MemoryIndex internal layout (where applicable to merges)."

	deferred := []struct {
		artefact  string // logical leg of the memory binary-parity gap
		luceneCls string // canonical Lucene 10.4.0 class name
		goceneRef string // Gocene source-file reference (relative)
		gapNotes  string // audit row gap_notes column (verbatim)
		reason    string // why this is deferred from Sprint 114 T25
	}{
		{
			artefact:  "Gocene MemoryIndex addIndexes flush-merge byte parity vs Lucene",
			luceneCls: "org.apache.lucene.index.memory.MemoryIndex",
			goceneRef: "memory/memory_index.go",
			gapNotes:  auditGap,
			reason: "rmp 4633 ships the Lucene-side memory-index-flush scenario " +
				"and its verifier (verify-memory-flush). The Gocene-side " +
				"replay (rebuild a Gocene MemoryIndex from the same token " +
				"stream, wrap it via the Gocene CodecReader equivalent and " +
				"flush it through Gocene's IndexWriter.AddIndexes so the " +
				"resulting segment matches Lucene byte-for-byte) is blocked " +
				"because the Gocene MemoryIndex equivalent may not support " +
				"the addIndexes(CodecReader...) flush path. Deferred until " +
				"that surface lands. The harness verifier IS exercised by " +
				"memory_index_flush_compat_test.go::TestMemoryIndexFlush_" +
				"VerifySubcommand and the read-fixture / byte-determinism " +
				"legs are already covered.",
		},
	}

	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Fatalf("deferred: %s (lucene_class=%q gocene_ref=%q gap_notes=%q): %s",
				row.artefact, row.luceneCls, row.goceneRef, row.gapNotes, row.reason)
		})
	}
}
