// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"regexp"
	"strconv"
	"strings"
)

// Constants and helpers for Lucene index file naming. Mirrors
// org.apache.lucene.index.IndexFileNames from Apache Lucene 10.4.0.
//
// NOTE: extensions used by codecs are not listed here; callers must interact
// with the Codec directly to learn extensions.

const (
	// SegmentsPrefix is the name (prefix) of the index segments file.
	// Generation suffixes are appended via FileNameFromGeneration.
	SegmentsPrefix = "segments"

	// PendingSegmentsPrefix is the name of the pending index segments file.
	// Generation suffixes are appended via FileNameFromGeneration.
	PendingSegmentsPrefix = "pending_segments"

	// maxRadix is Character.MAX_RADIX from the JDK; used by Lucene when
	// encoding numeric generations into file names.
	maxRadix = 36
)

// CodecFilePattern matches all files created by codecs (checked in SegmentInfo).
// The pattern is the exact equivalent of Lucene's IndexFileNames.CODEC_FILE_PATTERN.
var CodecFilePattern = regexp.MustCompile(`^_[a-z0-9]+(_.*)?\..*$`)

// FileNameFromGeneration computes the full file name from base, extension and
// generation.
//
//   - gen == -1 => returns "".
//   - gen ==  0 => returns "<base>.<ext>" (or "<base>" if ext is empty).
//   - gen >   0 => returns "<base>_<gen_as_base36>.<ext>" (or no .<ext> if ext is empty).
//
// Mirrors org.apache.lucene.index.IndexFileNames.fileNameFromGeneration.
// (Java returns null for gen == -1; Go returns the empty string.)
func FileNameFromGeneration(base, ext string, gen int64) string {
	switch {
	case gen == -1:
		return ""
	case gen == 0:
		return SegmentFileName(base, "", ext)
	default:
		// gen > 0
		var sb strings.Builder
		sb.Grow(len(base) + 6 + len(ext))
		sb.WriteString(base)
		sb.WriteByte('_')
		sb.WriteString(strconv.FormatInt(gen, maxRadix))
		if len(ext) > 0 {
			sb.WriteByte('.')
			sb.WriteString(ext)
		}
		return sb.String()
	}
}

// SegmentFileName returns a file name that includes the given segment name,
// custom segmentSuffix and ext: "<segmentName>(_<suffix>)(.<ext>)".
//
// Mirrors org.apache.lucene.index.IndexFileNames.segmentFileName.
func SegmentFileName(segmentName, segmentSuffix, ext string) string {
	if len(ext) > 0 || len(segmentSuffix) > 0 {
		var sb strings.Builder
		sb.Grow(len(segmentName) + 2 + len(segmentSuffix) + len(ext))
		sb.WriteString(segmentName)
		if len(segmentSuffix) > 0 {
			sb.WriteByte('_')
			sb.WriteString(segmentSuffix)
		}
		if len(ext) > 0 {
			sb.WriteByte('.')
			sb.WriteString(ext)
		}
		return sb.String()
	}
	return segmentName
}

// MatchesExtension reports whether filename ends with ".<ext>".
// ext must be the *pure* extension, without a leading '.'.
func MatchesExtension(filename, ext string) bool {
	return strings.HasSuffix(filename, "."+ext)
}

// indexOfSegmentName locates the boundary of the segment name, or returns -1.
func indexOfSegmentName(filename string) int {
	// If it is a .del file, there's an '_' after the first character.
	if len(filename) < 2 {
		return -1
	}
	idx := strings.Index(filename[1:], "_")
	if idx != -1 {
		return idx + 1
	}
	// If it's not, strip everything that's before the '.'.
	return strings.Index(filename, ".")
}

// StripSegmentName strips the segment name out of the given file name.
// If the file name was produced by SegmentFileName or FileNameFromGeneration,
// this returns the substring starting at the first '.' or the second '_'
// (excluding both). Otherwise returns filename unchanged.
func StripSegmentName(filename string) string {
	idx := indexOfSegmentName(filename)
	if idx != -1 {
		return filename[idx:]
	}
	return filename
}

// ParseGeneration returns the generation from this file name, or 0 if there
// is no generation. Mirrors Lucene's parseGeneration (asserts startsWith "_").
func ParseGeneration(filename string) int64 {
	if !strings.HasPrefix(filename, "_") {
		return 0
	}
	parts := strings.Split(StripExtension(filename)[1:], "_")
	// 4 cases (per Lucene):
	//   segment.ext
	//   segment_gen.ext
	//   segment_codec_suffix.ext
	//   segment_gen_codec_suffix.ext
	if len(parts) == 2 || len(parts) == 4 {
		if g, err := strconv.ParseInt(parts[1], maxRadix, 64); err == nil {
			return g
		}
	}
	return 0
}

// ParseSegmentName parses the segment name out of the given file name.
// Returns filename unchanged if it does not contain '.' or '_'.
func ParseSegmentName(filename string) string {
	idx := indexOfSegmentName(filename)
	if idx != -1 {
		return filename[:idx]
	}
	return filename
}

// StripExtension removes the extension (anything after the first '.').
// If no '.' is present, returns filename unchanged.
func StripExtension(filename string) string {
	if idx := strings.Index(filename, "."); idx != -1 {
		return filename[:idx]
	}
	return filename
}

// GetExtension returns the extension (anything after the first '.'), or ""
// if there is no '.' in the file name. Note: Lucene returns null in Java; Go
// returns the empty string for the same condition.
func GetExtension(filename string) string {
	if idx := strings.Index(filename, "."); idx != -1 {
		return filename[idx+1:]
	}
	return ""
}
