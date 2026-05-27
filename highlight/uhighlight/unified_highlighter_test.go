// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uhighlight_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/highlight/uhighlight"
	"github.com/FlavioCFOliveira/Gocene/highlight/uhighlight/testdata"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestUnifiedHighlighter_GoldenAnalysis drives the ANALYSIS offset source
// over every golden fixture and asserts the rendered snippet matches the
// expected string exactly.
func TestUnifiedHighlighter_GoldenAnalysis(t *testing.T) {
	for _, g := range testdata.Goldens() {
		g := g
		t.Run("ANALYSIS/"+g.Name, func(t *testing.T) {
			h := uhighlight.NewUnifiedHighlighter(
				g.Field,
				analysis.NewWhitespaceAnalyzer(),
				g.QueryTerms,
				nil,
			)
			h.SetBreakIterator(breakIteratorFor(g.BreakIter))
			h.SetMaxPassages(10)
			h.SetMaxNoHighlightPassages(1)

			snippet, err := h.Highlight(g.Content, nil)
			if err != nil {
				t.Fatalf("Highlight: %v", err)
			}
			if snippet != g.WantSnippet {
				t.Errorf("snippet mismatch\n have: %q\n want: %q", snippet, g.WantSnippet)
			}
		})
	}
}

// TestUnifiedHighlighter_GoldenTermVectors drives the TERM_VECTORS
// offset source against the same fixtures, with term-vector entries
// computed from the WhitespaceAnalyzer output of the corpus. This proves
// the two paths converge on the same snippet output, which is the
// Gocene-internal byte-parity contract this slice locks in.
func TestUnifiedHighlighter_GoldenTermVectors(t *testing.T) {
	for _, g := range testdata.Goldens() {
		g := g
		t.Run("TERM_VECTORS/"+g.Name, func(t *testing.T) {
			entries, err := buildTermVectorEntries(g.Content)
			if err != nil {
				t.Fatalf("buildTermVectorEntries: %v", err)
			}
			h := uhighlight.NewUnifiedHighlighter(
				g.Field,
				nil, // analyzer not used in term-vector mode
				g.QueryTerms,
				nil,
			)
			h.SetBreakIterator(breakIteratorFor(g.BreakIter))
			h.SetMaxPassages(10)
			h.SetMaxNoHighlightPassages(1)

			snippet, err := h.HighlightTermVector(g.Content, entries, nil)
			if err != nil {
				t.Fatalf("HighlightTermVector: %v", err)
			}
			if snippet != g.WantSnippet {
				t.Errorf("snippet mismatch\n have: %q\n want: %q", snippet, g.WantSnippet)
			}
		})
	}
}

// TestUnifiedHighlighter_RejectsNilAnalyzerOnAnalysisPath confirms the
// analysis path errors out cleanly when no analyzer is configured.
func TestUnifiedHighlighter_RejectsNilAnalyzerOnAnalysisPath(t *testing.T) {
	h := uhighlight.NewUnifiedHighlighter("body", nil, []string{"fox"}, nil)
	if _, err := h.Highlight("The quick fox.", nil); err == nil {
		t.Fatal("expected error when analyzer is nil; got nil")
	}
}

// TestUnifiedHighlighter_TermVectorRejectsMissingOffsets confirms the
// term-vector path errors when StartOffsets/EndOffsets are missing on a
// matched entry (i.e. the field was indexed WITHOUT_OFFSETS).
func TestUnifiedHighlighter_TermVectorRejectsMissingOffsets(t *testing.T) {
	h := uhighlight.NewUnifiedHighlighter("body", nil, []string{"fox"}, nil)
	entries := []uhighlight.TermVectorEntry{
		{Term: "fox", Frequency: 1}, // no StartOffsets/EndOffsets
	}
	_, err := h.HighlightTermVector("The quick fox.", entries, nil)
	if err == nil {
		t.Fatal("expected error when term-vector entries lack offsets; got nil")
	}
}

// TestUnifiedHighlighter_EmptyContentReturnsEmpty pins the contract that
// empty input yields an empty snippet, never an error.
func TestUnifiedHighlighter_EmptyContentReturnsEmpty(t *testing.T) {
	h := uhighlight.NewUnifiedHighlighter("body", analysis.NewWhitespaceAnalyzer(), []string{"fox"}, nil)
	snippet, err := h.Highlight("", nil)
	if err != nil {
		t.Fatalf("Highlight: %v", err)
	}
	if snippet != "" {
		t.Errorf("expected empty snippet, got %q", snippet)
	}
}

// breakIteratorFor maps the fixture's BreakIterKind to the concrete
// iterator the UH will use.
func breakIteratorFor(kind testdata.BreakIterKind) uhighlight.BreakIterator {
	switch kind {
	case testdata.BreakWhole:
		return uhighlight.WholeBreakIterator{}
	case testdata.BreakSentence:
		return uhighlight.SentenceBreakIterator{}
	default:
		return uhighlight.SplittingBreakIterator{}
	}
}

// buildTermVectorEntries runs the WhitespaceAnalyzer over content and
// emits the per-term offset list as if the codec had stored term vectors
// WITH_OFFSETS for the same field. The helper makes the term-vector
// fixture data a pure function of the same analyzer output the analysis
// path uses, which is what guarantees both paths converge on the same
// snippet.
func buildTermVectorEntries(content string) ([]uhighlight.TermVectorEntry, error) {
	a := analysis.NewWhitespaceAnalyzer()
	stream, err := a.TokenStream("body", strings.NewReader(content))
	if err != nil {
		return nil, err
	}
	defer func() { _ = stream.Close() }()

	type sourceProvider interface {
		GetAttributeSource() *util.AttributeSource
	}
	src := stream.(sourceProvider).GetAttributeSource()
	termAttr := src.GetAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	offsetAttr := src.GetAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)

	byTerm := make(map[string]*uhighlight.TermVectorEntry)
	var order []string
	for {
		more, err := stream.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !more {
			break
		}
		term := termAttr.String()
		entry, ok := byTerm[term]
		if !ok {
			entry = &uhighlight.TermVectorEntry{Term: term}
			byTerm[term] = entry
			order = append(order, term)
		}
		entry.Frequency++
		entry.StartOffsets = append(entry.StartOffsets, offsetAttr.StartOffset())
		entry.EndOffsets = append(entry.EndOffsets, offsetAttr.EndOffset())
	}
	_ = stream.End()

	entries := make([]uhighlight.TermVectorEntry, 0, len(order))
	for _, t := range order {
		entries = append(entries, *byTerm[t])
	}
	return entries, nil
}
