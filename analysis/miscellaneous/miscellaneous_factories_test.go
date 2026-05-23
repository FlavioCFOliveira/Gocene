// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/test/org/apache/lucene/analysis/miscellaneous/TestMiscellaneousFactories.java

package miscellaneous

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// TestMiscellaneousFactories_ASCIIFolding verifies that the miscellaneous
// ASCIIFolding pipeline (WhitespaceTokenizer → ASCIIFoldingFilter) folds the
// Czech string "Česká" to "Ceska".
//
// Source: TestMiscellaneousFactories.testASCIIFolding
func TestMiscellaneousFactories_ASCIIFolding(t *testing.T) {
	tokenizer := analysis.NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("Česká"))
	if err := tokenizer.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	filter := analysis.NewASCIIFoldingFilter(tokenizer)

	var tokens []string
	for {
		ok, err := filter.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		attr := filter.GetAttribute("CharTermAttribute")
		if attr == nil {
			t.Fatal("CharTermAttribute not found")
		}
		tokens = append(tokens, attr.(analysis.CharTermAttribute).String())
	}

	want := []string{"Ceska"}
	if len(tokens) != len(want) {
		t.Fatalf("got %v, want %v", tokens, want)
	}
	for i := range want {
		if tokens[i] != want[i] {
			t.Errorf("token[%d]: got %q, want %q", i, tokens[i], want[i])
		}
	}
}
