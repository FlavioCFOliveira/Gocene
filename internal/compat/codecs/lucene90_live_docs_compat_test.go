// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene90_live_docs_compat_test.go covers Lucene90LiveDocsFormat: the
// .liv per-generation deletes bitmap.
//
// Audit row cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene90LiveDocsFormat (.liv)" — gap_notes:
//	  "No Lucene-emitted .liv consumed by Gocene tests."
package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// TestLucene90LiveDocs_HeaderEnvelope validates the codec header on the
// .liv bitmap. The live-docs-format scenario deletes one document so the
// file is guaranteed to be produced. Empty bitmaps trigger different
// code paths and are out of scope here.
func TestLucene90LiveDocs_HeaderEnvelope(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "live-docs-format", seed)
			// The .liv generation number appears in the filename as
			// "_0_1.liv". For .liv the IndexHeader segment-suffix is
			// the generation number stringified — e.g. "1" for the
			// first deletes generation. See Lucene's
			// Lucene90LiveDocsFormat#fileGen() and the writer-side
			// CodecUtil.writeIndexHeader call.
			liv := findUniqueByExt(t, dir, ".liv")
			const suffix = "1"
			expectIndexCodecName(t, dir, liv, codecs.Lucene90LiveDocsCodec,
				0, 32, suffix)
			// CRC32 trailer.
			if err := validateOneEnvelope(t, dir, liv); err != nil {
				t.Fatalf("%s: CRC validation failed: %v", liv, err)
			}
		})
	}
}
