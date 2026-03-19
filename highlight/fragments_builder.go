// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import (
	"fmt"
	"strings"
)

// FragmentsBuilder builds highlighted fragments from FieldFragList.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.FragmentsBuilder.
type FragmentsBuilder struct {
	// preTag is the tag to insert before highlighted terms
	preTag string

	// postTag is the tag to insert after highlighted terms
	postTag string

	// encoder encodes the output text
	encoder Encoder
}

// NewFragmentsBuilder creates a new FragmentsBuilder.
//
// Returns:
//   - a new FragmentsBuilder instance with default HTML tags
func NewFragmentsBuilder() *FragmentsBuilder {
	return &FragmentsBuilder{
		preTag:  "<b>",
		postTag: "</b>",
		encoder: NewSimpleHTMLEncoder(),
	}
}

// NewFragmentsBuilderWithTags creates a new FragmentsBuilder with custom tags.
//
// Parameters:
//   - preTag: the tag to insert before highlighted terms
//   - postTag: the tag to insert after highlighted terms
//
// Returns:
//   - a new FragmentsBuilder instance
func NewFragmentsBuilderWithTags(preTag, postTag string) *FragmentsBuilder {
	return &FragmentsBuilder{
		preTag:  preTag,
		postTag: postTag,
		encoder: NewSimpleHTMLEncoder(),
	}
}

// SetEncoder sets the encoder for this builder.
//
// Parameters:
//   - encoder: the encoder to use
func (fb *FragmentsBuilder) SetEncoder(encoder Encoder) {
	fb.encoder = encoder
}

// CreateFragment creates a highlighted fragment from the given text and fragment info.
//
// Parameters:
//   - text: the original text
//   - fragInfo: the fragment info
//
// Returns:
//   - the highlighted fragment text
func (fb *FragmentsBuilder) CreateFragment(text string, fragInfo *WeightedFragInfo) string {
	if fragInfo == nil || text == "" {
		return ""
	}

	// Extract the fragment text
	start := fragInfo.StartOffset
	end := fragInfo.EndOffset
	if start < 0 {
		start = 0
	}
	if end > len(text) {
		end = len(text)
	}

	fragmentText := text[start:end]

	// Build the highlighted fragment
	var result strings.Builder
	lastEnd := 0

	// Sort sub-infos by start position
	subInfos := make([]SubInfo, len(fragInfo.SubInfos))
	copy(subInfos, fragInfo.SubInfos)
	for i := 0; i < len(subInfos); i++ {
		for j := i + 1; j < len(subInfos); j++ {
			if subInfos[j].StartOffset < subInfos[i].StartOffset {
				subInfos[i], subInfos[j] = subInfos[j], subInfos[i]
			}
		}
	}

	for _, subInfo := range subInfos {
		// Adjust positions to be relative to fragment start
		subStart := subInfo.StartOffset - start
		subEnd := subInfo.EndOffset - start

		if subStart < 0 {
			subStart = 0
		}
		if subEnd > len(fragmentText) {
			subEnd = len(fragmentText)
		}

		// Add text before this sub-info
		if subStart > lastEnd {
			result.WriteString(fb.encoder.EncodeText(fragmentText[lastEnd:subStart]))
		}

		// Add highlighted text
		result.WriteString(fb.preTag)
		result.WriteString(fb.encoder.EncodeText(fragmentText[subStart:subEnd]))
		result.WriteString(fb.postTag)

		lastEnd = subEnd
	}

	// Add remaining text
	if lastEnd < len(fragmentText) {
		result.WriteString(fb.encoder.EncodeText(fragmentText[lastEnd:]))
	}

	return result.String()
}

// CreateFragments creates highlighted fragments from the given text and fragment list.
//
// Parameters:
//   - text: the original text
//   - fragList: the fragment list
//   - maxNumFragments: the maximum number of fragments to return
//
// Returns:
//   - the highlighted fragment texts
func (fb *FragmentsBuilder) CreateFragments(text string, fragList *FieldFragList, maxNumFragments int) []string {
	if fragList == nil || text == "" || maxNumFragments <= 0 {
		return []string{}
	}

	// Get top fragments
	topFrags := fragList.GetTopFragments(maxNumFragments)

	// Create highlighted fragments
	fragments := make([]string, 0, len(topFrags))
	for _, fragInfo := range topFrags {
		fragment := fb.CreateFragment(text, fragInfo)
		if fragment != "" {
			fragments = append(fragments, fragment)
		}
	}

	return fragments
}

// GetBestFragment returns the best (highest scoring) fragment.
//
// Parameters:
//   - text: the original text
//   - fragList: the fragment list
//
// Returns:
//   - the best highlighted fragment text, or empty string if no fragments
func (fb *FragmentsBuilder) GetBestFragment(text string, fragList *FieldFragList) string {
	fragments := fb.CreateFragments(text, fragList, 1)
	if len(fragments) == 0 {
		return ""
	}
	return fragments[0]
}

// GetBestFragments returns the best fragments.
//
// Parameters:
//   - text: the original text
//   - fragList: the fragment list
//   - maxNumFragments: the maximum number of fragments to return
//
// Returns:
//   - the best highlighted fragment texts
func (fb *FragmentsBuilder) GetBestFragments(text string, fragList *FieldFragList, maxNumFragments int) []string {
	return fb.CreateFragments(text, fragList, maxNumFragments)
}

// String returns a string representation of this fragments builder.
//
// Returns:
//   - a string representation
func (fb *FragmentsBuilder) String() string {
	return fmt.Sprintf("FragmentsBuilder{preTag='%s', postTag='%s'}", fb.preTag, fb.postTag)
}

// ScoreOrderFragmentsBuilder builds fragments ordered by score.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.ScoreOrderFragmentsBuilder.
type ScoreOrderFragmentsBuilder struct {
	*FragmentsBuilder
}

// NewScoreOrderFragmentsBuilder creates a new ScoreOrderFragmentsBuilder.
//
// Returns:
//   - a new ScoreOrderFragmentsBuilder instance
func NewScoreOrderFragmentsBuilder() *ScoreOrderFragmentsBuilder {
	return &ScoreOrderFragmentsBuilder{
		FragmentsBuilder: NewFragmentsBuilder(),
	}
}

// NewScoreOrderFragmentsBuilderWithTags creates a new ScoreOrderFragmentsBuilder with custom tags.
//
// Parameters:
//   - preTag: the tag to insert before highlighted terms
//   - postTag: the tag to insert after highlighted terms
//
// Returns:
//   - a new ScoreOrderFragmentsBuilder instance
func NewScoreOrderFragmentsBuilderWithTags(preTag, postTag string) *ScoreOrderFragmentsBuilder {
	return &ScoreOrderFragmentsBuilder{
		FragmentsBuilder: NewFragmentsBuilderWithTags(preTag, postTag),
	}
}

// CreateFragments creates fragments ordered by score.
//
// Parameters:
//   - text: the original text
//   - fragList: the fragment list
//   - maxNumFragments: the maximum number of fragments to return
//
// Returns:
//   - the highlighted fragment texts ordered by score
func (fb *ScoreOrderFragmentsBuilder) CreateFragments(text string, fragList *FieldFragList, maxNumFragments int) []string {
	// The parent implementation already returns fragments ordered by score
	return fb.FragmentsBuilder.CreateFragments(text, fragList, maxNumFragments)
}

// SimpleFragmentsBuilder builds simple fragments without scoring.
//
// This is a simple implementation for basic highlighting needs.
type SimpleFragmentsBuilder struct {
	preTag  string
	postTag string
	encoder Encoder
}

// NewSimpleFragmentsBuilder creates a new SimpleFragmentsBuilder.
//
// Returns:
//   - a new SimpleFragmentsBuilder instance
func NewSimpleFragmentsBuilder() *SimpleFragmentsBuilder {
	return &SimpleFragmentsBuilder{
		preTag:  "<b>",
		postTag: "</b>",
		encoder: NewSimpleHTMLEncoder(),
	}
}

// CreateFragment creates a highlighted fragment.
//
// Parameters:
//   - text: the original text
//   - start: the start position
//   - end: the end position
//   - terms: the terms to highlight
//
// Returns:
//   - the highlighted fragment text
func (fb *SimpleFragmentsBuilder) CreateFragment(text string, start, end int, terms []string) string {
	if start < 0 {
		start = 0
	}
	if end > len(text) {
		end = len(text)
	}
	if start >= end {
		return ""
	}

	fragmentText := text[start:end]

	// Build the highlighted fragment
	var result strings.Builder
	lastEnd := 0

	// Find and highlight terms
	for _, term := range terms {
		if term == "" {
			continue
		}

		// Find all occurrences of this term
		for i := 0; i < len(fragmentText); {
			idx := indexOfIgnoreCase(fragmentText, term, i)
			if idx == -1 {
				break
			}

			// Add text before this term
			if idx > lastEnd {
				result.WriteString(fb.encoder.EncodeText(fragmentText[lastEnd:idx]))
			}

			// Add highlighted term
			result.WriteString(fb.preTag)
			result.WriteString(fb.encoder.EncodeText(fragmentText[idx : idx+len(term)]))
			result.WriteString(fb.postTag)

			lastEnd = idx + len(term)
			i = lastEnd
		}
	}

	// Add remaining text
	if lastEnd < len(fragmentText) {
		result.WriteString(fb.encoder.EncodeText(fragmentText[lastEnd:]))
	}

	return result.String()
}

// SetEncoder sets the encoder for this builder.
//
// Parameters:
//   - encoder: the encoder to use
func (fb *SimpleFragmentsBuilder) SetEncoder(encoder Encoder) {
	fb.encoder = encoder
}

// SetTags sets the highlight tags.
//
// Parameters:
//   - preTag: the tag to insert before highlighted terms
//   - postTag: the tag to insert after highlighted terms
func (fb *SimpleFragmentsBuilder) SetTags(preTag, postTag string) {
	fb.preTag = preTag
	fb.postTag = postTag
}
