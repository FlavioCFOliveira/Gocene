// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import (
	"fmt"
	"strings"
)

// Passage represents a passage (snippet) of text with highlighted terms.
// This is used by advanced highlighters to represent a fragment of text
// that contains one or more highlighted terms.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.Passage.
type Passage struct {
	// Text is the passage text
	Text string

	// StartOffset is the start offset of the passage in the original text
	StartOffset int

	// EndOffset is the end offset of the passage in the original text
	EndOffset int

	// Score is the score of this passage
	Score float32

	// Matches contains the positions of highlighted terms in the passage
	Matches []PassageMatch
}

// PassageMatch represents a match (highlighted term) within a passage.
type PassageMatch struct {
	// Start is the start position of the match in the passage text
	Start int

	// End is the end position of the match in the passage text
	End int

	// Term is the matched term
	Term string
}

// NewPassage creates a new Passage.
//
// Parameters:
//   - text: the passage text
//   - startOffset: the start offset in the original text
//   - endOffset: the end offset in the original text
//
// Returns:
//   - a new Passage instance
func NewPassage(text string, startOffset, endOffset int) *Passage {
	return &Passage{
		Text:        text,
		StartOffset: startOffset,
		EndOffset:   endOffset,
		Matches:     make([]PassageMatch, 0),
	}
}

// AddMatch adds a match to this passage.
//
// Parameters:
//   - start: the start position of the match
//   - end: the end position of the match
//   - term: the matched term
func (p *Passage) AddMatch(start, end int, term string) {
	p.Matches = append(p.Matches, PassageMatch{
		Start: start,
		End:   end,
		Term:  term,
	})
}

// GetMatchCount returns the number of matches in this passage.
//
// Returns:
//   - the number of matches
func (p *Passage) GetMatchCount() int {
	return len(p.Matches)
}

// String returns a string representation of this passage.
//
// Returns:
//   - a string representation
func (p *Passage) String() string {
	return fmt.Sprintf("Passage{text='%s', matches=%d, score=%.2f}",
		p.Text, len(p.Matches), p.Score)
}

// PassageFormatter formats passages with highlighted terms.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.PassageFormatter.
type PassageFormatter interface {
	// Format formats a passage with highlighted terms.
	//
	// Parameters:
	//   - passage: the passage to format
	//
	// Returns:
	//   - the formatted passage text with highlighted terms
	Format(passage *Passage) string
}

// DefaultPassageFormatter is the default implementation of PassageFormatter.
// It highlights matched terms using HTML bold tags.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.DefaultPassageFormatter.
type DefaultPassageFormatter struct {
	// PreTag is the tag to insert before highlighted terms
	PreTag string

	// PostTag is the tag to insert after highlighted terms
	PostTag string
}

// NewDefaultPassageFormatter creates a new DefaultPassageFormatter.
//
// Returns:
//   - a new DefaultPassageFormatter instance with default HTML bold tags
func NewDefaultPassageFormatter() *DefaultPassageFormatter {
	return &DefaultPassageFormatter{
		PreTag:  "<b>",
		PostTag: "</b>",
	}
}

// NewDefaultPassageFormatterWithTags creates a new DefaultPassageFormatter with custom tags.
//
// Parameters:
//   - preTag: the tag to insert before highlighted terms
//   - postTag: the tag to insert after highlighted terms
//
// Returns:
//   - a new DefaultPassageFormatter instance
func NewDefaultPassageFormatterWithTags(preTag, postTag string) *DefaultPassageFormatter {
	return &DefaultPassageFormatter{
		PreTag:  preTag,
		PostTag: postTag,
	}
}

// Format formats a passage with highlighted terms.
//
// Parameters:
//   - passage: the passage to format
//
// Returns:
//   - the formatted passage text with highlighted terms
func (f *DefaultPassageFormatter) Format(passage *Passage) string {
	if passage == nil || passage.Text == "" {
		return ""
	}

	if len(passage.Matches) == 0 {
		return passage.Text
	}

	var result strings.Builder
	lastEnd := 0

	// Sort matches by start position to process them in order
	matches := make([]PassageMatch, len(passage.Matches))
	copy(matches, passage.Matches)
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].Start < matches[i].Start {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	for _, match := range matches {
		// Add text before this match
		if match.Start > lastEnd {
			result.WriteString(passage.Text[lastEnd:match.Start])
		}

		// Add highlighted match
		result.WriteString(f.PreTag)
		result.WriteString(passage.Text[match.Start:match.End])
		result.WriteString(f.PostTag)

		lastEnd = match.End
	}

	// Add remaining text
	if lastEnd < len(passage.Text) {
		result.WriteString(passage.Text[lastEnd:])
	}

	return result.String()
}

// Ensure DefaultPassageFormatter implements PassageFormatter
var _ PassageFormatter = (*DefaultPassageFormatter)(nil)

// PassageScorer scores passages based on their content.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.PassageScorer.
type PassageScorer interface {
	// Score scores a passage.
	//
	// Parameters:
	//   - passage: the passage to score
	//
	// Returns:
	//   - the score for the passage
	Score(passage *Passage) float32
}

// DefaultPassageScorer is the default implementation of PassageScorer.
// It scores passages based on the number and density of matches.
type DefaultPassageScorer struct {
	// MatchScore is the base score for each match
	MatchScore float32
}

// NewDefaultPassageScorer creates a new DefaultPassageScorer.
//
// Returns:
//   - a new DefaultPassageScorer instance
func NewDefaultPassageScorer() *DefaultPassageScorer {
	return &DefaultPassageScorer{
		MatchScore: 1.0,
	}
}

// Score scores a passage.
//
// Parameters:
//   - passage: the passage to score
//
// Returns:
//   - the score for the passage
func (s *DefaultPassageScorer) Score(passage *Passage) float32 {
	if passage == nil {
		return 0
	}

	score := float32(0)

	// Base score from number of matches
	score += float32(len(passage.Matches)) * s.MatchScore

	// Bonus for match density (matches per character)
	if len(passage.Text) > 0 {
		density := float32(len(passage.Matches)) / float32(len(passage.Text))
		score += density * 100 // Scale up density score
	}

	return score
}

// Ensure DefaultPassageScorer implements PassageScorer
var _ PassageScorer = (*DefaultPassageScorer)(nil)
