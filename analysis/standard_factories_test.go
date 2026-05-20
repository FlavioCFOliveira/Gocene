// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/analysis/standard/TestStandardFactories.java
//
// Deviation: the Java test uses BaseTokenStreamFactoryTestCase.tokenizerFactory()
// which invokes the Java SPI registry (ServiceLoader). In Go the equivalent is
// calling the factory constructor directly.

package analysis

import (
	"strings"
	"testing"
)

// TestStandardFactories_TokenizerBasic mirrors TestStandardFactories.testStandardTokenizer
// (Lucene 10.4.0). It verifies that StandardTokenizerFactory produces tokens from a
// Unicode-accented input string.
func TestStandardFactories_TokenizerBasic(t *testing.T) {
	f := NewStandardTokenizerFactory()
	tok := f.Create()
	defer tok.Close()
	st, ok := tok.(*StandardTokenizer)
	if !ok {
		t.Fatalf("expected *StandardTokenizer, got %T", tok)
	}
	if err := st.SetReader(strings.NewReader("Whát's this thing do?")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}

	want := []string{"Whát's", "this", "thing", "do"}
	got := driveStandardTokenizer(t, st)
	if len(got) != len(want) {
		t.Fatalf("expected tokens %v, got %v", want, got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("token[%d]: expected %q, got %q", i, w, got[i])
		}
	}
}

// TestStandardFactories_MaxTokenLength mirrors testStandardTokenizerMaxTokenLength.
func TestStandardFactories_MaxTokenLength(t *testing.T) {
	f, err := NewStandardTokenizerFactoryWithArgs(map[string]string{"maxTokenLength": "1000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("abcdefg")
	}
	longWord := sb.String()
	input := "one two three " + longWord + " four five six"
	tok := f.Create()
	defer tok.Close()
	st := tok.(*StandardTokenizer)
	if err := st.SetReader(strings.NewReader(input)); err != nil {
		t.Fatalf("SetReader: %v", err)
	}

	want := []string{"one", "two", "three", longWord, "four", "five", "six"}
	got := driveStandardTokenizer(t, st)
	if len(got) != len(want) {
		t.Fatalf("expected %d tokens, got %d: %v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("token[%d]: expected %q, got %q", i, w, got[i])
		}
	}
}

// TestStandardFactories_BogusArguments mirrors testBogusArguments (Lucene 10.4.0).
func TestStandardFactories_BogusArguments(t *testing.T) {
	_, err := NewStandardTokenizerFactoryWithArgs(map[string]string{"bogusArg": "bogusValue"})
	if err == nil {
		t.Fatal("expected error for unknown parameters")
	}
	if !strings.Contains(err.Error(), "unknown") && !strings.Contains(err.Error(), "Unknown") {
		t.Fatalf("expected 'unknown parameters' error, got: %v", err)
	}
}

// driveStandardTokenizer drives a StandardTokenizer (after SetReader has been
// called) through its full output and returns the surface forms as strings.
// SetReader already resets the scanner, so Reset() is not called again here.
func driveStandardTokenizer(t *testing.T, st *StandardTokenizer) []string {
	t.Helper()
	ct := st.GetAttributeSource().GetAttribute(CharTermAttributeType).(CharTermAttribute)
	var tokens []string
	for {
		ok, err := st.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		tokens = append(tokens, string(ct.Buffer()[:ct.Length()]))
	}
	if err := st.End(); err != nil {
		t.Fatalf("End: %v", err)
	}
	return tokens
}
