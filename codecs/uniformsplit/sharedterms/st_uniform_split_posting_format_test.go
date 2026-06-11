// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	st "github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit/sharedterms"
)

// TestSTUniformSplitPostingFormat validates the STUniformSplitPostingsFormat
// constructor and its shared-terms block reader/writer infrastructure.
// Port of org.apache.lucene.codecs.uniformsplit.sharedterms.TestSTUniformSplitPostingFormat.
func TestSTUniformSplitPostingFormat(t *testing.T) {
	t.Run("constructor", func(t *testing.T) {
		format := st.NewSTUniformSplitPostingsFormat(64)
		if format == nil {
			t.Fatal("NewSTUniformSplitPostingsFormat returned nil")
		}
	})

	t.Run("field metadata term state", func(t *testing.T) {
		state := st.NewFieldMetadataTermState("content", []byte("state"))
		if state == nil {
			t.Fatal("NewFieldMetadataTermState returned nil")
		}
		if state.Field != "content" {
			t.Errorf("Field = %q, want %q", state.Field, "content")
		}
	})

	t.Run("ST block line", func(t *testing.T) {
		perField := []*st.FieldMetadataTermState{
			st.NewFieldMetadataTermState("f1", []byte("s1")),
			st.NewFieldMetadataTermState("f2", []byte("s2")),
		}
		line := st.NewSTBlockLine([]byte("term"), perField)
		if line == nil {
			t.Fatal("NewSTBlockLine returned nil")
		}
	})

	t.Run("ST block reader from uniformsplit reader", func(t *testing.T) {
		inner := uniformsplit.NewBlockReader([]byte("test data"))
		reader := st.NewSTBlockReader(inner)
		if reader == nil {
			t.Fatal("NewSTBlockReader returned nil")
		}
	})

	t.Run("ST block writer", func(t *testing.T) {
		inner := &uniformsplit.BlockWriter{}
		writer := st.NewSTBlockWriter(inner)
		if writer == nil {
			t.Fatal("NewSTBlockWriter returned nil")
		}
	})

	t.Run("ST intersect block reader", func(t *testing.T) {
		inner := uniformsplit.NewBlockReader([]byte("test"))
		stReader := st.NewSTBlockReader(inner)
		intersect := st.NewSTIntersectBlockReader(stReader)
		if intersect == nil {
			t.Fatal("NewSTIntersectBlockReader returned nil")
		}
	})

	t.Run("ST merging block reader", func(t *testing.T) {
		inner1 := uniformsplit.NewBlockReader([]byte("data1"))
		inner2 := uniformsplit.NewBlockReader([]byte("data2"))
		r1 := st.NewSTBlockReader(inner1)
		r2 := st.NewSTBlockReader(inner2)
		merger := st.NewSTMergingBlockReader([]*st.STBlockReader{r1, r2})
		if merger == nil {
			t.Fatal("NewSTMergingBlockReader returned nil")
		}
	})

	t.Run("ST merging terms enum", func(t *testing.T) {
		enum := st.NewSTMergingTermsEnum("field", 3)
		if enum == nil {
			t.Fatal("NewSTMergingTermsEnum returned nil")
		}
	})

	t.Run("multi segments postings enum", func(t *testing.T) {
		pe := st.NewMultiSegmentsPostingsEnum(5)
		if pe == nil {
			t.Fatal("NewMultiSegmentsPostingsEnum returned nil")
		}
	})
}
