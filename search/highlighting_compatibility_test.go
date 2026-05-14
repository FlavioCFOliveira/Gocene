// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// GC-907: Highlighting Compatibility Tests
// Validates highlighters generate identical highlighted fragments
// for search results with various fragmenter and scorer configurations.
//
// NOTE: These tests are skipped because the highlight package (highlight.NewHighlighter)
// is not yet implemented.

import "testing"

func TestHighlightingCompatibility_BasicHighlighting(t *testing.T) {
	t.Skip("highlight package not yet implemented")
}

func TestHighlightingCompatibility_MultipleTerms(t *testing.T) {
	t.Skip("highlight package not yet implemented")
}
