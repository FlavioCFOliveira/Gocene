// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"
)

// TestTokenizer_SetReader tests setting the input reader.
// Source: TestTokenizer.java
// Purpose: Tests that the input source can be set and reset.
func TestTokenizer_SetReader(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()

	reader := strings.NewReader("test input")
	err := tokenizer.SetReader(reader)
	if err != nil {
		t.Fatalf("SetReader() returned error: %v", err)
	}

	if tokenizer.GetReader() != reader {
		t.Error("GetReader() should return the set reader")
	}
}

// TestTokenizer_Reset tests resetting the tokenizer state.
// Source: TestTokenizer.java
// Purpose: Tests that tokenizer can be reset for reuse.
func TestTokenizer_Reset(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("first"))

	// Consume first input
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
	}

	// Reset and set new input
	err := tokenizer.Reset()
	if err != nil {
		t.Fatalf("Reset() returned error: %v", err)
	}

	tokenizer.SetReader(strings.NewReader("second"))

	// Should be able to tokenize new input
	hasToken, err := tokenizer.IncrementToken()
	if err != nil {
		t.Fatalf("Error after reset: %v", err)
	}
	if !hasToken {
		t.Error("Should have token after reset")
	}
}

// TestTokenizer_InputFinished tests input finished tracking.
// Source: TestTokenizer.java
// Purpose: Tests that input finished state is tracked correctly.
func TestTokenizer_InputFinished(t *testing.T) {
	tokenizer := NewBaseTokenizer()

	if tokenizer.IsInputFinished() {
		t.Error("IsInputFinished() should be false initially")
	}

	tokenizer.SetInputFinished(true)
	if !tokenizer.IsInputFinished() {
		t.Error("IsInputFinished() should be true after setting")
	}

	tokenizer.SetInputFinished(false)
	if tokenizer.IsInputFinished() {
		t.Error("IsInputFinished() should be false after resetting")
	}
}

// TestTokenizer_End tests end-of-stream operations.
// Source: TestTokenizer.java
// Purpose: Tests that End() can be called without error.
func TestTokenizer_End(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	// Consume all tokens
	for {
		hasToken, err := tokenizer.IncrementToken()
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if !hasToken {
			break
		}
	}

	err := tokenizer.End()
	if err != nil {
		t.Errorf("End() returned error: %v", err)
	}
}

// TestTokenizer_Close tests resource cleanup.
// Source: TestTokenizer.java
// Purpose: Tests that Close() properly releases resources.
func TestTokenizer_Close(t *testing.T) {
	tokenizer := NewWhitespaceTokenizer()
	tokenizer.SetReader(strings.NewReader("test"))

	err := tokenizer.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}

	// Reader should be nil after close
	if tokenizer.GetReader() != nil {
		t.Error("Reader should be nil after Close()")
	}
}

// TestTokenizerFactory tests TokenizerFactory implementations.
func TestTokenizerFactory(t *testing.T) {
	factory := NewLetterTokenizerFactory()

	tokenizer := factory.Create()
	if tokenizer == nil {
		t.Error("Factory.Create() should return a non-nil tokenizer")
	}

	// Verify it's the correct type
	if _, ok := tokenizer.(*LetterTokenizer); !ok {
		t.Error("Factory should create LetterTokenizer instances")
	}
}

// TestBaseTokenizer_Implementation tests BaseTokenizer methods.
func TestBaseTokenizer_Implementation(t *testing.T) {
	tokenizer := NewBaseTokenizer()

	// Test SetReader
	reader := strings.NewReader("test")
	err := tokenizer.SetReader(reader)
	if err != nil {
		t.Fatalf("SetReader() returned error: %v", err)
	}

	if tokenizer.GetReader() != reader {
		t.Error("GetReader() should return the set reader")
	}

	// Test Reset
	err = tokenizer.Reset()
	if err != nil {
		t.Errorf("Reset() returned error: %v", err)
	}

	// Test Close
	err = tokenizer.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}
