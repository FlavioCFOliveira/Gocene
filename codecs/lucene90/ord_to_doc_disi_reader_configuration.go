// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import (
	"fmt"
)

type OrdToDocDISIReaderConfiguration struct {
	size                int
	jumpTableEntryCount int16
	addressesOffset     int64
	addressesLength     int64
	docsWithFieldOffset int64
	docsWithFieldLength int64
	denseRankPower      byte
	meta                lucene90.DirectMonotonicReaderMeta
}

func FromStoredMeta(inputMeta IndexInput, size int) (*OrdToDocDISIReaderConfiguration, error) {
	docsWithFieldOffset := inputMeta.ReadLong()
	docsWithFieldLength := inputMeta.ReadLong()
	jumpTableEntryCount := inputMeta.ReadShort()
	denseRankPower := inputMeta.ReadByte()

	var addressesOffset int64
	var blockShift int
	var meta lucene90.DirectMonotonicReaderMeta
	var addressesLength int64

	if docsWithFieldOffset > -1 {
		addressesOffset = inputMeta.ReadLong()
		blockShift = inputMeta.ReadVInt()

		var err error
		meta, err = lucene90.DirectMonotonicReaderLoadMeta(inputMeta, size, blockShift)
		if err != nil {
			return nil, err
		}
		addressesLength = inputMeta.ReadLong()
	}

	return &OrdToDocDISIReaderConfiguration{
		size:                size,
		jumpTableEntryCount: jumpTableEntryCount,
		addressesOffset:     addressesOffset,
		addressesLength:     addressesLength,
		docsWithFieldOffset: docsWithFieldOffset,
		docsWithFieldLength: docsWithFieldLength,
		denseRankPower:      denseRankPower,
		meta:                meta,
	}, nil
}

func (c *OrdToDocDISIReaderConfiguration) GetIndexedDISI(dataIn IndexInput) (*lucene90.IndexedDISI, error) {
	return lucene90.NewIndexedDISI(
		dataIn,
		c.docsWithFieldOffset,
		c.docsWithFieldLength,
		c.jumpTableEntryCount,
		c.denseRankPower,
		c.size), nil
}

func (c *OrdToDocDISIReaderConfiguration) GetDirectMonotonicReader(dataIn IndexInput) (lucene90.DirectMonotonicReader, error) {
	addressesData := dataIn.RandomAccessSlice(c.addressesOffset, c.addressesLength)
	return lucene90.NewDirectMonotonicReader(c.meta, addressesData)
}

func (c *OrdToDocDISIReaderConfiguration) IsEmpty() bool {
	return c.docsWithFieldOffset == -2
}

func (c *OrdToDocDISIReaderConfiguration) IsDense() bool {
	return c.docsWithFieldOffset == -1
}
