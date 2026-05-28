// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentInfosFormat handles encoding/decoding of segment metadata (segments_N).
//
// Mirrors the role of org.apache.lucene.index.SegmentInfos read/commit in
// Apache Lucene 10.4.0: the JVM reference does not expose a public
// SegmentInfosFormat type (segments_N is handled by static methods on
// SegmentInfos itself), so Gocene's split into an interface is a deliberate
// divergence that lets a Codec swap segments_N encoding without rewriting
// callers.
//
// Lifted to the SPI by rmp #4706 alongside *SegmentInfos and
// *SegmentCommitInfo so that callers no longer need adapter shims between
// the index- and codecs-facing surfaces.
type SegmentInfosFormat interface {
	// Name returns the format name (e.g. "Lucene104SegmentInfosFormat").
	Name() string

	// Read reads the most recent segments_N file from dir and returns the
	// reconstructed SegmentInfos. The IOContext follows the Lucene 10.4.0
	// IndexInput contract for picking the appropriate read strategy.
	Read(dir store.Directory, ctx store.IOContext) (*SegmentInfos, error)

	// Write serialises the given SegmentInfos to a fresh segments_N file in
	// dir, honouring infos.Generation() as the generation suffix.
	Write(dir store.Directory, infos *SegmentInfos, ctx store.IOContext) error
}
