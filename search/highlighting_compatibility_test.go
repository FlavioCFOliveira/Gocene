// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// GC-907: Highlighting Compatibility Tests
// Validates highlighters generate identical highlighted fragments
// for search results with various fragmenter and scorer configurations.
//
// NOTE: These tests require the highlight package which is not yet fully implemented.
// The tests below serve as markers; full integration tests will be added when the
// highlight path is available.

import "testing"

func TestHighlightingCompatibility_BasicHighlighting(t *testing.T) {
	// Basic highlighting requires the highlight package (NewHighlighter).
	// This marker test preserves the entry point. A full test will index
	// documents, run queries, generate highlighted fragments, and compare
	// against Lucene 10.4.0 fixtures.
}

func TestHighlightingCompatibility_MultipleTerms(t *testing.T) {
	// Multiple-term highlighting tests the interaction of phrase matching,
	// term expansion, and fragment generation. This marker preserves the
	// entry point for when the highlight package is available.
}
