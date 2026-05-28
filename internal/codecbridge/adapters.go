// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecbridge

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// After the SPI unification (rmp #4693) every codec-facing interface
// — PostingsFormat, StoredFieldsFormat (and its companion
// Reader/Writer/Visitor types), FieldInfosFormat, SegmentInfoFormat,
// TermVectorsFormat (and its companion Reader/Writer), CompoundFormat,
// SegmentReadState/SegmentWriteState, IndexableField — became a Go
// type alias to the canonical declaration in package spi. The per-
// format adapters that this file used to host collapsed to identity
// functions and have been deleted.
//
// The only adapter that remains is SegmentInfosFormat: the codecs side
// uses the Lucene-faithful 2-arg signature (no IOContext) while the
// index side still carries the 3-arg signature (Lucene-divergent,
// IOContext-bearing). Reconciling that surface is tracked under
// rmp #4706, after which this entire file can be deleted alongside
// the package itself.

// segmentInfosFormatAdapter adapts the codecs.SegmentInfosFormat
// (2-arg Lucene-faithful) into the index.SegmentInfosFormat (3-arg,
// IOContext-bearing). The IOContext arguments are dropped; the
// codecs-side implementation opens the segments_N file with the
// appropriate IOContext internally.
type segmentInfosFormatAdapter struct {
	inner codecs.SegmentInfosFormat
}

var _ index.SegmentInfosFormat = (*segmentInfosFormatAdapter)(nil)

func (a *segmentInfosFormatAdapter) Name() string {
	return a.inner.Name()
}

func (a *segmentInfosFormatAdapter) Read(dir store.Directory, _ store.IOContext) (*index.SegmentInfos, error) {
	return a.inner.Read(dir)
}

func (a *segmentInfosFormatAdapter) Write(dir store.Directory, segmentInfos *index.SegmentInfos, _ store.IOContext) error {
	return a.inner.Write(dir, segmentInfos)
}
