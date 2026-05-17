// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// SortFieldProvider abstracts the codec-facing read/write side of a
// SortField. Mirrors org.apache.lucene.index.SortFieldProvider from Apache
// Lucene 10.4.0.
//
// Gocene skeleton: this interface keeps the contract surface (Name + the
// codec-facing read/write hooks) so that callers can declare SortFieldProvider
// parameters today. Concrete impls (built-in providers, SPI registry) land
// when the search/Sort package is ported.
type SortFieldProvider interface {
	// Name returns the canonical name of the provider. The name is what is
	// persisted on disk to dispatch back to the correct provider during read.
	Name() string

	// ReadSortField reconstructs a SortField from the codec-supplied input.
	// SortField itself lives in the search package; the interface returns
	// interface{} until that sprint runs.
	ReadSortField(in interface{}) (interface{}, error)

	// WriteSortField writes a SortField to the codec-supplied output.
	WriteSortField(sortField interface{}, out interface{}) error
}
