// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm

import (
	"bytes"
	_ "embed"
	"fmt"
	"sync"
)

//go:embed bigramdict.mem
var bigramdictMemData []byte

const (
	// PrimeBigramLength is the hash table size for bigram lookup.
	PrimeBigramLength = 402137

	// WordSegmentChar separates the two words of a bigram in the hash key.
	WordSegmentChar = rune('@')
)

// BigramDictionary is the SmartChineseAnalyzer bigram dictionary loaded from
// the bundled bigramdict.mem resource.
//
// Go port of org.apache.lucene.analysis.cn.smart.hhmm.BigramDictionary.
//
// Deviation: singleton protected by sync.Once; hash uses signed int64 to match
// Java's long arithmetic (see AbstractDictionary hash notes).
type BigramDictionary struct {
	abstractDictionary

	bigramHashTable []int64
	frequencyTable  []int32
}

var (
	bigramDictOnce     sync.Once
	bigramDictInstance *BigramDictionary
)

// GetBigramDictionary returns the singleton BigramDictionary, loading the
// bundled resource on first call.
func GetBigramDictionary() (*BigramDictionary, error) {
	var loadErr error
	bigramDictOnce.Do(func() {
		bd := &BigramDictionary{}
		if err := bd.loadFromBytes(bigramdictMemData); err != nil {
			loadErr = fmt.Errorf("BigramDictionary: load bigramdict.mem: %w", err)
			return
		}
		bigramDictInstance = bd
	})
	if loadErr != nil {
		return nil, loadErr
	}
	if bigramDictInstance == nil {
		return nil, fmt.Errorf("BigramDictionary: not initialised")
	}
	return bigramDictInstance, nil
}

// loadFromBytes parses a Java-serialised bigramdict.mem byte slice.
func (bd *BigramDictionary) loadFromBytes(data []byte) error {
	s, err := newJavaObjStream(bytes.NewReader(data))
	if err != nil {
		return err
	}

	longs, err := s.ReadLongArray()
	if err != nil {
		return fmt.Errorf("bigramHashTable: %w", err)
	}
	bd.bigramHashTable = longs

	ints, err := s.ReadIntArray()
	if err != nil {
		return fmt.Errorf("frequencyTable: %w", err)
	}
	bd.frequencyTable = ints

	return nil
}

// getBigramItemIndex returns the index into frequencyTable for the bigram
// represented by carray, or -1 if not found.
func (bd *BigramDictionary) getBigramItemIndex(carray []rune) int {
	hashID := bd.hash1(carray)
	h1 := int(hashID % int64(PrimeBigramLength))
	h2 := bd.hash2(carray) % PrimeBigramLength
	if h1 < 0 {
		h1 += PrimeBigramLength
	}
	if h2 < 0 {
		h2 += PrimeBigramLength
	}
	index := h1
	for i := 1; bd.bigramHashTable[index] != 0 && bd.bigramHashTable[index] != hashID && i < PrimeBigramLength; i++ {
		index = (h1 + i*h2) % PrimeBigramLength
	}
	if bd.bigramHashTable[index] == hashID {
		return index
	}
	return -1
}

// GetFrequency returns the frequency of the bigram represented by carray.
func (bd *BigramDictionary) GetFrequency(carray []rune) int {
	index := bd.getBigramItemIndex(carray)
	if index != -1 {
		return int(bd.frequencyTable[index])
	}
	return 0
}
