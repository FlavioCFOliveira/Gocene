// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package surround

import "testing"

// TestExceptions01 reproduces the assertion in Java's Test01Exceptions.test01Exceptions:
// every query listed in exceptionQueries must produce a parse error.
//
// Port of org.apache.lucene.queryparser.surround.query.Test01Exceptions.
func TestExceptions01(t *testing.T) {
	exceptionQueries := []string{
		"*",
		"a*",
		"ab*",
		"?",
		"a?",
		"ab?",
		"a???b",
		"a?",
		"a*b?",
		"word1 word2",
		"word2 AND",
		"word1 OR",
		"AND(word2)",
		"AND(word2,)",
		"AND(word2,word1,)",
		"OR(word2)",
		"OR(word2 ,",
		"OR(word2 , word1 ,)",
		"xx NOT",
		"xx (a AND b)",
		"(a AND b",
		"a OR b)",
		"or(word2+ not ord+, and xyz,def)",
		"",
	}

	m := getFailQueries(exceptionQueries, false)
	if len(m) > 0 {
		t.Errorf("expected parse errors but the following queries parsed successfully:\n%s", m)
	}
}
