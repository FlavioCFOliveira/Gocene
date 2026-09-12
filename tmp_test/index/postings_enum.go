// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// PostingsEnum and its helper sentinels live canonically in the leaf
// schema/ package as of rmp #4669 / phase 1.3 (T4699). The aliases below
// preserve the historical index.* names for source-level callers.

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// PostingsEnum is an alias of spi.PostingsEnum.
type PostingsEnum = spi.PostingsEnum

// PostingsEnumBase is an alias of spi.PostingsEnumBase.
type PostingsEnumBase = spi.PostingsEnumBase

// EmptyPostingsEnum is an alias of spi.EmptyPostingsEnum.
type EmptyPostingsEnum = spi.EmptyPostingsEnum

// SingleDocPostingsEnum is an alias of spi.SingleDocPostingsEnum.
type SingleDocPostingsEnum = spi.SingleDocPostingsEnum

// SinglePostingsEnum is an alias of spi.SinglePostingsEnum.
type SinglePostingsEnum = spi.SinglePostingsEnum

// NewSingleDocPostingsEnum creates a new SingleDocPostingsEnum.
func NewSingleDocPostingsEnum(docID, freq int) *SingleDocPostingsEnum {
	return spi.NewSingleDocPostingsEnum(docID, freq)
}

// NewSinglePostingsEnum creates a new SinglePostingsEnum.
func NewSinglePostingsEnum(docFreq, freq int) *SinglePostingsEnum {
	return spi.NewSinglePostingsEnum(docFreq, freq)
}

// NewPostingsEnumBase builds a PostingsEnumBase positioned at initialDocID.
func NewPostingsEnumBase(initialDocID int) PostingsEnumBase {
	return spi.NewPostingsEnumBase(initialDocID)
}

// Postings sentinels and flag constants re-exported from spi.
const (
	NO_MORE_DOCS          = spi.NO_MORE_DOCS
	NO_MORE_POSITIONS     = spi.NO_MORE_POSITIONS
	PostingsFlagFreqs     = spi.PostingsFlagFreqs
	PostingsFlagPositions = spi.PostingsFlagPositions
	PostingsFlagOffsets   = spi.PostingsFlagOffsets
	PostingsFlagPayloads  = spi.PostingsFlagPayloads
	PostingsFlagAll       = spi.PostingsFlagAll
)
