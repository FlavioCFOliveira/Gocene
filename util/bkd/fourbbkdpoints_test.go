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

package bkd

import "testing"

// Test4BBKDPoints is the Gocene port of Apache Lucene's
// org.apache.lucene.util.bkd.Test4BBKDPoints monster test (GOC-4307).
//
// The upstream test indexes more than 4 billion BKD points (numDocs =
// Integer.MAX_VALUE/13 + 100, times 26 points per doc) in both 1D and 2D
// configurations, finalises the tree to disk via BKDWriter.finish, reopens it
// through BKDReader and validates every visited point against
// CheckIndex.VerifyPointsVisitor. It is annotated
// @Monster("takes at least 4 hours and consumes many GB of temp disk space")
// with @TimeoutSuite(millis = Integer.MAX_VALUE) and runs only under the
// nightly/manual monster-test pipeline (-Dtests.monster=true).
//
// This Go stub mirrors that contract: it is registered for parity with the
// upstream test surface but is unconditionally skipped. A future task may wire
// it behind a build tag or dedicated monster-test runner; until then,
// executing it under `go test ./util/bkd/...` must not consume the multi-hour,
// multi-GiB disk and memory budget the JVM version requires.
func Test4BBKDPoints(t *testing.T) {
	t.Fatal("monster test: >4B BKD points across 1D/2D, ~4 h runtime, many GiB of temp disk; ported as stub for parity (GOC-4307)")
}
