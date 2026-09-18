// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.5.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version 2.0
//	(the "License"); you may not use this file except in compliance with
//	the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0

package lucene102

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// lucene102HnswBinaryQuantizedVectorsFormatName mirrors NAME.
const lucene102HnswBinaryQuantizedVectorsFormatName = "Lucene102HnswBinaryQuantizedVectorsFormat"

// lucene102HnswFlatVectorsFormat mirrors the protected static final
// flatVectorsFormat: the format for storing, reading and merging vectors on
// disk.
var lucene102HnswFlatVectorsFormat = NewLucene102BinaryQuantizedVectorsFormat()

// Lucene102HnswBinaryQuantizedVectorsFormat is the Go port of
// org.apache.lucene.backward_codecs.lucene102.Lucene102HnswBinaryQuantizedVectorsFormat
// (Apache Lucene 10.5.0): a vectors format that uses an HNSW graph to store
// and search for vectors, with the vectors binary quantized by
// [Lucene102BinaryQuantizedVectorsFormat].
type Lucene102HnswBinaryQuantizedVectorsFormat struct {
	*codecs.BaseKnnVectorsFormat

	// maxConn controls how many of the nearest neighbor candidates are
	// connected to the new node.
	maxConn int
	// beamWidth is the number of candidate neighbors to track while searching
	// the graph for each newly inserted node.
	beamWidth int
	// numMergeWorkers is the number of workers (threads) used when merging.
	numMergeWorkers int
	// mergeExec is the TaskExecutor used to merge, or nil.
	mergeExec *search.TaskExecutor
}

// NewLucene102HnswBinaryQuantizedVectorsFormat constructs a format using
// default graph construction parameters. Mirrors
// Lucene102HnswBinaryQuantizedVectorsFormat().
func NewLucene102HnswBinaryQuantizedVectorsFormat() *Lucene102HnswBinaryQuantizedVectorsFormat {
	return newLucene102HnswBinaryQuantizedVectorsFormat(
		codecs.Lucene99HnswVectorsFormat_DEFAULT_MAX_CONN,
		codecs.Lucene99HnswVectorsFormat_DEFAULT_BEAM_WIDTH,
		codecs.Lucene99HnswVectorsFormat_DEFAULT_NUM_MERGE_WORKER,
		nil)
}

// NewLucene102HnswBinaryQuantizedVectorsFormatWithParams constructs a format
// using the given graph construction parameters. Mirrors
// Lucene102HnswBinaryQuantizedVectorsFormat(int, int, int, ExecutorService),
// whose IllegalArgumentExceptions are returned as errors; the
// ExecutorService is rendered as the dispatch function a
// search.TaskExecutor runs tasks on, nil for none.
func NewLucene102HnswBinaryQuantizedVectorsFormatWithParams(
	maxConn, beamWidth, numMergeWorkers int, mergeExec func(func()),
) (*Lucene102HnswBinaryQuantizedVectorsFormat, error) {
	if maxConn <= 0 || maxConn > codecs.Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN {
		return nil, fmt.Errorf("maxConn must be positive and less than or equal to %d; maxConn=%d",
			codecs.Lucene99HnswVectorsFormat_MAXIMUM_MAX_CONN, maxConn)
	}
	if beamWidth <= 0 || beamWidth > codecs.Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH {
		return nil, fmt.Errorf("beamWidth must be positive and less than or equal to %d; beamWidth=%d",
			codecs.Lucene99HnswVectorsFormat_MAXIMUM_BEAM_WIDTH, beamWidth)
	}
	if numMergeWorkers == 1 && mergeExec != nil {
		return nil, errors.New("No executor service is needed as we'll use single thread to merge")
	}
	return newLucene102HnswBinaryQuantizedVectorsFormat(maxConn, beamWidth, numMergeWorkers, mergeExec), nil
}

// newLucene102HnswBinaryQuantizedVectorsFormat assigns the validated
// constructor arguments.
func newLucene102HnswBinaryQuantizedVectorsFormat(
	maxConn, beamWidth, numMergeWorkers int, mergeExec func(func()),
) *Lucene102HnswBinaryQuantizedVectorsFormat {
	f := &Lucene102HnswBinaryQuantizedVectorsFormat{
		BaseKnnVectorsFormat: codecs.NewBaseKnnVectorsFormat(lucene102HnswBinaryQuantizedVectorsFormatName),
		maxConn:              maxConn,
		beamWidth:            beamWidth,
		numMergeWorkers:      numMergeWorkers,
	}
	if mergeExec != nil {
		f.mergeExec = search.NewTaskExecutor(mergeExec)
	}
	return f
}

// FieldsWriter mirrors fieldsWriter(SegmentWriteState), which throws
// UnsupportedOperationException: old codecs may only be used for reading.
func (f *Lucene102HnswBinaryQuantizedVectorsFormat) FieldsWriter(_ *codecs.SegmentWriteState) (codecs.KnnVectorsWriter, error) {
	return nil, errOldCodecReadOnly
}

// FieldsReader mirrors fieldsReader(SegmentReadState):
// new Lucene99HnswVectorsReader(state, flatVectorsFormat.fieldsReader(state)).
func (f *Lucene102HnswBinaryQuantizedVectorsFormat) FieldsReader(state *codecs.SegmentReadState) (codecs.KnnVectorsReader, error) {
	flatVectorsReader, err := lucene102HnswFlatVectorsFormat.FlatFieldsReader(state)
	if err != nil {
		return nil, err
	}
	reader, err := codecs.NewLucene99HnswVectorsReader(state, flatVectorsReader)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

// GetMaxDimensions mirrors getMaxDimensions(String), which returns 1024.
func (f *Lucene102HnswBinaryQuantizedVectorsFormat) GetMaxDimensions(_ string) int {
	return 1024
}

// String mirrors toString().
func (f *Lucene102HnswBinaryQuantizedVectorsFormat) String() string {
	return fmt.Sprintf("Lucene102HnswBinaryQuantizedVectorsFormat(name=Lucene102HnswBinaryQuantizedVectorsFormat, maxConn=%d, beamWidth=%d, flatVectorFormat=%v)",
		f.maxConn, f.beamWidth, lucene102HnswFlatVectorsFormat)
}

var _ codecs.KnnVectorsFormat = (*Lucene102HnswBinaryQuantizedVectorsFormat)(nil)
