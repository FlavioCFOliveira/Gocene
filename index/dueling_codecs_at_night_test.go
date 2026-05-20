// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// TestDuelingCodecsAtNight ports org.apache.lucene.index.TestDuelingCodecsAtNight.
//
// It extends TestDuelingCodecs, building two random indexes (atLeast(2000)
// documents each) with different codecs and asserting the readers are
// equivalent. In Lucene it is annotated @Nightly and @SuppressCodecs("Direct"),
// so it is skipped by default.
func TestDuelingCodecsAtNight(t *testing.T) {
	t.Skip("nightly test: dueling-codecs equality over atLeast(2000) docs per side")
}
