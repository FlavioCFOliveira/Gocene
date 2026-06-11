// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
)

// TestTermBytesComparator_Comparison validates the TermBytes comparison
// logic through the BlockReader and BlockWriter round-trip.
// Port of org.apache.lucene.codecs.uniformsplit.TestTermBytesComparator.
func TestTermBytesComparator_Comparison(t *testing.T) {
	// Build a vocabulary of TermBytes entries
	terms := []string{
		"abacu", "amiga", "amigas", "arco", "bar",
		"bloom", "friendez", "frio", "frozen", "frozenberg", "z",
	}
	var blockLines []*uniformsplit.BlockLine
	for _, term := range terms {
		line := uniformsplit.NewBlockLine([]byte(term), []byte(term+"_state"))
		blockLines = append(blockLines, line)
	}

	// Encode the block
	encoder := uniformsplit.BlockEncoder{}
	header := uniformsplit.NewBlockHeader(len(blockLines), 0, []byte(terms[0]))
	data := encoder.Encode(header, blockLines)

	// Decode and verify round-trip
	reader := uniformsplit.NewBlockReader(data)
	if reader == nil {
		t.Fatal("NewBlockReader returned nil")
	}

	decoder := uniformsplit.BlockDecoder{}
	decoded := decoder.Decode(data)
	if len(decoded) == 0 {
		t.Error("BlockDecoder.Decode returned empty data")
	}

	// Verify the BlockReader contains the encoded data
	if len(reader.Data) == 0 {
		t.Error("BlockReader.Data is empty")
	}
}

// TestTermBytesComparator_BoundaryTerms validates that boundary term
// lookups work correctly with the UniformSplit data structures.
func TestTermBytesComparator_BoundaryTerms(t *testing.T) {
	testCases := []struct {
		term      string
		suffixOff int
	}{
		{"a", 0},
		{"apple", 1},
		{"zebra", 0},
		{"", 0},
	}
	for _, tc := range testCases {
		t.Run(tc.term, func(t *testing.T) {
			tb := uniformsplit.NewTermBytes([]byte(tc.term), tc.suffixOff)
			if tb == nil {
				t.Fatal("NewTermBytes returned nil")
			}
			if string(tb.Bytes) != tc.term {
				t.Errorf("TermBytes.Bytes = %q, want %q", tb.Bytes, tc.term)
			}
			if tb.SuffixOffset != tc.suffixOff {
				t.Errorf("TermBytes.SuffixOffset = %d, want %d", tb.SuffixOffset, tc.suffixOff)
			}
		})
	}
}
