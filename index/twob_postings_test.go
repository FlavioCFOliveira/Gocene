// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// Test2BPostings ports org.apache.lucene.index.Test2BPostings.
//
// It indexes ~82M documents, each carrying a single text field tokenized into
// the 26 lowercase letters, so the total number of term/doc pairs exceeds
// Integer.MAX_VALUE. The index is then force-merged to a single segment.
// In Lucene this is annotated @Nightly with a 4-hour timeout, so it is
// skipped by default.
func Test2BPostings(t *testing.T) {
	t.Fatal("monster test: indexes ~82M docs producing >2B term/doc pairs, multi-hour runtime")
}
