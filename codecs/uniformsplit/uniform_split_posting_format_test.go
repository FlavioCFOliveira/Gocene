// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
)

// TestUniformSplitPostingFormat validates the UniformSplitPostingsFormat
// constructor and the accompanying TermsWriter/TermsReader infrastructure.
// Port of org.apache.lucene.codecs.uniformsplit.TestUniformSplitPostingFormat.
func TestUniformSplitPostingFormat(t *testing.T) {
	t.Run("constructor default", func(t *testing.T) {
		format := uniformsplit.NewUniformSplitPostingsFormat(32)
		if format == nil {
			t.Fatal("NewUniformSplitPostingsFormat returned nil")
		}
		if format.TargetBlockSize != 32 {
			t.Errorf("TargetBlockSize = %d, want 32", format.TargetBlockSize)
		}
	})

	t.Run("constructor zero uses default", func(t *testing.T) {
		format := uniformsplit.NewUniformSplitPostingsFormat(0)
		if format == nil {
			t.Fatal("NewUniformSplitPostingsFormat(0) returned nil")
		}
		if format.TargetBlockSize < 1 {
			t.Errorf("TargetBlockSize = %d, want >= 1 (default clamped)", format.TargetBlockSize)
		}
	})

	t.Run("constructor negative uses default", func(t *testing.T) {
		format := uniformsplit.NewUniformSplitPostingsFormat(-1)
		if format == nil {
			t.Fatal("NewUniformSplitPostingsFormat(-1) returned nil")
		}
		if format.TargetBlockSize < 1 {
			t.Errorf("TargetBlockSize = %d, want >= 1 (default clamped)", format.TargetBlockSize)
		}
	})

	t.Run("terms writer and reader", func(t *testing.T) {
		format := uniformsplit.NewUniformSplitPostingsFormat(64)
		writer := uniformsplit.NewUniformSplitTermsWriter(format)
		if writer == nil {
			t.Fatal("NewUniformSplitTermsWriter returned nil")
		}
		if writer.Format != format {
			t.Error("TermsWriter.Format not set correctly")
		}

		reader := uniformsplit.NewUniformSplitTermsReader(format)
		if reader == nil {
			t.Fatal("NewUniformSplitTermsReader returned nil")
		}
		if reader.Format != format {
			t.Error("TermsReader.Format not set correctly")
		}
	})

	t.Run("field metadata", func(t *testing.T) {
		md := uniformsplit.NewFieldMetadata(1000, 50)
		if md == nil {
			t.Fatal("NewFieldMetadata returned nil")
		}
		if md.NumTerms != 1000 {
			t.Errorf("NumTerms = %d, want 1000", md.NumTerms)
		}
		if md.NumDocs != 50 {
			t.Errorf("NumDocs = %d, want 50", md.NumDocs)
		}

		terms := uniformsplit.NewUniformSplitTerms("myfield", md)
		if terms.Field != "myfield" {
			t.Errorf("Field = %q, want %q", terms.Field, "myfield")
		}
	})

	t.Run("FST dictionary", func(t *testing.T) {
		dict := uniformsplit.NewFSTDictionary("content", 10)
		if dict.GetField() != "content" {
			t.Errorf("GetField = %q, want %q", dict.GetField(), "content")
		}
		if dict.NumBlocks() != 10 {
			t.Errorf("NumBlocks = %d, want 10", dict.NumBlocks())
		}
		// Verify it implements IndexDictionary
		var _ uniformsplit.IndexDictionary = dict
	})
}
