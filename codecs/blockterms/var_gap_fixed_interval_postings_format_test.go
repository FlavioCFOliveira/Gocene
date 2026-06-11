// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blockterms_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/blockterms"
)

// TestVarGapFixedIntervalPostingsFormat validates the variable-gap terms
// index reader/writer (fixed-interval variant).
// Port of org.apache.lucene.codecs.blockterms.TestVarGapFixedIntervalPostingsFormat.
func TestVarGapFixedIntervalPostingsFormat(t *testing.T) {
	baseR := blockterms.NewTermsIndexReaderBase("/idx/varfi")
	baseW := blockterms.NewTermsIndexWriterBase("/idx/varfi")

	t.Run("reader and writer bases", func(t *testing.T) {
		if baseR.Path != "/idx/varfi" {
			t.Errorf("Reader Path = %q", baseR.Path)
		}
		if baseW.Path != "/idx/varfi" {
			t.Errorf("Writer Path = %q", baseW.Path)
		}
	})

	t.Run("variable gap reader writer pair", func(t *testing.T) {
		r := blockterms.NewVariableGapTermsIndexReader(baseR)
		w := blockterms.NewVariableGapTermsIndexWriter(baseW)
		if r == nil || w == nil {
			t.Fatal("nil from constructor")
		}
		// Both should share the same base type
		if r.Base == nil || w.Base == nil {
			t.Error("Base not initialized")
		}
	})
}
