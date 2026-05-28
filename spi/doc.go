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
//     FieldInfosFormat, SegmentInfoFormat, SegmentInfosFormat,
//     TermVectorsFormat (+ TermVectorsReader/Writer), CompoundFormat
//     (+ CompoundDirectory), KnnVectorsFormat (+ KnnVectorsWriter /
//     KnnVectorsReader / KnnFieldVectorsWriter),
//     DocValuesFormat (+ DocValuesProducer / DocValuesConsumer / the
//     six iterator-shaped value types — NumericDocValues,
//     BinaryDocValues, SortedDocValues, SortedSetDocValues,
//     SortedNumericDocValues, DocValuesSkipper — and the five
//     writer-side iterators consumed by Add*Field).
//   - SegmentInfos and SegmentCommitInfo (lifted by rmp #4706 so the
//     segments_N read/write path no longer needs to import index/).
//   - SegmentReadState and SegmentWriteState.
//   - SorterDocMap: the Sorter.DocMap surface the wide
//     KnnVectorsWriter.Flush signature requires, lifted alongside the
//     KnnVectorsFormat move by rmp #4707.
//   - IndexableField: a narrow, codec-facing subset of the document-
//     side IndexableField that the stored-fields write path consumes.
//   - BufferedUpdatesRef: a marker interface used by SegmentWriteState
//     to hold pending term deletions without dragging index/'s
//     BufferedUpdates type into the SPI surface.
//   - IndexNotFoundException: raised by ReadSegmentInfos when no
//     segments_N file is found in the directory.
//   - Codec envelope helpers: CodecMagic, FooterMagic, WriteIndexHeader,
//     CheckIndexHeader, WriteFooter, CheckFooter — lifted alongside the
//     SegmentInfos move so they can be reused by future codec ports.
//
// # Background
//
// This package is part of the SPI unification work tracked under rmp
// #4669. Sprint 117 phase 1 lifted the structural types
// (SegmentInfo, FieldInfo*, Term*, vector enums, …) into schema/.
// Sprint 118 phase 2 (rmp #4693) lifted the codec-facing interfaces,
// rmp #4706 completed the SegmentInfos / SegmentInfosFormat lift,
// rmp #4707 closed the KnnVectorsFormat lift (rewriting the narrow
// vector_values_consumer path onto the wide writer in the process),
// rmp #4708 closed the DocValuesFormat family lift, rmp #4709 added
// the iterator-shaped methods to the index-side value-type interfaces
// and implementations, and rmp #4710 completed the structural collapse
// by turning every index.X doc-values identifier into a Go type alias
// of its spi/ counterpart and removing the legacy random-access
// Get(docID) / GetOrd(docID) projection from every production
// implementation.
package spi
