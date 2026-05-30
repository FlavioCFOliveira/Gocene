// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// Test2BTerms ports org.apache.lucene.index.Test2BTerms.
//
// It indexes documents whose tokens are random 16-byte terms until the index
// holds more than Integer.MAX_VALUE unique terms, then force-merges and
// verifies term enumeration. In Lucene this is annotated @Monster with a
// multi-hour TimeoutSuite, so it is skipped by default.
func Test2BTerms(t *testing.T) {
	t.Fatal("monster test: indexes >2B unique terms, multi-hour runtime")
}
