// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm

import (
	"fmt"

)

// HHMMSegmenter finds the optimal segmentation of a Chinese sentence using a
// Hidden Markov Model.
//
// Go port of org.apache.lucene.analysis.cn.smart.hhmm.HHMMSegmenter.
type HHMMSegmenter struct {
	wordDict *WordDictionary
}

// NewHHMMSegmenter constructs a new HHMMSegmenter. The WordDictionary must
// already be loaded.
func NewHHMMSegmenter() (*HHMMSegmenter, error) {
	wd, err := GetWordDictionary()
	if err != nil {
		return nil, fmt.Errorf("HHMMSegmenter: %w", err)
	}
	return &HHMMSegmenter{wordDict: wd}, nil
}

// Process segments sentence into a list of SegTokens representing the best
// segmentation.
func (h *HHMMSegmenter) Process(sentence string) ([]*SegToken, error) {
	segGraph := h.createSegGraph(sentence)
	biSegGraph, err := NewBiSegGraph(segGraph)
	if err != nil {
		return nil, err
	}
	return biSegGraph.GetShortPath(), nil
}

// createSegGraph builds a SegGraph for the given sentence.
func (h *HHMMSegmenter) createSegGraph(sentence string) *SegGraph {
	runes := []rune(sentence)
	length := len(runes)
	charTypeArray := getCharTypes(runes)
	segGraph := NewSegGraph()

	i := 0
	for i < length {
		hasFullWidth := false
		switch charTypeArray[i] {
		case CharTypeSpaceLike:
			i++

		case CharTypeSurrogate:
			// Surrogate-pair: treat as a single Chinese word token.
			charArray := []rune{runes[i]}
			token := NewSegToken(charArray, i, i+1, WordTypeChineseWord, 0)
			segGraph.AddToken(token)
			i++

		case CharTypeHanzi:
			j := i + 1
			wordBuf := make([]rune, 0, 8)
			wordBuf = append(wordBuf, runes[i])

			// Single character always added.
			charArray := []rune{runes[i]}
			frequency := h.wordDict.GetFrequency(charArray)
			token := NewSegToken(charArray, i, j, WordTypeChineseWord, frequency)
			segGraph.AddToken(token)

			foundIndex := h.wordDict.GetPrefixMatch(charArray)
			for j <= length && foundIndex != -1 {
				if h.wordDict.IsEqual(wordBuf, foundIndex) && len(wordBuf) > 1 {
					frequency = h.wordDict.GetFrequency(wordBuf)
					cp := make([]rune, len(wordBuf))
					copy(cp, wordBuf)
					token = NewSegToken(cp, i, j, WordTypeChineseWord, frequency)
					segGraph.AddToken(token)
				}

				// Skip spaces.
				for j < length && charTypeArray[j] == CharTypeSpaceLike {
					j++
				}

				if j < length && charTypeArray[j] == CharTypeHanzi {
					wordBuf = append(wordBuf, runes[j])
					cp := make([]rune, len(wordBuf))
					copy(cp, wordBuf)
					foundIndex = h.wordDict.GetPrefixMatchFrom(cp, foundIndex)
					j++
				} else {
					break
				}
			}
			i++

		case CharTypeFullwidthLetter:
			hasFullWidth = true
			fallthrough
		case CharTypeLetter:
			j := i + 1
			for j < length && (charTypeArray[j] == CharTypeLetter || charTypeArray[j] == CharTypeFullwidthLetter) {
				if charTypeArray[j] == CharTypeFullwidthLetter {
					hasFullWidth = true
				}
				j++
			}
			charArray := StringCharArray
			frequency := h.wordDict.GetFrequency(charArray)
			wordType := WordTypeString
			if hasFullWidth {
				wordType = WordTypeFullwidthString
			}
			token := NewSegToken(charArray, i, j, wordType, frequency)
			segGraph.AddToken(token)
			i = j

		case CharTypeFullwidthDigit:
			hasFullWidth = true
			fallthrough
		case CharTypeDigit:
			j := i + 1
			for j < length && (charTypeArray[j] == CharTypeDigit || charTypeArray[j] == CharTypeFullwidthDigit) {
				if charTypeArray[j] == CharTypeFullwidthDigit {
					hasFullWidth = true
				}
				j++
			}
			charArray := NumberCharArray
			frequency := h.wordDict.GetFrequency(charArray)
			wordType := WordTypeNumber
			if hasFullWidth {
				wordType = WordTypeFullwidthNumber
			}
			token := NewSegToken(charArray, i, j, wordType, frequency)
			segGraph.AddToken(token)
			i = j

		case CharTypeDelimiter:
			j := i + 1
			charArray := []rune{runes[i]}
			token := NewSegToken(charArray, i, j, WordTypeDelimiter, MaxFrequence)
			segGraph.AddToken(token)
			i = j

		default:
			j := i + 1
			charArray := StringCharArray
			frequency := h.wordDict.GetFrequency(charArray)
			token := NewSegToken(charArray, i, j, WordTypeString, frequency)
			segGraph.AddToken(token)
			i = j
		}
	}

	// Sentence-begin sentinel.
	charArray := StartCharArray
	frequency := h.wordDict.GetFrequency(charArray)
	token := NewSegToken(charArray, -1, 0, WordTypeSentenceBegin, frequency)
	segGraph.AddToken(token)

	// Sentence-end sentinel.
	charArray = EndCharArray
	frequency = h.wordDict.GetFrequency(charArray)
	token = NewSegToken(charArray, length, length+1, WordTypeSentenceEnd, frequency)
	segGraph.AddToken(token)

	return segGraph
}

// getCharTypes returns the CharType for each rune in the sentence.
func getCharTypes(runes []rune) []int {
	types := make([]int, len(runes))
	for i, r := range runes {
		types[i] = GetCharType(r)
	}
	return types
}
