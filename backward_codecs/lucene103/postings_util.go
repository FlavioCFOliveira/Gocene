// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene103

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BlockSize is the postings block size for the Lucene 10.3 format.
// Matches ForUtil.BLOCK_SIZE.
const BlockSize = 128

// ReadVIntBlock reads values that have been written using variable-length
// encoding and group-varint encoding instead of bit-packing.
//
// Port of org.apache.lucene.backward_codecs.lucene103.PostingsUtil#readVIntBlock
// (Lucene 10.4.0).
func ReadVIntBlock(
	docIn store.DataInput,
	docBuffer []int32,
	freqBuffer []int32,
	num int,
	indexHasFreq bool,
	decodeFreq bool,
) error {
	if num < 0 {
		return fmt.Errorf("lucene103 postings: negative num %d", num)
	}
	if num > len(docBuffer) {
		return fmt.Errorf("lucene103 postings: docBuffer too short: len=%d num=%d", len(docBuffer), num)
	}
	if indexHasFreq && decodeFreq && num > len(freqBuffer) {
		return fmt.Errorf("lucene103 postings: freqBuffer too short: len=%d num=%d", len(freqBuffer), num)
	}

	if err := util.ReadGroupVInts(docIn, docBuffer, num); err != nil {
		return err
	}

	if indexHasFreq && decodeFreq {
		for i := 0; i < num; i++ {
			freqBuffer[i] = docBuffer[i] & 0x01
			docBuffer[i] = int32(uint32(docBuffer[i]) >> 1)
			if freqBuffer[i] == 0 {
				v, err := store.ReadVInt(docIn)
				if err != nil {
					return err
				}
				freqBuffer[i] = v
			}
		}
	} else if indexHasFreq {
		for i := 0; i < num; i++ {
			docBuffer[i] = int32(uint32(docBuffer[i]) >> 1)
		}
	}
	return nil
}

// WriteVIntBlock writes freq buffer with variable-length encoding and doc
// buffer with group-varint encoding.
//
// Port of org.apache.lucene.backward_codecs.lucene103.PostingsUtil#writeVIntBlock
// (Lucene 10.4.0).
func WriteVIntBlock(
	docOut store.DataOutput,
	docBuffer []int32,
	freqBuffer []int32,
	num int,
	writeFreqs bool,
) error {
	if num < 0 {
		return fmt.Errorf("lucene103 postings: negative num %d", num)
	}
	if num > len(docBuffer) {
		return fmt.Errorf("lucene103 postings: docBuffer too short: len=%d num=%d", len(docBuffer), num)
	}
	if writeFreqs && num > len(freqBuffer) {
		return fmt.Errorf("lucene103 postings: freqBuffer too short: len=%d num=%d", len(freqBuffer), num)
	}

	if writeFreqs {
		for i := 0; i < num; i++ {
			if freqBuffer[i] == 1 {
				docBuffer[i] = (docBuffer[i] << 1) | 1
			} else {
				docBuffer[i] = docBuffer[i] << 1
			}
		}
	}

	scratch := make([]byte, util.GroupVIntMaxLengthPerGroup)
	if err := util.WriteGroupVInts(docOut, scratch, docBuffer, num); err != nil {
		return err
	}

	if writeFreqs {
		for i := 0; i < num; i++ {
			if freqBuffer[i] != 1 {
				if err := store.WriteVInt(docOut, freqBuffer[i]); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
