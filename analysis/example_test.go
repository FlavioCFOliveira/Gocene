// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis_test

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// ExampleTokenizeWithAnalyzer shows how to tokenize a text string using
// WhitespaceAnalyzer and the TokenizeWithAnalyzer helper.
func ExampleTokenizeWithAnalyzer() {
	a := analysis.NewWhitespaceAnalyzer()
	tokens, err := analysis.TokenizeWithAnalyzer(a, "content", "hello world foo")
	if err != nil {
		panic(err)
	}
	for _, tok := range tokens {
		fmt.Println(tok)
	}
	// Output:
	// hello
	// world
	// foo
}
