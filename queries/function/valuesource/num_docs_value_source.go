// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// NumDocsValueSource returns the value of IndexReader.NumDocs() for every
// document. This is the number of documents excluding deletions.
//
// Mirrors org.apache.lucene.queries.function.valuesource.NumDocsValueSource.
type NumDocsValueSource struct {
	function.BaseValueSource
}

var _ function.ValueSource = (*NumDocsValueSource)(nil)

// Name mirrors name().
func (v *NumDocsValueSource) Name() string { return "numdocs" }

// Description mirrors description().
func (v *NumDocsValueSource) Description() string { return v.Name() + "()" }

// GetValues mirrors getValues(Map, LeafReaderContext). The searcher has no
// numDocs, so Lucene uses the top-level reader instead.
func (v *NumDocsValueSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	numDocs := index.ReaderUtilGetTopLevelContext(readerContext).Reader().NumDocs()
	return newConstIntDocValues(int32(numDocs), v), nil
}

// Equals mirrors equals(Object): this.getClass() == o.getClass().
func (v *NumDocsValueSource) Equals(other function.ValueSource) bool {
	_, ok := other.(*NumDocsValueSource)
	return ok
}

// HashCode mirrors hashCode(): this.getClass().hashCode().
func (v *NumDocsValueSource) HashCode() int32 { return hashString("NumDocsValueSource") }
