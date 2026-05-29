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

package codecs

import (
	"errors"
	"fmt"
)

// Lucene 9.0 points format constants. Per Lucene 10.4.0 source, the
// format stores data across three files:
//
//   - .kdm (META_EXTENSION)  : per-field metadata (dimensions, byte width,
//     min/max packed values, BKD root offset).
//   - .kdi (INDEX_EXTENSION) : inner nodes of the BKD tree.
//   - .kdd (DATA_EXTENSION)  : leaf nodes (the bulk of the data).
const (
	// Lucene90PointsDataCodec is the codec name stamped into the .kdd
	// IndexHeader.
	Lucene90PointsDataCodec = "Lucene90PointsFormatData"
	// Lucene90PointsIndexCodec is the codec name stamped into the .kdi
	// IndexHeader.
	Lucene90PointsIndexCodec = "Lucene90PointsFormatIndex"
	// Lucene90PointsMetaCodec is the codec name stamped into the .kdm
	// IndexHeader.
	Lucene90PointsMetaCodec = "Lucene90PointsFormatMeta"

	// Lucene90PointsDataExtension is the file extension for leaf blocks.
	Lucene90PointsDataExtension = "kdd"
	// Lucene90PointsIndexExtension is the file extension for inner nodes.
	Lucene90PointsIndexExtension = "kdi"
	// Lucene90PointsMetaExtension is the file extension for per-field
	// metadata.
	Lucene90PointsMetaExtension = "kdm"

	// Lucene90PointsVersionStart is the inclusive minimum supported
	// format version.
	Lucene90PointsVersionStart int32 = 0
	// Lucene90PointsVersionBKDVectorizedBPV24 is the version that
	// introduced the vectorised 24-bit-per-value BKD encoding and the
	// 21-bpv variant. Currently the only newer version beyond
	// VersionStart.
	Lucene90PointsVersionBKDVectorizedBPV24 int32 = 1
	// Lucene90PointsVersionCurrent is the current format version.
	Lucene90PointsVersionCurrent int32 = Lucene90PointsVersionBKDVectorizedBPV24
)

// Lucene90PointsBKDVersion maps the Lucene90Points format version to the
// BKDWriter version expected by util/bkd.
//
// Mirrors Lucene10's Lucene90PointsFormat.VERSION_TO_BKD_VERSION.
func Lucene90PointsBKDVersion(version int32) (int32, error) {
	switch version {
	case Lucene90PointsVersionStart:
		return 9, nil // BKDWriter.VERSION_META_FILE
	case Lucene90PointsVersionBKDVectorizedBPV24:
		return 10, nil // BKDWriter.VERSION_VECTORIZE_BPV24_AND_INTRODUCE_BPV21
	default:
		return 0, fmt.Errorf("lucene90 points: invalid version=%d", version)
	}
}

// Lucene90PointsFormat is the Lucene 9.0 points format.
//
// FieldsWriter drives util/bkd.BKDWriter to serialise each point field's
// BKD tree into the shared .kdd / .kdi / .kdm trio, and FieldsReader opens
// those files and exposes a util/bkd.BKDReader-backed PointValues per
// field. The on-disk framing (per-file CodecUtil IndexHeader, the meta
// stream's per-field [fieldNumber][BKD-meta] records, the -1 sentinel, the
// index/data lengths, and the trailing footers) is byte-faithful to
// org.apache.lucene.codecs.lucene90.Lucene90PointsWriter (Lucene 10.4.0).
//
// This is the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90PointsFormat (Lucene 10.4.0).
type Lucene90PointsFormat struct {
	*BasePointsFormat
	version int32
}

// NewLucene90PointsFormat creates a new Lucene90PointsFormat at the
// current (latest) version.
func NewLucene90PointsFormat() *Lucene90PointsFormat {
	return NewLucene90PointsFormatWithVersion(Lucene90PointsVersionCurrent)
}

// NewLucene90PointsFormatWithVersion lets callers pin a specific format
// version. Used by back-compat tests; production code should use the
// current-version constructor.
func NewLucene90PointsFormatWithVersion(version int32) *Lucene90PointsFormat {
	if _, err := Lucene90PointsBKDVersion(version); err != nil {
		// Match the Java reference which throws
		// IllegalArgumentException; Go callers signal via a panic at
		// construction time so a bad version is caught at startup.
		panic(err)
	}
	return &Lucene90PointsFormat{
		BasePointsFormat: NewBasePointsFormat("Lucene90PointsFormat"),
		version:          version,
	}
}

// Version returns the format version this instance produces / accepts.
func (f *Lucene90PointsFormat) Version() int32 { return f.version }

// FieldsWriter returns a writer that drives util/bkd.BKDWriter to serialise
// each point field into the shared .kdd / .kdi / .kdm trio.
//
// The BKD machinery lives in the codecs/lucene90 sub-package because
// util/bkd imports this (codecs) package for its CodecUtil/Relation
// primitives, so codecs cannot import util/bkd directly without a cycle.
// codecs/lucene90 sits above both and installs the real implementation via
// SetLucene90PointsImpl at init time; FieldsWriter therefore dispatches
// through the installed hook. A nil hook (the sub-package was not linked)
// yields an explicit error rather than a silent miswrite.
func (f *Lucene90PointsFormat) FieldsWriter(state *SegmentWriteState) (PointsWriter, error) {
	if lucene90PointsWriterHook == nil {
		return nil, errors.New("lucene90 points: BKD writer impl not linked; blank-import codecs/lucene90")
	}
	return lucene90PointsWriterHook(state, f.version)
}

// FieldsReader returns a reader that opens the .kdd / .kdi / .kdm trio and
// exposes a util/bkd.BKDReader-backed PointValues per field. See FieldsWriter
// for why the implementation is installed via a hook.
func (f *Lucene90PointsFormat) FieldsReader(state *SegmentReadState) (PointsReader, error) {
	if lucene90PointsReaderHook == nil {
		return nil, errors.New("lucene90 points: BKD reader impl not linked; blank-import codecs/lucene90")
	}
	return lucene90PointsReaderHook(state)
}

// Lucene90PointsWriterHook / Lucene90PointsReaderHook are the function shapes
// the codecs/lucene90 sub-package installs through SetLucene90PointsImpl.
type Lucene90PointsWriterHook func(state *SegmentWriteState, version int32) (PointsWriter, error)

// Lucene90PointsReaderHook constructs the BKD-backed points reader for a
// segment.
type Lucene90PointsReaderHook func(state *SegmentReadState) (PointsReader, error)

var (
	lucene90PointsWriterHook Lucene90PointsWriterHook
	lucene90PointsReaderHook Lucene90PointsReaderHook
)

// SetLucene90PointsImpl installs the BKD-backed Lucene90 points writer/reader
// implementations. It is called from the codecs/lucene90 sub-package init so
// that the import edge runs lucene90 -> codecs (and lucene90 -> util/bkd ->
// codecs), never codecs -> util/bkd. Passing nil for either hook is rejected.
func SetLucene90PointsImpl(writer Lucene90PointsWriterHook, reader Lucene90PointsReaderHook) {
	if writer == nil || reader == nil {
		panic("codecs: SetLucene90PointsImpl requires non-nil writer and reader hooks")
	}
	lucene90PointsWriterHook = writer
	lucene90PointsReaderHook = reader
}

// -----------------------------------------------------------------------------
// Shared file-list helper (used by the codecs/lucene90 BKD implementation).
// -----------------------------------------------------------------------------

// Lucene90PointFileEntry pairs a points file name with its CodecUtil codec
// name. Exported so the codecs/lucene90 BKD writer/reader can compute the
// canonical .kdd / .kdi / .kdm names without re-deriving the constants.
type Lucene90PointFileEntry struct {
	Name  string
	Codec string
}

// Lucene90PointFileList returns the (.kdd, .kdi, .kdm) file names and codec
// names for the given segment, in that fixed order.
func Lucene90PointFileList(segmentName, segmentSuffix string) []Lucene90PointFileEntry {
	build := func(ext string) string {
		if segmentSuffix == "" {
			return segmentName + "." + ext
		}
		return segmentName + "_" + segmentSuffix + "." + ext
	}
	return []Lucene90PointFileEntry{
		{Name: build(Lucene90PointsDataExtension), Codec: Lucene90PointsDataCodec},
		{Name: build(Lucene90PointsIndexExtension), Codec: Lucene90PointsIndexCodec},
		{Name: build(Lucene90PointsMetaExtension), Codec: Lucene90PointsMetaCodec},
	}
}
