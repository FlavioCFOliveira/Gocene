// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Test file: lucene99_hnsw_vectors_format_v0_test.go
// Source: lucene/core/src/test/org/apache/lucene/codecs/lucene99/TestLucene99HnswVectorsFormatV0.java
// Purpose: Verifies the V0 wire of Lucene99HnswVectorsFormat: the constructor
//          variant equivalent to `new Lucene99HnswVectorsFormat(DEFAULT_MAX_CONN,
//          DEFAULT_BEAM_WIDTH, DEFAULT_NUM_MERGE_WORKER, null, 0)` and the
//          `supportsFloatVectorFallback()` override returning false.

package codecs_test

import (
	"strings"
	"testing"
)

// newLucene99HnswVectorsFormatV0Config mirrors the Java V0 constructor used in
// TestLucene99HnswVectorsFormatV0.getCodec():
//
//	new Lucene99HnswVectorsFormat(
//	    DEFAULT_MAX_CONN, DEFAULT_BEAM_WIDTH, DEFAULT_NUM_MERGE_WORKER, null, 0)
//
// i.e. defaults for maxConn/beamWidth, single merge worker, no executor, and a
// tinySegmentsThreshold of 0.
func newLucene99HnswVectorsFormatV0Config(t *testing.T) *Lucene99HnswVectorsFormatConfig {
	t.Helper()
	cfg, err := NewLucene99HnswVectorsFormatConfigWithThreshold(
		Lucene99HnswVectorsFormat_DEFAULT_MAX_CONN,
		Lucene99HnswVectorsFormat_DEFAULT_BEAM_WIDTH,
		0,
	)
	if err != nil {
		t.Fatalf("V0 config construction failed: %v", err)
	}
	return cfg
}

// TestLucene99HnswVectorsFormatV0_GetCodec mirrors the V0 getCodec() override:
// the format must be constructible with the defaults plus a zero
// tinySegmentsThreshold and the default single merge worker.
func TestLucene99HnswVectorsFormatV0_GetCodec(t *testing.T) {
	cfg := newLucene99HnswVectorsFormatV0Config(t)

	if cfg.MaxConn != Lucene99HnswVectorsFormat_DEFAULT_MAX_CONN {
		t.Errorf("MaxConn = %d, want %d", cfg.MaxConn, Lucene99HnswVectorsFormat_DEFAULT_MAX_CONN)
	}
	if cfg.BeamWidth != Lucene99HnswVectorsFormat_DEFAULT_BEAM_WIDTH {
		t.Errorf("BeamWidth = %d, want %d", cfg.BeamWidth, Lucene99HnswVectorsFormat_DEFAULT_BEAM_WIDTH)
	}
	if cfg.NumMergeWorkers != Lucene99HnswVectorsFormat_DEFAULT_NUM_MERGE_WORKER {
		t.Errorf("NumMergeWorkers = %d, want %d", cfg.NumMergeWorkers, Lucene99HnswVectorsFormat_DEFAULT_NUM_MERGE_WORKER)
	}
	if cfg.TinySegmentsThreshold != 0 {
		t.Errorf("TinySegmentsThreshold = %d, want 0 (V0 sentinel)", cfg.TinySegmentsThreshold)
	}
}

// TestLucene99HnswVectorsFormatV0_ToString verifies the format string carries
// the V0 tinySegmentsThreshold=0 value through the same template as the base
// format (no V0-specific tag exists in Lucene).
func TestLucene99HnswVectorsFormatV0_ToString(t *testing.T) {
	cfg := newLucene99HnswVectorsFormatV0Config(t)

	str := cfg.String()

	for _, want := range []string{
		"Lucene99HnswVectorsFormat",
		"maxConn=16",
		"beamWidth=100",
		"tinySegmentsThreshold=0",
	} {
		if !strings.Contains(str, want) {
			t.Errorf("V0 String() missing %q\n got: %s", want, str)
		}
	}
}

// TestLucene99HnswVectorsFormatV0_SupportsFloatVectorFallback ports the
// `supportsFloatVectorFallback()` override: the V0 format must report false
// just like the parent test class.
func TestLucene99HnswVectorsFormatV0_SupportsFloatVectorFallback(t *testing.T) {
	const supportsFloatVectorFallback = false
	if supportsFloatVectorFallback {
		t.Error("Lucene99HnswVectorsFormat V0 must not support float vector fallback")
	}
}
