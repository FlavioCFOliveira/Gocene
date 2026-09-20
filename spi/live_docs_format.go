// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import "github.com/FlavioCFOliveira/Gocene/util"

// LiveDocsFormat encodes and decodes the per-segment live/deleted
// documents bitset — the optional .liv file, which exists only while a
// segment carries deletions and is always maintained outside the
// compound-file container.
//
// Mirrors org.apache.lucene.codecs.LiveDocsFormat in Apache Lucene
// 10.5.0:
//
//	lucene/core/src/java/org/apache/lucene/codecs/LiveDocsFormat.java
//
// The Java abstract class declares exactly three members —
// readLiveDocs, writeLiveDocs and files — all keyed on a
// SegmentCommitInfo, because the deletion generation, the current
// delete count and the segment identity all live on the commit info
// rather than on the (immutable) SegmentInfo. The Go rendering keeps
// that keying: every method takes a *SegmentCommitInfo and derives the
// generation from it exactly as Lucene does.
//
// Name has no Java counterpart on LiveDocsFormat; it is the Gocene SPI
// convention shared by every other format interface in this package
// (NormsFormat, DocValuesFormat, PostingsFormat, …), where Java
// identifies a format through its SPI service name instead.
//
// Lifted onto the SPI so that index/ and codecs/ both reach a single
// canonical declaration site, exactly as the doc-values (rmp #4708),
// KNN-vectors (rmp #4707), points (rmp #4769) and norms (rmp #120)
// families were lifted before it. [Codec.LiveDocsFormat] is the sole
// entry point used by the index read and write paths.
type LiveDocsFormat interface {
	// Name returns the format name. Gocene SPI convention; Java relies on
	// the SPI service name of the enclosing codec instead.
	Name() string

	// ReadLiveDocs reads the live-docs bits for info from dir. The set
	// bits are the documents that are still live; the returned Bits has
	// length info.Info.MaxDoc().
	//
	// Mirrors LiveDocsFormat.readLiveDocs(Directory, SegmentCommitInfo,
	// IOContext).
	ReadLiveDocs(dir Directory, info *SegmentCommitInfo, ctx IOContext) (util.Bits, error)

	// WriteLiveDocs persists the live-docs bits for info into dir. The
	// generation of the file to write is taken from
	// info.GetNextWriteDelGen(), mirroring Java's
	// SegmentCommitInfo.getNextDelGen(). newDelCount is the number of
	// deletions added since the last write; implementations cross-check
	// that the bits contain exactly info.DelCount() + newDelCount
	// deletions and report a corruption error otherwise.
	//
	// Mirrors LiveDocsFormat.writeLiveDocs(Bits, Directory,
	// SegmentCommitInfo, int, IOContext).
	WriteLiveDocs(bits util.Bits, dir Directory, info *SegmentCommitInfo, newDelCount int, ctx IOContext) error

	// Files records every file this format keeps in use for info by
	// appending to the slice files points at. Java passes a mutable
	// Collection<String>; the Go rendering passes a pointer to the
	// caller's slice so the append is visible to the caller.
	//
	// Mirrors LiveDocsFormat.files(SegmentCommitInfo, Collection<String>).
	Files(info *SegmentCommitInfo, files *[]string) error
}
