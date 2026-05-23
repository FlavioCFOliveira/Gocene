// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package tools_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis/opennlp/tools"
)

// stubSentenceModel splits at every space for testing.
type stubSentenceModel struct{}

func (s stubSentenceModel) SplitSentences(text string) []tools.Span {
	spans := []tools.Span{{Start: 0, End: len(text)}}
	return spans
}

// stubTokenizerModel returns one span per word separated by space.
type stubTokenizerModel struct{}

func (s stubTokenizerModel) TokenizePos(sentence string) []tools.Span {
	var spans []tools.Span
	start := -1
	for i, ch := range sentence {
		if ch != ' ' && start < 0 {
			start = i
		} else if ch == ' ' && start >= 0 {
			spans = append(spans, tools.Span{Start: start, End: i})
			start = -1
		}
	}
	if start >= 0 {
		spans = append(spans, tools.Span{Start: start, End: len(sentence)})
	}
	return spans
}

// stubPOSModel returns "NN" for every word.
type stubPOSModel struct{}

func (s stubPOSModel) Tag(words []string) []string {
	tags := make([]string, len(words))
	for i := range tags {
		tags[i] = "NN"
	}
	return tags
}

// TestNLPSentenceDetectorOp_NoModel verifies that without a model the entire
// input is treated as one sentence.
func TestNLPSentenceDetectorOp_NoModel(t *testing.T) {
	op := tools.NewNLPSentenceDetectorOp()
	spans := op.SplitSentences("Hello world")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Start != 0 || spans[0].End != 11 {
		t.Errorf("unexpected span: %v", spans[0])
	}
}

// TestNLPSentenceDetectorOp_WithModel verifies delegation to the model.
func TestNLPSentenceDetectorOp_WithModel(t *testing.T) {
	op := tools.NewNLPSentenceDetectorOpWithModel(stubSentenceModel{})
	spans := op.SplitSentences("Hello world")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

// TestNLPTokenizerOp_NoModel verifies that without a model the sentence is a
// single token.
func TestNLPTokenizerOp_NoModel(t *testing.T) {
	op := tools.NewNLPTokenizerOp()
	spans := op.GetTerms("Hello world")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

// TestNLPTokenizerOp_WithModel verifies delegation to the model.
func TestNLPTokenizerOp_WithModel(t *testing.T) {
	op := tools.NewNLPTokenizerOpWithModel(stubTokenizerModel{})
	spans := op.GetTerms("Hello world")
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
}

// TestNLPPOSTaggerOp verifies POS tag delegation.
func TestNLPPOSTaggerOp(t *testing.T) {
	op := tools.NewNLPPOSTaggerOp(stubPOSModel{})
	tags := op.GetPOSTags([]string{"Hello", "world"})
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	for _, tag := range tags {
		if tag != "NN" {
			t.Errorf("expected NN, got %q", tag)
		}
	}
}

// TestNLPLemmatizerOp_DictionaryOnly verifies dictionary lemmatization.
func TestNLPLemmatizerOp_DictionaryOnly(t *testing.T) {
	dict := &stubDictionary{lemmas: map[string]string{"running": "run"}}
	op := tools.NewNLPLemmatizerOp(dict, nil)
	lemmas := op.Lemmatize([]string{"running", "unknown"}, []string{"VBG", "NN"})
	if lemmas[0] != "run" {
		t.Errorf("lemma[0] = %q, want %q", lemmas[0], "run")
	}
	if lemmas[1] != "unknown" {
		t.Errorf("lemma[1] = %q, want %q", lemmas[1], "unknown")
	}
}

// stubDictionary is a simple dictionary lemmatizer for testing.
type stubDictionary struct {
	lemmas map[string]string
}

func (d *stubDictionary) Lemmatize(words, _ []string) []string {
	result := make([]string, len(words))
	for i, w := range words {
		if l, ok := d.lemmas[w]; ok {
			result[i] = l
		} else {
			result[i] = "O"
		}
	}
	return result
}

// TestOpenNLPOpsFactory_PutAndGet verifies the global cache put/get cycle.
func TestOpenNLPOpsFactory_PutAndGet(t *testing.T) {
	defer tools.ClearModels()
	tools.PutSentenceModel("test-sent", stubSentenceModel{})
	op := tools.GetSentenceDetector("test-sent")
	spans := op.SplitSentences("Hello")
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}
}

// TestOpenNLPOpsFactory_EmptyName verifies fallback for empty name.
func TestOpenNLPOpsFactory_EmptyName(t *testing.T) {
	op := tools.GetSentenceDetector("")
	spans := op.SplitSentences("Hello")
	if len(spans) != 1 {
		t.Fatalf("expected 1 span for empty model name, got %d", len(spans))
	}
}

// TestSpan_Length verifies the Length helper.
func TestSpan_Length(t *testing.T) {
	s := tools.Span{Start: 3, End: 7}
	if s.Length() != 4 {
		t.Errorf("Length() = %d, want 4", s.Length())
	}
}
