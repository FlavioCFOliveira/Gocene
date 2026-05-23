// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm

import "golang.org/x/text/encoding/simplifiedchinese"

// GB2312 grid layout constants.
const (
	// GB2312FirstChar is the first Chinese character index in GB2312 (15×94).
	GB2312FirstChar = 1410

	// GB2312CharNum is the total number of Chinese characters (87×94).
	GB2312CharNum = 87 * 94

	// CharNumInFile is the number of Chinese characters with frequency data.
	CharNumInFile = 6768
)

// abstractDictionary provides GB2312 encoding helpers shared by
// WordDictionary and BigramDictionary.
//
// Go port of org.apache.lucene.analysis.cn.smart.hhmm.AbstractDictionary.
//
// Deviation: Java uses java.io.ObjectInputStream for deserialisation of the
// bundled .mem files. Go uses encoding/gob or raw binary reads; the concrete
// subclasses choose the strategy. GB2312 transcoding uses
// golang.org/x/text/encoding/simplifiedchinese instead of
// java.nio.charset.Charset.
type abstractDictionary struct{}

// getCCByGB2312Id transcodes a GB2312 position to a Unicode string.
func (d *abstractDictionary) getCCByGB2312Id(ccid int) string {
	if ccid < 0 || ccid > GB2312CharNum {
		return ""
	}
	cc1 := ccid/94 + 161
	cc2 := ccid%94 + 161
	buf := []byte{byte(cc1), byte(cc2)}
	dec := simplifiedchinese.GB18030.NewDecoder()
	out, err := dec.Bytes(buf)
	if err != nil {
		return ""
	}
	return string(out)
}

// getGB2312Id transcodes a Unicode character to its GB2312 position, or -1.
func (d *abstractDictionary) getGB2312Id(ch rune) int16 {
	enc := simplifiedchinese.GB18030.NewEncoder()
	src := string(ch)
	out, err := enc.Bytes([]byte(src))
	if err != nil || len(out) != 2 {
		return -1
	}
	b0 := int(out[0]&0xFF) - 161
	b1 := int(out[1]&0xFF) - 161
	return int16(b0*94 + b1)
}

// hash1Rune computes the 64-bit FNV-1a hash for a single rune.
//
// Must use signed int64 arithmetic to match Java's AbstractDictionary.hash1(char c)
// exactly. Java long arithmetic uses arithmetic right-shifts (>>) which differ
// from Go's logical right-shifts on uint64.
func (d *abstractDictionary) hash1Rune(c rune) int64 {
	const p = int64(1099511628211)
	// Java initialises hash to 0xcbf29ce484222325L, a negative signed long.
	hash := int64(-3750763034362895579)
	hash = (hash ^ int64(c&0x00FF)) * p
	hash = (hash ^ int64(c>>8)) * p
	hash += hash << 13
	hash ^= hash >> 7
	hash += hash << 3
	hash ^= hash >> 17
	hash += hash << 5
	return hash
}

// hash1 computes the 64-bit FNV-1a hash for a rune slice.
//
// The array variant does NOT apply the avalanche mixing at the end
// (the extra mixing lines are commented out in the Java source).
func (d *abstractDictionary) hash1(carray []rune) int64 {
	const p = int64(1099511628211)
	hash := int64(-3750763034362895579)
	for _, c := range carray {
		hash = (hash ^ int64(c&0x00FF)) * p
		hash = (hash ^ int64(c>>8)) * p
	}
	return hash
}

// hash2Rune computes the djb2 hash for a single rune.
func (d *abstractDictionary) hash2Rune(c rune) int {
	hash := 5381
	hash = (hash<<5) + hash + int(c&0x00FF)
	hash = (hash<<5) + hash + int(c>>8)
	return hash
}

// hash2 computes the djb2 hash for a rune slice.
func (d *abstractDictionary) hash2(carray []rune) int {
	hash := 5381
	for _, c := range carray {
		hash = (hash<<5) + hash + int(c&0x00FF)
		hash = (hash<<5) + hash + int(c>>8)
	}
	return hash
}
