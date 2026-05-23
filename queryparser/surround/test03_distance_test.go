// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package surround

import "testing"

// TestDistance03_ExceptionQueries verifies that distance queries that mix
// unsupported constructs with proximity operators produce parse errors.
//
// Port of Test03Distance.test00Exceptions.
func TestDistance03_ExceptionQueries(t *testing.T) {
	exceptionQueries := []string{
		"(aa and bb) w cc",
		"(aa or bb) w (cc and dd)",
		"(aa opt bb) w cc",
		"(aa not bb) w cc",
		"(aa or bb) w (bi:cc)",
		"(aa or bb) w bi:cc",
		"(aa or bi:bb) w cc",
		"(aa or (bi:bb)) w cc",
		"(aa or (bb and dd)) w cc",
	}

	m := getFailQueries(exceptionQueries, false)
	if len(m) > 0 {
		t.Errorf("expected parse errors for:\n%s", m)
	}
}

// TestDistance03_ParsesOnly verifies that valid distance query expressions
// parse without error.
//
// Port of Test03Distance distance query tests (parse-only; index execution
// deferred until IndexWriter is available).
func TestDistance03_ParsesOnly(t *testing.T) {
	fieldName := "bi"
	maxBasicQueries := 16
	qf := NewBasicQueryFactoryWithLimit(maxBasicQueries)

	// The surround grammar requires a numeric prefix on W/N operators (1W, 2N, etc.).
	// Bare "w"/"n" are tokenised as terms, not operators.
	queries := []string{
		"word1 1W word2",
		"word1 1N word2",
		"word2 1N word1",
		"word2 1W word1",
		"2W(word1,word2)",
		"2N(word1,word2)",
	}

	for _, q := range queries {
		facade := newBooleanQueryTestFacade(q, nil, fieldName, qf)
		_, err := facade.parseOnly()
		if err != nil {
			t.Errorf("unexpected parse error for %q: %v", q, err)
		}
	}
}
