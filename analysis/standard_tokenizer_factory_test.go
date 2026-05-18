// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"
	"strings"
	"testing"
)

// TestStandardTokenizerFactory_DefaultConstructor verifies that
// the no-arg constructor produces a usable factory configured with
// the Lucene default token length.
func TestStandardTokenizerFactory_DefaultConstructor(t *testing.T) {
	f := NewStandardTokenizerFactory()
	if got := f.MaxTokenLength(); got != DefaultMaxTokenLength {
		t.Errorf("MaxTokenLength: got %d, want %d", got, DefaultMaxTokenLength)
	}

	tk := f.Create()
	defer tk.Close()
	st, ok := tk.(*StandardTokenizer)
	if !ok {
		t.Fatalf("Create returned %T, want *StandardTokenizer", tk)
	}
	if got := st.MaxTokenLength(); got != DefaultMaxTokenLength {
		t.Errorf("created tokenizer MaxTokenLength: got %d, want %d", got, DefaultMaxTokenLength)
	}
}

// TestStandardTokenizerFactory_WithArgs_NilEmpty verifies that the
// args-based constructor accepts both a nil and an empty map and
// falls back to the defaults in both cases.
func TestStandardTokenizerFactory_WithArgs_NilEmpty(t *testing.T) {
	for name, args := range map[string]map[string]string{
		"nil":   nil,
		"empty": {},
	} {
		t.Run(name, func(t *testing.T) {
			f, err := NewStandardTokenizerFactoryWithArgs(args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := f.MaxTokenLength(); got != DefaultMaxTokenLength {
				t.Errorf("MaxTokenLength: got %d, want %d", got, DefaultMaxTokenLength)
			}
		})
	}
}

// TestStandardTokenizerFactory_WithArgs_MaxTokenLength verifies that
// the "maxTokenLength" parameter is parsed and propagated to every
// Tokenizer the factory produces. Mirrors the Lucene reference
// factory's Map<String,String> contract.
func TestStandardTokenizerFactory_WithArgs_MaxTokenLength(t *testing.T) {
	f, err := NewStandardTokenizerFactoryWithArgs(map[string]string{
		"maxTokenLength": "42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := f.MaxTokenLength(); got != 42 {
		t.Errorf("MaxTokenLength: got %d, want 42", got)
	}

	tk := f.Create().(*StandardTokenizer)
	defer tk.Close()
	if got := tk.MaxTokenLength(); got != 42 {
		t.Errorf("created tokenizer MaxTokenLength: got %d, want 42", got)
	}
}

// TestStandardTokenizerFactory_WithArgs_Unknown verifies the
// "Unknown parameters" enforcement: anything left in the args map
// after consumption produces an error, matching Lucene's runtime
// guard.
func TestStandardTokenizerFactory_WithArgs_Unknown(t *testing.T) {
	_, err := NewStandardTokenizerFactoryWithArgs(map[string]string{
		"maxTokenLength": "100",
		"surprise":       "value",
	})
	if err == nil {
		t.Fatal("expected error for unknown parameter")
	}
	if !strings.Contains(err.Error(), "unknown") || !strings.Contains(err.Error(), "surprise") {
		t.Errorf("error message %q does not mention unknown parameter", err.Error())
	}
}

// TestStandardTokenizerFactory_WithArgs_InvalidValue verifies the
// per-parameter parse-failure path.
func TestStandardTokenizerFactory_WithArgs_InvalidValue(t *testing.T) {
	_, err := NewStandardTokenizerFactoryWithArgs(map[string]string{
		"maxTokenLength": "not-a-number",
	})
	if err == nil {
		t.Fatal("expected error for invalid maxTokenLength value")
	}
	if !strings.Contains(err.Error(), "maxTokenLength") {
		t.Errorf("error message %q does not mention failing parameter", err.Error())
	}
}

// TestStandardTokenizerFactory_TokenizerProducesExpectedTokens makes
// sure the factory's Create() returns a tokenizer that emits the
// expected token sequence for a representative input.
func TestStandardTokenizerFactory_TokenizerProducesExpectedTokens(t *testing.T) {
	f := NewStandardTokenizerFactory()
	tk := f.Create()
	defer tk.Close()
	if err := tk.SetReader(strings.NewReader("Hello, World 123!")); err != nil {
		t.Fatalf("SetReader: %v", err)
	}
	var tokens []string
	for {
		ok, err := tk.IncrementToken()
		if err != nil {
			t.Fatalf("IncrementToken: %v", err)
		}
		if !ok {
			break
		}
		ct := tk.(*StandardTokenizer).
			GetAttributeSource().
			GetAttribute(CharTermAttributeType).(CharTermAttribute)
		tokens = append(tokens, ct.String())
	}
	want := []string{"Hello", "World", "123"}
	if !reflect.DeepEqual(tokens, want) {
		t.Errorf("tokens: got %v, want %v", tokens, want)
	}
}

// TestStandardTokenizerFactoryName asserts the SPI-faithful name
// constant matches Lucene's StandardTokenizerFactory.NAME.
func TestStandardTokenizerFactoryName(t *testing.T) {
	if StandardTokenizerFactoryName != "standard" {
		t.Errorf("StandardTokenizerFactoryName = %q, want %q",
			StandardTokenizerFactoryName, "standard")
	}
}
