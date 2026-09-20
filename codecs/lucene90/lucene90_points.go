// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//   Licensed to the Apache Software Foundation (ASF) under one or more
//   contributor license agreements. See the NOTICE file distributed with
//   this work for additional information regarding copyright ownership.
//   The ASF licenses this file to You under the Apache License, Version
//   2.0 (the "License"); you may not use this file except in compliance
//   with the License. You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//   implied. See the License for the specific language governing
//   permissions and limitations under the License.

package lucene90

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// This file holds the BKD-backed Lucene 9.0 points writer/reader and their
// PointValues adapter. It lives in the codecs/lucene90 sub-package — not the
// top-level codecs package — because util/bkd imports codecs for its
// CodecUtil/Relation primitives; codecs therefore cannot import util/bkd
// without an import cycle. This package sits above both and installs the
// implementations into the top-level Lucene90PointsFormat facade via
// codecs.SetLucene90PointsImpl in init.
//
// On-disk framing is byte-faithful to
// org.apache.lucene.codecs.lucene90.Lucene90PointsWriter /
// Lucene90PointsReader (Lucene 10.4.0).

func init() {
	codecs.SetLucene90PointsImpl(newPointsWriter, newPointsReader)
}

// -----------------------------------------------------------------------------
// pointsReader — byte-faithful BKD reader.
// -----------------------------------------------------------------------------
// -----------------------------------------------------------------------------
// pointsReader — byte-faithful BKD reader.
// -----------------------------------------------------------------------------

// pointsReader reads points in Lucene 9.0 format. It is the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90PointsReader (Lucene 10.4.0).
type pointsReader struct {
	state   *codecs.SegmentReadState
	indexIn store.IndexInput
	dataIn  store.IndexInput
	readers map[int]*bkd.BKDReader
	closed  bool
}

// newPointsReader opens the points trio for the segment and builds a BKDReader
// per indexed field. Installed as the codecs Lucene90 points reader hook.
func newPointsReader(state *codecs.SegmentReadState) (codecs.PointsReader, error) {
	files := codecs.Lucene90PointFileList(state.SegmentInfo.Name(), state.SegmentSuffix)
	dataEntry, indexEntry, metaEntry := files[0], files[1], files[2]

	r := &pointsReader{state: state, readers: make(map[int]*bkd.BKDReader)}

	success := false
	defer func() {
		if !success {
			_ = r.Close()
		}
	}()

	var err error
	r.indexIn, err = state.Directory.OpenInput(indexEntry.Name, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene90 points: open %q: %w", indexEntry.Name, err)
	}
	if _, err = codecs.CheckIndexHeader(r.indexIn, indexEntry.Codec, codecs.Lucene90PointsVersionStart, codecs.Lucene90PointsVersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("lucene90 points: header %q: %w", indexEntry.Name, err)
	}

	r.dataIn, err = state.Directory.OpenInput(dataEntry.Name, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene90 points: open %q: %w", dataEntry.Name, err)
	}
	if _, err = codecs.CheckIndexHeader(r.dataIn, dataEntry.Codec, codecs.Lucene90PointsVersionStart, codecs.Lucene90PointsVersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
		return nil, fmt.Errorf("lucene90 points: header %q: %w", dataEntry.Name, err)
	}

	metaRaw, err := state.Directory.OpenInput(metaEntry.Name, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene90 points: open %q: %w", metaEntry.Name, err)
	}
	metaIn := store.NewChecksumIndexInput(metaRaw)
	if err := r.loadMeta(metaIn, metaEntry.Codec); err != nil {
		_ = metaRaw.Close()
		return nil, err
	}
	if err := metaRaw.Close(); err != nil {
		return nil, err
	}

	success = true
	return r, nil
}

// loadMeta validates the meta header, replays every per-field BKD metadata
// record into a BKDReader, reads the trailing index/data lengths, and checks
// the meta footer. Mirrors the metaIn loop in Lucene90PointsReader's
// constructor.
func (r *pointsReader) loadMeta(metaIn *store.ChecksumIndexInput, codec string) error {
	if _, err := codecs.CheckIndexHeader(metaIn, codec, codecs.Lucene90PointsVersionStart, codecs.Lucene90PointsVersionCurrent, r.state.SegmentInfo.GetID(), r.state.SegmentSuffix); err != nil {
		return fmt.Errorf("lucene90 points: meta header: %w", err)
	}
	for {
		fieldNumber, err := metaIn.ReadInt()
		if err != nil {
			return fmt.Errorf("lucene90 points: read field number: %w", err)
		}
		if fieldNumber == -1 {
			break
		}
		if fieldNumber < 0 {
			return fmt.Errorf("lucene90 points: illegal field number: %d", fieldNumber)
		}
		bkdReader, err := bkd.NewBKDReader(metaIn, r.indexIn, r.dataIn)
		if err != nil {
			return fmt.Errorf("lucene90 points: build bkd reader for field %d: %w", fieldNumber, err)
		}
		r.readers[int(fieldNumber)] = bkdReader
	}
	if _, err := metaIn.ReadLong(); err != nil { // indexLength
		return fmt.Errorf("lucene90 points: read index length: %w", err)
	}
	if _, err := metaIn.ReadLong(); err != nil { // dataLength
		return fmt.Errorf("lucene90 points: read data length: %w", err)
	}
	if _, err := store.CheckFooter(metaIn); err != nil {
		return fmt.Errorf("lucene90 points: meta footer: %w", err)
	}
	return nil
}

// GetValues returns the BKDReader-backed PointValues for fieldName, or nil
// when the field carries no indexed points. Mirrors
// Lucene90PointsReader.getValues.
func (r *pointsReader) GetValues(fieldName string) (index.PointValues, error) {
	if r.closed {
		return nil, errors.New("lucene90 points: reader closed")
	}
	fieldInfo := r.state.FieldInfos.GetByName(fieldName)
	if fieldInfo == nil {
		return nil, fmt.Errorf("lucene90 points: field %q is unrecognized", fieldName)
	}
	if fieldInfo.PointDimensionCount() == 0 {
		return nil, fmt.Errorf("lucene90 points: field %q did not index point values", fieldName)
	}
	bkdReader, ok := r.readers[fieldInfo.Number()]
	if !ok {
		return nil, nil
	}
	return bkdReader, nil
}

// CheckIntegrity verifies the index and data file checksums.
func (r *pointsReader) CheckIntegrity() error {
	if r.closed {
		return errors.New("lucene90 points: reader closed")
	}
	if _, err := codecs.ChecksumEntireFile(r.indexIn); err != nil {
		return err
	}
	if _, err := codecs.ChecksumEntireFile(r.dataIn); err != nil {
		return err
	}
	return nil
}

// Close releases the index and data inputs. Idempotent.
func (r *pointsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	var lastErr error
	if r.indexIn != nil {
		if err := r.indexIn.Close(); err != nil {
			lastErr = err
		}
	}
	if r.dataIn != nil {
		if err := r.dataIn.Close(); err != nil {
			lastErr = err
		}
	}
	r.readers = nil
	return lastErr
}

// GetMergeInstance returns this reader. Lucene90PointsReader does not override
// getMergeInstance(), so it keeps the PointsReader default body,
// `return this;` (PointsReader.java).
func (r *pointsReader) GetMergeInstance() codecs.PointsReader { return r }

// pointsReaderGetValues is the wide read surface a Lucene90 points reader
// exposes beyond the narrow codecs.PointsReader (CheckIntegrity/Close). It
// mirrors org.apache.lucene.codecs.PointsReader.getValues. The index-side
// SegmentReader.GetPointValues type-asserts the codec reader to this
// interface.
type pointsReaderGetValues interface {
	GetValues(field string) (index.PointValues, error)
}

var (
	_ codecs.PointsReader   = (*pointsReader)(nil)
	_ pointsReaderGetValues = (*pointsReader)(nil)
)

// -----------------------------------------------------------------------------
// Java's Lucene90PointsReader.getValues returns the BKDReader itself
// (org.apache.lucene.util.bkd.BKDReader extends
// org.apache.lucene.index.PointValues), so there is no wrapper type here. The
// `pointValues` struct and the `bkdVisitorBridge` that used to live at this
// point bridged Gocene's two incompatible PointValues renderings and its two
// IntersectVisitor renderings; with one PointValues and one IntersectVisitor
// in the module they have no Lucene counterpart and are gone.
