// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import "testing"

// TestMultiFieldQPHelper is a port of
// org.apache.lucene.queryparser.flexible.standard.TestMultiFieldQPHelper.
//
// The Java test verifies multi-field query parsing (MultiFieldQueryParser
// behaviour) in the flexible standard query parser.
//
// Execution is deferred because Gocene does not yet expose a MultiFieldQueryParser
// wrapper over the flexible StandardQueryParser.
//
// Port of: queryparser/src/test/.../flexible/standard/TestMultiFieldQPHelper.java
func TestMultiFieldQPHelper(t *testing.T) {
	t.Fatal("deferred: requires MultiFieldQueryParser for the flexible StandardQueryParser")
}
