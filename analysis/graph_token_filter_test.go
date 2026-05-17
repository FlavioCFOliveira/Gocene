// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestGraphTokenFilter_LinearStream verifies that the graph walker behaves
// correctly on a linear (no-alternatives) token stream: IncrementBaseToken
// produces the same sequence as the underlying tokenizer, and
// IncrementGraphToken on a linear path reads tokens one at a time.
func TestGraphTokenFilter_LinearStream(t *testing.T) {
	tok := NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader("a b c d")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	termAttr := lookupCharTermAttribute(t, tok)

	f := NewGraphTokenFilter(tok)
	if err := f.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// Walk all base tokens; on a linear stream IncrementGraph should always
	// return false (no alternative routes) but IncrementBaseToken should
	// produce every token from the underlying stream in order.
	var bases []string
	for {
		ok, err := f.IncrementBaseToken()
		if err != nil {
			t.Fatalf("IncrementBaseToken: %v", err)
		}
		if !ok {
			break
		}
		bases = append(bases, termAttr.String())

		// No alternative paths in a linear stream.
		alt, err := f.IncrementGraph()
		if err != nil {
			t.Fatalf("IncrementGraph: %v", err)
		}
		if alt {
			t.Fatal("linear stream should not have alternative graph paths")
		}
	}
	if !reflect.DeepEqual(bases, []string{"a", "b", "c", "d"}) {
		t.Errorf("expected [a b c d], got %v", bases)
	}

	if err := f.End(); err != nil {
		t.Fatalf("End: %v", err)
	}
}

// TestGraphTokenFilter_IncrementGraphToken verifies that IncrementGraphToken
// walks one position forward at a time on a linear stream.
func TestGraphTokenFilter_IncrementGraphToken(t *testing.T) {
	tok := NewWhitespaceTokenizer()
	if err := tok.SetReader(strings.NewReader("alpha beta gamma")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	termAttr := lookupCharTermAttribute(t, tok)

	f := NewGraphTokenFilter(tok)
	if err := f.Reset(); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	ok, err := f.IncrementBaseToken()
	if err != nil || !ok {
		t.Fatalf("expected base token, got ok=%v err=%v", ok, err)
	}
	if got := termAttr.String(); got != "alpha" {
		t.Errorf("expected base 'alpha', got %q", got)
	}

	// Walk one position forward along the graph path.
	ok, err = f.IncrementGraphToken()
	if err != nil || !ok {
		t.Fatalf("expected graph token, got ok=%v err=%v", ok, err)
	}
	if got := termAttr.String(); got != "beta" {
		t.Errorf("expected graph token 'beta', got %q", got)
	}

	// And once more.
	ok, err = f.IncrementGraphToken()
	if err != nil || !ok {
		t.Fatalf("expected graph token, got ok=%v err=%v", ok, err)
	}
	if got := termAttr.String(); got != "gamma" {
		t.Errorf("expected graph token 'gamma', got %q", got)
	}

	// End of stream.
	ok, err = f.IncrementGraphToken()
	if err != nil {
		t.Fatalf("IncrementGraphToken: %v", err)
	}
	if ok {
		t.Error("expected no more graph tokens at EOS")
	}
}
