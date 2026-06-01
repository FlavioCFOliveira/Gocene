// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// Binary-resource loading for the Korean (Nori) built-in dictionaries.
//
// All nine .dat files produced by Apache Lucene 10.4.0's DictionaryBuilder are
// embedded at compile time using //go:embed.  The loaders parse those bytes
// byte-for-byte identically to the Java reference:
//
//	org.apache.lucene.analysis.morph.ConnectionCosts (reader)
//	org.apache.lucene.analysis.morph.BinaryDictionary (reader)
//	org.apache.lucene.analysis.morph.CharacterDefinition (reader)
//	org.apache.lucene.analysis.ko.dict.TokenInfoMorphData (reader)
//
// Each exported singleton is initialised exactly once via sync.Once; panics on
// load failure reproduce the Java behaviour (static initialiser throws an Error).

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/analysis/morph"
	"github.com/FlavioCFOliveira/Gocene/store"
	gofst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// ---- embedded binary resources -----------------------------------------------

//go:embed resources/ConnectionCosts.dat
var connCostsData []byte

//go:embed resources/CharacterDefinition.dat
var charDefData []byte

//go:embed resources/TokenInfoDictionary_buffer.dat
var tokenInfoBufferData []byte

//go:embed resources/TokenInfoDictionary_fst.dat
var tokenInfoFSTData []byte

//go:embed resources/TokenInfoDictionary_posDict.dat
var tokenInfoPosDictData []byte

//go:embed resources/TokenInfoDictionary_targetMap.dat
var tokenInfoTargetMapData []byte

//go:embed resources/UnknownDictionary_buffer.dat
var unknownBufferData []byte

//go:embed resources/UnknownDictionary_posDict.dat
var unknownPosDictData []byte

//go:embed resources/UnknownDictionary_targetMap.dat
var unknownTargetMapData []byte

// ---- codec header helpers ----------------------------------------------------

// codecMagic is the Lucene codec file magic (big-endian 0x3FD76C17).
const codecMagic int32 = 0x3FD76C17

// checkHeader reads and validates a Lucene codec header from r.  On success it
// returns the version; on failure it returns a descriptive error.
//
// Header layout:
//
//	4 bytes  big-endian magic (0x3FD76C17)
//	VInt     len(codec)
//	n bytes  codec name (ASCII)
//	4 bytes  big-endian version
func checkHeader(r *store.ByteArrayDataInput, codec string, minV, maxV int32) (int32, error) {
	magic, err := store.ReadInt32(r)
	if err != nil {
		return 0, fmt.Errorf("ko/dict: checkHeader(%s): read magic: %w", codec, err)
	}
	if magic != codecMagic {
		return 0, fmt.Errorf("ko/dict: checkHeader(%s): bad magic 0x%08X", codec, magic)
	}
	name, err := store.ReadString(r)
	if err != nil {
		return 0, fmt.Errorf("ko/dict: checkHeader(%s): read codec name: %w", codec, err)
	}
	if name != codec {
		return 0, fmt.Errorf("ko/dict: checkHeader(%s): codec name mismatch: got %q", codec, name)
	}
	version, err := store.ReadInt32(r)
	if err != nil {
		return 0, fmt.Errorf("ko/dict: checkHeader(%s): read version: %w", codec, err)
	}
	if version < minV || version > maxV {
		return 0, fmt.Errorf("ko/dict: checkHeader(%s): version %d out of [%d,%d]", codec, version, minV, maxV)
	}
	return version, nil
}

// ---- ConnectionCosts ---------------------------------------------------------

var (
	connCostsOnce sync.Once
	connCostsInst *ConnectionCosts
)

// GetConnectionCostsInstance returns the singleton ConnectionCosts loaded from
// the embedded ConnectionCosts.dat resource.
func GetConnectionCostsInstance() *ConnectionCosts {
	connCostsOnce.Do(func() {
		cc, err := loadConnectionCosts(connCostsData)
		if err != nil {
			panic(fmt.Sprintf("ko/dict: cannot load ConnectionCosts: %v", err))
		}
		connCostsInst = cc
	})
	return connCostsInst
}

// loadConnectionCosts parses a ConnectionCosts.dat byte slice.
//
// Binary format (from ConnectionCostsWriter.write):
//
//	codec header "ko_cc" version 1
//	VInt  forwardSize
//	VInt  backwardSize
//	backwardSize×forwardSize delta-zigzag VInts (each encodes one int16)
func loadConnectionCosts(data []byte) (*ConnectionCosts, error) {
	r := store.NewByteArrayDataInput(data)
	if _, err := checkHeader(r, ConnCostsHeader, Version, Version); err != nil {
		return nil, err
	}
	forwardSize, err := store.ReadVInt(r)
	if err != nil {
		return nil, fmt.Errorf("ko/dict: ConnectionCosts: forwardSize: %w", err)
	}
	backwardSize, err := store.ReadVInt(r)
	if err != nil {
		return nil, fmt.Errorf("ko/dict: ConnectionCosts: backwardSize: %w", err)
	}
	size := int(forwardSize) * int(backwardSize)
	matrix := make([]int16, size)
	var accum int32
	for i := 0; i < size; i++ {
		raw, err := store.ReadVInt(r)
		if err != nil {
			return nil, fmt.Errorf("ko/dict: ConnectionCosts: matrix[%d]: %w", i, err)
		}
		// zigzag decode: (raw >>> 1) ^ -(raw & 1)
		delta := int32(uint32(raw)>>1) ^ -(raw & 1)
		accum += delta
		matrix[i] = int16(accum)
	}
	return NewConnectionCosts(matrix, int(forwardSize)), nil
}

// ---- CharacterDefinition -----------------------------------------------------

var (
	charDefOnce sync.Once
	charDefInst *CharacterDefinition
)

// GetCharacterDefinitionInstance returns the singleton CharacterDefinition
// loaded from the embedded CharacterDefinition.dat resource.
func GetCharacterDefinitionInstance() *CharacterDefinition {
	charDefOnce.Do(func() {
		cd, err := loadCharacterDefinition(charDefData)
		if err != nil {
			panic(fmt.Sprintf("ko/dict: cannot load CharacterDefinition: %v", err))
		}
		charDefInst = cd
	})
	return charDefInst
}

// loadCharacterDefinition parses a CharacterDefinition.dat byte slice.
//
// Binary format (from CharacterDefinitionWriter.write):
//
//	codec header "ko_cd" version 1
//	0x10000 bytes  character-category map (one byte per BMP code point)
//	ClassCount bytes  per-class flags: bit0=invoke, bit1=group
func loadCharacterDefinition(data []byte) (*CharacterDefinition, error) {
	r := store.NewByteArrayDataInput(data)
	if _, err := checkHeader(r, CharDefHeader, Version, Version); err != nil {
		return nil, err
	}
	var categoryMap [0x10000]byte
	if err := r.ReadBytes(categoryMap[:]); err != nil {
		return nil, fmt.Errorf("ko/dict: CharacterDefinition: category map: %w", err)
	}
	invokeMap := make([]bool, CharClassCount)
	groupMap := make([]bool, CharClassCount)
	for i := 0; i < CharClassCount; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("ko/dict: CharacterDefinition: flags[%d]: %w", i, err)
		}
		invokeMap[i] = (b & 0x01) != 0
		groupMap[i] = (b & 0x02) != 0
	}
	base := morph.NewCharacterDefinitionRaw(categoryMap, invokeMap, groupMap)
	return &CharacterDefinition{CharacterDefinition: *base}, nil
}

// ---- BinaryDictionary helpers ------------------------------------------------

// loadBinaryDict parses a $buffer.dat + $targetMap.dat pair and returns the
// components needed to construct a morph.BinaryDictionary.
//
// targetMapData format (BinaryDictionaryWriter.writeTargetMap):
//
//	codec header targetMapCodec
//	VInt  targetMapLen        (total entries in the flat map array)
//	VInt  numSourceIds+1      (length of the offset array)
//	For each entry: VInt (delta<<1 | isNewSource)
//
// bufferData format (DictionaryEntryWriter.writeDictionary):
//
//	codec header dictCodec
//	VInt  size   (number of raw bytes)
//	size bytes   (packed morpheme data)
func loadBinaryDict(targetMapData []byte, targetMapCodec string,
	bufferData []byte, dictCodec string) (*morph.BinaryDictionary, error) {

	// --- target map ---
	tmr := store.NewByteArrayDataInput(targetMapData)
	if _, err := checkHeader(tmr, targetMapCodec, Version, Version); err != nil {
		return nil, err
	}
	tmLen, err := store.ReadVInt(tmr)
	if err != nil {
		return nil, fmt.Errorf("ko/dict: BinaryDict(%s): targetMapLen: %w", dictCodec, err)
	}
	offLen, err := store.ReadVInt(tmr)
	if err != nil {
		return nil, fmt.Errorf("ko/dict: BinaryDict(%s): offsetsLen: %w", dictCodec, err)
	}
	targetMap := make([]int, int(tmLen))
	targetMapOff := make([]int, int(offLen))
	var accum, sourceID int
	for ofs := 0; ofs < int(tmLen); ofs++ {
		raw, err := store.ReadVInt(tmr)
		if err != nil {
			return nil, fmt.Errorf("ko/dict: BinaryDict(%s): targetMap[%d]: %w", dictCodec, ofs, err)
		}
		if (raw & 1) != 0 {
			targetMapOff[sourceID] = ofs
			sourceID++
		}
		accum += int(raw >> 1)
		targetMap[ofs] = accum
	}
	if sourceID+1 != int(offLen) {
		return nil, fmt.Errorf("ko/dict: BinaryDict(%s): sourceId mismatch: got %d, offLen %d",
			dictCodec, sourceID, offLen)
	}
	targetMapOff[sourceID] = int(tmLen)

	// --- buffer ---
	dr := store.NewByteArrayDataInput(bufferData)
	if _, err := checkHeader(dr, dictCodec, Version, Version); err != nil {
		return nil, err
	}
	bufSize, err := store.ReadVInt(dr)
	if err != nil {
		return nil, fmt.Errorf("ko/dict: BinaryDict(%s): bufSize: %w", dictCodec, err)
	}
	buf := make([]byte, int(bufSize))
	if err := dr.ReadBytes(buf); err != nil {
		return nil, fmt.Errorf("ko/dict: BinaryDict(%s): buffer: %w", dictCodec, err)
	}

	return morph.NewBinaryDictionaryRaw(buf, targetMap, targetMapOff), nil
}

// ---- Korean POS dictionary loader -------------------------------------------

// loadKoPosDictBytes reads a $posDict.dat and returns a []POSTag slice.
//
// Korean POS dict format (TokenInfoDictionaryEntryWriter.writePosDict):
//
//	codec header posDictCodec
//	VInt  posSize
//	posSize bytes  one POSTag ordinal per entry
func loadKoPosDictBytes(data []byte, posDictCodec string) ([]POSTag, error) {
	r := store.NewByteArrayDataInput(data)
	if _, err := checkHeader(r, posDictCodec, Version, Version); err != nil {
		return nil, err
	}
	posSize, err := store.ReadVInt(r)
	if err != nil {
		return nil, fmt.Errorf("ko/dict: posDict(%s): posSize: %w", posDictCodec, err)
	}
	tags := make([]POSTag, int(posSize))
	for i := 0; i < int(posSize); i++ {
		b, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("ko/dict: posDict(%s): tags[%d]: %w", posDictCodec, i, err)
		}
		tags[i] = ResolveTagByByte(b)
	}
	return tags, nil
}

// ---- FST loader --------------------------------------------------------------

// loadFST reads a TokenInfoFST from the embedded $fst.dat bytes.
//
// The FST file contains:
//  1. FST metadata block (read by fst.ReadMetadata)
//  2. FST byte store (read by fst.NewFSTFromDataInput using metadata.NumBytes())
func loadFST(data []byte) (*TokenInfoFST, error) {
	r := store.NewByteArrayDataInput(data)
	meta, err := gofst.ReadMetadata[int64](r, gofst.PositiveIntOutputs())
	if err != nil {
		return nil, fmt.Errorf("ko/dict: FST metadata: %w", err)
	}
	f, err := gofst.NewFSTFromDataInput(meta, r)
	if err != nil {
		return nil, fmt.Errorf("ko/dict: FST data: %w", err)
	}
	return NewTokenInfoFSTFromFST(f), nil
}

// ---- TokenInfoDictionary singleton ------------------------------------------

var (
	tokenInfoOnce sync.Once
	tokenInfoInst *TokenInfoDictionary
)

// GetTokenInfoDictionaryInstance returns the singleton TokenInfoDictionary
// loaded from the embedded binary resources.
func GetTokenInfoDictionaryInstance() *TokenInfoDictionary {
	tokenInfoOnce.Do(func() {
		d, err := loadTokenInfoDictionary()
		if err != nil {
			panic(fmt.Sprintf("ko/dict: cannot load TokenInfoDictionary: %v", err))
		}
		tokenInfoInst = d
	})
	return tokenInfoInst
}

func loadTokenInfoDictionary() (*TokenInfoDictionary, error) {
	binaryDict, err := loadBinaryDict(
		tokenInfoTargetMapData, TargetMapHeader,
		tokenInfoBufferData, DictHeader,
	)
	if err != nil {
		return nil, err
	}
	posDict, err := loadKoPosDictBytes(tokenInfoPosDictData, PosDictHeader)
	if err != nil {
		return nil, err
	}
	morphAtts := NewTokenInfoMorphData(binaryDict.Buffer(), posDict)
	fstInst, err := loadFST(tokenInfoFSTData)
	if err != nil {
		return nil, err
	}
	return NewTokenInfoDictionary(fstInst, morphAtts, buildTargetMapSlices(binaryDict)), nil
}

// buildTargetMapSlices converts the flat BinaryDictionary lookup table to the
// [][]int form expected by TokenInfoDictionary.LookupWordIDs.
func buildTargetMapSlices(bd *morph.BinaryDictionary) [][]int {
	// Determine the number of source IDs from the offset table length - 1.
	// We probe increasing IDs until Lookup returns nil.
	var result [][]int
	for id := 0; ; id++ {
		ids := bd.Lookup(id)
		if ids == nil {
			break
		}
		cp := make([]int, len(ids))
		copy(cp, ids)
		result = append(result, cp)
	}
	return result
}

// ---- UnknownDictionary singleton --------------------------------------------

var (
	unknownDictOnce sync.Once
	unknownDictInst *UnknownDictionary
)

// GetUnknownDictionaryInstance returns the singleton UnknownDictionary loaded
// from the embedded binary resources.
func GetUnknownDictionaryInstance() *UnknownDictionary {
	unknownDictOnce.Do(func() {
		d, err := loadUnknownDictionary()
		if err != nil {
			panic(fmt.Sprintf("ko/dict: cannot load UnknownDictionary: %v", err))
		}
		unknownDictInst = d
	})
	return unknownDictInst
}

func loadUnknownDictionary() (*UnknownDictionary, error) {
	binaryDict, err := loadBinaryDict(
		unknownTargetMapData, TargetMapHeader,
		unknownBufferData, DictHeader,
	)
	if err != nil {
		return nil, err
	}
	posDict, err := loadKoPosDictBytes(unknownPosDictData, PosDictHeader)
	if err != nil {
		return nil, err
	}
	morphAtts := NewUnknownMorphData(binaryDict.Buffer(), posDict)
	charDef := GetCharacterDefinitionInstance()
	return NewUnknownDictionary(morphAtts, charDef, buildTargetMapSlices(binaryDict)), nil
}

