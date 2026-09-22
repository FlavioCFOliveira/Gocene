// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// This file bridges unexported symbols to the external util_test package.
// Tests that import packages depending on util (store, for example) must live
// in util_test to avoid an import cycle, mirroring Lucene, whose tests sit
// outside the production dependency graph.

// NumBytes exposes numBytes (GroupVIntUtil.numBytes) to util_test.
var NumBytes = numBytes
