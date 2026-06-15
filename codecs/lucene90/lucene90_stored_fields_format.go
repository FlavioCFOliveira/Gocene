// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version
//	2.0 (the "License"); you may not use this file except in compliance
//	with the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//	implied. See the License for the specific language governing
//	permissions and limitations under the License.

// Package lucene90 ports org.apache.lucene.codecs.lucene90.
//
// This file implements Lucene90StoredFieldsFormat, the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90StoredFieldsFormat. The
// format delegates the actual data layout to
// Lucene90CompressingStoredFieldsFormat, configuring the BEST_SPEED or
// BEST_COMPRESSION presets (chunk length, max docs per chunk, block
// shift) defined by the Apache Lucene 10.4.0 reference.
package lucene90

import (
	"fmt"

	codecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	lucene90compressing "github.com/FlavioCFOliveira/Gocene/codecs/lucene90/compressing"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene90StoredFieldsMode selects the compression preset used by
// Lucene90StoredFieldsFormat.
//
// It is the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90StoredFieldsFormat.Mode.
type Lucene90StoredFieldsMode int

const (
	// Lucene90StoredFieldsBestSpeed trades compression ratio for retrieval
	// speed. It is the Go port of Mode.BEST_SPEED.
	Lucene90StoredFieldsBestSpeed Lucene90StoredFieldsMode = iota
	// Lucene90StoredFieldsBestCompression trades retrieval speed for
	// compression ratio. It is the Go port of Mode.BEST_COMPRESSION.
	Lucene90StoredFieldsBestCompression
)

// String returns the canonical name of the mode, matching
// java.lang.Enum#name() output for Mode.BEST_SPEED and
// Mode.BEST_COMPRESSION. The textual form is what is persisted as the
// segment attribute under [Lucene90StoredFieldsModeKey].
func (m Lucene90StoredFieldsMode) String() string {
	switch m {
	case Lucene90StoredFieldsBestSpeed:
		return "BEST_SPEED"
	case Lucene90StoredFieldsBestCompression:
		return "BEST_COMPRESSION"
	default:
		return fmt.Sprintf("Lucene90StoredFieldsMode(%d)", int(m))
	}
}

// parseLucene90StoredFieldsMode resolves the textual mode form produced
// by Lucene90StoredFieldsMode.String back into the typed value. It is
// the Go counterpart of Mode.valueOf(String).
func parseLucene90StoredFieldsMode(s string) (Lucene90StoredFieldsMode, error) {
	switch s {
	case "BEST_SPEED":
		return Lucene90StoredFieldsBestSpeed, nil
	case "BEST_COMPRESSION":
		return Lucene90StoredFieldsBestCompression, nil
	default:
		return 0, fmt.Errorf("lucene90: unknown stored-fields mode %q", s)
	}
}

// Lucene90StoredFieldsModeKey is the SegmentInfo attribute key under
// which Lucene90StoredFieldsFormat persists the selected
// [Lucene90StoredFieldsMode]. It mirrors
// Lucene90StoredFieldsFormat.MODE_KEY in Apache Lucene 10.4.0:
// "Lucene90StoredFieldsFormat.mode".
const Lucene90StoredFieldsModeKey = "Lucene90StoredFieldsFormat.mode"

// Tuning constants copied verbatim from
// org.apache.lucene.codecs.lucene90.Lucene90StoredFieldsFormat.
//
// Each Lucene block is sliced into ten LZ4/DEFLATE sub-blocks; the
// chunk lengths below are therefore 10 * <sub-block size>.
const (
	// lucene90StoredFieldsBestSpeedBlockLength shoots for ten sub-blocks
	// of 8 KiB each (the LZ4 BEST_SPEED layout).
	lucene90StoredFieldsBestSpeedBlockLength = 10 * 8 * 1024
	// lucene90StoredFieldsBestSpeedMaxDocsPerChunk is the BEST_SPEED cap
	// on documents per stored-fields chunk.
	lucene90StoredFieldsBestSpeedMaxDocsPerChunk = 1024

	// lucene90StoredFieldsBestCompressionBlockLength shoots for ten
	// sub-blocks of 48 KiB each (the DEFLATE BEST_COMPRESSION layout).
	lucene90StoredFieldsBestCompressionBlockLength = 10 * 48 * 1024
	// lucene90StoredFieldsBestCompressionMaxDocsPerChunk is the
	// BEST_COMPRESSION cap on documents per stored-fields chunk.
	lucene90StoredFieldsBestCompressionMaxDocsPerChunk = 4096

	// lucene90StoredFieldsBlockShift is shared by both modes (10 chunks
	// per fields-index block). It mirrors the trailing argument passed to
	// Lucene90CompressingStoredFieldsFormat in
	// Lucene90StoredFieldsFormat.impl(Mode).
	lucene90StoredFieldsBlockShift = 10

	// lucene90StoredFieldsBestSpeedFormatName names the
	// Lucene90CompressingStoredFieldsFormat segment that is produced for
	// BEST_SPEED. It is the on-disk file-format tag and must match the
	// Java reference for byte-for-byte compatibility.
	lucene90StoredFieldsBestSpeedFormatName = "Lucene90StoredFieldsFastData"
	// lucene90StoredFieldsBestCompressionFormatName is the BEST_COMPRESSION
	// counterpart of lucene90StoredFieldsBestSpeedFormatName.
	lucene90StoredFieldsBestCompressionFormatName = "Lucene90StoredFieldsHighData"
)

// bestSpeedMode and bestCompressionMode hold the package-level
// CompressionMode singletons used by impl(). They mirror the public
// BEST_SPEED_MODE / BEST_COMPRESSION_MODE statics on the Java class.
var (
	// Lucene90StoredFieldsBestSpeedMode is the CompressionMode used by
	// Lucene90StoredFieldsBestSpeed. It is the Go counterpart of
	// Lucene90StoredFieldsFormat.BEST_SPEED_MODE.
	Lucene90StoredFieldsBestSpeedMode compressing.CompressionMode = NewLZ4WithPresetDictCompressionMode()
	// Lucene90StoredFieldsBestCompressionMode is the CompressionMode used
	// by Lucene90StoredFieldsBestCompression. It is the Go counterpart of
	// Lucene90StoredFieldsFormat.BEST_COMPRESSION_MODE.
	Lucene90StoredFieldsBestCompressionMode compressing.CompressionMode = NewDeflateWithPresetDictCompressionMode()
)

// Lucene90StoredFieldsFormat is the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90StoredFieldsFormat.
//
// The format compresses blocks of stored-field documents using either
// LZ4 (BEST_SPEED, 8 KiB sub-blocks) or DEFLATE (BEST_COMPRESSION,
// 48 KiB sub-blocks) and writes three segment files (.fdt data, .fdx
// per-block index, .fdm monotonic-arrays metadata). The compression
// mode is persisted on the SegmentInfo under
// [Lucene90StoredFieldsModeKey].
type Lucene90StoredFieldsFormat struct {
	mode Lucene90StoredFieldsMode
}

// NewLucene90StoredFieldsFormat builds the format with the default
// preset (BEST_SPEED), matching the no-arg Java constructor.
func NewLucene90StoredFieldsFormat() *Lucene90StoredFieldsFormat {
	return NewLucene90StoredFieldsFormatWithMode(Lucene90StoredFieldsBestSpeed)
}

// NewLucene90StoredFieldsFormatWithMode builds the format with the
// supplied preset. It is the Go counterpart of the
// Lucene90StoredFieldsFormat(Mode) constructor.
func NewLucene90StoredFieldsFormatWithMode(mode Lucene90StoredFieldsMode) *Lucene90StoredFieldsFormat {
	switch mode {
	case Lucene90StoredFieldsBestSpeed, Lucene90StoredFieldsBestCompression:
		return &Lucene90StoredFieldsFormat{mode: mode}
	default:
		// Java raises NullPointerException via Objects.requireNonNull when
		// the mode is null; rejecting an unrecognised enum here mirrors
		// that intent while keeping the API total in Go.
		panic(fmt.Sprintf("lucene90: invalid stored-fields mode %d", int(mode)))
	}
}

// Name returns the format's stable identifier. The Java class exposes
// the name implicitly through its concrete type; Gocene threads it
// through the [codecs.StoredFieldsFormat] interface.
func (f *Lucene90StoredFieldsFormat) Name() string {
	return "Lucene90StoredFieldsFormat"
}

// Mode returns the configured compression preset.
func (f *Lucene90StoredFieldsFormat) Mode() Lucene90StoredFieldsMode {
	return f.mode
}

// FieldsReader opens the stored-fields reader for the given segment.
//
// The mode is read back from the SegmentInfo attribute written by
// FieldsWriter; an empty value triggers an "IllegalStateException"-like
// error, matching the Java reference behaviour.
func (f *Lucene90StoredFieldsFormat) FieldsReader(
	dir store.Directory,
	si *index.SegmentInfo,
	fn *index.FieldInfos,
	ctx store.IOContext,
) (codecs.StoredFieldsReader, error) {
	value := si.GetAttribute(Lucene90StoredFieldsModeKey)
	if value == "" {
		return nil, fmt.Errorf(
			"lucene90: missing value for %s for segment: %s",
			Lucene90StoredFieldsModeKey, si.Name(),
		)
	}
	mode, err := parseLucene90StoredFieldsMode(value)
	if err != nil {
		return nil, fmt.Errorf(
			"lucene90: segment %s: %w", si.Name(), err,
		)
	}
	return f.impl(mode).FieldsReader(dir, si, fn, ctx)
}

// FieldsWriter opens the stored-fields writer for the given segment.
//
// The configured mode is stamped onto the SegmentInfo attributes; an
// existing, differing value is rejected to preserve append-only
// segment immutability, matching the Java reference.
func (f *Lucene90StoredFieldsFormat) FieldsWriter(
	dir store.Directory,
	si *index.SegmentInfo,
	ctx store.IOContext,
) (codecs.StoredFieldsWriter, error) {
	name := f.mode.String()
	// SegmentInfo.SetAttribute is the Gocene equivalent of Java's
	// putAttribute(K,V), which returns the previous value. Until that
	// API gains a Put variant we emulate it with a read-then-write
	// against the SegmentInfo. Stored-fields writes are single-threaded
	// per segment, so the read/write is not a race in practice.
	previous := si.GetAttribute(Lucene90StoredFieldsModeKey)
	if previous != "" && previous != name {
		return nil, fmt.Errorf(
			"lucene90: found existing value for %s for segment: %s old=%s, new=%s",
			Lucene90StoredFieldsModeKey, si.Name(), previous, name,
		)
	}
	si.SetAttribute(Lucene90StoredFieldsModeKey, name)
	return f.impl(f.mode).FieldsWriter(dir, si, ctx)
}

// impl returns the underlying Lucene90CompressingStoredFieldsFormat
// configured for the requested mode. It mirrors the package-private
// impl(Mode) method on the Java class.
func (f *Lucene90StoredFieldsFormat) impl(mode Lucene90StoredFieldsMode) codecs.StoredFieldsFormat {
	switch mode {
	case Lucene90StoredFieldsBestSpeed:
		return lucene90compressing.NewLucene90CompressingStoredFieldsFormatWithOptions(
			lucene90StoredFieldsBestSpeedFormatName,
			Lucene90StoredFieldsBestSpeedMode,
			lucene90StoredFieldsBestSpeedBlockLength,
			lucene90StoredFieldsBestSpeedMaxDocsPerChunk,
			lucene90StoredFieldsBlockShift,
		)
	case Lucene90StoredFieldsBestCompression:
		return lucene90compressing.NewLucene90CompressingStoredFieldsFormatWithOptions(
			lucene90StoredFieldsBestCompressionFormatName,
			Lucene90StoredFieldsBestCompressionMode,
			lucene90StoredFieldsBestCompressionBlockLength,
			lucene90StoredFieldsBestCompressionMaxDocsPerChunk,
			lucene90StoredFieldsBlockShift,
		)
	default:
		panic(fmt.Sprintf("lucene90: invalid stored-fields mode %d", int(mode)))
	}
}

// init registers the Lucene90 stored-fields format factory so that
// codecs.Lucene104StoredFieldsFormat can read back indexes written with
// the Lucene90 wire format (the byte-compatible path required for
// Apache Lucene 10.4.0 interoperability).
func init() {
	codecs.RegisterLucene90StoredFieldsFormat(func() codecs.StoredFieldsFormat {
		return NewLucene90StoredFieldsFormat()
	})
}

// Compile-time guarantee that the format satisfies
// codecs.StoredFieldsFormat.
var _ codecs.StoredFieldsFormat = (*Lucene90StoredFieldsFormat)(nil)
