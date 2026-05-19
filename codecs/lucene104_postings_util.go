// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// lucene104PostingsUtilScratch is the scratch buffer reused across
// writeLucene104VIntBlock calls. Each goroutine that writes postings
// allocates its own slice; the constant is exposed only so callers can
// match the Java MAX_LENGTH_PER_GROUP sizing.
const lucene104PostingsUtilScratchSize = util.GroupVIntMaxLengthPerGroup

// readLucene104VIntBlock reads values that have been written using
// variable-length encoding and group-varint encoding instead of bit-packing.
//
// Ports org.apache.lucene.codecs.lucene104.PostingsUtil#readVIntBlock from
// Lucene 10.4.0.
func readLucene104VIntBlock(
	docIn store.DataInput,
	docBuffer []int32,
	freqBuffer []int32,
	num int,
	indexHasFreq bool,
	decodeFreq bool,
) error {
	if num < 0 {
		return fmt.Errorf("lucene104 postings: negative num %d", num)
	}
	if num > len(docBuffer) {
		return fmt.Errorf("lucene104 postings: docBuffer length %d shorter than num %d", len(docBuffer), num)
	}
	if indexHasFreq && decodeFreq && num > len(freqBuffer) {
		return fmt.Errorf("lucene104 postings: freqBuffer length %d shorter than num %d", len(freqBuffer), num)
	}
	if err := util.ReadGroupVInts(docIn, docBuffer, num); err != nil {
		return err
	}
	if indexHasFreq && decodeFreq {
		// VariableLengthInput is required to decode the freq tail VInts; the
		// Java reference relies on DataInput.readVInt being available on every
		// DataInput implementation, which is the case in Gocene only when the
		// concrete type also satisfies store.VariableLengthInput.
		vlin, ok := docIn.(store.VariableLengthInput)
		if !ok {
			return fmt.Errorf("lucene104 postings: %T does not implement store.VariableLengthInput", docIn)
		}
		for i := 0; i < num; i++ {
			freqBuffer[i] = int32(uint32(docBuffer[i]) & 0x01)
			docBuffer[i] = int32(uint32(docBuffer[i]) >> 1)
			if freqBuffer[i] == 0 {
				v, err := vlin.ReadVInt()
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

// writeLucene104VIntBlock writes the freq buffer with variable-length
// encoding and the doc buffer with group-varint encoding.
//
// Ports org.apache.lucene.codecs.lucene104.PostingsUtil#writeVIntBlock from
// Lucene 10.4.0. The doc buffer is mutated in place when writeFreqs is true
// (each entry is shifted left by 1 with the low bit set when the matching
// freq is 1) to match the Java reference behaviour.
func writeLucene104VIntBlock(
	docOut store.DataOutput,
	docBuffer []int32,
	freqBuffer []int32,
	num int,
	writeFreqs bool,
) error {
	if num < 0 {
		return fmt.Errorf("lucene104 postings: negative num %d", num)
	}
	if num > len(docBuffer) {
		return fmt.Errorf("lucene104 postings: docBuffer length %d shorter than num %d", len(docBuffer), num)
	}
	if writeFreqs && num > len(freqBuffer) {
		return fmt.Errorf("lucene104 postings: freqBuffer length %d shorter than num %d", len(freqBuffer), num)
	}
	if writeFreqs {
		for i := 0; i < num; i++ {
			low := int32(0)
			if freqBuffer[i] == 1 {
				low = 1
			}
			docBuffer[i] = int32((uint32(docBuffer[i]) << 1) | uint32(low))
		}
	}
	scratch := make([]byte, lucene104PostingsUtilScratchSize)
	if err := util.WriteGroupVInts(docOut, scratch, docBuffer, num); err != nil {
		return err
	}
	if writeFreqs {
		vlout, ok := docOut.(store.VariableLengthOutput)
		if !ok {
			return fmt.Errorf("lucene104 postings: %T does not implement store.VariableLengthOutput", docOut)
		}
		for i := 0; i < num; i++ {
			freq := freqBuffer[i]
			if freq != 1 {
				if err := vlout.WriteVInt(freq); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
