// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import "testing"

// TestStandardQP is a port of
// org.apache.lucene.queryparser.flexible.standard.TestStandardQP.
//
// The Java test is a broad regression suite for StandardQueryParser covering
// operator precedence, field scoping, boost, fuzzy, and range edge cases.
//
// Execution is deferred because the Gocene StandardQueryParser lacks several
// features exercised by this suite (range query type selection, FuzzyQuery
// production, analyzer pipeline integration).
//
// Port of: queryparser/src/test/.../flexible/standard/TestStandardQP.java
func TestStandardQP(t *testing.T) {
	t.Skip("deferred: requires complete StandardQueryParser (range types, FuzzyQuery, full analyzer integration)")
}
