// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package surround

// booleanQueryTestFacade is a test helper that parses a surround query and
// (when an IndexSearcher is available) executes it, verifying that the result
// set matches the expected document numbers.
//
// This is the Go equivalent of the Java test class
// org.apache.lucene.queryparser.surround.query.BooleanQueryTestFacade.
//
// Full index-round-trip behaviour is deferred until Gocene's IndexWriter /
// DirectoryReader stack is available. The facade is defined here so that other
// surround test files can reference the type.
type booleanQueryTestFacade struct {
	queryText      string
	expectedDocNrs []int
	fieldName      string
	qf             *BasicQueryFactory
	verbose        bool
}

// newBooleanQueryTestFacade builds a test facade.
func newBooleanQueryTestFacade(
	queryText string,
	expectedDocNrs []int,
	fieldName string,
	qf *BasicQueryFactory,
) *booleanQueryTestFacade {
	return &booleanQueryTestFacade{
		queryText:      queryText,
		expectedDocNrs: expectedDocNrs,
		fieldName:      fieldName,
		qf:             qf,
		verbose:        false,
	}
}

// setVerbose enables verbose output (mirrors Java setVerbose).
func (f *booleanQueryTestFacade) setVerbose(v bool) { f.verbose = v }

// parseOnly parses the query and returns any parse error. This can be used by
// tests that only want to verify that the query is syntactically valid.
func (f *booleanQueryTestFacade) parseOnly() (SrndQuery, error) {
	p := NewQueryParser(f.fieldName)
	return p.Parse(f.queryText)
}
