// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blockterms_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/blockterms"
)

// TestFixedGapPostingsFormat validates the fixed-gap terms index
// reader/writer and the BlockTerms reader/writer infrastructure.
// Port of org.apache.lucene.codecs.blockterms.TestFixedGapPostingsFormat.
func TestFixedGapPostingsFormat(t *testing.T) {
	baseR := blockterms.NewTermsIndexReaderBase("/idx/fixedgap")
	baseW := blockterms.NewTermsIndexWriterBase("/idx/fixedgap")

	t.Run("reader base", func(t *testing.T) {
		if baseR.Path != "/idx/fixedgap" {
			t.Errorf("TermsIndexReaderBase.Path = %q, want /idx/fixedgap", baseR.Path)
		}
	})

	t.Run("writer base", func(t *testing.T) {
		if baseW.Path != "/idx/fixedgap" {
			t.Errorf("TermsIndexWriterBase.Path = %q, want /idx/fixedgap", baseW.Path)
		}
	})

	t.Run("fixed gap reader default", func(t *testing.T) {
		r := blockterms.NewFixedGapTermsIndexReader(baseR, 0)
		if r.Gap != 32 {
			t.Errorf("default gap = %d, want 32", r.Gap)
		}
		if r.Base != baseR {
			t.Error("Base not set on reader")
		}
	})

	t.Run("fixed gap reader explicit", func(t *testing.T) {
		r := blockterms.NewFixedGapTermsIndexReader(baseR, 64)
		if r.Gap != 64 {
			t.Errorf("gap = %d, want 64", r.Gap)
		}
	})

	t.Run("fixed gap writer", func(t *testing.T) {
		w := blockterms.NewFixedGapTermsIndexWriter(baseW, 128)
		if w.Gap != 128 {
			t.Errorf("gap = %d, want 128", w.Gap)
		}
		if w.Base != baseW {
			t.Error("Base not set on writer")
		}
	})

	t.Run("block terms reader", func(t *testing.T) {
		r := blockterms.NewBlockTermsReader(baseR)
		if r.Base != baseR {
			t.Error("Base not set on BlockTermsReader")
		}
	})

	t.Run("block terms writer", func(t *testing.T) {
		w := blockterms.NewBlockTermsWriter(baseW)
		if w.Base != baseW {
			t.Error("Base not set on BlockTermsWriter")
		}
	})
}
