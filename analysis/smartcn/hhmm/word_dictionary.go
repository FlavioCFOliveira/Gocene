// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm

import (
	_ "embed"
	"bytes"
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis/smartcn"
)

//go:embed coredict.mem
var coredictMemData []byte

const (
	// PrimeIndexLength is the hash table size for word lookup.
	PrimeIndexLength = 12071
)

// WordDictionary is the SmartChineseAnalyzer word dictionary loaded from
// the bundled coredict.mem resource.
//
// Go port of org.apache.lucene.analysis.cn.smart.hhmm.WordDictionary.
//
// Deviation: Java ObjectInputStream serialisation is parsed by javaObjStream;
// the singleton is protected by sync.Once rather than synchronized().
type WordDictionary struct {
	abstractDictionary

	wordIndexTable         []int16   // hash → GB2312 row index
	charIndexTable         []rune    // hash → first character
	wordItemCharArrayTable [][][]rune // GB2312 row → sorted word suffixes
	wordItemFrequencyTable [][]int32  // GB2312 row → frequencies
}

var (
	wordDictOnce     sync.Once
	wordDictInstance *WordDictionary
)

// GetWordDictionary returns the singleton WordDictionary, loading the
// bundled resource on first call.
func GetWordDictionary() (*WordDictionary, error) {
	var loadErr error
	wordDictOnce.Do(func() {
		wd := &WordDictionary{}
		if err := wd.loadFromBytes(coredictMemData); err != nil {
			loadErr = fmt.Errorf("WordDictionary: load coredict.mem: %w", err)
			// Reset so next call retries (sync.Once cannot be reset; use fallback).
			return
		}
		wordDictInstance = wd
	})
	if loadErr != nil {
		return nil, loadErr
	}
	if wordDictInstance == nil {
		return nil, fmt.Errorf("WordDictionary: not initialised")
	}
	return wordDictInstance, nil
}

// loadFromBytes parses a Java-serialised coredict.mem byte slice.
func (wd *WordDictionary) loadFromBytes(data []byte) error {
	s, err := newJavaObjStream(bytes.NewReader(data))
	if err != nil {
		return err
	}

	shorts, err := s.ReadShortArray()
	if err != nil {
		return fmt.Errorf("wordIndexTable: %w", err)
	}
	wd.wordIndexTable = shorts

	chars, err := s.ReadCharArray()
	if err != nil {
		return fmt.Errorf("charIndexTable: %w", err)
	}
	wd.charIndexTable = chars

	charArr, err := s.ReadChar3D()
	if err != nil {
		return fmt.Errorf("wordItem_charArrayTable: %w", err)
	}
	wd.wordItemCharArrayTable = charArr

	freqArr, err := s.ReadInt2D()
	if err != nil {
		return fmt.Errorf("wordItem_frequencyTable: %w", err)
	}
	wd.wordItemFrequencyTable = freqArr

	return nil
}

// getWordItemTableIndex returns the hash-table slot for character c,
// or -1 if not found.
func (wd *WordDictionary) getWordItemTableIndex(c rune) int16 {
	h1 := int(wd.hash1Rune(c) % int64(PrimeIndexLength))
	h2 := wd.hash2Rune(c) % PrimeIndexLength
	if h1 < 0 {
		h1 += PrimeIndexLength
	}
	if h2 < 0 {
		h2 += PrimeIndexLength
	}
	index := h1
	i := 1
	for wd.charIndexTable[index] != 0 && wd.charIndexTable[index] != c && i < PrimeIndexLength {
		index = (h1 + i*h2) % PrimeIndexLength
		i++
	}
	if i < PrimeIndexLength && wd.charIndexTable[index] == c {
		return int16(index)
	}
	return -1
}

// findInTable binary-searches for charArray in the word table at knownHashIndex.
// knownHashIndex is the result of getWordItemTableIndex(charArray[0]).
// Returns the word item index, or -1 if not found.
func (wd *WordDictionary) findInTable(knownHashIndex int16, charArray []rune) int {
	if len(charArray) == 0 {
		return -1
	}
	rowIdx := wd.wordIndexTable[knownHashIndex]
	if int(rowIdx) >= len(wd.wordItemCharArrayTable) {
		return -1
	}
	items := wd.wordItemCharArrayTable[rowIdx]
	if items == nil {
		return -1
	}
	start, end := 0, len(items)-1
	for start <= end {
		mid := (start + end) / 2
		cmp := smartcn.CompareArray(items[mid], 0, charArray, 1)
		if cmp == 0 {
			return mid
		} else if cmp < 0 {
			start = mid + 1
		} else {
			end = mid - 1
		}
	}
	return -1
}

// GetPrefixMatch returns the index of the first word in the dictionary that
// uses charArray as a prefix. Returns -1 if not found.
func (wd *WordDictionary) GetPrefixMatch(charArray []rune) int {
	return wd.GetPrefixMatchFrom(charArray, 0)
}

// GetPrefixMatchFrom returns the nth word (starting from knownStart) that uses
// charArray as a prefix. Returns -1 if not found.
func (wd *WordDictionary) GetPrefixMatchFrom(charArray []rune, knownStart int) int {
	if len(charArray) == 0 {
		return -1
	}
	index := wd.getWordItemTableIndex(charArray[0])
	if index == -1 {
		return -1
	}
	rowIdx := wd.wordIndexTable[index]
	items := wd.wordItemCharArrayTable[rowIdx]
	if items == nil {
		return -1
	}
	start, end := knownStart, len(items)-1
	for start <= end {
		mid := (start + end) / 2
		cmp := smartcn.CompareArrayByPrefix(charArray, 1, items[mid], 0)
		if cmp == 0 {
			// Find the first item matching the prefix.
			for mid >= 0 && smartcn.CompareArrayByPrefix(charArray, 1, items[mid], 0) == 0 {
				mid--
			}
			mid++
			return mid
		} else if cmp < 0 {
			end = mid - 1
		} else {
			start = mid + 1
		}
	}
	return -1
}

// GetFrequency returns the frequency of charArray, or 0 if not found.
func (wd *WordDictionary) GetFrequency(charArray []rune) int {
	if len(charArray) == 0 {
		return 0
	}
	hashIndex := wd.getWordItemTableIndex(charArray[0])
	if hashIndex == -1 {
		return 0
	}
	itemIndex := wd.findInTable(hashIndex, charArray)
	if itemIndex == -1 {
		return 0
	}
	rowIdx := wd.wordIndexTable[hashIndex]
	return int(wd.wordItemFrequencyTable[rowIdx][itemIndex])
}

// IsEqual returns true if the dictionary entry at itemIndex for table
// charArray[0] equals charArray.
func (wd *WordDictionary) IsEqual(charArray []rune, itemIndex int) bool {
	hashIndex := wd.getWordItemTableIndex(charArray[0])
	if hashIndex == -1 {
		return false
	}
	rowIdx := wd.wordIndexTable[hashIndex]
	return smartcn.CompareArray(charArray, 1, wd.wordItemCharArrayTable[rowIdx][itemIndex], 0) == 0
}
