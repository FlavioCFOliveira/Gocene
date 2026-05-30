// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_suggest_compat_test.go is the explicit landing pad for the
// suggest audit rows whose Gocene-side legs (class b write-and-verify
// and class c round-trip) Sprint 114 T13 (rmp 4621) acknowledged but
// did NOT complete. Each entry below cites its audit row verbatim from
// docs/compat-coverage.tsv with the reason it remains deferred.
//
// Every deferral runs as a t.Skip subtest so the row appears in the
// `go test -v` output (evidence the row was considered) without failing
// the build. The Lucene-side fixture + verifier IS exercised by the
// per-scenario *_compat_test.go files in this package; what defers is
// the symmetric Gocene-side reader/writer path that does not yet
// implement the Lucene 10.4.0 wire format.
package suggest

import "testing"

// TestSuggestAudit_DeferredRows iterates every suggest-side leg that T13
// recognised but could not complete with the current state of the
// Gocene suggest port. The body of each subtest is a t.Skip with the
// row's audit citation.
//
// Each gap_notes string is reproduced VERBATIM from docs/compat-coverage.tsv
// rows 58..61 (lucene_class column is the canonical Lucene 10.4.0 type
// pulled from /tmp/lucene/lucene/suggest/src/java/...).
func TestSuggestAudit_DeferredRows(t *testing.T) {
	deferred := []struct {
		artefact  string // logical leg of the suggest parity gap
		luceneCls string // canonical Lucene 10.4.0 class name
		gocenePkg string // canonical Gocene gocene_class column
		gapNotes  string // audit row gap_notes column (verbatim)
		reason    string // why this is deferred from Sprint 114 T13
	}{
		{
			artefact:  "Gocene AnalyzingSuggester Store/Load round-trip vs Lucene",
			luceneCls: "org.apache.lucene.search.suggest.fst.FSTCompletionBuilder",
			gocenePkg: "suggest/fst/fst_completion_builder.go",
			gapNotes:  "No round-trip against Lucene-compiled completion FST.",
			reason: "rmp 4621 ships the Lucene-side scenario \"completion-fst\" " +
				"and its verifier. The Gocene-side replay (write a completion.fst " +
				"that Lucene's AnalyzingSuggester.load can consume, and read a " +
				"Lucene-emitted blob with Gocene) is blocked because " +
				"suggest/analyzing/analyzing_suggester.go exposes only Build/" +
				"LookupResults/GetCount; the Store(DataOutput)/Load(DataInput) " +
				"surface from Lucene's AnalyzingSuggester is not yet ported. " +
				"The harness verifier IS exercised by " +
				"completion_fst_compat_test.go::TestCompletionFst_VerifySubcommand.",
		},
		{
			artefact:  "Gocene WFSTCompletionLookup Store/Load round-trip vs Lucene",
			luceneCls: "org.apache.lucene.search.suggest.fst.WFSTCompletionLookup",
			gocenePkg: "suggest/fst/wfst_completion_lookup.go",
			gapNotes:  "No combined test; no Lucene fixture.",
			reason: "rmp 4621 ships the Lucene-side scenario \"wfst-blob\" and " +
				"its verifier. The Gocene-side replay is blocked because " +
				"suggest/fst/wfst_completion_lookup.go exposes only Build; the " +
				"Store(DataOutput)/Load(DataInput) methods that emit the Lucene " +
				"10.4.0 wire format (writeVLong(count) followed by FST.save) are " +
				"not yet ported. The harness verifier IS exercised by " +
				"wfst_compat_test.go::TestWfst_VerifySubcommand.",
		},
		{
			artefact:  "Gocene AnalyzingInfixSuggester sidecar round-trip vs Lucene",
			luceneCls: "org.apache.lucene.search.suggest.analyzing.AnalyzingInfixSuggester",
			gocenePkg: "suggest/analyzing_infix_suggester.go",
			gapNotes:  "No tests for this writer; data files never validated.",
			reason: "rmp 4621 ships the Lucene-side scenario \"analyzing-infix-" +
				"sidecar\" and its verifier. The Gocene-side replay requires a " +
				"Gocene writer that emits a Lucene-readable single-segment " +
				"compound-file index, plus a reader that consumes one. Both are " +
				"blocked by the SegmentReader core-readers gap recorded under " +
				"memory-index reference 'gocene-segmentreader-corereaders-gap'. " +
				"The harness verifier IS exercised by " +
				"analyzing_infix_compat_test.go::TestAnalyzingInfix_VerifySubcommand.",
		},
		{
			artefact:  "Gocene Completion104PostingsFormat .lkp/.cmp round-trip vs Lucene",
			luceneCls: "org.apache.lucene.search.suggest.document.Completion104PostingsFormat",
			gocenePkg: "suggest/document/completion_postings_format.go",
			gapNotes:  "No isolated, combined, or fixture coverage of completion postings format.",
			reason: "rmp 4621 ships the Lucene-side scenario \"completion104-" +
				"postings\" and its verifier (PrefixCompletionQuery over the " +
				"SuggestField corpus). The Gocene-side replay requires a Gocene " +
				"PostingsFormat that emits the .lkp dictionary FST and the " +
				"matching .cmp index, plus a reader that consumes a Lucene-" +
				"emitted pair. Both are blocked by " +
				"'gocene-segmentreader-corereaders-gap' (SegmentWriteState " +
				"plumbing) and the missing concrete CompletionFieldsConsumer " +
				"writer body. The harness verifier IS exercised by " +
				"completion104_postings_compat_test.go::" +
				"TestCompletion104Postings_VerifySubcommand.",
		},
	}

	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Fatalf("deferred: %s (lucene_class=%q gocene_class=%q gap_notes=%q): %s",
				row.artefact, row.luceneCls, row.gocenePkg, row.gapNotes, row.reason)
		})
	}
}
