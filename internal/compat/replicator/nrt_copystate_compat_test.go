// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// nrt_copystate_compat_test.go addresses the replicator audit row
// (verbatim from docs/compat-coverage.tsv): "No interop frame captured
// from a Java Lucene replicator peer.". Scenario
// "replicator-nrt-copystate" emits nrt-copystate.bin — a CodecUtil-framed
// CopyState payload using the canonical NRT wire layout copied verbatim
// from Lucene 10.4.0's SimplePrimaryNode#writeCopyState (and
// TestSimpleServer#writeFilesMetaData).
//
// Three classes per the rmp 4627 contract:
//
//	(a) read-fixture        — drive the harness, parse the file shape.
//	(b) write-and-verify    — byte-determinism + verify-replicator
//	                          copystate CLI.
//	(c) full L -> G -> L round-trip — t.Skip with the verbatim audit
//	                          gap_notes because Gocene's replicator/nrt
//	                          port has the in-memory CopyState /
//	                          FileMetaData types (replicator/nrt/nrt.go
//	                          lines 67-113) but NO binary wire encoder /
//	                          decoder (audit category is "partial"). The
//	                          Lucene-side Java verifier IS exercised by
//	                          TestReplicatorNrtCopyState_VerifySubcommand.
package replicator

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// luceneCodecUtilIndexHeaderMagic is the BE int32 0x3FD76C17 that prefixes
// every CodecUtil-framed Lucene file. Pinning it here gives the read-fixture
// test a cheap structural assertion that does not require parsing the
// CopyState payload.
var luceneCodecUtilIndexHeaderMagic = []byte{0x3F, 0xD7, 0x6C, 0x17}

// TestReplicatorNrtCopyState_ReadFixture (class a) drives the harness and
// pins the structural shape of nrt-copystate.bin: it exists, starts with
// the CodecUtil header magic, and is non-trivial in length. The minimum
// floor (CodecUtil IndexHeader = 16 + Footer = 16 = 32 bytes plus the
// fixed scenario payload of 3 FileMetaData + completedMergeFiles +
// primaryGen) comfortably exceeds 200 bytes, so 64 is a very safe lower
// bound that catches any "scenario silently produced nothing" failure.
func TestReplicatorNrtCopyState_ReadFixture(t *testing.T) {
	const lowerBound = 64
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioReplicatorNrtCopyState, seed)
			files := listFiles(t, dir)
			if len(files) != 1 || files[0] != fileNrtCopyStateBin {
				t.Fatalf("expected exactly %q under fixture dir, got %v",
					fileNrtCopyStateBin, files)
			}
			blob := readFileBytes(t, dir, fileNrtCopyStateBin)
			if len(blob) <= lowerBound {
				t.Fatalf("%s suspiciously small (%d bytes); the CopyState "+
					"payload + CodecUtil framing should comfortably exceed %d",
					fileNrtCopyStateBin, len(blob), lowerBound)
			}
			if !bytes.HasPrefix(blob, luceneCodecUtilIndexHeaderMagic) {
				t.Errorf("%s does not start with CodecUtil IndexHeader magic %x; got prefix %x",
					fileNrtCopyStateBin, luceneCodecUtilIndexHeaderMagic, blob[:4])
			}
		})
	}
}

// TestReplicatorNrtCopyState_ByteDeterminism (class b, part 1) runs the
// scenario twice at the same seed and confirms nrt-copystate.bin is
// byte-identical across runs. Catches any iteration-order drift in the
// LinkedHashMap-backed files / completedMergeFiles members of CopyState
// (mitigated upstream by using LinkedHashMap/LinkedHashSet), and any
// header-id non-determinism upstream of Determinism.seed.
func TestReplicatorNrtCopyState_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioReplicatorNrtCopyState, seed)
			b := generate(t, ScenarioReplicatorNrtCopyState, seed)
			ab := readFileBytes(t, a, fileNrtCopyStateBin)
			bb := readFileBytes(t, b, fileNrtCopyStateBin)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("%s drift between two runs at seed=%d (lenA=%d lenB=%d)",
					fileNrtCopyStateBin, seed, len(ab), len(bb))
			}
		})
	}
}

// TestReplicatorNrtCopyState_VerifySubcommand (class b, part 2) drives the
// new `verify-replicator copystate <dir> <seed>` subcommand. A clean exit
// (code 0) proves the Java verifier reads the file, decodes the CopyState
// payload with TestSimpleServer.readCopyState, and asserts every
// FileMetaData round-trips byte-for-byte against the expectation.
func TestReplicatorNrtCopyState_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioReplicatorNrtCopyState, seed)
			out, err := runHarness(t, "verify-replicator", "copystate", dir,
				strconv.FormatInt(seed, 10))
			if err != nil {
				t.Fatalf("verify-replicator copystate failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-replicator variant=copystate") {
				t.Errorf("expected 'ok verify-replicator variant=copystate' in stdout, got: %s", out)
			}
		})
	}
}

// TestReplicatorNrtCopyState_RoundTrip (class c) — full Lucene -> Gocene ->
// Lucene -> Gocene replay is blocked on Gocene's replicator/nrt port. The
// in-memory CopyState and FileMetaData types exist (replicator/nrt/nrt.go
// lines 67-113) but the package ships NO binary wire encoder / decoder
// (no SimplePrimaryNode.writeCopyState / TestSimpleServer.readCopyState
// equivalent in Go); the audit row classifies it as "partial". The audit
// gap_notes is reproduced verbatim in the Skipf message so it surfaces
// in `go test -v` output as evidence the gap was considered.
func TestReplicatorNrtCopyState_RoundTrip(t *testing.T) {
	const auditGap = "No interop frame captured from a Java Lucene replicator peer."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene round-trip for scenario %q at seed=%d is "+
				"blocked on the Gocene replicator/nrt port — the in-memory "+
				"CopyState/FileMetaData types exist (replicator/nrt/nrt.go "+
				"lines 67-113) but the package ships no SimplePrimaryNode."+
				"writeCopyState / TestSimpleServer.readCopyState wire "+
				"equivalent; audit category is 'partial'. The Lucene-side "+
				"verifier IS exercised by "+
				"TestReplicatorNrtCopyState_VerifySubcommand. "+
				"Audit gap_notes (verbatim): %q",
				ScenarioReplicatorNrtCopyState, seed, auditGap)
		})
	}
}
