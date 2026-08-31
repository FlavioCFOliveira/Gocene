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

package lucene104

import (
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PostingsUtil provides utility functions to encode and decode postings blocks
// for the Lucene 10.4.0 codec.
//
// Ports org.apache.lucene.codecs.lucene104.PostingsUtil from Lucene 10.5.0.
type PostingsUtil struct{}

// ReadVIntBlock reads values that have been written using variable-length encoding and
// group-varint encoding instead of bit-packing.
//
// Ports PostingsUtil.readVIntBlock(IndexInput, int[], int[], int, boolean, boolean).
func (p PostingsUtil) ReadVIntBlock(
	docIn store.IndexInput,
	docBuffer []int32,
	freqBuffer []int32,
	num int,
	indexHasFreq bool,
	decodeFreq bool,
) error {
	if err := util.ReadGroupVInts(docIn, docBuffer, num); err != nil {
		return err
	}

	if indexHasFreq && decodeFreq {
		for i := 0; i < num; i++ {
			freqBuffer[i] = docBuffer[i] & 0x01
			docBuffer[i] >>= 1
			if freqBuffer[i] == 0 {
				v, err := docIn.ReadVInt()
				if err != nil {
					return err
				}
				freqBuffer[i] = v
			}
		}
	} else if indexHasFreq {
		for i := 0; i < num; i++ {
			docBuffer[i] >>= 1
		}
	}

	return nil
}

// WriteVIntBlock writes freq buffer with variable-length encoding and doc buffer with
// group-varint encoding.
//
// Ports PostingsUtil.writeVIntBlock(DataOutput, int[], int[], int, boolean).
func (p PostingsUtil) WriteVIntBlock(
	docOut store.DataOutput,
	docBuffer []int32,
	freqBuffer []int32,
	num int,
	writeFreqs bool,
) error {
	if writeFreqs {
		for i := 0; i < num; i++ {
			freqFlag := int32(0)
			if freqBuffer[i] == 1 {
				freqFlag = 1
			}
			docBuffer[i] = (docBuffer[i] << 1) | freqFlag
		}
	}

	// util.WriteGroupVInts requires a scratch buffer.
	scratch := make([]byte, util.GroupVIntMaxLengthPerGroup)
	if err := util.WriteGroupVInts(docOut, scratch, docBuffer, num); err != nil {
		return err
	}

	if writeFreqs {
		for i := 0; i < num; i++ {
			freq := freqBuffer[i]
			if freq != 1 {
				if err := docOut.WriteVInt(freq); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
