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

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs/hnsw"
)

// lucene99FlatVectorsFormatName mirrors Lucene99FlatVectorsFormat.NAME.
const lucene99FlatVectorsFormatName = "Lucene99FlatVectorsFormat"

// Lucene99FlatVectorsFormat is the Go port of
// org.apache.lucene.codecs.lucene99.Lucene99FlatVectorsFormat (Apache Lucene
// 10.5.0), the Lucene 9.9 flat vector format, which encodes numeric vector
// values into the .vec data file and the .vemf metadata file. See
// [Lucene99FlatVectorsWriter] for the byte layout.
//
// The Java class lives in org.apache.lucene.codecs.lucene99; its Go port
// lives in the codecs package with the rest of that Java package because
// the codecs/lucene90 and codecs/lucene95 packages import codecs.
type Lucene99FlatVectorsFormat struct {
	*hnsw.BaseFlatVectorsFormat
	vectorsScorer hnsw.FlatVectorsScorer
}

// NewLucene99FlatVectorsFormat constructs a format that scores vectors with
// vectorsScorer. Mirrors Lucene99FlatVectorsFormat(FlatVectorsScorer).
func NewLucene99FlatVectorsFormat(vectorsScorer hnsw.FlatVectorsScorer) *Lucene99FlatVectorsFormat {
	return &Lucene99FlatVectorsFormat{
		BaseFlatVectorsFormat: hnsw.NewBaseFlatVectorsFormat(lucene99FlatVectorsFormatName),
		vectorsScorer:         vectorsScorer,
	}
}

// FlatFieldsWriter returns a new [Lucene99FlatVectorsWriter]. Mirrors
// fieldsWriter(SegmentWriteState).
func (f *Lucene99FlatVectorsFormat) FlatFieldsWriter(state *SegmentWriteState) (hnsw.FlatVectorsWriter, error) {
	w, err := NewLucene99FlatVectorsWriter(state, f.vectorsScorer)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// FlatFieldsReader returns a new [Lucene99FlatVectorsReader]. Mirrors
// fieldsReader(SegmentReadState).
func (f *Lucene99FlatVectorsFormat) FlatFieldsReader(state *SegmentReadState) (hnsw.FlatVectorsReader, error) {
	r, err := NewLucene99FlatVectorsReader(state, f.vectorsScorer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// FieldsWriter returns the same writer as FlatFieldsWriter, typed as the
// inherited KnnVectorsFormat factory.
func (f *Lucene99FlatVectorsFormat) FieldsWriter(state *SegmentWriteState) (KnnVectorsWriter, error) {
	return f.FlatFieldsWriter(state)
}

// FieldsReader returns the same reader as FlatFieldsReader, typed as the
// inherited KnnVectorsFormat factory.
func (f *Lucene99FlatVectorsFormat) FieldsReader(state *SegmentReadState) (KnnVectorsReader, error) {
	return f.FlatFieldsReader(state)
}

// String mirrors Lucene99FlatVectorsFormat.toString().
func (f *Lucene99FlatVectorsFormat) String() string {
	return fmt.Sprintf("Lucene99FlatVectorsFormat(vectorsScorer=%v)", f.vectorsScorer)
}

var _ hnsw.FlatVectorsFormat = (*Lucene99FlatVectorsFormat)(nil)
