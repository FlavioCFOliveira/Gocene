// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm

import (
	"os"
	"testing"
)

const coredictMemPath = "/tmp/lucene/lucene/analysis/smartcn/src/resources/org/apache/lucene/analysis/cn/smart/hhmm/coredict.mem"
const bigramdictMemPath = "/tmp/lucene/lucene/analysis/smartcn/src/resources/org/apache/lucene/analysis/cn/smart/hhmm/bigramdict.mem"

// TestJavaSerialCoredict verifies that the Java serialization reader can
// parse coredict.mem and produces the expected array dimensions.
func TestJavaSerialCoredict(t *testing.T) {
	f, err := os.Open(coredictMemPath)
	if err != nil {
		t.Fatalf("coredict.mem not available: %v", err)
	}
	defer f.Close()

	s, err := newJavaObjStream(f)
	if err != nil {
		t.Fatalf("newJavaObjStream: %v", err)
	}

	wordIndexTable, err := s.ReadShortArray()
	if err != nil {
		t.Fatalf("ReadShortArray (wordIndexTable): %v", err)
	}
	if len(wordIndexTable) != 12071 {
		t.Errorf("wordIndexTable: want 12071 elements, got %d", len(wordIndexTable))
	}

	charIndexTable, err := s.ReadCharArray()
	if err != nil {
		t.Fatalf("ReadCharArray (charIndexTable): %v", err)
	}
	if len(charIndexTable) != 12071 {
		t.Errorf("charIndexTable: want 12071 elements, got %d", len(charIndexTable))
	}

	wordItemCharArrayTable, err := s.ReadChar3D()
	if err != nil {
		t.Fatalf("ReadChar3D (wordItem_charArrayTable): %v", err)
	}
	// GB2312_CHAR_NUM = 87*94 = 8178
	if len(wordItemCharArrayTable) != GB2312CharNum {
		t.Errorf("wordItem_charArrayTable: want %d elements, got %d", GB2312CharNum, len(wordItemCharArrayTable))
	}

	wordItemFreqTable, err := s.ReadInt2D()
	if err != nil {
		t.Fatalf("ReadInt2D (wordItem_frequencyTable): %v", err)
	}
	if len(wordItemFreqTable) != GB2312CharNum {
		t.Errorf("wordItem_frequencyTable: want %d elements, got %d", GB2312CharNum, len(wordItemFreqTable))
	}

	t.Logf("coredict.mem parsed: wordIndexTable=%d, charIndexTable=%d, charArray3D=%d, freqTable=%d",
		len(wordIndexTable), len(charIndexTable), len(wordItemCharArrayTable), len(wordItemFreqTable))
}

// TestJavaSerialBigramdict verifies that the Java serialization reader can
// parse bigramdict.mem.
func TestJavaSerialBigramdict(t *testing.T) {
	f, err := os.Open(bigramdictMemPath)
	if err != nil {
		t.Fatalf("bigramdict.mem not available: %v", err)
	}
	defer f.Close()

	s, err := newJavaObjStream(f)
	if err != nil {
		t.Fatalf("newJavaObjStream: %v", err)
	}

	bigramHashTable, err := s.ReadLongArray()
	if err != nil {
		t.Fatalf("ReadLongArray (bigramHashTable): %v", err)
	}
	// PRIME_BIGRAM_LENGTH = 402137
	if len(bigramHashTable) != 402137 {
		t.Errorf("bigramHashTable: want 402137 elements, got %d", len(bigramHashTable))
	}

	frequencyTable, err := s.ReadIntArray()
	if err != nil {
		t.Fatalf("ReadIntArray (frequencyTable): %v", err)
	}
	if len(frequencyTable) != 402137 {
		t.Errorf("frequencyTable: want 402137 elements, got %d", len(frequencyTable))
	}

	t.Logf("bigramdict.mem parsed: bigramHashTable=%d, frequencyTable=%d",
		len(bigramHashTable), len(frequencyTable))
}
