// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import "testing"

// TestPointQueryParser is a port of
// org.apache.lucene.queryparser.flexible.standard.TestPointQueryParser.
//
// The Java test verifies that StandardQueryParser produces IntPoint/LongPoint/
// FloatPoint/DoublePoint range queries when a PointsConfig map is registered.
//
// Execution is deferred because:
//   - SetPointsConfigMap is not yet wired into Gocene's StandardQueryParser
//   - IntPoint/LongPoint/FloatPoint/DoublePoint range query factories are not
//     yet available in the Gocene document package
//
// Port of: queryparser/src/test/.../flexible/standard/TestPointQueryParser.java
func TestPointQueryParser(t *testing.T) {
	t.Fatal("deferred: requires SetPointsConfigMap and point range query factories (IntPoint/LongPoint/etc.)")
}
