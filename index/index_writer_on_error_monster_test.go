// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// Package index_test contains the @Nightly test ports from Apache Lucene's
// org.apache.lucene.index.TestIndexWriterOnError.
//
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterOnError.java
// Reference tag: releases/lucene/10.4.0 (commit 9983b7c)
//
// These tests are excluded from the standard suite because Lucene annotates
// them @Nightly. They share the helper doIndexWriterOnErrorTest defined in
// index_writer_on_error_test.go (same package, default build).
package index_test

import (
	"testing"
)

// TestIndexWriterOnError_Checkpoint ports testCheckpoint().
//
// The Java test is @Nightly: it injects a fake OutOfMemoryError specifically
// from IndexFileDeleter.checkpoint frames. This Go port uses the same generic
// Commit failure path.
func TestIndexWriterOnError_Checkpoint(t *testing.T) {
	doIndexWriterOnErrorTest(t, "Fake OutOfMemoryError (IndexFileDeleter.checkpoint)")
}
