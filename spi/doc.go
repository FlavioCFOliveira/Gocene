// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package spi declares the canonical service-provider interfaces (SPIs)
// shared by the index/ and codecs/ packages.
//
// # Why this package exists
//
// Before SPI unification, the index/ and codecs/ packages each defined
// their own copy of every codec-facing interface (Codec, PostingsFormat,
// StoredFieldsFormat, FieldInfosFormat, SegmentInfoFormat,
// TermVectorsFormat, CompoundFormat, …) plus the SegmentReadState /
// SegmentWriteState structs that travel through them. The duplication
// existed because codecs/ imports index/ for concrete types
// (*SegmentInfo, *FieldInfos, *Term, …) and index/ therefore could not
// import codecs/ in return without creating a cycle.
//
// The two copies were structurally similar but not identical, so a
// dedicated bridge package (internal/codecbridge) had to translate
// between them whenever index/ called into a codec implementation. The
// bridge added overhead, masked subtle signature drift, and inflated the
// build graph.
//
// spi/ resolves the asymmetry by lifting the shared interfaces and
// state structs into a leaf package that depends only on schema/,
// store/, and util/. Both index/ and codecs/ then re-export the SPI
// types via Go type aliases, making the duplicated identifiers
// indistinguishable at the type-system level. Code that historically
// reached for index.Codec or codecs.Codec continues to compile against
// the same underlying interface.
//
// # What lives here
//
//   - Codec, PostingsFormat (+ FieldsConsumer/FieldsProducer),
//     StoredFieldsFormat (+ StoredFieldsReader/Writer/FieldVisitor),
//     FieldInfosFormat, SegmentInfoFormat, TermVectorsFormat
//     (+ TermVectorsReader/Writer), CompoundFormat (+ CompoundDirectory).
//   - SegmentReadState and SegmentWriteState.
//   - IndexableField: a narrow, codec-facing subset of the document-
//     side IndexableField that the stored-fields write path consumes.
//   - BufferedUpdatesRef: a marker interface used by SegmentWriteState
//     to hold pending term deletions without dragging index/'s
//     BufferedUpdates type into the SPI surface.
//
// # What is intentionally NOT here
//
//   - SegmentInfosFormat: deferred to rmp #4706 because its codecs-side
//     signature (no IOContext on Read/Write) diverges from the index-side
//     signature, and unifying it requires changing several call sites
//     beyond the SPI surface.
//   - KnnVectorsFormat: deferred to rmp #4707 because the codecs-side
//     interface name collides with KnnVectorsFormatFactory and the index
//     side exposes a Factory abstraction that needs reconciliation.
//   - DocValuesFormat (and its companion producer/consumer/iterator
//     types): deferred to rmp #4708 because the codecs-side family pulls
//     in a large web of value-type and iterator interfaces that live
//     only in index/ today.
//
// Each deferral is marked with a TODO(T46XX) comment at its source-of-
// truth declaration in codecs/ and index/.
//
// # Background
//
// This package is part of the SPI unification work tracked under rmp
// #4669. Sprint 117 phase 1 lifted the structural types
// (SegmentInfo, FieldInfo*, Term*, vector enums, …) into schema/.
// Sprint 118 phase 2 (rmp #4693) — this package — completes the
// interface lift.
package spi
