// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// rate_limited_output_compat_test.go addresses the audit row
// "RateLimitedIndexOutput envelope" from docs/compat-coverage.tsv. Rate
// limiting only affects WRITE TIMING; on-disk bytes MUST be byte-identical
// to a non-rate-limited write. This test:
//
//	(a) writes the store-primitives payload twice — once directly, once
//	    through RateLimitedIndexOutput — and asserts byte equality;
//	(b) asks the Java harness to verify the rate-limited output, proving
//	    cross-engine compatibility.
package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/internal/compat"
	gostore "github.com/FlavioCFOliveira/Gocene/store"
)

// TestRateLimitedOutput_ByteIdenticalToPlain asserts (a) byte equality and
// (b) cross-engine acceptance of the rate-limited output.
func TestRateLimitedOutput_ByteIdenticalToPlain(t *testing.T) {
	requireHarness(t)

	const seed int64 = 12648430 // 0xC0FFEE

	plain := t.TempDir()
	limited := t.TempDir()

	// (1) plain Gocene write.
	if err := WriteStorePrimitives(plain, seed); err != nil {
		t.Fatalf("plain write: %v", err)
	}

	// (2) rate-limited Gocene write. 1 MB/s is fast enough that the test
	//     completes in well under a second yet still exercises the
	//     RateLimiter.Pause path (the payload is ~500 bytes so checkRate
	//     fires only at close-time, which is the realistic case anyway —
	//     what matters here is that the byte stream survives the wrap).
	if err := writeStorePrimitivesRateLimited(limited, seed, 1.0); err != nil {
		t.Fatalf("rate-limited write: %v", err)
	}

	plainBytes, err := os.ReadFile(filepath.Join(plain, FileName))
	if err != nil {
		t.Fatalf("read plain: %v", err)
	}
	limitedBytes, err := os.ReadFile(filepath.Join(limited, FileName))
	if err != nil {
		t.Fatalf("read limited: %v", err)
	}
	if !bytes.Equal(plainBytes, limitedBytes) {
		t.Fatalf("rate-limited output diverges from plain output (lengths %d vs %d)",
			len(plainBytes), len(limitedBytes))
	}

	// (3) cross-engine verification: Lucene must accept the rate-limited
	//     output.
	if err := compat.Verify("store-primitives", seed, limited); err != nil {
		t.Fatalf("Lucene verify of rate-limited Gocene output: %v", err)
	}
}

// writeStorePrimitivesRateLimited mirrors WriteStorePrimitives but inserts
// a RateLimitedIndexOutput between the on-disk IndexOutput and the
// ChecksumIndexOutput that owns the CodecUtil framing. The wrap MUST NOT
// affect the on-disk byte stream.
func writeStorePrimitivesRateLimited(targetDir string, seed int64, mbPerSec float64) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	dir, err := gostore.NewNIOFSDirectory(targetDir)
	if err != nil {
		return fmt.Errorf("open dir: %w", err)
	}
	defer dir.Close()

	raw, err := dir.CreateOutput(FileName, gostore.IOContextDefault)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}

	limiter := gostore.NewSimpleRateLimiter(mbPerSec)
	rateLimited := gostore.NewRateLimitedIndexOutput(limiter, raw)
	out := gostore.NewChecksumIndexOutput(rateLimited)

	if err := codecs.WriteIndexHeader(out, Codec, Version, IDFromSeed(seed), ""); err != nil {
		out.Close()
		return fmt.Errorf("header: %w", err)
	}
	if err := gostore.WriteVInt(out, Count); err != nil {
		out.Close()
		return fmt.Errorf("count: %w", err)
	}
	for i := 0; i < Count; i++ {
		if err := writeFrame(out, seed, i); err != nil {
			out.Close()
			return fmt.Errorf("frame[%d]: %w", i, err)
		}
	}
	if err := codecs.WriteFooter(out); err != nil {
		out.Close()
		return fmt.Errorf("footer: %w", err)
	}
	return out.Close()
}
