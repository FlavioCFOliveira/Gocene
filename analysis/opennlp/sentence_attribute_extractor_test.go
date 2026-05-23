// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package opennlp

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// stubTokenStream provides a fixed sequence of tokens for testing.
// Each token is associated with a sentence index stored in sentenceAtt.
type stubTokenStream struct {
	analysis.BaseTokenStream
	tokens  []stubToken
	pos     int
	termAtt *analysis.CharTermAttributeImpl
	sentAtt *analysis.SentenceAttributeImpl
}

type stubToken struct {
	term     string
	sentence int
}

func newStubTokenStream(tokens []stubToken) *stubTokenStream {
	s := &stubTokenStream{
		BaseTokenStream: *analysis.NewBaseTokenStream(),
		tokens:          tokens,
	}
	s.termAtt = analysis.NewCharTermAttributeImpl()
	s.sentAtt = analysis.NewSentenceAttributeImpl()
	s.AddAttribute(s.termAtt)
	s.AddAttribute(s.sentAtt)
	return s
}

func (s *stubTokenStream) IncrementToken() (bool, error) {
	s.ClearAttributes()
	if s.pos >= len(s.tokens) {
		return false, nil
	}
	tok := s.tokens[s.pos]
	s.pos++
	s.termAtt.SetEmpty()
	s.termAtt.Append([]byte(tok.term))
	s.sentAtt.SetSentenceIndex(tok.sentence)
	return true, nil
}

func (s *stubTokenStream) Reset() error {
	s.pos = 0
	s.ClearAttributes()
	return nil
}

// TestSentenceAttributeExtractor_ExtractTwoSentences verifies that the
// extractor groups tokens by sentence index and returns two separate slices.
func TestSentenceAttributeExtractor_ExtractTwoSentences(t *testing.T) {
	tokens := []stubToken{
		{"Hello", 0},
		{"world", 0},
		{"Foo", 1},
		{"bar", 1},
	}
	stream := newStubTokenStream(tokens)
	// Use the same sentAtt that the stream uses so the extractor reads
	// live values when IncrementToken is called.
	sentAtt := stream.sentAtt

	ext := NewSentenceAttributeExtractor(stream, sentAtt)

	// Extract first sentence.
	sent0, err := ext.ExtractSentenceAttributes()
	if err != nil {
		t.Fatalf("ExtractSentenceAttributes: %v", err)
	}
	if got := len(sent0); got != 2 {
		t.Errorf("sentence 0: got %d tokens, want 2", got)
	}
	if ext.AllSentencesProcessed() {
		t.Error("AllSentencesProcessed should be false after first sentence")
	}

	// Extract second sentence.
	sent1, err := ext.ExtractSentenceAttributes()
	if err != nil {
		t.Fatalf("ExtractSentenceAttributes sentence 1: %v", err)
	}
	if got := len(sent1); got != 2 {
		t.Errorf("sentence 1: got %d tokens, want 2", got)
	}

	// After the second sentence is extracted, AllSentencesProcessed should be true.
	if !ext.AllSentencesProcessed() {
		t.Error("AllSentencesProcessed should be true after stream is exhausted")
	}
}

// TestSentenceAttributeExtractor_Reset verifies that Reset clears state.
func TestSentenceAttributeExtractor_Reset(t *testing.T) {
	stream := newStubTokenStream([]stubToken{{"a", 0}})
	sentAtt := stream.sentAtt

	ext := NewSentenceAttributeExtractor(stream, sentAtt)

	// Extract the single sentence; after this AllSentencesProcessed should be true.
	_, _ = ext.ExtractSentenceAttributes()

	if !ext.AllSentencesProcessed() {
		t.Fatal("should be exhausted after extracting all sentences")
	}

	ext.Reset()
	if ext.AllSentencesProcessed() {
		t.Error("AllSentencesProcessed should be false after Reset")
	}
	if len(ext.GetSentenceAttributes()) != 0 {
		t.Error("GetSentenceAttributes should be empty after Reset")
	}
}

// TestSentenceAttributeExtractor_GetSentenceAttributes verifies that
// GetSentenceAttributes returns the same slice as ExtractSentenceAttributes.
func TestSentenceAttributeExtractor_GetSentenceAttributes(t *testing.T) {
	stream := newStubTokenStream([]stubToken{{"x", 0}, {"y", 1}})
	sentAtt := stream.sentAtt

	ext := NewSentenceAttributeExtractor(stream, sentAtt)

	got, _ := ext.ExtractSentenceAttributes()
	same := ext.GetSentenceAttributes()
	if len(got) != len(same) {
		t.Errorf("GetSentenceAttributes len mismatch: %d vs %d", len(got), len(same))
	}
	for i := range got {
		if got[i] != same[i] {
			t.Errorf("index %d: pointer mismatch", i)
		}
	}
}
