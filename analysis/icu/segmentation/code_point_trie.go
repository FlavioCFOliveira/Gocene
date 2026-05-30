// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation

import (
	"encoding/binary"
	"fmt"
)

// codePointTrie is a Go port of ICU4J's com.ibm.icu.util.CodePointTrie, the
// immutable code-point -> value lookup structure used by the compiled RBBI
// character-category table (the "Tri3" section of a .brk blob).
//
// Reference: unicode-org/icu (tag release-70-1)
// icu4j/main/classes/core/src/com/ibm/icu/util/CodePointTrie.java.
//
// The on-disk format is the same one ICU's UCPTrie serialiser emits. Gocene
// only needs the FAST trie type (the RBBI char-category trie is always FAST;
// see RBBIDataWrapper.fromBinary which passes CodePointTrie.Type.FAST). All
// three value widths (8/16/32-bit) are supported for completeness; the RBBI
// tries Lucene ships use the 8-bit width.
//
// Deviation: this implementation reads only the FAST type. A SMALL trie would
// be rejected by parseCodePointTrie with an error. ICU's RBBI compiler never
// emits SMALL tries for break rules, so this is safe for the .brk contract.
type codePointTrie struct {
	// index is the serialized index table (uint16 entries).
	index []uint16

	// One of the following holds the data table, selected by valueWidth.
	data8  []byte   // valueWidth == cptBits8
	data16 []uint16 // valueWidth == cptBits16
	data32 []uint32 // valueWidth == cptBits32

	valueWidth int
	highStart  int
	dataLength int
	highValue  int // value for code points >= highStart (data[dataLength-2])
	errorValue int // value for out-of-range code points (data[dataLength-1])
}

// Value-width selectors, matching CodePointTrie.ValueWidth ordinals.
const (
	cptBits16 = 0
	cptBits32 = 1
	cptBits8  = 2
)

// Trie binary-format constants ported verbatim from CodePointTrie.java.
const (
	cptSignature = 0x54726933 // "Tri3"

	cptFastShift = 6
	cptShift3    = 4
	cptShift2    = 5 + cptShift3 // 9
	cptShift1    = 5 + cptShift2 // 14

	cptFastDataBlockLength = 1 << cptFastShift // 64
	cptFastDataMask        = cptFastDataBlockLength - 1

	cptBMPIndexLength           = 0x10000 >> cptFastShift // 1024
	cptSmallLimit               = 0x1000
	cptSmallIndexLength         = cptSmallLimit >> cptFastShift // 32
	cptShift23                  = cptShift2 - cptShift3         // 5
	cptShift12                  = cptShift1 - cptShift2         // 5
	cptOmittedBMPIndex1Length   = 0x10000 >> cptShift1          // 4
	cptIndex2BlockLength        = 1 << cptShift12               // 32
	cptIndex2Mask               = cptIndex2BlockLength - 1
	cptIndex3BlockLength        = 1 << cptShift23 // 32
	cptIndex3Mask               = cptIndex3BlockLength - 1
	cptSmallDataBlockLength     = 1 << cptShift3 // 16
	cptSmallDataMask            = cptSmallDataBlockLength - 1
	cptErrorValueNegDataOffset  = 1
	cptHighValueNegDataOffset   = 2
	cptOptionsDataLengthMask    = 0xf000
	cptOptionsDataNullOffsetMsk = 0xf00
	cptTypeFast                 = 0
)

// trieHeaderSize is the size in bytes of the serialized UCPTrie header that
// precedes the index/data arrays.
const trieHeaderSize = 16

// parseCodePointTrie deserialises a CodePointTrie from buf, which must begin at
// the "Tri3" signature. order is the byte order recorded by the enclosing ICU
// data file (.brk blobs from Lucene are big-endian).
//
// It returns the trie and the number of bytes consumed (header + index + data),
// so the caller can advance past the trie section.
//
// Reference: CodePointTrie.fromBinary in CodePointTrie.java.
func parseCodePointTrie(buf []byte, order binary.ByteOrder) (*codePointTrie, int, error) {
	if len(buf) < trieHeaderSize {
		return nil, 0, fmt.Errorf("%w: trie header truncated (%d bytes)", ErrInvalidBRK, len(buf))
	}
	sig := order.Uint32(buf[0:4])
	if sig != cptSignature {
		return nil, 0, fmt.Errorf("%w: bad trie signature 0x%08x (want 0x%08x)", ErrInvalidBRK, sig, cptSignature)
	}

	options := int(order.Uint16(buf[4:6]))
	indexLength := int(order.Uint16(buf[6:8]))
	dataLength := int(order.Uint16(buf[8:10]))
	// index3NullOffset (buf[10:12]) and dataNullOffset low bits (buf[12:14])
	// are not needed for read-only lookups.
	shiftedHighStart := int(order.Uint16(buf[14:16]))

	typeInt := (options >> 6) & 3
	valueWidth := options & 7
	// High bits of dataLength and dataNullOffset are packed into options.
	dataLength |= (options & cptOptionsDataLengthMask) << 4
	// (dataNullOffset high bits are likewise in options but unused here.)
	_ = (options & cptOptionsDataNullOffsetMsk)

	if typeInt != cptTypeFast {
		return nil, 0, fmt.Errorf("%w: unsupported trie type %d (only FAST is supported)", ErrInvalidBRK, typeInt)
	}
	if valueWidth != cptBits16 && valueWidth != cptBits32 && valueWidth != cptBits8 {
		return nil, 0, fmt.Errorf("%w: unsupported trie value width %d", ErrInvalidBRK, valueWidth)
	}
	if indexLength < cptBMPIndexLength {
		return nil, 0, fmt.Errorf("%w: trie indexLength %d too small", ErrInvalidBRK, indexLength)
	}

	t := &codePointTrie{
		valueWidth: valueWidth,
		highStart:  shiftedHighStart << cptShift2,
		dataLength: dataLength,
	}

	pos := trieHeaderSize

	// index[]: indexLength uint16 entries.
	idxBytes := indexLength * 2
	if pos+idxBytes > len(buf) {
		return nil, 0, fmt.Errorf("%w: trie index out of range (need %d bytes)", ErrInvalidBRK, idxBytes)
	}
	t.index = make([]uint16, indexLength)
	for i := 0; i < indexLength; i++ {
		t.index[i] = order.Uint16(buf[pos+i*2 : pos+i*2+2])
	}
	pos += idxBytes

	// data[]: dataLength entries, width per valueWidth.
	switch valueWidth {
	case cptBits8:
		if pos+dataLength > len(buf) {
			return nil, 0, fmt.Errorf("%w: trie data8 out of range", ErrInvalidBRK)
		}
		t.data8 = make([]byte, dataLength)
		copy(t.data8, buf[pos:pos+dataLength])
		pos += dataLength
	case cptBits16:
		if pos+dataLength*2 > len(buf) {
			return nil, 0, fmt.Errorf("%w: trie data16 out of range", ErrInvalidBRK)
		}
		t.data16 = make([]uint16, dataLength)
		for i := 0; i < dataLength; i++ {
			t.data16[i] = order.Uint16(buf[pos+i*2 : pos+i*2+2])
		}
		pos += dataLength * 2
	case cptBits32:
		if pos+dataLength*4 > len(buf) {
			return nil, 0, fmt.Errorf("%w: trie data32 out of range", ErrInvalidBRK)
		}
		t.data32 = make([]uint32, dataLength)
		for i := 0; i < dataLength; i++ {
			t.data32[i] = order.Uint32(buf[pos+i*4 : pos+i*4+4])
		}
		pos += dataLength * 4
	}

	t.highValue = t.dataAt(dataLength - cptHighValueNegDataOffset)
	t.errorValue = t.dataAt(dataLength - cptErrorValueNegDataOffset)

	return t, pos, nil
}

// dataAt returns the value stored at data index i for the active value width.
func (t *codePointTrie) dataAt(i int) int {
	switch t.valueWidth {
	case cptBits8:
		return int(t.data8[i])
	case cptBits16:
		return int(t.data16[i])
	default: // cptBits32
		return int(t.data32[i])
	}
}

// Get returns the trie value for code point c, mirroring CodePointTrie.Fast.get.
// Out-of-range code points return the trie's error value.
func (t *codePointTrie) Get(c int) int {
	return t.dataAt(t.cpIndex(c))
}

// cpIndex maps a code point to a data-array index, mirroring
// CodePointTrie.Fast.cpIndex.
func (t *codePointTrie) cpIndex(c int) int {
	if c >= 0 {
		if c <= 0xffff {
			return t.fastIndex(c)
		} else if c <= 0x10ffff {
			return t.smallIndex(c)
		}
	}
	return t.dataLength - cptErrorValueNegDataOffset
}

// fastIndex is the BMP fast path: index[c>>6] + (c & 0x3f).
func (t *codePointTrie) fastIndex(c int) int {
	return int(t.index[c>>cptFastShift]) + (c & cptFastDataMask)
}

// smallIndex handles supplementary code points (c > 0xffff) for a FAST trie,
// mirroring CodePointTrie.smallIndex/internalSmallIndex with Type.FAST.
func (t *codePointTrie) smallIndex(c int) int {
	if c >= t.highStart {
		return t.dataLength - cptHighValueNegDataOffset
	}
	i1 := c >> cptShift1
	i1 += cptBMPIndexLength - cptOmittedBMPIndex1Length
	i3Block := int(t.index[int(t.index[i1])+((c>>cptShift2)&cptIndex2Mask)])
	i3 := (c >> cptShift3) & cptIndex3Mask
	var dataBlock int
	if (i3Block & 0x8000) == 0 {
		// 16-bit indexes.
		dataBlock = int(t.index[i3Block+i3])
	} else {
		// 18-bit indexes stored in groups of 9 entries per 8 indexes.
		i3Block = (i3Block & 0x7fff) + (i3 &^ 7) + (i3 >> 3)
		i3 &= 7
		dataBlock = (int(t.index[i3Block]) << (2 + (2 * i3))) & 0x30000
		i3Block++
		dataBlock |= int(t.index[i3Block+i3])
	}
	return dataBlock + (c & cptSmallDataMask)
}
