// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// TermsEnum and the canonical empty/single-term implementations live in
// the leaf schema/ package as of rmp #4669 / phase 1.3 (T4699). This
// file aliases the historical index.* names to the schema-canonical
// declarations.

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// TermsEnum is an alias of spi.TermsEnum.
type TermsEnum = spi.TermsEnum

// TermsEnumBase is an alias of spi.TermsEnumBase.
type TermsEnumBase = spi.TermsEnumBase

// SeekStatus is an alias of spi.SeekStatus, the port of the nested enum
// org.apache.lucene.index.TermsEnum.SeekStatus, which lives beside the
// TermsEnum interface in spi.
type SeekStatus = spi.SeekStatus

const (
	// SeekStatusFound is an alias of spi.SeekStatusFound.
	SeekStatusFound = spi.SeekStatusFound
	// SeekStatusNotFound is an alias of spi.SeekStatusNotFound.
	SeekStatusNotFound = spi.SeekStatusNotFound
	// SeekStatusEnd is an alias of spi.SeekStatusEnd.
	SeekStatusEnd = spi.SeekStatusEnd
)

// EmptyTermsEnum is an alias of spi.EmptyTermsEnum.
type EmptyTermsEnum = spi.EmptyTermsEnum

// SingleTermsEnum is an alias of spi.SingleTermsEnum.
type SingleTermsEnum = spi.SingleTermsEnum

// NewSingleTermsEnum creates a new SingleTermsEnum.
func NewSingleTermsEnum(term *Term, docFreq int, totalFreq int64) *SingleTermsEnum {
	return spi.NewSingleTermsEnum(term, docFreq, totalFreq)
}
