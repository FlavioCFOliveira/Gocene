// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package surround

import "testing"

// TestBoolean02_ParsesOnly verifies that the surround parser correctly parses
// the boolean queries from Java's Test02Boolean without running them against
// an index (index-round-trip tests are deferred until IndexWriter is available).
//
// Port of org.apache.lucene.queryparser.surround.query.Test02Boolean.
func TestBoolean02_ParsesOnly(t *testing.T) {
	fieldName := "bi"
	maxBasicQueries := 16
	qf := NewBasicQueryFactoryWithLimit(maxBasicQueries)

	queries := []string{
		"word1",
		"word*",
		"ord2",
		"kxork*",
		"wor*",
		"word1 AND word2",
		"word1 OR word2",
		"word1 NOT word2",
		"word1 AND word2 AND word3",
		"word1 AND (word2 OR word3)",
		"word2 OR word3",
	}

	for _, q := range queries {
		facade := newBooleanQueryTestFacade(q, nil, fieldName, qf)
		_, err := facade.parseOnly()
		if err != nil {
			t.Errorf("unexpected parse error for %q: %v", q, err)
		}
	}
}

// TestBoolean02_InvalidQueriesError verifies that queries that should fail do fail.
func TestBoolean02_InvalidQueriesError(t *testing.T) {
	invalid := []string{
		"",
		"word1 word2", // two bare terms without operator (not valid in surround)
		"AND",
		"OR",
	}

	for _, q := range invalid {
		p := NewQueryParser("bi")
		_, err := p.Parse(q)
		if err == nil {
			t.Errorf("expected error for %q but got none", q)
		}
	}
}
