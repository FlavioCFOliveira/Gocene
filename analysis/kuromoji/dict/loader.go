// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// Binary-resource loading for the Japanese (Kuromoji) built-in dictionaries.
//
// All nine .dat files produced by Apache Lucene 10.4.0's DictionaryBuilder are
// embedded at compile time using //go:embed.  The loaders parse those bytes
// byte-for-byte identically to the Java reference:
//
//	org.apache.lucene.analysis.morph.ConnectionCosts (reader)
//	org.apache.lucene.analysis.morph.BinaryDictionary (reader)
//	org.apache.lucene.analysis.morph.CharacterDefinition (reader)
//	org.apache.lucene.analysis.ja.dict.TokenInfoMorphData (reader)
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
		return 0, fmt.Errorf("kuromoji/dict: checkHeader(%s): read magic: %w", codec, err)
	}
	if magic != codecMagic {
		return 0, fmt.Errorf("kuromoji/dict: checkHeader(%s): bad magic 0x%08X", codec, magic)
	}
	name, err := store.ReadString(r)
	if err != nil {
		return 0, fmt.Errorf("kuromoji/dict: checkHeader(%s): read codec name: %w", codec, err)
	}
	if name != codec {
		return 0, fmt.Errorf("kuromoji/dict: checkHeader(%s): codec name mismatch: got %q", codec, name)
	}
	version, err := store.ReadInt32(r)
	if err != nil {
		return 0, fmt.Errorf("kuromoji/dict: checkHeader(%s): read version: %w", codec, err)
	}
	if version < minV || version > maxV {
		return 0, fmt.Errorf("kuromoji/dict: checkHeader(%s): version %d out of [%d,%d]", codec, version, minV, maxV)
	}
	return version, nil
}

// ---- ConnectionCosts singleton -----------------------------------------------

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
			panic(fmt.Sprintf("kuromoji/dict: cannot load ConnectionCosts: %v", err))
		}
		connCostsInst = cc
	})
	return connCostsInst
}

// loadConnectionCosts parses a ConnectionCosts.dat byte slice.
//
// Binary format (ConnectionCostsWriter.write):
//
//	codec header "kuromoji_cc" version 1
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
		return nil, fmt.Errorf("kuromoji/dict: ConnectionCosts: forwardSize: %w", err)
	}
	backwardSize, err := store.ReadVInt(r)
	if err != nil {
		return nil, fmt.Errorf("kuromoji/dict: ConnectionCosts: backwardSize: %w", err)
	}
	size := int(forwardSize) * int(backwardSize)
	matrix := make([]int16, size)
	var accum int32
	for i := 0; i < size; i++ {
		raw, err := store.ReadVInt(r)
		if err != nil {
			return nil, fmt.Errorf("kuromoji/dict: ConnectionCosts: matrix[%d]: %w", i, err)
		}
		// zigzag decode: (raw >>> 1) ^ -(raw & 1)
		delta := int32(uint32(raw)>>1) ^ -(raw & 1)
		accum += delta
		matrix[i] = int16(accum)
	}
	base := morph.NewConnectionCosts(matrix, int(forwardSize))
	return NewConnectionCosts(*base), nil
}

// ---- CharacterDefinition singleton -------------------------------------------

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
			panic(fmt.Sprintf("kuromoji/dict: cannot load CharacterDefinition: %v", err))
		}
		charDefInst = cd
	})
	return charDefInst
}

// loadCharacterDefinition parses a CharacterDefinition.dat byte slice.
//
// Binary format (CharacterDefinitionWriter.write):
//
//	codec header "kuromoji_cd" version 1
//	0x10000 bytes  character-category map
//	CharClassCount bytes  per-class flags: bit0=invoke, bit1=group
func loadCharacterDefinition(data []byte) (*CharacterDefinition, error) {
	r := store.NewByteArrayDataInput(data)
	if _, err := checkHeader(r, CharDefHeader, Version, Version); err != nil {
		return nil, err
	}
	var categoryMap [0x10000]byte
	if err := r.ReadBytes(categoryMap[:]); err != nil {
		return nil, fmt.Errorf("kuromoji/dict: CharacterDefinition: category map: %w", err)
	}
	invokeMap := make([]bool, CharClassCount)
	groupMap := make([]bool, CharClassCount)
	for i := 0; i < CharClassCount; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("kuromoji/dict: CharacterDefinition: flags[%d]: %w", i, err)
		}
		invokeMap[i] = (b & 0x01) != 0
		groupMap[i] = (b & 0x02) != 0
	}
	base := morph.NewCharacterDefinitionRaw(categoryMap, invokeMap, groupMap)
	return &CharacterDefinition{CharacterDefinition: *base}, nil
}

// ---- BinaryDictionary helpers ------------------------------------------------

// loadBinaryDict parses a $targetMap.dat + $buffer.dat pair.
//
// targetMapData format (BinaryDictionaryWriter.writeTargetMap):
//
//	codec header targetMapCodec
//	VInt  targetMapLen
//	VInt  numSourceIds+1
//	For each entry: VInt (delta<<1 | isNewSource)
//
// bufferData format (DictionaryEntryWriter.writeDictionary):
//
//	codec header dictCodec
//	VInt  size (bytes)
//	size raw bytes
func loadBinaryDict(targetMapData []byte, targetMapCodec string,
	bufferData []byte, dictCodec string) (*morph.BinaryDictionary, error) {

	// --- target map ---
	tmr := store.NewByteArrayDataInput(targetMapData)
	if _, err := checkHeader(tmr, targetMapCodec, Version, Version); err != nil {
		return nil, err
	}
	tmLen, err := store.ReadVInt(tmr)
	if err != nil {
		return nil, fmt.Errorf("kuromoji/dict: BinaryDict(%s): targetMapLen: %w", dictCodec, err)
	}
	offLen, err := store.ReadVInt(tmr)
	if err != nil {
		return nil, fmt.Errorf("kuromoji/dict: BinaryDict(%s): offsetsLen: %w", dictCodec, err)
	}
	targetMap := make([]int, int(tmLen))
	targetMapOff := make([]int, int(offLen))
	var accum, sourceID int
	for ofs := 0; ofs < int(tmLen); ofs++ {
		raw, err := store.ReadVInt(tmr)
		if err != nil {
			return nil, fmt.Errorf("kuromoji/dict: BinaryDict(%s): targetMap[%d]: %w", dictCodec, ofs, err)
		}
		if (raw & 1) != 0 {
			targetMapOff[sourceID] = ofs
			sourceID++
		}
		accum += int(raw >> 1)
		targetMap[ofs] = accum
	}
	if sourceID+1 != int(offLen) {
		return nil, fmt.Errorf("kuromoji/dict: BinaryDict(%s): sourceId mismatch: got %d, offLen %d",
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
		return nil, fmt.Errorf("kuromoji/dict: BinaryDict(%s): bufSize: %w", dictCodec, err)
	}
	buf := make([]byte, int(bufSize))
	if err := dr.ReadBytes(buf); err != nil {
		return nil, fmt.Errorf("kuromoji/dict: BinaryDict(%s): buffer: %w", dictCodec, err)
	}

	return morph.NewBinaryDictionaryRaw(buf, targetMap, targetMapOff), nil
}

// ---- Kuromoji POS dictionary loader -----------------------------------------

// loadJaPosDicts reads a $posDict.dat and returns the three POS string tables.
//
// Kuromoji POS dict format (TokenInfoDictionaryEntryWriter.writePosDict):
//
//	codec header "kuromoji_dict_pos" version 1
//	VInt  posSize
//	For each entry: readString posDict, readString inflTypeDict, readString inflFormDict
//	(empty inflType/inflForm strings are stored as "" and decoded to nil)
func loadJaPosDicts(data []byte) (posDict, inflTypeDict, inflFormDict []string, err error) {
	r := store.NewByteArrayDataInput(data)
	if _, err = checkHeader(r, PosDictHeader, Version, Version); err != nil {
		return
	}
	var posSize int32
	posSize, err = store.ReadVInt(r)
	if err != nil {
		err = fmt.Errorf("kuromoji/dict: posDict: posSize: %w", err)
		return
	}
	n := int(posSize)
	posDict = make([]string, n)
	inflTypeDict = make([]string, n)
	inflFormDict = make([]string, n)
	for j := 0; j < n; j++ {
		posDict[j], err = store.ReadString(r)
		if err != nil {
			err = fmt.Errorf("kuromoji/dict: posDict[%d] posDict: %w", j, err)
			return
		}
		inflTypeDict[j], err = store.ReadString(r)
		if err != nil {
			err = fmt.Errorf("kuromoji/dict: posDict[%d] inflTypeDict: %w", j, err)
			return
		}
		inflFormDict[j], err = store.ReadString(r)
		if err != nil {
			err = fmt.Errorf("kuromoji/dict: posDict[%d] inflFormDict: %w", j, err)
			return
		}
		// Java encodes null inflections as empty strings; decode them back to nil.
		if inflTypeDict[j] == "" {
			inflTypeDict[j] = ""
		}
		if inflFormDict[j] == "" {
			inflFormDict[j] = ""
		}
	}
	return
}

// ---- FST loader --------------------------------------------------------------

// loadFST reads a TokenInfoFST from the embedded $fst.dat bytes.
func loadFST(data []byte) (*gofst.FST[int64], error) {
	r := store.NewByteArrayDataInput(data)
	meta, err := gofst.ReadMetadata[int64](r, gofst.PositiveIntOutputs())
	if err != nil {
		return nil, fmt.Errorf("kuromoji/dict: FST metadata: %w", err)
	}
	f, err := gofst.NewFSTFromDataInput(meta, r)
	if err != nil {
		return nil, fmt.Errorf("kuromoji/dict: FST data: %w", err)
	}
	return f, nil
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
			panic(fmt.Sprintf("kuromoji/dict: cannot load TokenInfoDictionary: %v", err))
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
	posDict, inflTypeDict, inflFormDict, err := loadJaPosDicts(tokenInfoPosDictData)
	if err != nil {
		return nil, err
	}
	morphAttrs := NewTokenInfoMorphData(binaryDict.Buffer(), posDict, inflTypeDict, inflFormDict)
	realFST, err := loadFST(tokenInfoFSTData)
	if err != nil {
		return nil, err
	}
	return newTokenInfoDictionaryFromBinary(*binaryDict, realFST, morphAttrs), nil
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
			panic(fmt.Sprintf("kuromoji/dict: cannot load UnknownDictionary: %v", err))
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
	posDict, inflTypeDict, inflFormDict, err := loadJaPosDicts(unknownPosDictData)
	if err != nil {
		return nil, err
	}
	morphAttrs := NewUnknownMorphData(binaryDict.Buffer(), posDict, inflTypeDict, inflFormDict)
	charDef := GetCharacterDefinitionInstance()
	return NewUnknownDictionary(*binaryDict, morphAttrs, charDef), nil
}
