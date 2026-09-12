// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"regexp"
	"strconv"
	"strings"
)

// This file is the Go port of Lucene's
// org.apache.lucene.index.IndexFileNames.
//
// It contains the constants representing the index control-file names and the
// convenience methods for querying whether a file name matches an extension
// (MatchesExtension) and for generating file names from a segment name,
// generation and extension (FileNameFromGeneration, SegmentFileName).
//
// NOTE: extensions used by codecs are not listed here; the Codec must be
// queried directly.

const (
	// SegmentsPrefix is the base name of the index commit file
	// (Lucene's IndexFileNames.SEGMENTS). Commit files are named
	// SegmentsPrefix + "_" + generation in base 36, e.g. "segments_1".
	SegmentsPrefix = "segments"

	// PendingSegmentsPrefix is the base name of the pending-index-commit
	// file (Lucene's IndexFileNames.PENDING_SEGMENTS). Pending files are
	// named PendingSegmentsPrefix + "_" + generation, e.g.
	// "pending_segments_1".
	PendingSegmentsPrefix = "pending_segments"

	// maxRadix is the radix (base 36) used for all file generations and
	// segment names, mirroring Lucene's Character.MAX_RADIX.
	maxRadix = 36
)

// CodecFilePattern matches all files created by codecs (checked by
// SegmentInfo when it records its files). Mirrors Lucene's
// IndexFileNames.CODEC_FILE_PATTERN.
var CodecFilePattern = regexp.MustCompile(`_[a-z0-9]+(_.*)?\..*`)

// FileNameFromGeneration computes the full file name from base, extension and
// generation. If the generation is -1, the file name is the empty string. If
// it is 0, the file name is <base>.<ext>. If it is > 0, the file name is
// <base>_<gen>.<ext>, with the generation rendered in base 36.
//
// NOTE: .<ext> is added to the name only if ext is not an empty string.
func FileNameFromGeneration(base, ext string, gen int64) string {
	if gen == -1 {
		return ""
	} else if gen == 0 {
		return SegmentFileName(base, "", ext)
	} else {
		// Mirrors Long.toString(gen, Character.MAX_RADIX): base-36, lowercase.
		return SegmentFileName(base, strconv.FormatInt(gen, maxRadix), ext)
	}
}

// SegmentFileName returns a file name that includes the given segment name,
// a custom segment suffix and an extension. The format of the file name is:
// <segmentName>(_<suffix>)(.<ext>).
//
// NOTE: .<ext> is added to the result file name only if ext is not empty, and
// _<suffix> only if it is not empty.
func SegmentFileName(segmentName, segmentSuffix, ext string) string {
	if len(ext) > 0 || len(segmentSuffix) > 0 {
		var sb strings.Builder
		sb.WriteString(segmentName)
		if len(segmentSuffix) > 0 {
			sb.WriteString("_")
			sb.WriteString(segmentSuffix)
		}
		if len(ext) > 0 {
			sb.WriteString(".")
			sb.WriteString(ext)
		}
		return sb.String()
	}
	return segmentName
}

// MatchesExtension returns true if the given file name ends with the given
// extension. A pure extension must be provided, without the leading '.'.
func MatchesExtension(filename, ext string) bool {
	return strings.HasSuffix(filename, "."+ext)
}

// indexOfSegmentName locates the boundary of the segment name, or -1.
func indexOfSegmentName(filename string) int {
	// If it is a .del file, there is an '_' after the first character.
	idx := strings.IndexByte(filename[1:], '_')
	if idx != -1 {
		return idx + 1
	}
	// If it is not, strip everything that is before the '.'.
	return strings.IndexByte(filename, '.')
}

// StripSegmentName strips the segment name out of the given file name. If
// SegmentFileName or FileNameFromGeneration was used to create the file, this
// method simply removes whatever comes before the second '_' (or the first
// '.'), excluding both.
//
// It returns the file name with the segment name removed, or the given file
// name if it does not contain a '.' or '_'.
func StripSegmentName(filename string) string {
	idx := indexOfSegmentName(filename)
	if idx != -1 {
		filename = filename[idx:]
	}
	return filename
}

// ParseGeneration returns the generation from this file name, or 0 if there is
// no generation. The generation is base 36.
func ParseGeneration(filename string) int64 {
	parts := javaSplit(strings.TrimPrefix(StripExtension(filename), "_"), "_")
	// 4 cases:
	// segment.ext
	// segment_gen.ext
	// segment_codec_suffix.ext
	// segment_gen_codec_suffix.ext
	if len(parts) == 2 || len(parts) == 4 {
		g, err := strconv.ParseInt(parts[1], maxRadix, 64)
		if err == nil {
			return g
		}
	}
	return 0
}

// ParseSegmentName parses the segment name out of the given file name.
//
// It returns the segment name only, or the file name if it does not contain a
// '.' or '_'.
func ParseSegmentName(filename string) string {
	idx := indexOfSegmentName(filename)
	if idx != -1 {
		filename = filename[:idx]
	}
	return filename
}

// StripExtension removes the extension (anything after the first '.'),
// otherwise it returns the original file name.
func StripExtension(filename string) string {
	idx := strings.IndexByte(filename, '.')
	if idx != -1 {
		filename = filename[:idx]
	}
	return filename
}

// GetExtension returns the extension (anything after the first '.'), or the
// empty string if there is no '.' in the file name.
func GetExtension(filename string) string {
	idx := strings.IndexByte(filename, '.')
	if idx == -1 {
		return ""
	}
	return filename[idx+1:]
}

// javaSplit mirrors java.lang.String.split(sep) for the single non-regex
// separator used by ParseGeneration: consecutive separators produce empty
// entries and trailing empty entries are discarded.
func javaSplit(s, sep string) []string {
	if s == "" {
		return []string{""}
	}
	parts := strings.Split(s, sep)
	// Drop trailing empty entries, like the Java implementation.
	for len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}
