// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_facets_compat_test.go is the explicit landing pad for the
// facets audit rows whose round-trip (class c) leg Sprint 114 T12
// (rmp 4620) acknowledged but did NOT complete.
//
// ALL FOUR deferred rows from Sprint 114 T12 are now RESOLVED by T13
// (rmp 4684 + this task). The SegmentReader core-readers gap has been
// closed, and Gocene can read Lucene-emitted taxonomy directories,
// association payloads, sorted-set ords, and facet-set packed bytes.
//
// No remaining deferred rows in the facets compat suite.
package facets

import "testing"

// TestFacetsAudit_DeferredRows enumerates the facets-side legs that remain
// deferred. Currently empty — all Sprint 114 T12 rows are resolved.
func TestFacetsAudit_DeferredRows(t *testing.T) {
	// All rows resolved by T13 (rmp 4684 + facet round-trips).
	t.Log("All 4 deferred facets compat rows resolved by T13")
}
