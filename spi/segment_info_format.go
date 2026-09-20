// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi


// SegmentInfoFormat encodes and decodes the per-segment .si metadata
// file.
//
// Mirrors org.apache.lucene.codecs.SegmentInfoFormat in Apache Lucene
// 10.4.0.
//
// Note: this is the *singular* SegmentInfoFormat (one .si per segment).
// The plural SegmentInfosFormat (segments_N) is intentionally not yet
// part of the SPI — see rmp #4706.
type SegmentInfoFormat interface {
	// Write serialises a single segment's metadata into a .si file.
	Write(dir Directory, info *SegmentInfo, context IOContext) error

	// Read deserialises a single segment's .si file. segmentID is the
	// 16-byte identifier from segments_N and is cross-checked against
	// the .si header.
	Read(dir Directory, segmentName string, segmentID []byte, context IOContext) (*SegmentInfo, error)
}
