// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blockterms_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/blockterms"
)

// TestVarGapDocFreqIntervalPostingsFormat validates the variable-gap
// terms index reader/writer (doc-freq-interval variant).
// Port of org.apache.lucene.codecs.blockterms.TestVarGapDocFreqIntervalPostingsFormat.
func TestVarGapDocFreqIntervalPostingsFormat(t *testing.T) {
	baseR := blockterms.NewTermsIndexReaderBase("/idx/vardfi")
	baseW := blockterms.NewTermsIndexWriterBase("/idx/vardfi")

	t.Run("variable gap reader", func(t *testing.T) {
		r := blockterms.NewVariableGapTermsIndexReader(baseR)
		if r == nil {
			t.Fatal("NewVariableGapTermsIndexReader returned nil")
		}
		if r.Base != baseR {
			t.Error("Base not set on reader")
		}
	})

	t.Run("variable gap writer", func(t *testing.T) {
		w := blockterms.NewVariableGapTermsIndexWriter(baseW)
		if w == nil {
			t.Fatal("NewVariableGapTermsIndexWriter returned nil")
		}
		if w.Base != baseW {
			t.Error("Base not set on writer")
		}
	})
}
