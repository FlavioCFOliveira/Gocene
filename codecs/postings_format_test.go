// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestLucene104PostingsFormat_Placeholder(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	tester := codecs.NewPostingsTester(t)
	format := codecs.NewLucene104PostingsFormat()

	// Verify it's a placeholder by checking if FieldsConsumer returns an error
	si := index.NewSegmentInfo("_0", 1, dir)
	fieldInfos := index.NewFieldInfos()

	state := &codecs.SegmentWriteState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fieldInfos,
	}

	consumer, err := format.FieldsConsumer(state)
	if err == nil {
		consumer.Close()
		t.Error("Expected error from Lucene104PostingsFormat.FieldsConsumer placeholder, got nil")
	}

	producer, err := format.FieldsProducer(&codecs.SegmentReadState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fieldInfos,
	})
	if err == nil {
		producer.Close()
		t.Error("Expected error from Lucene104PostingsFormat.FieldsProducer placeholder, got nil")
	}

	// Run "full" test which currently just logs that it's a placeholder
	tester.TestFull(format, index.IndexOptionsDocs, dir)
}
