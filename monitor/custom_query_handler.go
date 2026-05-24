// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// CustomQueryHandler builds a QueryTree for a query that needs custom treatment.
//
// The default query analyzers use the QueryVisitor API to extract terms from
// queries.  If different handling is needed, implement a CustomQueryHandler and
// pass it to the Presearcher.
//
// Port of org.apache.lucene.monitor.CustomQueryHandler.
type CustomQueryHandler interface {
	// HandleQuery builds a QueryTree node from the given query.
	HandleQuery(query search.Query, weightor TermWeightor) QueryTree

	// WrapTermStream adds additional processing to the TokenStream over a
	// document's terms index.  The default implementation returns in unchanged.
	WrapTermStream(field string, in analysis.TokenStream) analysis.TokenStream
}

// DefaultWrapTermStream is a helper that returns the stream unchanged.
// Embed it or use it to satisfy the WrapTermStream method with default behaviour.
func DefaultWrapTermStream(_ string, in analysis.TokenStream) analysis.TokenStream { return in }
