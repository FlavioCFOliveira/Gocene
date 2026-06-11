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
//   - Gocene's [codecs.Lucene104Codec] now supports Mode via
//     [codecs.NewLucene104CodecWithMode]. BEST_SPEED uses LZ4 fast
//     compression (16KB chunks); BEST_COMPRESSION uses Deflate (64KB
//     chunks). The mixed-codec scenario uses two codec instances with
//     different modes against isolated directories.
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
// Java reference (Lucene104Codec with BEST_COMPRESSION). Uses the
// mode-aware constructor to match the Java test's compression setting.
func TestLucene90StoredFieldsFormatHighCompression_Basic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	codec := codecs.NewLucene104CodecWithMode(codecs.Lucene104CodecBestCompression)
	if codec.Mode() != codecs.Lucene104CodecBestCompression {
		t.Fatalf("Mode: got %d, want %d", codec.Mode(), codecs.Lucene104CodecBestCompression)
	}
	format := codec.StoredFieldsFormat()
	if format == nil {
		t.Fatal("Lucene104Codec.StoredFieldsFormat() returned nil")
	}

	tester := codecs.NewStoredFieldsTester(t)
	tester.TestFull(format, dir)
}

// TestLucene90StoredFieldsFormatHighCompression_MixedCompressions is the
// counterpart of testMixedCompressions. Lucene's Java test alternates the
// per-segment compression preset by passing different Mode values to the
// codec constructor and verifies that the resulting indexes read back
// cleanly. We create two isolated directories, one per mode, and verify
// that each format produced by the mode-aware constructor satisfies the
// end-to-end contract.
func TestLucene90StoredFieldsFormatHighCompression_MixedCompressions(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode codecs.Lucene104CodecMode
	}{
		{"BestSpeed", codecs.Lucene104CodecBestSpeed},
		{"BestCompression", codecs.Lucene104CodecBestCompression},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			codec := codecs.NewLucene104CodecWithMode(tc.mode)
			if codec.Mode() != tc.mode {
				t.Fatalf("Mode: got %d, want %d", codec.Mode(), tc.mode)
			}
			format := codec.StoredFieldsFormat()
			if format == nil {
				t.Fatal("StoredFieldsFormat() returned nil")
			}

			tester := codecs.NewStoredFieldsTester(t)
			tester.TestFull(format, dir)
		})
	}
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
