// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ext

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// extensionStub is a minimal ParserExtension implementation used by ext
// package tests.  It builds a TermQuery from the ExtensionQuery's field and
// raw query string.
//
// Port of: queryparser/src/test/.../ext/ExtensionStub.java
type extensionStub struct{}

// Parse implements ParserExtension.  Returns a TermQuery for the given field
// and raw query string.
func (e *extensionStub) Parse(query *ExtensionQuery) (search.Query, error) {
	return search.NewTermQuery(index.NewTerm(query.GetField(), query.GetRawQueryString())), nil
}
