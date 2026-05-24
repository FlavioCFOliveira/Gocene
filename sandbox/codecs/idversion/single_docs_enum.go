// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.codecs.idversion.SingleDocsEnum and
// org.apache.lucene.sandbox.codecs.idversion.SinglePostingsEnum.
package idversion

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// SingleDocsEnum is a PostingsEnum that returns a single document.
// It supports reuse via Reset.
//
// Mirrors org.apache.lucene.sandbox.codecs.idversion.SingleDocsEnum.
type SingleDocsEnum struct {
	doc         int
	singleDocID int
}

// Reset reinitialises the enumerator for a new singleDocID.
func (e *SingleDocsEnum) Reset(singleDocID int) {
	e.doc = -1
	e.singleDocID = singleDocID
}

// DocID returns the current document ID.
func (e *SingleDocsEnum) DocID() int { return e.doc }

// NextDoc advances to the next document.
func (e *SingleDocsEnum) NextDoc() (int, error) {
	if e.doc == -1 {
		e.doc = e.singleDocID
	} else {
		e.doc = index.NO_MORE_DOCS
	}
	return e.doc, nil
}

// Advance advances to the first document at or after target.
func (e *SingleDocsEnum) Advance(target int) (int, error) {
	if e.doc == -1 && target <= e.singleDocID {
		e.doc = e.singleDocID
	} else {
		e.doc = index.NO_MORE_DOCS
	}
	return e.doc, nil
}

// Cost returns 1.
func (e *SingleDocsEnum) Cost() int64 { return 1 }

// Freq returns 1.
func (e *SingleDocsEnum) Freq() (int, error) { return 1, nil }

// NextPosition returns -1 (no positions).
func (e *SingleDocsEnum) NextPosition() (int, error) { return -1, nil }

// StartOffset returns -1.
func (e *SingleDocsEnum) StartOffset() (int, error) { return -1, nil }

// EndOffset returns -1.
func (e *SingleDocsEnum) EndOffset() (int, error) { return -1, nil }

// GetPayload returns an error (not supported in docs-only enum).
func (e *SingleDocsEnum) GetPayload() ([]byte, error) {
	return nil, errors.New("SingleDocsEnum: GetPayload not supported")
}

var _ index.PostingsEnum = (*SingleDocsEnum)(nil)

// SinglePostingsEnum is a PostingsEnum that returns a single document at a
// single position, with an 8-byte version payload.
// It supports reuse via Reset.
//
// Mirrors org.apache.lucene.sandbox.codecs.idversion.SinglePostingsEnum.
type SinglePostingsEnum struct {
	doc         int
	pos         int
	singleDocID int
	version     int64
	payload     [8]byte
}

// Reset reinitialises the enumerator for a new singleDocID and version.
func (e *SinglePostingsEnum) Reset(singleDocID int, version int64) {
	e.doc = -1
	e.singleDocID = singleDocID
	e.version = version
}

// DocID returns the current document ID.
func (e *SinglePostingsEnum) DocID() int { return e.doc }

// NextDoc advances to the next document.
func (e *SinglePostingsEnum) NextDoc() (int, error) {
	if e.doc == -1 {
		e.doc = e.singleDocID
	} else {
		e.doc = index.NO_MORE_DOCS
	}
	e.pos = -1
	return e.doc, nil
}

// Advance advances to the first document at or after target.
func (e *SinglePostingsEnum) Advance(target int) (int, error) {
	if e.doc == -1 && target <= e.singleDocID {
		e.doc = e.singleDocID
		e.pos = -1
	} else {
		e.doc = index.NO_MORE_DOCS
	}
	return e.doc, nil
}

// Cost returns 1.
func (e *SinglePostingsEnum) Cost() int64 { return 1 }

// Freq returns 1.
func (e *SinglePostingsEnum) Freq() (int, error) { return 1, nil }

// NextPosition advances to position 0 and encodes the version into the payload.
func (e *SinglePostingsEnum) NextPosition() (int, error) {
	e.pos = 0
	LongToBytes(e.version, e.payload[:])
	return e.pos, nil
}

// GetPayload returns the 8-byte big-endian version payload.
// Only valid after NextPosition has been called.
func (e *SinglePostingsEnum) GetPayload() ([]byte, error) {
	return e.payload[:], nil
}

// StartOffset returns -1.
func (e *SinglePostingsEnum) StartOffset() (int, error) { return -1, nil }

// EndOffset returns -1.
func (e *SinglePostingsEnum) EndOffset() (int, error) { return -1, nil }

var _ index.PostingsEnum = (*SinglePostingsEnum)(nil)
