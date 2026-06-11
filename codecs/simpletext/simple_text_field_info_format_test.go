// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/simpletext"
)

// TestSimpleTextFieldInfoFormat validates the SimpleTextFieldInfosFormat
// constructor and the surrounding SimpleText format types.
// Port of org.apache.lucene.codecs.simpletext.TestSimpleTextFieldInfoFormat.
func TestSimpleTextFieldInfoFormat(t *testing.T) {
	t.Run("FieldInfosFormat constructor", func(t *testing.T) {
		format := simpletext.NewSimpleTextFieldInfosFormat()
		if format == nil {
			t.Fatal("NewSimpleTextFieldInfosFormat returned nil")
		}
		format2 := simpletext.NewSimpleTextFieldInfosFormat()
		if format == format2 {
			t.Error("expected distinct instances")
		}
	})

	t.Run("SimpleTextCodec", func(t *testing.T) {
		codec := simpletext.NewSimpleTextCodec()
		if codec == nil {
			t.Fatal("NewSimpleTextCodec returned nil")
		}
	})

	t.Run("all SimpleText format types createable", func(t *testing.T) {
		if f := simpletext.NewSimpleTextCompoundFormat(); f == nil {
			t.Error("NewSimpleTextCompoundFormat returned nil")
		}
		if f := simpletext.NewSimpleTextLiveDocsFormat(); f == nil {
			t.Error("NewSimpleTextLiveDocsFormat returned nil")
		}
		if f := simpletext.NewSimpleTextNormsFormat(); f == nil {
			t.Error("NewSimpleTextNormsFormat returned nil")
		}
		if f := simpletext.NewSimpleTextPointsFormat(); f == nil {
			t.Error("NewSimpleTextPointsFormat returned nil")
		}
		if f := simpletext.NewSimpleTextSegmentInfoFormat(); f == nil {
			t.Error("NewSimpleTextSegmentInfoFormat returned nil")
		}
		if f := simpletext.NewSimpleTextStoredFieldsFormat(); f == nil {
			t.Error("NewSimpleTextStoredFieldsFormat returned nil")
		}
	})
}
