// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// TermsEnum and the canonical empty/single-term implementations live in
// the leaf schema/ package as of rmp #4669 / phase 1.3 (T4699). This
// file aliases the historical index.* names to the schema-canonical
// declarations.

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
)

// TermsEnum is an alias of schema.TermsEnum.
type TermsEnum = schema.TermsEnum

// TermsEnumBase is an alias of schema.TermsEnumBase.
type TermsEnumBase = schema.TermsEnumBase

// EmptyTermsEnum is an alias of schema.EmptyTermsEnum.
type EmptyTermsEnum = schema.EmptyTermsEnum

// SingleTermsEnum is an alias of schema.SingleTermsEnum.
type SingleTermsEnum = schema.SingleTermsEnum

// NewSingleTermsEnum creates a new SingleTermsEnum.
func NewSingleTermsEnum(term *Term, docFreq int, totalFreq int64) *SingleTermsEnum {
	return schema.NewSingleTermsEnum(term, docFreq, totalFreq)
}
