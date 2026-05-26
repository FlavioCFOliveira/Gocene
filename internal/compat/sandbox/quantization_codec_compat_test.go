// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// quantization_codec_compat_test.go addresses the sandbox audit row for
// the quantization sampling codec (verbatim from the task contract):
//
//	"Quantization sampling codec: Pure port without tests, fixtures, or
//	 writer parity"
//
// The row is structurally DEFERRED at the Java harness layer because
// Apache Lucene 10.4.0 sandbox `codecs/quantization`
// (lucene/sandbox/src/java/org/apache/lucene/sandbox/codecs/quantization/
// at tag releases/lucene/10.4.0) ships ONLY two classes:
//
//   - KMeans.java       — clustering algorithm, in-memory only
//   - SampleReader.java — wrapper over FloatVectorValues for sampling
//
// There is NO KnnVectorsFormat / PostingsFormat / Codec under that
// subpackage. The scalar-quantized HNSW persisted artefact is the
// production org.apache.lucene.codecs.lucene104
// .Lucene104HnswScalarQuantizedVectorsFormat, which lives in lucene-core
// (not sandbox) and is already covered by the Sprint 114 T7 scenario
// "scalar-quantized-knn" (manifest row of the same name).
//
// The row is therefore preserved as a DEFERRED_ROW in
// manifests/baseline.tsv ("sandbox-quantization-codec") to keep the audit
// footprint visible. The Java verifier
// `verify-sandbox quantization <dir> <seed>` emits a deferred-status line
// so callers can branch on it without parsing the manifest.
//
// All three classes below are Skip subtests with the audit gap_notes
// reproduced verbatim — same layout as deferred_sandbox_compat_test.go
// to keep the three-class contract explicit per file.
package sandbox

import (
	"strconv"
	"strings"
	"testing"
)

// auditGapQuantization is the Sprint 114 T23 task-contract reframing of
// the docs/compat-coverage.tsv audit row "No tests; no fixture; no writer
// parity." — used verbatim by every Skip subtest below.
const auditGapQuantization = "Quantization sampling codec: Pure port without tests, fixtures, or writer parity"

// quantizationDeferralReason is repeated in every Skip body so the reason
// the row is deferred (rather than missing) is unambiguous in test
// output.
const quantizationDeferralReason = "Lucene 10.4.0 sandbox/codecs/quantization ships ONLY " +
	"org.apache.lucene.sandbox.codecs.quantization.KMeans and SampleReader " +
	"(no KnnVectorsFormat/PostingsFormat/Codec under that subpackage). The " +
	"scalar-quantized HNSW persisted artefact is the production " +
	"org.apache.lucene.codecs.lucene104.Lucene104HnswScalarQuantizedVectorsFormat " +
	"which is already covered by the T7 scenario \"scalar-quantized-knn\" " +
	"(manifest row of the same name). Sandbox-specific binary parity is " +
	"therefore not applicable. Tracked as DEFERRED_ROW " +
	"\"sandbox-quantization-codec\" in manifests/baseline.tsv."

// TestSandboxQuantizationCodec_ReadFixture (class a) — there is no Java
// scenario to drive: no on-disk artefact exists for the sandbox
// quantization codec in Lucene 10.4.0. Skip with the verbatim audit
// gap_notes so the row is visible in `go test -v` output.
func TestSandboxQuantizationCodec_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Skipf("deferred: no Java fixture for sandbox quantization at seed=%d; "+
				"%s Audit gap_notes (verbatim): %q",
				seed, quantizationDeferralReason, auditGapQuantization)
		})
	}
}

// TestSandboxQuantizationCodec_VerifySubcommand (class b) drives the
// deferred-status branch of `verify-sandbox quantization <dir> <seed>`.
// The harness emits a single `ok ... status=deferred manifest_row=...`
// line and exits 0; we assert both substrings so the deferral is
// surfaced explicitly rather than silently passing.
func TestSandboxQuantizationCodec_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			requireHarness(t)
			// The directory argument is unused by the deferred path; pass
			// t.TempDir() to keep the CLI signature happy.
			dir := t.TempDir()
			out, err := runHarness(t, "verify-sandbox", "quantization", dir,
				strconv.FormatInt(seed, 10))
			if err != nil {
				t.Fatalf("verify-sandbox quantization failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-sandbox variant=quantization") {
				t.Errorf("expected 'ok verify-sandbox variant=quantization' in stdout, got: %s", out)
			}
			if !strings.Contains(out, "status=deferred") {
				t.Errorf("expected 'status=deferred' marker in stdout, got: %s", out)
			}
			if !strings.Contains(out, "manifest_row=sandbox-quantization-codec") {
				t.Errorf("expected 'manifest_row=sandbox-quantization-codec' marker in stdout, got: %s", out)
			}
		})
	}
}

// TestSandboxQuantizationCodec_RoundTrip (class c) — same root cause as
// class a: there is no sandbox-specific quantization artefact in Lucene
// 10.4.0 to round-trip. The Gocene port (sandbox/codecs/quantization/
// quantization.go) mirrors the in-memory KMeans + SampleReader surface
// only.
func TestSandboxQuantizationCodec_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Skipf("deferred: no Gocene round-trip target for sandbox quantization "+
				"at seed=%d; %s Audit gap_notes (verbatim): %q",
				seed, quantizationDeferralReason, auditGapQuantization)
		})
	}
}
