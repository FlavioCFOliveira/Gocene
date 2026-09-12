// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// Terms and its helpers live canonically in the leaf schema/ package as
// of rmp #4669 / phase 1.3 (T4699). This file re-exports the names that
// historically lived under index.* for source-level back-compat.

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// Terms is an alias of spi.Terms.
type Terms = spi.Terms

// TermsBase is an alias of spi.TermsBase.
type TermsBase = spi.TermsBase

// TermsStats is an alias of spi.TermsStats.
type TermsStats = spi.TermsStats

// EmptyTerms is an alias of spi.EmptyTerms.
type EmptyTerms = spi.EmptyTerms

// SingleTermTerms is an alias of spi.SingleTermTerms.
type SingleTermTerms = spi.SingleTermTerms

// NewSingleTermTerms creates a new SingleTermTerms.
func NewSingleTermTerms(term *Term, docFreq int, totalFreq int64) *SingleTermTerms {
	return spi.NewSingleTermTerms(term, docFreq, totalFreq)
}
