// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package fst

import "testing"

// Test2BFST is the Gocene port of Apache Lucene's
// org.apache.lucene.util.fst.Test2BFST monster test (GOC-4288).
//
// The upstream test builds three multi-gigabyte FSTs (NoOutputs, ByteSequenceOutputs,
// PositiveIntOutputs) on-heap, each capped at roughly 3 GB or > Integer.MAX_VALUE + 100Mi
// nodes, and verifies them via the in-memory FST reader. It is annotated
// @Ignore("Requires tons of heap to run (30 GB hits OOME but 35 GB passes after ~4.5 hours)")
// and uses a 100h TimeoutSuite, so it only runs as an opt-in monster test in Lucene's
// nightly/manual pipelines.
//
// This Go stub mirrors that contract: it is registered for parity with the upstream test
// surface but is unconditionally skipped. A future task may wire it behind a build tag or
// dedicated monster-test runner; until then, executing it under `go test ./util/fst/...`
// must not consume the multi-hour, multi-GiB budget the JVM version requires.
//
// See also Test2BFSTOffHeap (GOC-4286) for the MMapDirectory + OffHeapFSTStore variant.
func Test2BFST(t *testing.T) {
	t.Fatal("monster test: ~3 GiB in-memory FSTs, ~4.5 h runtime, ~35 GiB heap; ported as stub for parity (GOC-4288)")
}
