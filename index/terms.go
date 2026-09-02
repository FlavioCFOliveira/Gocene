// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// Terms and its helpers live canonically in the leaf schema/ package as
// of rmp #4669 / phase 1.3 (T4699). This file re-exports the names that
// historically lived under index.* for source-level back-compat.

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
)

// Terms is an alias of schema.Terms.
type Terms = schema.Terms

// TermsBase is an alias of schema.TermsBase.
type TermsBase = schema.TermsBase

// TermsStats is an alias of schema.TermsStats.
type TermsStats = schema.TermsStats

// EmptyTerms is an alias of schema.EmptyTerms.
type EmptyTerms = schema.EmptyTerms

// SingleTermTerms is an alias of schema.SingleTermTerms.
type SingleTermTerms = schema.SingleTermTerms

// NewSingleTermTerms creates a new SingleTermTerms.
func NewSingleTermTerms(term *Term, docFreq int, totalFreq int64) *SingleTermTerms {
	return schema.NewSingleTermTerms(term, docFreq, totalFreq)
}
