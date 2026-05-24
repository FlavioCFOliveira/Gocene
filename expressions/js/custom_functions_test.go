// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.js.TestCustomFunctions.
// The Java original tests passing custom MethodHandles to JavascriptCompiler.
// In Go, custom functions would be registered as named func values; this
// mechanism is not yet implemented in Gocene's JavascriptCompiler.
// Tests are deferred until a custom-function registry is added.
package js_test

import "testing"

// TestCustomFunctions skips because it requires a custom-function registry
// (MethodHandle equivalent in Go) not yet implemented in JavascriptCompiler.
func TestCustomFunctions(t *testing.T) {
	t.Skip("requires custom function registry in JavascriptCompiler (not yet implemented)")
}
