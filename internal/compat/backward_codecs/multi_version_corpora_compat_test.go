// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// multi_version_corpora_compat_test.go is the audit anchor for the
// backward_codecs row (verbatim from docs/compat-coverage.tsv):
//
//	backward_codecs	Backwards-compat full index corpora (multi-version)
//	    lucene_class:  org.apache.lucene.backward_index.TestBasicBackwardsCompatibility
//	    gocene_class:  backward_codecs/backward_index/
//	    isolated:      no
//	    integration:   yes:backward_codecs/backward_index/backwards_compatibility_test_base_test.go
//	    binary_compat: no
//	    gap_notes:     "Tests are skeletons; no actual multi-version Lucene index ZIPs committed."
//
// The corresponding harness scenario "bwc-multi-version-corpora" is NOT
// registered in tools/lucene-fixtures/Scenarios.java; it lives in
// Manifest.DEFERRED_ROWS because producing the per-major-version index
// ZIPs that TestBasicBackwardsCompatibility consumes requires building
// EACH old Lucene major (7.x, 8.x, 9.x, 10.x) and emitting an index per
// branch — outside the binary-compatibility mandate's 10.4.0 reference
// pin.
package backward_codecs

import "testing"

// TestMultiVersionCorpora_Deferred surfaces the audit row in
// `go test -v` output with the verbatim audit citation and the
// multi-version corpus deferral reason.
func TestMultiVersionCorpora_Deferred(t *testing.T) {
	const (
		auditRow  = "Backwards-compat full index corpora (multi-version)"
		luceneCls = "org.apache.lucene.backward_index.TestBasicBackwardsCompatibility"
		gocenePkg = "backward_codecs/backward_index/"
		gapNotes  = "Tests are skeletons; no actual multi-version Lucene index ZIPs committed."
		reason    = "Producing the per-major-version index ZIPs that " +
			"TestBasicBackwardsCompatibility consumes requires building EACH " +
			"old Lucene major (7/8/9/10) and emitting an index per branch; " +
			"out of binary-compat-mandate scope (10.4.0 reference pin); " +
			"covered by a future backward-compat sprint that maintains a " +
			"multi-version fixture corpus."
	)
	t.Skipf("deferred: %s (lucene_class=%q gocene_class=%q gap_notes=%q): %s "+
		"(scenario %q lives in tools/lucene-fixtures/Manifest.DEFERRED_ROWS)",
		auditRow, luceneCls, gocenePkg, gapNotes, reason, ScenarioBwcMultiVersionCorpora)
}
