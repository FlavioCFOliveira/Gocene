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
// bridge added overhead, masked subtle signature drift, and inflated
// the build graph.
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

import "fmt"

// ScoreDoc represents a scored document.
type ScoreDoc struct {
	Doc        int
	Score      float32
	ShardIndex int
}

// NewScoreDoc creates a new ScoreDoc.
func NewScoreDoc(doc int, score float32, shardIndex int) *ScoreDoc {
	return &ScoreDoc{
		Doc:        doc,
		Score:      score,
		ShardIndex: shardIndex,
	}
}

// FieldDoc is a ScoreDoc which also contains information about how to sort the referenced document.
type FieldDoc struct {
	ScoreDoc
	Fields []any
}

// NewFieldDoc creates a FieldDoc with empty sort information.
func NewFieldDoc(doc int, score float32) *FieldDoc {
	return &FieldDoc{
		ScoreDoc: ScoreDoc{
			Doc:        doc,
			Score:      score,
			ShardIndex: -1,
		},
	}
}

// NewFieldDocWithFields creates a FieldDoc with the given sort information.
func NewFieldDocWithFields(doc int, score float32, fields []any) *FieldDoc {
	return &FieldDoc{
		ScoreDoc: ScoreDoc{
			Doc:        doc,
			Score:      score,
			ShardIndex: -1,
		},
		Fields: fields,
	}
}

// NewFieldDocWithShard creates a FieldDoc with the given sort information and shard index.
func NewFieldDocWithShard(doc int, score float32, fields []any, shardIndex int) *FieldDoc {
	return &FieldDoc{
		ScoreDoc: ScoreDoc{
			Doc:        doc,
			Score:      score,
			ShardIndex: shardIndex,
		},
		Fields: fields,
	}
}

// Equals checks for equality between two FieldDocs.
func (fd *FieldDoc) Equals(other *FieldDoc) bool {
	if fd == other {
		return true
	}
	if other == nil {
		return false
	}
	if fd.Doc != other.Doc || fd.Score != other.Score || fd.ShardIndex != other.ShardIndex {
		return false
	}
	if len(fd.Fields) != len(other.Fields) {
		return false
	}
	for i := range fd.Fields {
		if fd.Fields[i] != other.Fields[i] {
			return false
		}
	}
	return true
}

// HashCode computes a hash value for the FieldDoc.
func (fd *FieldDoc) HashCode() int {
	h := 17
	h = 31*h + fd.Doc
	return h
}

// String returns a string representation of the FieldDoc.
func (fd *FieldDoc) String() string {
	return fmt.Sprintf("doc=%d score=%f shardIndex=%d fields=%v",
		fd.Doc, fd.Score, fd.ShardIndex, fd.Fields)
}

// Relation indicates how the total hit count relates to the actual value.
type Relation int

const (
	// EQUAL_TO means the value is exact.
	EQUAL_TO Relation = iota
	// GREATER_THAN_OR_EQUAL_TO means the value is at least the given value.
	GREATER_THAN_OR_EQUAL_TO
)

// TotalHits represents the total number of hits.
type TotalHits struct {
	Value    int64
	Relation Relation
}

// NewTotalHits creates a new TotalHits.
func NewTotalHits(value int64, relation Relation) *TotalHits {
	return &TotalHits{
		Value:    value,
		Relation: relation,
	}
}

// IsExact returns true if the hit count is exact.
func (t *TotalHits) IsExact() bool {
	return t.Relation == EQUAL_TO
}

// TopDocs represents hits returned by IndexSearcher.Search.
type TopDocs struct {
	TotalHits *TotalHits
	ScoreDocs []*ScoreDoc
}

// NewTopDocs creates a new TopDocs.
func NewTopDocs(totalHits *TotalHits, scoreDocs []*ScoreDoc) *TopDocs {
	return &TopDocs{
		TotalHits: totalHits,
		ScoreDocs: scoreDocs,
	}
}
