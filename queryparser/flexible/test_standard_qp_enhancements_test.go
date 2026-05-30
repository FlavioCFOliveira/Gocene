// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import "testing"

// TestStandardQPEnhancements is a port of
// org.apache.lucene.queryparser.flexible.standard.TestStandardQPEnhancements.
//
// The Java test covers enhancements such as MultiTermQuery rewrite method
// control, date resolution, and boost handling in the flexible parser.
//
// Execution is deferred because MultiTermQuery rewrite and date-range
// resolution are not yet implemented in the Gocene StandardQueryParser.
//
// Port of: queryparser/src/test/.../flexible/standard/TestStandardQPEnhancements.java
func TestStandardQPEnhancements(t *testing.T) {
	t.Fatal("deferred: requires MultiTermQuery rewrite method and date-range resolution in StandardQueryParser")
}
