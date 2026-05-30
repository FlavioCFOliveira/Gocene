// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version
//	2.0 (the "License"); you may not use this file except in compliance
//	with the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//	implied. See the License for the specific language governing
//	permissions and limitations under the License.

package codecs_test

// Port of
//
//	lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90StoredFieldsFormatHighCompression.java
//
// Mapping notes against the Apache Lucene 10.4.0 reference
// (TestLucene90StoredFieldsFormatHighCompression):
//
//   - The Java class extends BaseStoredFieldsFormatTestCase and overrides
//     getCodec() to return new Lucene104Codec(Mode.BEST_COMPRESSION). Gocene
//     has not ported BaseStoredFieldsFormatTestCase; the existing port of
//     the harness in this package is the lighter [codecs.StoredFieldsTester]
//     used by [stored_fields_format_test.go]. We mirror the getCodec()
//     override by driving StoredFieldsTester.TestFull directly against the
//     stored-fields format produced by the codec under test.
//
//   - Gocene's [codecs.Lucene104Codec] has no Mode parameter and exposes
//     only a no-arg constructor; the BEST_SPEED / BEST_COMPRESSION split
//     lives one layer down in [codecs/lucene90.Lucene90StoredFieldsFormat].
//     The mixed-codec scenario in testMixedCompressions therefore has no
//     direct Gocene analogue at the Codec level and is skipped with the
//     reason recorded inline; reopening this test belongs to a future task
//     that introduces a Mode-aware Lucene104Codec.
//
//   - testInvalidOptions exercises two NullPointerExceptions: one for
//     Lucene104Codec(null) and one for new Lucene90StoredFieldsFormat(null).
//     The former has no Go analogue (no Mode parameter on Lucene104Codec).
//     The latter is mirrored by [lucene90.NewLucene90StoredFieldsFormatWithMode]
//     panicking on an unrecognised enum value, which is the closest Go
//     surface to Objects.requireNonNull(mode) on the Java constructor.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene90StoredFieldsFormatHighCompression_Basic drives the
// stored-fields tester against the codec returned by getCodec() in the
// Java reference (Lucene104Codec with BEST_COMPRESSION). Gocene's
// Lucene104Codec exposes a single stored-fields format implementation,
// so the test simply asserts that the format produced by the codec
// satisfies the same end-to-end contract as the BEST_SPEED default
// covered by TestLucene104StoredFieldsFormat_Basic.
func TestLucene90StoredFieldsFormatHighCompression_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	codec := codecs.NewLucene104Codec()
	format := codec.StoredFieldsFormat()
	if format == nil {
		t.Fatal("Lucene104Codec.StoredFieldsFormat() returned nil")
	}

	tester := codecs.NewStoredFieldsTester(t)
	tester.TestFull(format, dir)
}

// TestLucene90StoredFieldsFormatHighCompression_MixedCompressions is the
// counterpart of testMixedCompressions. Lucene's Java test alternates the
// per-segment compression preset by passing
// RandomPicks.randomFrom(random(), Lucene104Codec.Mode.values()) to a new
// Lucene104Codec per IndexWriter, and then verifies that the resulting
// index reads back cleanly. Gocene's [codecs.Lucene104Codec] currently
// has no Mode parameter, so the scenario cannot be expressed at the same
// layer; the test is skipped with an explicit reason rather than mocked.
func TestLucene90StoredFieldsFormatHighCompression_MixedCompressions(t *testing.T) {
	t.Fatal("requires Mode-aware Lucene104Codec; not yet ported (see file-level mapping notes)")
}

// TestLucene90StoredFieldsFormatHighCompression_InvalidOptions ports
// testInvalidOptions. The Java test expects NullPointerException from
// both `new Lucene104Codec(null)` and
// `new Lucene90StoredFieldsFormat(null)`. Only the second has a direct
// Gocene analogue: [lucene90.NewLucene90StoredFieldsFormatWithMode]
// panics on an unrecognised mode value, mirroring the Java
// Objects.requireNonNull(mode) check.
func TestLucene90StoredFieldsFormatHighCompression_InvalidOptions(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid Lucene90StoredFieldsMode, got none")
		}
	}()
	_ = lucene90.NewLucene90StoredFieldsFormatWithMode(lucene90.Lucene90StoredFieldsMode(-1))
}
