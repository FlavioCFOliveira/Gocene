// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import "testing"

// TestMultiAnalyzerQPHelper is a port of
// org.apache.lucene.queryparser.flexible.standard.TestMultiAnalyzerQPHelper.
//
// The Java test exercises the flexible StandardQueryParser with analyzers that
// return multiple tokens per position (synonym expansion).
//
// Execution is deferred because multi-token position handling (SynonymQuery
// production) is not yet implemented in the Gocene flexible parser.
//
// Port of: queryparser/src/test/.../flexible/standard/TestMultiAnalyzerQPHelper.java
func TestMultiAnalyzerQPHelper(t *testing.T) {
	t.Fatal("deferred: requires multi-token position handling (SynonymQuery) in the flexible StandardQueryParser")
}
