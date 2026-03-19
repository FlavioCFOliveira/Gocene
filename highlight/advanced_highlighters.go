// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import (
	"fmt"
)

// FastVectorHighlighter is a highlighter that uses term vectors for fast highlighting.
// It is optimized for fields with term vectors and can provide very fast highlighting
// performance.
//
// This is the Go port of Lucene's org.apache.lucene.search.vectorhighlight.FastVectorHighlighter.
type FastVectorHighlighter struct {
	// fragCharSize is the target size of each fragment
	fragCharSize int

	// maxNumFragments is the maximum number of fragments to return
	maxNumFragments int

	// fragmentsBuilder builds the highlighted fragments
	fragmentsBuilder *FragmentsBuilder

	// fragListBuilder builds the fragment list
	fragListBuilder *FragListBuilder
}

// NewFastVectorHighlighter creates a new FastVectorHighlighter.
//
// Returns:
//   - a new FastVectorHighlighter instance
func NewFastVectorHighlighter() *FastVectorHighlighter {
	return &FastVectorHighlighter{
		fragCharSize:     100,
		maxNumFragments:  3,
		fragmentsBuilder: NewFragmentsBuilder(),
		fragListBuilder:  NewFragListBuilder(100),
	}
}

// SetFragCharSize sets the target size of each fragment.
//
// Parameters:
//   - size: the target fragment size
func (fvh *FastVectorHighlighter) SetFragCharSize(size int) {
	fvh.fragCharSize = size
	fvh.fragListBuilder = NewFragListBuilder(size)
}

// SetMaxNumFragments sets the maximum number of fragments to return.
//
// Parameters:
//   - max: the maximum number of fragments
func (fvh *FastVectorHighlighter) SetMaxNumFragments(max int) {
	fvh.maxNumFragments = max
}

// SetFragmentsBuilder sets the fragments builder.
//
// Parameters:
//   - builder: the fragments builder to use
func (fvh *FastVectorHighlighter) SetFragmentsBuilder(builder *FragmentsBuilder) {
	fvh.fragmentsBuilder = builder
}

// SetTextFragmenter sets the fragmenter for this highlighter.
//
// Parameters:
//   - fragmenter: the fragmenter to use
func (fvh *FastVectorHighlighter) SetTextFragmenter(fragmenter Fragmenter) {
	// FastVectorHighlighter uses FragListBuilder for fragmentation
	// This method is for interface compliance
}

// SetFormatter sets the formatter for this highlighter.
//
// Parameters:
//   - formatter: the formatter to use
func (fvh *FastVectorHighlighter) SetFormatter(formatter Formatter) {
	// FastVectorHighlighter uses FragmentsBuilder for formatting
	// This method is for interface compliance
}

// GetBestFragment returns the best fragment with highlighted terms.
//
// Parameters:
//   - text: the text to highlight
//   - maxNumFragments: the maximum number of fragments (ignored, always returns 1)
//
// Returns:
//   - the best highlighted fragment, or error if highlighting fails
func (fvh *FastVectorHighlighter) GetBestFragment(text string, maxNumFragments int) (string, error) {
	// Extract terms from the query (simplified - in real implementation would parse query)
	terms := []string{} // This would come from query parsing
	fragments, err := fvh.GetBestFragmentsWithTerms(text, terms, 1)
	if err != nil {
		return "", err
	}
	if len(fragments) == 0 {
		return "", nil
	}
	return fragments[0], nil
}

// GetBestFragmentsWithTerms returns the best fragments with highlighted terms.
//
// Parameters:
//   - text: the text to highlight
//   - terms: the terms to highlight
//   - maxNumFragments: the maximum number of fragments to return
//
// Returns:
//   - the highlighted fragments, or error if highlighting fails
func (fvh *FastVectorHighlighter) GetBestFragmentsWithTerms(text string, terms []string, maxNumFragments int) ([]string, error) {
	if text == "" || len(terms) == 0 {
		return []string{}, nil
	}

	// Create fragment list
	fragList := fvh.fragListBuilder.CreateFieldFragList("field", text, terms)

	// Build fragments
	fragments := fvh.fragmentsBuilder.CreateFragments(text, fragList, maxNumFragments)

	return fragments, nil
}

// GetBestFragments returns the best fragments with highlighted terms.
//
// Parameters:
//   - text: the text to highlight
//   - maxNumFragments: the maximum number of fragments to return
//
// Returns:
//   - the highlighted fragments, or error if highlighting fails
func (fvh *FastVectorHighlighter) GetBestFragments(text string, maxNumFragments int) ([]string, error) {
	return fvh.GetBestFragmentsWithTerms(text, []string{}, maxNumFragments)
}

// String returns a string representation of this highlighter.
//
// Returns:
//   - a string representation
func (fvh *FastVectorHighlighter) String() string {
	return fmt.Sprintf("FastVectorHighlighter{fragCharSize=%d, maxNumFragments=%d}",
		fvh.fragCharSize, fvh.maxNumFragments)
}

// PostingsHighlighter is a highlighter that uses the postings (term positions) to
// create highlighted fragments. It is efficient and doesn't require term vectors.
//
// This is the Go port of Lucene's org.apache.lucene.search.postingshighlight.PostingsHighlighter.
type PostingsHighlighter struct {
	// maxLength is the maximum length of text to analyze
	maxLength int

	// maxNumFragments is the maximum number of fragments to return
	maxNumFragments int

	// fragmentsBuilder builds the highlighted fragments
	fragmentsBuilder *FragmentsBuilder

	// passageFormatter formats passages
	passageFormatter PassageFormatter
}

// NewPostingsHighlighter creates a new PostingsHighlighter.
//
// Returns:
//   - a new PostingsHighlighter instance
func NewPostingsHighlighter() *PostingsHighlighter {
	return &PostingsHighlighter{
		maxLength:        10000,
		maxNumFragments:  5,
		fragmentsBuilder: NewFragmentsBuilder(),
		passageFormatter: NewDefaultPassageFormatter(),
	}
}

// SetMaxLength sets the maximum length of text to analyze.
//
// Parameters:
//   - maxLength: the maximum length
func (ph *PostingsHighlighter) SetMaxLength(maxLength int) {
	ph.maxLength = maxLength
}

// SetMaxNumFragments sets the maximum number of fragments to return.
//
// Parameters:
//   - max: the maximum number of fragments
func (ph *PostingsHighlighter) SetMaxNumFragments(max int) {
	ph.maxNumFragments = max
}

// SetPassageFormatter sets the passage formatter.
//
// Parameters:
//   - formatter: the passage formatter to use
func (ph *PostingsHighlighter) SetPassageFormatter(formatter PassageFormatter) {
	ph.passageFormatter = formatter
}

// SetTextFragmenter sets the fragmenter for this highlighter.
//
// Parameters:
//   - fragmenter: the fragmenter to use
func (ph *PostingsHighlighter) SetTextFragmenter(fragmenter Fragmenter) {
	// PostingsHighlighter uses internal passage creation
	// This method is for interface compliance
}

// SetFormatter sets the formatter for this highlighter.
//
// Parameters:
//   - formatter: the formatter to use
func (ph *PostingsHighlighter) SetFormatter(formatter Formatter) {
	// PostingsHighlighter uses PassageFormatter for formatting
	// This method is for interface compliance
}

// GetBestFragment returns the best fragment with highlighted terms.
//
// Parameters:
//   - text: the text to highlight
//   - maxNumFragments: the maximum number of fragments (ignored, always returns 1)
//
// Returns:
//   - the best highlighted fragment, or error if highlighting fails
func (ph *PostingsHighlighter) GetBestFragment(text string, maxNumFragments int) (string, error) {
	terms := []string{} // Would come from query
	fragments, err := ph.GetBestFragmentsWithTerms(text, terms, 1)
	if err != nil {
		return "", err
	}
	if len(fragments) == 0 {
		return "", nil
	}
	return fragments[0], nil
}

// GetBestFragmentsWithTerms returns the best fragments with highlighted terms.
//
// Parameters:
//   - text: the text to highlight
//   - terms: the terms to highlight
//   - maxNumFragments: the maximum number of fragments to return
//
// Returns:
//   - the highlighted fragments, or error if highlighting fails
func (ph *PostingsHighlighter) GetBestFragmentsWithTerms(text string, terms []string, maxNumFragments int) ([]string, error) {
	if text == "" || len(terms) == 0 {
		return []string{}, nil
	}

	// Limit text length
	if len(text) > ph.maxLength {
		text = text[:ph.maxLength]
	}

	// Create passages
	passages := ph.createPassages(text, terms)

	// Format passages
	fragments := make([]string, 0, len(passages))
	for _, passage := range passages {
		if len(fragments) >= maxNumFragments {
			break
		}
		formatted := ph.passageFormatter.Format(passage)
		if formatted != "" {
			fragments = append(fragments, formatted)
		}
	}

	return fragments, nil
}

// GetBestFragments returns the best fragments with highlighted terms.
//
// Parameters:
//   - text: the text to highlight
//   - maxNumFragments: the maximum number of fragments to return
//
// Returns:
//   - the highlighted fragments, or error if highlighting fails
func (ph *PostingsHighlighter) GetBestFragments(text string, maxNumFragments int) ([]string, error) {
	return ph.GetBestFragmentsWithTerms(text, []string{}, maxNumFragments)
}

// createPassages creates passages from text and terms.
func (ph *PostingsHighlighter) createPassages(text string, terms []string) []*Passage {
	passages := make([]*Passage, 0)

	// Find term positions
	for _, term := range terms {
		if term == "" {
			continue
		}

		// Find all occurrences of this term
		for i := 0; i < len(text); {
			idx := indexOfIgnoreCase(text, term, i)
			if idx == -1 {
				break
			}

			// Create a passage around this term
			passageStart := idx - 50
			if passageStart < 0 {
				passageStart = 0
			}
			passageEnd := idx + len(term) + 50
			if passageEnd > len(text) {
				passageEnd = len(text)
			}

			passage := NewPassage(text[passageStart:passageEnd], passageStart, passageEnd)
			passage.AddMatch(idx-passageStart, idx-passageStart+len(term), term)
			passage.Score = 1.0

			passages = append(passages, passage)
			i = idx + 1
		}
	}

	return passages
}

// String returns a string representation of this highlighter.
//
// Returns:
//   - a string representation
func (ph *PostingsHighlighter) String() string {
	return fmt.Sprintf("PostingsHighlighter{maxLength=%d, maxNumFragments=%d}",
		ph.maxLength, ph.maxNumFragments)
}

// UnifiedHighlighter is a unified highlighter that combines the best features
// of other highlighters. It can use term vectors, postings, or plain term
// extraction depending on what's available.
//
// This is the Go port of Lucene's org.apache.lucene.uninverted.highlight.UnifiedHighlighter.
type UnifiedHighlighter struct {
	// maxLength is the maximum length of text to analyze
	maxLength int

	// maxNumFragments is the maximum number of fragments to return
	maxNumFragments int

	// passageFormatter formats passages
	passageFormatter PassageFormatter

	// passageScorer scores passages
	passageScorer PassageScorer

	// breakIterator breaks text into passages
	breakIterator BreakIterator
}

// NewUnifiedHighlighter creates a new UnifiedHighlighter.
//
// Returns:
//   - a new UnifiedHighlighter instance
func NewUnifiedHighlighter() *UnifiedHighlighter {
	return &UnifiedHighlighter{
		maxLength:        10000,
		maxNumFragments:  5,
		passageFormatter: NewDefaultPassageFormatter(),
		passageScorer:    NewDefaultPassageScorer(),
		breakIterator:    NewSentenceBreakIterator(),
	}
}

// SetMaxLength sets the maximum length of text to analyze.
//
// Parameters:
//   - maxLength: the maximum length
func (uh *UnifiedHighlighter) SetMaxLength(maxLength int) {
	uh.maxLength = maxLength
}

// SetMaxNumFragments sets the maximum number of fragments to return.
//
// Parameters:
//   - max: the maximum number of fragments
func (uh *UnifiedHighlighter) SetMaxNumFragments(max int) {
	uh.maxNumFragments = max
}

// SetPassageFormatter sets the passage formatter.
//
// Parameters:
//   - formatter: the passage formatter to use
func (uh *UnifiedHighlighter) SetPassageFormatter(formatter PassageFormatter) {
	uh.passageFormatter = formatter
}

// SetPassageScorer sets the passage scorer.
//
// Parameters:
//   - scorer: the passage scorer to use
func (uh *UnifiedHighlighter) SetPassageScorer(scorer PassageScorer) {
	uh.passageScorer = scorer
}

// SetBreakIterator sets the break iterator.
//
// Parameters:
//   - breakIterator: the break iterator to use
func (uh *UnifiedHighlighter) SetBreakIterator(breakIterator BreakIterator) {
	uh.breakIterator = breakIterator
}

// SetTextFragmenter sets the fragmenter for this highlighter.
//
// Parameters:
//   - fragmenter: the fragmenter to use
func (uh *UnifiedHighlighter) SetTextFragmenter(fragmenter Fragmenter) {
	// UnifiedHighlighter uses BreakIterator for fragmentation
	// This method is for interface compliance
}

// SetFormatter sets the formatter for this highlighter.
//
// Parameters:
//   - formatter: the formatter to use
func (uh *UnifiedHighlighter) SetFormatter(formatter Formatter) {
	// UnifiedHighlighter uses PassageFormatter for formatting
	// This method is for interface compliance
}

// GetBestFragment returns the best fragment with highlighted terms.
//
// Parameters:
//   - text: the text to highlight
//   - maxNumFragments: the maximum number of fragments (ignored, always returns 1)
//
// Returns:
//   - the best highlighted fragment, or error if highlighting fails
func (uh *UnifiedHighlighter) GetBestFragment(text string, maxNumFragments int) (string, error) {
	terms := []string{} // Would come from query
	fragments, err := uh.GetBestFragmentsWithTerms(text, terms, 1)
	if err != nil {
		return "", err
	}
	if len(fragments) == 0 {
		return "", nil
	}
	return fragments[0], nil
}

// GetBestFragmentsWithTerms returns the best fragments with highlighted terms.
//
// Parameters:
//   - text: the text to highlight
//   - terms: the terms to highlight
//   - maxNumFragments: the maximum number of fragments to return
//
// Returns:
//   - the highlighted fragments, or error if highlighting fails
func (uh *UnifiedHighlighter) GetBestFragmentsWithTerms(text string, terms []string, maxNumFragments int) ([]string, error) {
	if text == "" || len(terms) == 0 {
		return []string{}, nil
	}

	// Limit text length
	if len(text) > uh.maxLength {
		text = text[:uh.maxLength]
	}

	// Create passages using break iterator
	passages := uh.createPassagesWithBreakIterator(text, terms)

	// Score passages
	for _, passage := range passages {
		passage.Score = uh.passageScorer.Score(passage)
	}

	// Format top passages
	fragments := make([]string, 0, len(passages))
	for _, passage := range passages {
		if len(fragments) >= maxNumFragments {
			break
		}
		formatted := uh.passageFormatter.Format(passage)
		if formatted != "" {
			fragments = append(fragments, formatted)
		}
	}

	return fragments, nil
}

// GetBestFragments returns the best fragments with highlighted terms.
//
// Parameters:
//   - text: the text to highlight
//   - maxNumFragments: the maximum number of fragments to return
//
// Returns:
//   - the highlighted fragments, or error if highlighting fails
func (uh *UnifiedHighlighter) GetBestFragments(text string, maxNumFragments int) ([]string, error) {
	return uh.GetBestFragmentsWithTerms(text, []string{}, maxNumFragments)
}

// createPassagesWithBreakIterator creates passages using the break iterator.
func (uh *UnifiedHighlighter) createPassagesWithBreakIterator(text string, terms []string) []*Passage {
	passages := make([]*Passage, 0)

	uh.breakIterator.SetText(text)

	// Find term positions
	for _, term := range terms {
		if term == "" {
			continue
		}

		// Find all occurrences of this term
		for i := 0; i < len(text); {
			idx := indexOfIgnoreCase(text, term, i)
			if idx == -1 {
				break
			}

			// Find passage boundaries using break iterator
			uh.breakIterator.SetText(text)
			passageStart := uh.findPreviousBoundary(idx)
			passageEnd := uh.findNextBoundary(idx + len(term))

			if passageEnd > len(text) {
				passageEnd = len(text)
			}

			passage := NewPassage(text[passageStart:passageEnd], passageStart, passageEnd)
			passage.AddMatch(idx-passageStart, idx-passageStart+len(term), term)

			passages = append(passages, passage)
			i = idx + 1
		}
	}

	return passages
}

// findPreviousBoundary finds the previous boundary position.
func (uh *UnifiedHighlighter) findPreviousBoundary(position int) int {
	uh.breakIterator.SetText(uh.breakIterator.(*SentenceBreakIterator).text)
	uh.breakIterator.(*SentenceBreakIterator).position = position
	return uh.breakIterator.Previous()
}

// findNextBoundary finds the next boundary position.
func (uh *UnifiedHighlighter) findNextBoundary(position int) int {
	uh.breakIterator.SetText(uh.breakIterator.(*SentenceBreakIterator).text)
	uh.breakIterator.(*SentenceBreakIterator).position = position
	return uh.breakIterator.Next()
}

// String returns a string representation of this highlighter.
//
// Returns:
//   - a string representation
func (uh *UnifiedHighlighter) String() string {
	return fmt.Sprintf("UnifiedHighlighter{maxLength=%d, maxNumFragments=%d}",
		uh.maxLength, uh.maxNumFragments)
}

// Ensure all highlighters implement the Highlighter interface
var (
	_ Highlighter = (*FastVectorHighlighter)(nil)
	_ Highlighter = (*PostingsHighlighter)(nil)
	_ Highlighter = (*UnifiedHighlighter)(nil)
)

// Helper function for UnifiedHighlighter
func (uh *UnifiedHighlighter) getText() string {
	if sbi, ok := uh.breakIterator.(*SentenceBreakIterator); ok {
		return sbi.text
	}
	return ""
}
