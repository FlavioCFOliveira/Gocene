// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import (
	"fmt"
	"sort"
)

// WeightedFragInfo represents a fragment with its weight/score.
// This is used to track fragments and their scores during highlighting.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.WeightedFragInfo.
type WeightedFragInfo struct {
	// StartOffset is the start offset of the fragment in the original text
	StartOffset int

	// EndOffset is the end offset of the fragment in the original text
	EndOffset int

	// Score is the weight/score of this fragment
	Score float32

	// SubInfos contains information about sub-fragments
	SubInfos []SubInfo
}

// SubInfo represents information about a sub-fragment.
type SubInfo struct {
	// SeqNum is the sequence number of this sub-fragment
	SeqNum int

	// StartOffset is the start offset of the sub-fragment
	StartOffset int

	// EndOffset is the end offset of the sub-fragment
	EndOffset int
}

// NewWeightedFragInfo creates a new WeightedFragInfo.
//
// Parameters:
//   - startOffset: the start offset in the original text
//   - endOffset: the end offset in the original text
//   - score: the weight/score of this fragment
//
// Returns:
//   - a new WeightedFragInfo instance
func NewWeightedFragInfo(startOffset, endOffset int, score float32) *WeightedFragInfo {
	return &WeightedFragInfo{
		StartOffset: startOffset,
		EndOffset:   endOffset,
		Score:       score,
		SubInfos:    make([]SubInfo, 0),
	}
}

// AddSubInfo adds a sub-fragment info.
//
// Parameters:
//   - seqNum: the sequence number
//   - startOffset: the start offset
//   - endOffset: the end offset
func (wfi *WeightedFragInfo) AddSubInfo(seqNum, startOffset, endOffset int) {
	wfi.SubInfos = append(wfi.SubInfos, SubInfo{
		SeqNum:      seqNum,
		StartOffset: startOffset,
		EndOffset:   endOffset,
	})
}

// GetLength returns the length of this fragment.
//
// Returns:
//   - the length of the fragment
func (wfi *WeightedFragInfo) GetLength() int {
	return wfi.EndOffset - wfi.StartOffset
}

// String returns a string representation of this fragment info.
//
// Returns:
//   - a string representation
func (wfi *WeightedFragInfo) String() string {
	return fmt.Sprintf("WeightedFragInfo{start=%d, end=%d, score=%.2f, subInfos=%d}",
		wfi.StartOffset, wfi.EndOffset, wfi.Score, len(wfi.SubInfos))
}

// FieldFragList represents a list of fragments for a field.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.FieldFragList.
type FieldFragList struct {
	// Field is the field name
	Field string

	// Fragments is the list of weighted fragment info
	Fragments []*WeightedFragInfo
}

// NewFieldFragList creates a new FieldFragList.
//
// Parameters:
//   - field: the field name
//
// Returns:
//   - a new FieldFragList instance
func NewFieldFragList(field string) *FieldFragList {
	return &FieldFragList{
		Field:     field,
		Fragments: make([]*WeightedFragInfo, 0),
	}
}

// AddFragment adds a fragment to this list.
//
// Parameters:
//   - fragInfo: the fragment info to add
func (ffl *FieldFragList) AddFragment(fragInfo *WeightedFragInfo) {
	ffl.Fragments = append(ffl.Fragments, fragInfo)
}

// GetFragmentCount returns the number of fragments.
//
// Returns:
//   - the number of fragments
func (ffl *FieldFragList) GetFragmentCount() int {
	return len(ffl.Fragments)
}

// GetTopFragments returns the top N fragments sorted by score.
//
// Parameters:
//   - n: the maximum number of fragments to return
//
// Returns:
//   - the top N fragments
func (ffl *FieldFragList) GetTopFragments(n int) []*WeightedFragInfo {
	if n <= 0 {
		return []*WeightedFragInfo{}
	}

	// Sort fragments by score (descending)
	sorted := make([]*WeightedFragInfo, len(ffl.Fragments))
	copy(sorted, ffl.Fragments)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	// Return top N
	if len(sorted) > n {
		return sorted[:n]
	}
	return sorted
}

// GetTotalScore returns the total score of all fragments.
//
// Returns:
//   - the total score
func (ffl *FieldFragList) GetTotalScore() float32 {
	total := float32(0)
	for _, frag := range ffl.Fragments {
		total += frag.Score
	}
	return total
}

// Clear clears all fragments.
func (ffl *FieldFragList) Clear() {
	ffl.Fragments = make([]*WeightedFragInfo, 0)
}

// String returns a string representation of this field frag list.
//
// Returns:
//   - a string representation
func (ffl *FieldFragList) String() string {
	return fmt.Sprintf("FieldFragList{field='%s', fragments=%d}",
		ffl.Field, len(ffl.Fragments))
}

// FragListBuilder builds FieldFragList instances.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.FragListBuilder.
type FragListBuilder struct {
	// fragCharSize is the target size of each fragment
	fragCharSize int
}

// NewFragListBuilder creates a new FragListBuilder.
//
// Parameters:
//   - fragCharSize: the target size of each fragment
//
// Returns:
//   - a new FragListBuilder instance
func NewFragListBuilder(fragCharSize int) *FragListBuilder {
	return &FragListBuilder{
		fragCharSize: fragCharSize,
	}
}

// CreateFieldFragList creates a FieldFragList from the given text and terms.
//
// Parameters:
//   - field: the field name
//   - text: the text to analyze
//   - terms: the terms to highlight
//
// Returns:
//   - a new FieldFragList instance
func (flb *FragListBuilder) CreateFieldFragList(field, text string, terms []string) *FieldFragList {
	fragList := NewFieldFragList(field)

	if text == "" || len(terms) == 0 {
		return fragList
	}

	// Find term positions and create fragments
	termPositions := flb.findTermPositions(text, terms)
	if len(termPositions) == 0 {
		return fragList
	}

	// Create fragments around term positions
	fragments := flb.createFragments(text, termPositions)
	for _, frag := range fragments {
		fragList.AddFragment(frag)
	}

	return fragList
}

// findTermPositions finds all positions of terms in the text.
func (flb *FragListBuilder) findTermPositions(text string, terms []string) []TermPosition {
	positions := make([]TermPosition, 0)

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
			positions = append(positions, TermPosition{
				Term:  term,
				Start: idx,
				End:   idx + len(term),
			})
			i = idx + 1
		}
	}

	// Sort by start position
	sort.Slice(positions, func(i, j int) bool {
		return positions[i].Start < positions[j].Start
	})

	return positions
}

// createFragments creates fragments from term positions.
func (flb *FragListBuilder) createFragments(text string, termPositions []TermPosition) []*WeightedFragInfo {
	fragments := make([]*WeightedFragInfo, 0)

	if len(termPositions) == 0 {
		return fragments
	}

	// Group nearby terms into fragments
	currentFrag := &WeightedFragInfo{
		StartOffset: termPositions[0].Start,
		EndOffset:   termPositions[0].End,
		Score:       1.0,
		SubInfos:    make([]SubInfo, 0),
	}
	currentFrag.AddSubInfo(0, termPositions[0].Start, termPositions[0].End)

	for i := 1; i < len(termPositions); i++ {
		pos := termPositions[i]

		// Check if this term is close enough to be in the same fragment
		if pos.Start-currentFrag.EndOffset < flb.fragCharSize {
			// Extend current fragment
			currentFrag.EndOffset = pos.End
			currentFrag.Score += 1.0
			currentFrag.AddSubInfo(i, pos.Start, pos.End)
		} else {
			// Start a new fragment
			fragments = append(fragments, currentFrag)
			currentFrag = &WeightedFragInfo{
				StartOffset: pos.Start,
				EndOffset:   pos.End,
				Score:       1.0,
				SubInfos:    make([]SubInfo, 0),
			}
			currentFrag.AddSubInfo(i, pos.Start, pos.End)
		}
	}

	// Add the last fragment
	fragments = append(fragments, currentFrag)

	return fragments
}

// TermPosition represents the position of a term in text.
type TermPosition struct {
	Term  string
	Start int
	End   int
}

// indexOfIgnoreCase finds the index of a substring ignoring case.
func indexOfIgnoreCase(s, substr string, start int) int {
	if start < 0 || start >= len(s) {
		return -1
	}

	if substr == "" {
		return start
	}

	lowerS := toLower(s[start:])
	lowerSubstr := toLower(substr)

	idx := indexOf(lowerS, lowerSubstr)
	if idx == -1 {
		return -1
	}

	return start + idx
}

// toLower converts a string to lowercase.
func toLower(s string) string {
	result := make([]rune, len(s))
	for i, ch := range s {
		if ch >= 'A' && ch <= 'Z' {
			result[i] = ch + ('a' - 'A')
		} else {
			result[i] = ch
		}
	}
	return string(result)
}

// indexOf finds the index of a substring.
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
