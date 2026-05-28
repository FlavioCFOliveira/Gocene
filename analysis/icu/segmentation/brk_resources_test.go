// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestEmbeddedBRKByteIdentity verifies the embedded production .brk blobs are
// byte-identical to the testdata copies (which are themselves byte-identical to
// the Apache Lucene 10.4.0 reference resources). The Binary Compatibility
// Mandate requires these ICU artefacts to match Lucene exactly (rmp #4702).
func TestEmbeddedBRKByteIdentity(t *testing.T) {
	cases := []struct {
		name      string
		embedded  []byte
		testdata  string
		wantBytes int
	}{
		{EmbeddedDefaultBRKName, defaultBRK, "Default.brk", 22640},
		{EmbeddedMyanmarSyllableBRKName, myanmarSyllableBRK, "MyanmarSyllable.brk", 13816},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.embedded) != tc.wantBytes {
				t.Errorf("embedded %s = %d bytes, want %d", tc.name, len(tc.embedded), tc.wantBytes)
			}
			ref, err := os.ReadFile(filepath.Join("testdata", tc.testdata))
			if err != nil {
				t.Fatalf("read testdata %s: %v", tc.testdata, err)
			}
			if !bytes.Equal(tc.embedded, ref) {
				t.Errorf("embedded %s is not byte-identical to testdata/%s", tc.name, tc.testdata)
			}
		})
	}
}

// TestLoadEmbeddedBRK verifies the embedded blobs parse via the loader hook and
// that an unknown name is rejected.
func TestLoadEmbeddedBRK(t *testing.T) {
	for _, name := range []string{EmbeddedDefaultBRKName, EmbeddedMyanmarSyllableBRKName} {
		dict, err := LoadEmbeddedBRK(name)
		if err != nil {
			t.Fatalf("LoadEmbeddedBRK(%q): %v", name, err)
		}
		if dict == nil {
			t.Fatalf("LoadEmbeddedBRK(%q) returned nil dictionary", name)
		}
	}
	if _, err := LoadEmbeddedBRK("Nope.brk"); err == nil {
		t.Error("LoadEmbeddedBRK(unknown) = nil error, want rejection")
	}
}
