// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene95

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// OrdToDocDISIReaderConfiguration is the configuration for
// DirectMonotonicReader and IndexedDISI for reading sparse vectors. The format
// in the static writing methods adheres to the Lucene95HnswVectorsFormat.
//
// Port of org.apache.lucene.codecs.lucene95.OrdToDocDISIReaderConfiguration
// (Apache Lucene 10.5.0).
type OrdToDocDISIReaderConfiguration struct {
	size int
	// the following four variables used to read docIds encoded by IndexDISI
	// special values of docsWithFieldOffset are -1 and -2
	// -1 : dense
	// -2 : empty
	// other: sparse
	jumpTableEntryCount                      int16
	docsWithFieldOffset, docsWithFieldLength int64
	denseRankPower                           byte

	// the following four variables used to read ordToDoc encoded by DirectMonotonicWriter
	// note that only spare case needs to store ordToDoc
	addressesOffset, addressesLength int64
	meta                             *packed.DirectMonotonicMeta
}

// WriteStoredMeta writes out the docsWithField and ordToDoc mapping to the
// outputMeta and vectorData respectively. This is in adherence to the
// Lucene95HnswVectorsFormat.
//
// Within outputMeta the format is as follows:
//
//   - [int8] if equals to -2, empty - no vector values. If equals to -1,
//     dense – all documents have values for a field. If equals to 0, sparse –
//     some documents missing values.
//   - DocIds were encoded by IndexedDISI.writeBitSet(DocIdSetIterator,
//     IndexOutput, byte)
//   - OrdToDoc was encoded by DirectMonotonicWriter, note that only in sparse
//     case
//
// Within the vectorData the format is as follows:
//
//   - DocIds encoded by IndexedDISI.writeBitSet(DocIdSetIterator, IndexOutput,
//     byte), note that only in sparse case
//   - OrdToDoc was encoded by DirectMonotonicWriter, note that only in sparse
//     case
//
// Port of the static writeStoredMeta(int, IndexOutput, IndexOutput, int, int,
// DocsWithFieldSet).
func WriteStoredMeta(
	directMonotonicBlockShift int,
	outputMeta store.IndexOutput,
	vectorData store.IndexOutput,
	count int,
	maxDoc int,
	docsWithField *index.DocsWithFieldSet,
) error {
	if count == 0 {
		if err := outputMeta.WriteLong(-2); err != nil { // docsWithFieldOffset
			return err
		}
		if err := outputMeta.WriteLong(0); err != nil { // docsWithFieldLength
			return err
		}
		if err := outputMeta.WriteShort(-1); err != nil { // jumpTableEntryCount
			return err
		}
		return outputMeta.WriteByte(0xFF) // denseRankPower, (byte) -1
	} else if count == maxDoc {
		if err := outputMeta.WriteLong(-1); err != nil { // docsWithFieldOffset
			return err
		}
		if err := outputMeta.WriteLong(0); err != nil { // docsWithFieldLength
			return err
		}
		if err := outputMeta.WriteShort(-1); err != nil { // jumpTableEntryCount
			return err
		}
		return outputMeta.WriteByte(0xFF) // denseRankPower, (byte) -1
	}

	offset := vectorData.GetFilePointer()
	if err := outputMeta.WriteLong(offset); err != nil { // docsWithFieldOffset
		return err
	}
	jumpTableEntryCount, err := lucene90.WriteBitSetWithRank(
		docsWithField.Iterator(), vectorData, lucene90.DefaultDenseRankPower)
	if err != nil {
		return err
	}
	if err := outputMeta.WriteLong(vectorData.GetFilePointer() - offset); err != nil { // docsWithFieldLength
		return err
	}
	if err := outputMeta.WriteShort(jumpTableEntryCount); err != nil {
		return err
	}
	if err := outputMeta.WriteByte(lucene90.DefaultDenseRankPower); err != nil {
		return err
	}

	// write ordToDoc mapping
	start := vectorData.GetFilePointer()
	if err := outputMeta.WriteLong(start); err != nil {
		return err
	}
	if err := outputMeta.WriteVInt(int32(directMonotonicBlockShift)); err != nil {
		return err
	}
	// dense case and empty case do not need to store ordToMap mapping
	ordToDocWriter, err := packed.NewDirectMonotonicWriter(
		outputMeta, vectorData, int64(count), directMonotonicBlockShift)
	if err != nil {
		return err
	}
	iterator := docsWithField.Iterator()
	doc, err := iterator.NextDoc()
	for err == nil && doc != util.NO_MORE_DOCS {
		if err := ordToDocWriter.Add(int64(doc)); err != nil {
			return err
		}
		doc, err = iterator.NextDoc()
	}
	if err != nil {
		return err
	}
	if err := ordToDocWriter.Finish(); err != nil {
		return err
	}
	return outputMeta.WriteLong(vectorData.GetFilePointer() - start)
}

// FromStoredMeta reads in the necessary fields stored in the outputMeta to
// configure DirectMonotonicReader and IndexedDISI. inputMeta was previously
// written to via [WriteStoredMeta]; size is the number of vectors.
//
// Port of the static fromStoredMeta(IndexInput, int).
func FromStoredMeta(inputMeta store.IndexInput, size int) (*OrdToDocDISIReaderConfiguration, error) {
	docsWithFieldOffset, err := inputMeta.ReadLong()
	if err != nil {
		return nil, err
	}
	docsWithFieldLength, err := inputMeta.ReadLong()
	if err != nil {
		return nil, err
	}
	jumpTableEntryCount, err := inputMeta.ReadShort()
	if err != nil {
		return nil, err
	}
	denseRankPower, err := inputMeta.ReadByte()
	if err != nil {
		return nil, err
	}
	var addressesOffset int64
	var blockShift int
	var meta *packed.DirectMonotonicMeta
	var addressesLength int64
	if docsWithFieldOffset > -1 {
		if addressesOffset, err = inputMeta.ReadLong(); err != nil {
			return nil, err
		}
		shift, err := inputMeta.ReadVInt()
		if err != nil {
			return nil, err
		}
		blockShift = int(shift)
		if meta, err = packed.LoadDirectMonotonicMeta(inputMeta, int64(size), blockShift); err != nil {
			return nil, err
		}
		if addressesLength, err = inputMeta.ReadLong(); err != nil {
			return nil, err
		}
	}
	return newOrdToDocDISIReaderConfiguration(
		size,
		jumpTableEntryCount,
		addressesOffset,
		addressesLength,
		docsWithFieldOffset,
		docsWithFieldLength,
		denseRankPower,
		meta), nil
}

// newOrdToDocDISIReaderConfiguration renders the package-private constructor.
func newOrdToDocDISIReaderConfiguration(
	size int,
	jumpTableEntryCount int16,
	addressesOffset int64,
	addressesLength int64,
	docsWithFieldOffset int64,
	docsWithFieldLength int64,
	denseRankPower byte,
	meta *packed.DirectMonotonicMeta,
) *OrdToDocDISIReaderConfiguration {
	return &OrdToDocDISIReaderConfiguration{
		size:                size,
		jumpTableEntryCount: jumpTableEntryCount,
		addressesOffset:     addressesOffset,
		addressesLength:     addressesLength,
		docsWithFieldOffset: docsWithFieldOffset,
		docsWithFieldLength: docsWithFieldLength,
		denseRankPower:      denseRankPower,
		meta:                meta,
	}
}

// GetIndexedDISI returns the IndexedDISI for sparse values read from dataIn.
//
// Port of getIndexedDISI(IndexInput).
func (c *OrdToDocDISIReaderConfiguration) GetIndexedDISI(dataIn store.IndexInput) (*lucene90.IndexedDISI, error) {
	return lucene90.NewIndexedDISI(
		dataIn,
		c.docsWithFieldOffset,
		c.docsWithFieldLength,
		int(c.jumpTableEntryCount),
		c.denseRankPower,
		int64(c.size))
}

// GetDirectMonotonicReader returns the DirectMonotonicReader for sparse values
// read from dataIn.
//
// Port of getDirectMonotonicReader(IndexInput). Java obtains the addresses
// through IndexInput.randomAccessSlice(offset, length), whose default body
// slices the input and returns the slice itself when it already supports
// random access. store.IndexInput declares no randomAccessSlice, so this
// renders that first branch; a slice without random access is reported.
func (c *OrdToDocDISIReaderConfiguration) GetDirectMonotonicReader(dataIn store.IndexInput) (*packed.DirectMonotonicReader, error) {
	slice, err := dataIn.Slice("randomaccess", c.addressesOffset, c.addressesLength)
	if err != nil {
		return nil, err
	}
	addressesData, ok := slice.(packed.RandomAccessInput)
	if !ok {
		return nil, fmt.Errorf("lucene95: slice %T of %T does not support random access", slice, dataIn)
	}
	return packed.NewDirectMonotonicReader(c.meta, addressesData)
}

// IsEmpty reports whether the field is empty, with no vector values. If false,
// the field is either dense or sparse.
func (c *OrdToDocDISIReaderConfiguration) IsEmpty() bool {
	return c.docsWithFieldOffset == -2
}

// IsDense reports whether the field is dense: all documents have values for a
// field. If false, the field is sparse, some documents missing values.
func (c *OrdToDocDISIReaderConfiguration) IsDense() bool {
	return c.docsWithFieldOffset == -1
}
