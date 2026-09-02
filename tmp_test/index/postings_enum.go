// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// PostingsEnum and its helper sentinels live canonically in the leaf
// schema/ package as of rmp #4669 / phase 1.3 (T4699). The aliases below
// preserve the historical index.* names for source-level callers.

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
)

// PostingsEnum is an alias of schema.PostingsEnum.
type PostingsEnum = schema.PostingsEnum

// PostingsEnumBase is an alias of schema.PostingsEnumBase.
type PostingsEnumBase = schema.PostingsEnumBase

// EmptyPostingsEnum is an alias of schema.EmptyPostingsEnum.
type EmptyPostingsEnum = schema.EmptyPostingsEnum

// SingleDocPostingsEnum is an alias of schema.SingleDocPostingsEnum.
type SingleDocPostingsEnum = schema.SingleDocPostingsEnum

// SinglePostingsEnum is an alias of schema.SinglePostingsEnum.
type SinglePostingsEnum = schema.SinglePostingsEnum

// NewSingleDocPostingsEnum creates a new SingleDocPostingsEnum.
func NewSingleDocPostingsEnum(docID, freq int) *SingleDocPostingsEnum {
	return schema.NewSingleDocPostingsEnum(docID, freq)
}

// NewSinglePostingsEnum creates a new SinglePostingsEnum.
func NewSinglePostingsEnum(docFreq, freq int) *SinglePostingsEnum {
	return schema.NewSinglePostingsEnum(docFreq, freq)
}

// NewPostingsEnumBase builds a PostingsEnumBase positioned at initialDocID.
func NewPostingsEnumBase(initialDocID int) PostingsEnumBase {
	return schema.NewPostingsEnumBase(initialDocID)
}

// Postings sentinels and flag constants re-exported from schema.
const (
	NO_MORE_DOCS          = schema.NO_MORE_DOCS
	NO_MORE_POSITIONS     = schema.NO_MORE_POSITIONS
	PostingsFlagFreqs     = schema.PostingsFlagFreqs
	PostingsFlagPositions = schema.PostingsFlagPositions
	PostingsFlagOffsets   = schema.PostingsFlagOffsets
	PostingsFlagPayloads  = schema.PostingsFlagPayloads
	PostingsFlagAll       = schema.PostingsFlagAll
)
