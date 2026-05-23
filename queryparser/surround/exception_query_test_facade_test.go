// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package surround

import "strings"

// exceptionQueryTestFacade tries to parse a query and records it in a failure
// buffer when the parse does NOT produce an error (i.e. when it should have
// failed but did not).
//
// This is the Go equivalent of the Java test class
// org.apache.lucene.queryparser.surround.query.ExceptionQueryTestFacade.
type exceptionQueryTestFacade struct {
	queryText string
	verbose   bool
}

// newExceptionQueryTestFacade builds an exception-test facade.
func newExceptionQueryTestFacade(queryText string, verbose bool) *exceptionQueryTestFacade {
	return &exceptionQueryTestFacade{queryText: queryText, verbose: verbose}
}

// doTest tries to parse the query. If parsing succeeds (no error), the query
// text is appended to failQueries so the caller can report it.
func (f *exceptionQueryTestFacade) doTest(failQueries *strings.Builder) {
	p := NewQueryParser("field")
	lq, err := p.Parse(f.queryText)
	if err != nil {
		// expected: parse correctly rejected the query
		return
	}
	// unexpected success — record it
	failQueries.WriteString(f.queryText)
	failQueries.WriteString("\nParsed as: ")
	failQueries.WriteString(lqString(lq))
	failQueries.WriteString("\n")
}

// lqString returns a best-effort string representation of a SrndQuery.
func lqString(q SrndQuery) string {
	if q == nil {
		return "<nil>"
	}
	type stringer interface{ String() string }
	if s, ok := q.(stringer); ok {
		return s.String()
	}
	return "<query>"
}

// getFailQueries runs each query through an exception facade and returns the
// concatenated failure report. An empty return means all queries correctly
// produced parse errors.
func getFailQueries(exceptionQueries []string, verbose bool) string {
	var buf strings.Builder
	for _, q := range exceptionQueries {
		newExceptionQueryTestFacade(q, verbose).doTest(&buf)
	}
	return buf.String()
}
