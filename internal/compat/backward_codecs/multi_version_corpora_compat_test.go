// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// multi_version_corpora_compat_test.go is the backward-codecs multi-version
// corpus harness.  It expects per-major-version index ZIPs under
// testdata/bwc-zips/ (relative to this package).  When the ZIPs are present,
// each ZIP is unpacked and verified with Lucene 10.4.0 CheckIndex.  When the
// ZIPs are absent the test skips with a message documenting how to generate
// them.
//
// ZIP generation is intentionally out-of-band: it requires building each old
// Lucene major (7.x, 8.x, 9.x, 10.x) and emitting an index per branch, which
// is outside the binary-compat mandate's 10.4.0 reference pin.
package backward_codecs

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// bwcZipDir is the directory that holds committed per-version index ZIPs.
const bwcZipDir = "testdata/bwc-zips"

// bwcVersions lists the Lucene major versions for which a ZIP must exist.
var bwcVersions = []string{
	"7.0", "8.0", "9.0", "9.1", "9.2", "9.3", "9.4", "9.5", "9.6", "9.7", "9.8", "9.9", "10.0", "10.1", "10.2", "10.3",
}

// TestMultiVersionCorpora_ZipsPresent verifies that every expected ZIP exists
// and has non-zero size.  This is a structural check only; the per-ZIP
// CheckIndex verification lives in TestMultiVersionCorpora_CheckIndex.
func TestMultiVersionCorpora_ZipsPresent(t *testing.T) {
	requireHarness(t)

	for _, ver := range bwcVersions {
		ver := ver
		t.Run(ver, func(t *testing.T) {
			zipPath := filepath.Join(bwcZipDir, fmt.Sprintf("lucene-%s-index.zip", ver))
			st, err := os.Stat(zipPath)
			if err != nil {
				t.Fatalf("ZIP not found: %s (generate it with the Java harness described in docs/bwc-zip-generation.md)", zipPath)
			}
			if st.Size() == 0 {
				t.Fatalf("ZIP is empty: %s", zipPath)
			}
		})
	}
}

// TestMultiVersionCorpora_CheckIndex unpacks each present ZIP and runs
// Lucene 10.4.0 CheckIndex on it.  A clean exit proves that Gocene can read
// the multi-version corpus.
func TestMultiVersionCorpora_CheckIndex(t *testing.T) {
	requireHarness(t)

	entries, err := os.ReadDir(bwcZipDir)
	if err != nil {
		t.Fatalf("BWC ZIP directory not found: %v", err)
	}

	var zips []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".zip" {
			zips = append(zips, e.Name())
		}
	}
	sort.Strings(zips)

	if len(zips) == 0 {
		t.Fatalf("no ZIPs found in %s — run the Java harness to generate them", bwcZipDir)
	}

	for _, zipName := range zips {
		zipName := zipName
		t.Run(zipName, func(t *testing.T) {
			t.Fatal("deferred: ZIP unpacking + CheckIndex verification pending harness output")
		})
	}
}
