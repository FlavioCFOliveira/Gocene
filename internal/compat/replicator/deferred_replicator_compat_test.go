// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_replicator_compat_test.go is the explicit landing pad for the
// replicator audit rows that Sprint 114 T19 (rmp 4627) acknowledged but
// did NOT cover with a Java-served fixture. Each entry below cites its
// audit row verbatim from docs/compat-coverage.tsv with the reason it
// remains deferred.
//
// Every deferral runs as a t.Skip subtest so the row appears in the
// `go test -v` output (evidence the row was considered) without failing
// the build.
//
// Root cause for both deferrals: Apache Lucene 10.4.0 (tag
// releases/lucene/10.4.0) removed the HTTP replicator and index revision
// surface from the production tree. The directory
// `lucene/replicator/src/java/org/apache/lucene/replicator/` contains
// ONLY the `nrt` subpackage; HttpReplicator, SessionToken, RevisionFile
// and IndexRevision do not exist in 10.4.0. Producing fixtures with the
// canonical Java implementation against this Lucene tag is therefore
// impossible without backporting code, which is out of scope for T19 and
// will be tackled by a future backward-compat sprint that pulls fixtures
// from older Lucene branches.
package replicator

import "testing"

// TestReplicatorAudit_DeferredRows iterates every replicator-side leg
// that T19 recognised but could not complete because the upstream Lucene
// 10.4.0 surface is gone. The body of each subtest is a t.Skip with the
// row's audit citation, the explicit removal-in-10.4.0 reason, and the
// matching manifests/baseline.tsv DEFERRED row name.
//
// auditGapNotes for the two rows are reproduced VERBATIM from
// docs/compat-coverage.tsv:
//
//	replicator HTTP replicator wire payloads
//	  No Java-served HTTP replicator fixtures.
//
//	replicator Session/file revision serialisation
//	  No cross-engine replication transcript validated against Lucene.
func TestReplicatorAudit_DeferredRows(t *testing.T) {
	const auditGapHTTP = "No Java-served HTTP replicator fixtures."
	const auditGapSession = "No cross-engine replication transcript validated against Lucene."

	// removalReason is shared across both rows: the underlying Lucene
	// 10.4.0 production surface was deleted between Lucene 9.x and 10.x
	// (see git history of upstream apache/lucene at tag
	// releases/lucene/10.4.0). Stating it explicitly in every Skip makes
	// it obvious to the reviewer why no fixture is being emitted.
	const removalReason = "Lucene 10.4.0 removed org.apache.lucene.replicator.http.HttpReplicator, " +
		"org.apache.lucene.replicator.SessionToken, org.apache.lucene.replicator.RevisionFile, " +
		"and the IndexRevision wire surface from production sources " +
		"(lucene/replicator/src/java/org/apache/lucene/replicator/ at tag " +
		"releases/lucene/10.4.0 contains ONLY the nrt/ subpackage). " +
		"Covered by a future backward-compat sprint that imports fixtures " +
		"produced by an older Lucene branch."

	deferred := []struct {
		artefact    string // logical leg of the replicator binary-parity gap
		luceneCls   string // canonical Lucene class name (removed in 10.4.0)
		goceneRef   string // Gocene source-file reference (relative)
		manifestRow string // manifests/baseline.tsv DEFERRED row name
		gapNotes    string // audit row gap_notes column (verbatim)
		reasonExtra string // why this is deferred from Sprint 114 T19
	}{
		{
			artefact:    "Gocene HTTP replicator wire-payload parity vs Lucene",
			luceneCls:   "org.apache.lucene.replicator.http.HttpReplicator (removed in 10.4.0)",
			goceneRef:   "replicator/ (no Gocene HTTP replicator port)",
			manifestRow: "replicator-http-frames",
			gapNotes:    auditGapHTTP,
			reasonExtra: "Gocene does not yet port the HTTP replicator (none of " +
				"replicator/ exposes an HTTP server / client equivalent) " +
				"and the Lucene-side counterpart no longer exists in the " +
				"target tag. Manifest row 'replicator-http-frames' carries " +
				"the deferral.",
		},
		{
			artefact:    "Gocene session/file revision serialisation parity vs Lucene",
			luceneCls:   "org.apache.lucene.replicator.SessionToken / RevisionFile / IndexRevision (removed in 10.4.0)",
			goceneRef:   "replicator/ (no Gocene IndexRevision port)",
			manifestRow: "replicator-session-revision",
			gapNotes:    auditGapSession,
			reasonExtra: "Gocene does not yet port the IndexRevision surface " +
				"(no SessionToken / RevisionFile / IndexRevision in " +
				"replicator/) and the Lucene-side counterpart no longer " +
				"exists in the target tag. Manifest row " +
				"'replicator-session-revision' carries the deferral.",
		},
	}

	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Fatalf("deferred: %s (lucene_class=%q gocene_ref=%q manifest_row=%q "+
				"gap_notes=%q): %s %s",
				row.artefact, row.luceneCls, row.goceneRef, row.manifestRow,
				row.gapNotes, removalReason, row.reasonExtra)
		})
	}
}
