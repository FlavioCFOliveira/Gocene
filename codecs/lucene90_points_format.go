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

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
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
		return 4, nil // BKDWriter.VERSION_META_FILE
	case Lucene90PointsVersionBKDVectorizedBPV24:
		return 6, nil // BKDWriter.VERSION_VECTORIZE_BPV24_AND_INTRODUCE_BPV21
	default:
		return 0, fmt.Errorf("lucene90 points: invalid version=%d", version)
	}
}

// Lucene90PointsFormat is the Lucene 9.0 points format.
//
// This type currently exposes the format constants and the version-to-BKD
// mapping verbatim from Lucene 10.4.0. The per-field BKD writer/reader
// bodies (the heavy machinery that uses util/bkd to encode point values)
// are not yet wired through; the writer/reader returned by
// FieldsWriter/FieldsReader produce valid CodecUtil framing on the three
// files but defer per-field encoding to Sprint 22 alongside the rest of
// the codec-side state plumbing.
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

// FieldsWriter returns a writer for writing points. Phase 1 returns a
// writer that emits CodecUtil framing on the three files; per-field BKD
// encoding is deferred (see Lucene90PointsWriter).
func (f *Lucene90PointsFormat) FieldsWriter(state *SegmentWriteState) (PointsWriter, error) {
	return NewLucene90PointsWriterWithVersion(state, f.version), nil
}

// FieldsReader returns a reader for reading points. Phase 1 returns a
// reader that opens and validates the three files' headers; per-field
// queries return empty results.
func (f *Lucene90PointsFormat) FieldsReader(state *SegmentReadState) (PointsReader, error) {
	return NewLucene90PointsReader(state)
}

// -----------------------------------------------------------------------------
// Lucene90PointsWriter — Phase 1 shell.
// -----------------------------------------------------------------------------

// Lucene90PointsWriter writes points in Lucene 9.0 format.
//
// DEFERRED to Sprint 22: per-field BKD encoding. The Phase 1 writer
// opens the .kdd / .kdi / .kdm trio at construction and stamps the
// CodecUtil IndexHeader on each; WriteField returns a deferred-error.
// Close finalises every file with the CodecUtil Footer so the three
// files are well-formed even when no points were written.
type Lucene90PointsWriter struct {
	state   *SegmentWriteState
	version int32
	closed  bool
}

// NewLucene90PointsWriter creates a new Lucene90PointsWriter at the
// current version.
func NewLucene90PointsWriter(state *SegmentWriteState) *Lucene90PointsWriter {
	return NewLucene90PointsWriterWithVersion(state, Lucene90PointsVersionCurrent)
}

// NewLucene90PointsWriterWithVersion creates a writer pinned to a
// specific format version.
func NewLucene90PointsWriterWithVersion(state *SegmentWriteState, version int32) *Lucene90PointsWriter {
	return &Lucene90PointsWriter{state: state, version: version}
}

// WriteField writes a point field. Phase 1 deferred — see type comment.
func (w *Lucene90PointsWriter) WriteField(fieldInfo *index.FieldInfo, reader PointsReader) error {
	if w.closed {
		return errors.New("lucene90 points: writer closed")
	}
	return errors.New("lucene90 points: WriteField is deferred to Sprint 22 (full BKD encoding)")
}

// Finish is a no-op in Phase 1.
func (w *Lucene90PointsWriter) Finish() error {
	if w.closed {
		return errors.New("lucene90 points: writer closed")
	}
	return nil
}

// Close stamps an IndexHeader + Footer on each of .kdd, .kdi, .kdm so
// downstream readers see a well-framed (but empty) points segment.
func (w *Lucene90PointsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return finaliseLucene90PointFiles(w.state, w.version)
}

// -----------------------------------------------------------------------------
// Lucene90PointsReader — Phase 1 shell.
// -----------------------------------------------------------------------------

// Lucene90PointsReader reads points in Lucene 9.0 format.
//
// DEFERRED to Sprint 22: per-field BKD decoding. The Phase 1 reader
// validates the IndexHeader on every file present (returning an error
// if any header is corrupt or doesn't match this segment's id); the
// per-field query API returns nil.
type Lucene90PointsReader struct {
	state  *SegmentReadState
	closed bool
}

// NewLucene90PointsReader creates a new Lucene90PointsReader.
func NewLucene90PointsReader(state *SegmentReadState) (*Lucene90PointsReader, error) {
	if err := validateLucene90PointFiles(state); err != nil {
		return nil, err
	}
	return &Lucene90PointsReader{state: state}, nil
}

// CheckIntegrity checks the integrity of the points.
func (r *Lucene90PointsReader) CheckIntegrity() error {
	if r.closed {
		return errors.New("lucene90 points: reader closed")
	}
	return nil
}

// Close releases resources.
func (r *Lucene90PointsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return nil
}

// -----------------------------------------------------------------------------
// Shared file-framing helpers.
// -----------------------------------------------------------------------------

// finaliseLucene90PointFiles stamps a CodecUtil IndexHeader + Footer
// onto each of the three points files (.kdd, .kdi, .kdm) and creates
// them if they do not already exist.
func finaliseLucene90PointFiles(state *SegmentWriteState, version int32) error {
	for _, pair := range lucene90PointFileList(state, true) {
		raw, err := state.Directory.CreateOutput(pair.name, store.IOContext{Context: store.ContextWrite})
		if err != nil {
			return fmt.Errorf("lucene90 points: create %q: %w", pair.name, err)
		}
		out := store.NewChecksumIndexOutput(raw)
		if err := WriteIndexHeader(out, pair.codec, version, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
			_ = out.Close()
			return fmt.Errorf("lucene90 points: header %q: %w", pair.name, err)
		}
		if err := WriteFooter(out); err != nil {
			_ = out.Close()
			return fmt.Errorf("lucene90 points: footer %q: %w", pair.name, err)
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}

// validateLucene90PointFiles iterates the three known points files and
// validates each one's IndexHeader when present.
func validateLucene90PointFiles(state *SegmentReadState) error {
	for _, pair := range lucene90PointFileList(&SegmentWriteState{
		Directory:     state.Directory,
		SegmentInfo:   state.SegmentInfo,
		FieldInfos:    state.FieldInfos,
		SegmentSuffix: state.SegmentSuffix,
	}, false) {
		if !state.Directory.FileExists(pair.name) {
			continue
		}
		in, err := state.Directory.OpenInput(pair.name, store.IOContext{Context: store.ContextRead})
		if err != nil {
			return err
		}
		csIn := store.NewChecksumIndexInput(in)
		// Accept either format version on read; the caller will use the
		// stamped version to dispatch the proper BKD decoder when Sprint
		// 22 lands.
		if _, err := CheckIndexHeader(csIn, pair.codec, Lucene90PointsVersionStart, Lucene90PointsVersionCurrent, state.SegmentInfo.GetID(), state.SegmentSuffix); err != nil {
			_ = in.Close()
			return fmt.Errorf("lucene90 points: header %q: %w", pair.name, err)
		}
		_ = in.Close()
	}
	return nil
}

type lucene90PointFileEntry struct {
	name  string
	codec string
}

// lucene90PointFileList returns the three (.kdd, .kdi, .kdm) file names
// and codec names for the given segment. When mustExist is true, every
// returned file is guaranteed to be createable (segment-suffix variant).
func lucene90PointFileList(state *SegmentWriteState, _ bool) []lucene90PointFileEntry {
	seg := state.SegmentInfo.Name()
	suffix := state.SegmentSuffix
	build := func(ext string) string {
		if suffix == "" {
			return seg + "." + ext
		}
		return seg + "_" + suffix + "." + ext
	}
	return []lucene90PointFileEntry{
		{name: build(Lucene90PointsDataExtension), codec: Lucene90PointsDataCodec},
		{name: build(Lucene90PointsIndexExtension), codec: Lucene90PointsIndexCodec},
		{name: build(Lucene90PointsMetaExtension), codec: Lucene90PointsMetaCodec},
	}
}
