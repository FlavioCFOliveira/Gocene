// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_facets_compat_test.go is the explicit landing pad for the
// facets audit rows whose round-trip (class c) leg Sprint 114 T12
// (rmp 4620) acknowledged but did NOT complete. Each entry below cites
// its audit row verbatim from docs/compat-coverage.tsv with the reason
// it remains deferred.
//
// Every deferral runs as a t.Skip subtest so the row appears in the
// `go test -v` output (evidence the row was considered) without failing
// the build. The Lucene-side fixture + verifier IS exercised by the
// per-scenario *_compat_test.go files in this package; what defers is
// the symmetric Gocene-side reader path that cannot yet open a Lucene-
// emitted segment due to the SegmentReader core-readers gap recorded
// under memory-index reference 'gocene-segmentreader-corereaders-gap'.
package facets

import "testing"

// TestFacetsAudit_DeferredRows iterates every facets-side leg that T12
// recognised but could not complete with the current state of the
// Gocene facets port. The body of each subtest is a t.Skip with the
// row's audit citation.
//
// Each gap_notes string is reproduced VERBATIM from docs/compat-coverage.tsv
// row 54..57 (lucene_class column is the canonical Lucene 10.4.0 type
// pulled from /tmp/lucene/lucene/facet/src/java/...).
func TestFacetsAudit_DeferredRows(t *testing.T) {
	deferred := []struct {
		artefact  string // logical leg of the facets parity gap
		luceneCls string // canonical Lucene 10.4.0 class name
		gocenePkg string // canonical Gocene gocene_class column
		gapNotes  string // audit row gap_notes column (verbatim)
		reason    string // why this is deferred from Sprint 114 T12
	}{
		{
			artefact:  "Gocene DirectoryTaxonomyReader round-trip vs Lucene",
			luceneCls: "org.apache.lucene.facet.taxonomy.directory.DirectoryTaxonomyWriter",
			gocenePkg: "facets/directory_taxonomy_writer.go",
			gapNotes:  "No fixture from Lucene-emitted taxonomy directory.",
			reason: "rmp 4620 ships the Lucene-side scenario " +
				"\"taxonomy-directory\" and its verifier. The Gocene-side " +
				"replay (open the Lucene-emitted taxo/ sidecar with " +
				"Gocene's DirectoryTaxonomyReader and re-assert every " +
				"FacetLabel ord) is blocked on the SegmentReader core-" +
				"readers gap recorded under memory-index reference " +
				"'gocene-segmentreader-corereaders-gap': the leaf-reader " +
				"Terms API trips on a nil core reader before the taxonomy " +
				"reader is reached. The harness verifier IS exercised by " +
				"taxonomy_directory_compat_test.go::" +
				"TestTaxonomyDirectory_VerifySubcommand.",
		},
		{
			artefact:  "Gocene AssociationFacetField decoder round-trip vs Lucene",
			luceneCls: "org.apache.lucene.facet.taxonomy.AssociationFacetField",
			gocenePkg: "facets/taxonomy/association_facet_field.go",
			gapNotes:  "No byte-level fixture for association payloads.",
			reason: "Association payloads persist into $facets.int / " +
				"$facets.float BinaryDocValues fields. The Gocene-side " +
				"replay requires the SegmentReader core-readers wiring " +
				"AND a Gocene leaf-reader path that exposes " +
				"BinaryDocValues from a Lucene-emitted .dvd/.dvm pair. " +
				"Both are blocked by 'gocene-segmentreader-corereaders-" +
				"gap'. The harness verifier IS exercised by " +
				"association_payload_compat_test.go::" +
				"TestFacetAssociationPayload_VerifySubcommand.",
		},
		{
			artefact:  "Gocene DefaultSortedSetDocValuesReaderState round-trip vs Lucene",
			luceneCls: "org.apache.lucene.facet.sortedset.DefaultSortedSetDocValuesReaderState",
			gocenePkg: "facets/sortedset/default_sorted_set_doc_values_reader_state.go",
			gapNotes:  "No Lucene-emitted sorted-set ord file consumed by tests.",
			reason: "Sorted-set facet ords persist into a " +
				"SortedSetDocValues field. The Gocene-side replay " +
				"requires the SegmentReader core-readers wiring AND a " +
				"Gocene leaf-reader path that exposes SortedSetDocValues " +
				"from a Lucene-emitted .dvd/.dvm pair. Both are blocked " +
				"by 'gocene-segmentreader-corereaders-gap'. The harness " +
				"verifier IS exercised by sortedset_ords_compat_test.go::" +
				"TestFacetSortedsetOrds_VerifySubcommand.",
		},
		{
			artefact:  "Gocene FacetSetDecoder round-trip vs Lucene",
			luceneCls: "org.apache.lucene.facet.facetset.FacetSet",
			gocenePkg: "facets/facetset/facet_set.go",
			gapNotes:  "No Lucene-produced FacetSet bytes used in tests.",
			reason: "FacetSets persist as packed-bytes into a " +
				"BinaryDocValues field via FacetSetsField. The Gocene-" +
				"side replay requires the SegmentReader core-readers " +
				"wiring AND a Gocene FacetSetDecoder path that can " +
				"consume a Lucene-emitted BinaryDocValues stream. Both " +
				"are blocked by 'gocene-segmentreader-corereaders-gap'. " +
				"The harness verifier IS exercised by " +
				"facet_set_compat_test.go::" +
				"TestFacetSetPackedBytes_VerifySubcommand.",
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
