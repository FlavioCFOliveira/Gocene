// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestMutationDiagnostic exercises rmp 4611 acceptance criterion #4: on
// forced divergence, the harness MUST emit a structured record that
// names the affected file, byte offset, and expected vs actual bytes.
//
// Procedure:
//  1. Generate the S1 fixture (multi-segment Lucene index + s1-hits.tsv).
//  2. Mutate exactly one byte at offset 100 of the largest non-`.si`,
//     non-`write.lock` file.
//  3. Run `verify-diagnostic combined-multi-segment-index-search <seed> <dir>`.
//  4. Assert stdout is a single JSON object with file, offset, expected,
//     actual; offset == 100; expected != actual; file matches what we
//     mutated.
//
// The harness exit code is 4 on diagnostic failure; we tolerate it.
func TestMutationDiagnostic(t *testing.T) {
	const mutationOffset = 100
	const seed = int64(0xC0FFEE)
	dir := generate(t, scenarioS1, seed)
	// Find the largest non-".si", non-"write.lock" file. Skip the manifest
	// excluded files because the diagnostic walker excludes them too, so
	// mutating them would not surface a diagnostic record.
	target, err := pickLargestEligibleFile(dir)
	if err != nil {
		t.Fatalf("pick file: %v", err)
	}
	rel, err := filepath.Rel(dir, target)
	if err != nil {
		t.Fatalf("rel %s under %s: %v", target, dir, err)
	}
	// Mutate byte at offset 100. The hits TSV and most segment files in
	// the fixture are comfortably > 100 bytes.
	if err := mutateByte(target, mutationOffset, 0xAA); err != nil {
		t.Fatalf("mutate %s: %v", target, err)
	}
	stdout, stderr, runErr := runHarness(t, "verify-diagnostic", scenarioS1,
		strconv.FormatInt(seed, 10), dir)
	if runErr == nil {
		t.Fatalf("expected non-zero exit from verify-diagnostic after mutation, got clean exit\nstdout: %s",
			stdout)
	}
	line := strings.TrimSpace(stdout)
	if !strings.HasPrefix(line, "{") {
		t.Fatalf("expected JSON diagnostic on stdout, got %q (stderr: %s)",
			stdout, stderr)
	}
	var diag struct {
		File     string `json:"file"`
		Offset   int64  `json:"offset"`
		Expected int    `json:"expected"`
		Actual   int    `json:"actual"`
	}
	if err := json.Unmarshal([]byte(line), &diag); err != nil {
		t.Fatalf("unmarshal %q: %v", line, err)
	}
	if diag.File != rel {
		t.Errorf("diagnostic file = %q, want %q", diag.File, rel)
	}
	if diag.Offset != mutationOffset {
		t.Errorf("diagnostic offset = %d, want %d", diag.Offset, mutationOffset)
	}
	if diag.Expected == diag.Actual {
		t.Errorf("expected != actual; both = %d", diag.Expected)
	}
	if diag.Expected < 0 || diag.Expected > 255 ||
		diag.Actual < 0 || diag.Actual > 255 {
		t.Errorf("byte values out of range: expected=%d actual=%d",
			diag.Expected, diag.Actual)
	}
}

// pickLargestEligibleFile returns the path of the largest regular file
// under dir, excluding `.si` (non-deterministic timestamp) and
// `write.lock` (excluded from the diagnostic walker for parity with
// Manifest.includeForHash).
func pickLargestEligibleFile(dir string) (string, error) {
	type fileSize struct {
		path string
		size int64
	}
	var candidates []fileSize
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".si") || name == "write.lock" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		candidates = append(candidates, fileSize{path: path, size: info.Size()})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].size > candidates[j].size
	})
	if len(candidates) == 0 {
		return "", os.ErrNotExist
	}
	return candidates[0].path, nil
}

// mutateByte XORs one byte in path at offset with mask. Returns an error
// if the file is shorter than offset+1.
func mutateByte(path string, offset int64, mask byte) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if int64(len(b)) <= offset {
		return os.ErrInvalid
	}
	b[offset] ^= mask
	return os.WriteFile(path, b, 0o644)
}
