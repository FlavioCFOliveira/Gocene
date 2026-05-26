// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// header_helpers_test.go centralises the small parsing helpers that every
// per-format compat test under this directory needs. Keeping them in one
// place avoids 14 nearly-identical local helpers and one place to keep the
// CodecUtil invariants in sync if upstream Lucene ever bumps the format.
package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// expectCodecName opens dir/name as an IndexInput, reads the CodecUtil
// header (MAGIC + codec-string + version) and asserts:
//
//   - the magic is CODEC_MAGIC
//   - the embedded codec string equals wantCodec
//   - the embedded version is in [minVer, maxVer]
//
// It does NOT consume the optional IndexHeader trailer (16-byte index id
// + length-prefixed suffix); callers that need that should use
// expectIndexCodecName instead.
//
// On success the function returns the version that was read.
func expectCodecName(t *testing.T, dir, name, wantCodec string, minVer, maxVer int32) int32 {
	t.Helper()
	d, err := store.NewSimpleFSDirectory(dir)
	if err != nil {
		t.Fatalf("open dir %s: %v", dir, err)
	}
	defer d.Close()
	in, err := d.OpenInput(name, store.IOContextDefault)
	if err != nil {
		t.Fatalf("open %s/%s: %v", dir, name, err)
	}
	defer in.Close()
	version, err := codecs.CheckHeader(in, wantCodec, minVer, maxVer)
	if err != nil {
		t.Fatalf("%s: CheckHeader(%q): %v", name, wantCodec, err)
	}
	return version
}

// expectIndexCodecName is the IndexHeader variant: it asserts the codec
// name AND the 16-byte segment id + length-prefixed suffix. The expected
// suffix is the per-field "_0" segment suffix string emitted by Lucene
// for files like _0_Lucene104_0.tim ("0" in that example).
//
// Pass expectedId = nil to skip the id check (the harness re-runs each
// scenario with a fresh seed so the id is not stable across runs).
func expectIndexCodecName(t *testing.T, dir, name, wantCodec string, minVer, maxVer int32, expectedSuffix string) int32 {
	t.Helper()
	d, err := store.NewSimpleFSDirectory(dir)
	if err != nil {
		t.Fatalf("open dir %s: %v", dir, err)
	}
	defer d.Close()
	in, err := d.OpenInput(name, store.IOContextDefault)
	if err != nil {
		t.Fatalf("open %s/%s: %v", dir, name, err)
	}
	defer in.Close()
	version, err := codecs.CheckIndexHeader(in, wantCodec, minVer, maxVer, nil, expectedSuffix)
	if err != nil {
		t.Fatalf("%s: CheckIndexHeader(%q, suffix=%q): %v", name, wantCodec, expectedSuffix, err)
	}
	return version
}

// findUniqueByExt returns the single filename in dir whose extension is
// ext (e.g. ".dvd"). Fails the test if there are zero or more than one
// matches. Useful for scenarios that emit exactly one file per extension.
func findUniqueByExt(t *testing.T, dir, ext string) string {
	t.Helper()
	var matches []string
	for _, n := range listSegmentFiles(t, dir, true) {
		// Use a plain suffix match: filepath.Ext on "_0_Lucene104_0.tim"
		// returns ".tim", which is what we want.
		if len(n) > len(ext) && n[len(n)-len(ext):] == ext {
			matches = append(matches, n)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0]
	case 0:
		t.Fatalf("no file with extension %q in %s; have: %v", ext, dir,
			listSegmentFiles(t, dir, true))
	default:
		t.Fatalf("multiple files with extension %q in %s: %v", ext, dir, matches)
	}
	return ""
}

// findAllByExt returns every filename in dir whose extension matches.
func findAllByExt(t *testing.T, dir, ext string) []string {
	t.Helper()
	var matches []string
	for _, n := range listSegmentFiles(t, dir, true) {
		if len(n) > len(ext) && n[len(n)-len(ext):] == ext {
			matches = append(matches, n)
		}
	}
	if len(matches) == 0 {
		t.Fatalf("no file with extension %q in %s; have: %v", ext, dir,
			listSegmentFiles(t, dir, true))
	}
	return matches
}

// mustNonEmpty asserts the file is large enough to plausibly carry a
// payload between the IndexHeader and the 16-byte CodecUtil footer.
// minHeader is the caller's estimate of the header size (not strict — a
// safe lower bound is fine; this test guards against the regression
// where a write path emits an envelope-only file).
func mustNonEmpty(t *testing.T, dir, name string, minHeader int64) {
	t.Helper()
	d, err := store.NewSimpleFSDirectory(dir)
	if err != nil {
		t.Fatalf("open dir %s: %v", dir, err)
	}
	defer d.Close()
	in, err := d.OpenInput(name, store.IOContextDefault)
	if err != nil {
		t.Fatalf("open %s/%s: %v", dir, name, err)
	}
	defer in.Close()
	got := in.Length()
	want := minHeader + int64(codecs.FooterLength())
	if got <= want {
		t.Errorf("%s: file is %d bytes (header+footer ≈ %d) — empty payload",
			name, got, want)
	}
}
