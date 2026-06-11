// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package blocktreeords

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
)

// BlockTreeOrdsPostingsFormatName is the SPI name under which this format
// is registered. Mirrors the Java constructor super("BlockTreeOrds").
const BlockTreeOrdsPostingsFormatName = "BlockTreeOrds"

// BlockTreeOrdsPostingsFormatBlockSize is the fixed packed block size used
// by the Lucene104 postings writer/reader when paired with this format.
// Mirrors Java's BlockTreeOrdsPostingsFormat.BLOCK_SIZE = 128.
const BlockTreeOrdsPostingsFormatBlockSize = 128

// errOrdsReadOnly is returned by FieldsProducer when the read path has not
// yet been fully ported. The write path is functional.
var errOrdsReadPathDeferred = errors.New(
	"BlockTreeOrdsPostingsFormat: read path is not yet fully wired; " +
		"the OrdsBlockTreeTermsReader requires its own implementation task")

// BlockTreeOrdsPostingsFormat wires OrdsBlockTreeTermsWriter (the
// ordinals-aware term-dictionary writer) with Lucene104PostingsWriter (the
// PFor-delta doc/pos/pay encoder) on the write side.
//
// On the read side it wires OrdsBlockTreeTermsReader with
// Lucene104PostingsReader; the reader is deferred but the type is
// registered so that PostingsFormatByName resolves the canonical name.
//
// Port of org.apache.lucene.codecs.blocktreeords.BlockTreeOrdsPostingsFormat
// from Apache Lucene 10.4.0.
type BlockTreeOrdsPostingsFormat struct {
	*codecs.BasePostingsFormat

	minTermBlockSize int
	maxTermBlockSize int
}

// NewBlockTreeOrdsPostingsFormat creates a format with default term-block
// sizes (25/48).
func NewBlockTreeOrdsPostingsFormat() *BlockTreeOrdsPostingsFormat {
	return NewBlockTreeOrdsPostingsFormatWithBlockSizes(
		ordsBlockTreeDefaultMinBlockSize,
		ordsBlockTreeDefaultMaxBlockSize,
	)
}

// NewBlockTreeOrdsPostingsFormatWithBlockSizes creates a format pinned to
// specific term-block size bounds. Mirrors the Java constructor
// BlockTreeOrdsPostingsFormat(int, int).
func NewBlockTreeOrdsPostingsFormatWithBlockSizes(minBlock, maxBlock int) *BlockTreeOrdsPostingsFormat {
	return &BlockTreeOrdsPostingsFormat{
		BasePostingsFormat: codecs.NewBasePostingsFormat(BlockTreeOrdsPostingsFormatName),
		minTermBlockSize:   minBlock,
		maxTermBlockSize:   maxBlock,
	}
}

// FieldsConsumer returns a FieldsConsumer that wires
// Lucene104PostingsWriter through OrdsBlockTreeTermsWriter.
//
// Mirrors BlockTreeOrdsPostingsFormat.fieldsConsumer(SegmentWriteState).
func (f *BlockTreeOrdsPostingsFormat) FieldsConsumer(state *codecs.SegmentWriteState) (codecs.FieldsConsumer, error) {
	postingsWriter, err := codecs.NewLucene104PostingsWriter(state)
	if err != nil {
		return nil, fmt.Errorf("BlockTreeOrdsPostingsFormat.FieldsConsumer: %w", err)
	}

	btw, err := newOrdsBlockTreeTermsWriter(
		state,
		postingsWriter,
		f.minTermBlockSize,
		f.maxTermBlockSize,
	)
	if err != nil {
		_ = postingsWriter.Close()
		return nil, fmt.Errorf("BlockTreeOrdsPostingsFormat.FieldsConsumer: block-tree writer: %w", err)
	}
	return btw, nil
}

// FieldsProducer returns a FieldsProducer that wires
// Lucene104PostingsReader through OrdsBlockTreeTermsReader.
//
// Mirrors BlockTreeOrdsPostingsFormat.fieldsProducer(SegmentReadState).
// The read path is deferred; once the OrdsBlockTreeTermsReader constructor
// is fully ported, this method should be updated accordingly.
func (f *BlockTreeOrdsPostingsFormat) FieldsProducer(state *codecs.SegmentReadState) (codecs.FieldsProducer, error) {
	// The read path requires a full OrdsBlockTreeTermsReader constructor
	// that opens .tio / .tipo files and reads per-field FST indexes. That
	// work is tracked in a separate task.
	return nil, fmt.Errorf("%w: FieldsProducer", errOrdsReadPathDeferred)
}

// Compile-time interface check.
var _ codecs.PostingsFormat = (*BlockTreeOrdsPostingsFormat)(nil)
