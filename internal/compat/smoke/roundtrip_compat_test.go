// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// roundtrip_compat_test.go is gated by the "compat" build tag. It runs the
// Sprint 114 T2 end-to-end smoke round-trip, which requires the Java fixture
// harness under tools/lucene-fixtures/ to be built and the resulting jar to
// be reachable via the LUCENE_FIXTURES_JAR environment variable (set by the
// Makefile and by CI).
//
//	Lucene-write   →  java -jar lucene-fixtures.jar gen smoke <seed> <dirA>
//	Gocene-read    →  smoke.Read(dirA, seed)
//	Gocene-write   →  smoke.Write(dirB, seed)
//	Lucene-verify  →  java -jar lucene-fixtures.jar verify smoke <seed> <dirB>
//	Byte equality  →  bytes(dirA/smoke.dat) == bytes(dirB/smoke.dat)
package smoke

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func locateHarnessJar(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("LUCENE_FIXTURES_JAR"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
		t.Skipf("LUCENE_FIXTURES_JAR=%q is set but the file does not exist; skipping", v)
	}
	repoRoot, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not inside a git repo and LUCENE_FIXTURES_JAR not set; skipping")
	}
	jar := filepath.Join(string(bytes.TrimSpace(repoRoot)),
		"tools", "lucene-fixtures", "target", "lucene-fixtures.jar")
	if _, err := os.Stat(jar); err != nil {
		t.Skipf("harness jar not built (%s); run 'make -f tools/lucene-fixtures/Makefile harness-build' first", jar)
	}
	return jar
}

func runHarness(t *testing.T, jar string, args ...string) {
	t.Helper()
	cmd := exec.Command("java", append([]string{"-jar", jar}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("java -jar %s %v failed: %v\noutput:\n%s", jar, args, err, out)
	}
}

func TestSmoke_LuceneGoceneByteRoundTrip(t *testing.T) {
	jar := locateHarnessJar(t)

	for _, seed := range []int64{0, 1, 42, 0xC0FFEE} {
		seed := seed
		t.Run("", func(t *testing.T) {
			dirA := t.TempDir() // Lucene-written
			dirB := t.TempDir() // Gocene-written

			// 1. Lucene writes dirA/smoke.dat.
			runHarness(t, jar, "gen", "smoke", itoa(seed), dirA)

			// 2. Gocene reads dirA/smoke.dat.
			values, err := Read(dirA, seed)
			if err != nil {
				t.Fatalf("Gocene Read of Lucene-written file: %v", err)
			}
			if len(values) != Count {
				t.Fatalf("Gocene Read returned %d values, want %d", len(values), Count)
			}

			// 3. Gocene writes dirB/smoke.dat.
			if err := Write(dirB, seed); err != nil {
				t.Fatalf("Gocene Write: %v", err)
			}

			// 4. Lucene verifies dirB/smoke.dat.
			runHarness(t, jar, "verify", "smoke", itoa(seed), dirB)

			// 5. Byte equality: the two engines produced identical bytes.
			bA, err := os.ReadFile(filepath.Join(dirA, FileName))
			if err != nil {
				t.Fatalf("read dirA/%s: %v", FileName, err)
			}
			bB, err := os.ReadFile(filepath.Join(dirB, FileName))
			if err != nil {
				t.Fatalf("read dirB/%s: %v", FileName, err)
			}
			if !bytes.Equal(bA, bB) {
				t.Fatalf("byte mismatch between Lucene-written and Gocene-written smoke.dat\n"+
					"  Lucene size = %d\n  Gocene size = %d\n  Lucene = %x\n  Gocene = %x",
					len(bA), len(bB), bA, bB)
			}
		})
	}
}

// itoa avoids strconv import just for a single int64 -> string conversion.
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
