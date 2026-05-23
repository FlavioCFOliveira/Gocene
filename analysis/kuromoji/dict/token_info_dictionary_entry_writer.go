// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import (
	"fmt"
	"strings"
)

// idLimit is the maximum permitted left/right ID value.
const idLimit = 8192

// TokenInfoDictionaryEntryWriter packs system-dictionary CSV entries into the
// compact binary format consumed by TokenInfoMorphData.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.TokenInfoDictionaryEntryWriter from
// Apache Lucene 10.4.0.
//
// mecab-ipadic column layout:
//
//	0   surface
//	1   left cost
//	2   right cost
//	3   word cost
//	4–9 POS fields (pos1..pos4 + inflectionType + inflectionForm)
//	10  base form
//	11  reading
//	12  pronunciation
//
// Binary layout per entry (written to buffer):
//
//	[0..1]  uint16 BE  — (leftID << 3) | flags
//	[2..3]  int16  BE  — word cost
//	[4]     byte       — (sharedPrefix << 4) | suffixLen   (only when HasBaseform)
//	[5..]   chars      — UTF-16 BE suffix chars              (only when HasBaseform)
//	next    byte       — (len << 1) | kanaFlag               (only when HasReading)
//	next..  bytes      — kana-encoded or UTF-16 reading      (only when HasReading)
//	next    byte       — (len << 1) | kanaFlag               (only when HasPronunciation)
//	next..  bytes      — kana-encoded or UTF-16 pronunciation (only when HasPronunciation)
type TokenInfoDictionaryEntryWriter struct {
	buffer  []byte
	posDict []string
}

// NewTokenInfoDictionaryEntryWriter creates an entry writer with an initial
// buffer capacity hint.
func NewTokenInfoDictionaryEntryWriter(size int) *TokenInfoDictionaryEntryWriter {
	return &TokenInfoDictionaryEntryWriter{
		buffer: make([]byte, 0, size),
	}
}

// CurrentPosition returns the current write position in the buffer (i.e., the
// byte offset that will be assigned to the next entry as its word ID).
func (w *TokenInfoDictionaryEntryWriter) CurrentPosition() int {
	return len(w.buffer)
}

// PosDict returns the POS dictionary built so far. Callers must not mutate the
// returned slice.
func (w *TokenInfoDictionaryEntryWriter) PosDict() []string {
	return w.posDict
}

// Buffer returns the packed binary buffer. Callers must not mutate the
// returned slice.
func (w *TokenInfoDictionaryEntryWriter) Buffer() []byte {
	return w.buffer
}

// PutEntry packs one CSV entry into the buffer and returns the byte offset of
// the entry (its word ID).
//
// entry must have at least 13 columns matching the mecab-ipadic layout
// described on the type doc.
func (w *TokenInfoDictionaryEntryWriter) PutEntry(entry []string) (int, error) {
	if len(entry) < 13 {
		return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: expected 13 columns, got %d", len(entry))
	}

	leftID, err := parseInt16(entry[1])
	if err != nil {
		return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: leftID: %w", err)
	}
	rightID, err := parseInt16(entry[2])
	if err != nil {
		return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: rightID: %w", err)
	}
	wordCost, err := parseInt16(entry[3])
	if err != nil {
		return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: wordCost: %w", err)
	}

	if leftID != rightID {
		return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: rightID(%d) != leftID(%d)", rightID, leftID)
	}
	if int(leftID) >= idLimit {
		return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: leftID %d >= %d", leftID, idLimit)
	}

	// Build POS string: pos1–pos4 joined by '-', then inflectionType and
	// inflectionForm appended as CSV.
	var sb strings.Builder
	for i := 4; i < 8; i++ {
		part := entry[i]
		if part == "*" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteByte('-')
		}
		sb.WriteString(part)
	}
	if sb.Len() == 0 {
		return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: POS fields are empty")
	}
	posData := sb.String()

	// fullPOSData = "<posData>,<inflType>,<inflForm>"
	sb.Reset()
	sb.WriteString(csvQuoteEscape(posData))
	sb.WriteByte(',')
	if entry[8] != "*" {
		sb.WriteString(csvQuoteEscape(entry[8]))
	}
	sb.WriteByte(',')
	if entry[9] != "*" {
		sb.WriteString(csvQuoteEscape(entry[9]))
	}
	fullPOSData := sb.String()

	baseForm := entry[10]
	reading := entry[11]
	pronunciation := entry[12]

	if baseForm == "" {
		return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: base form is empty")
	}

	// Grow posDict to accommodate leftID index.
	id := int(leftID)
	for len(w.posDict) <= id {
		w.posDict = append(w.posDict, "")
	}
	if existing := w.posDict[id]; existing != "" && existing != fullPOSData {
		return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: multiple POS entries for leftID=%d", id)
	}
	w.posDict[id] = fullPOSData

	// Compute flags.
	var flags uint16
	if baseForm != "*" && baseForm != entry[0] {
		flags |= HasBaseform
	}
	if reading != toKatakanaEntry(entry[0]) {
		flags |= HasReading
	}
	if pronunciation != reading {
		flags |= HasPronunciation
	}

	wordID := len(w.buffer)

	// Header: (leftID << 3) | flags  as uint16 BE, then word cost as int16 BE.
	header := uint16(id<<3) | flags
	w.buffer = appendUint16BE(w.buffer, header)
	w.buffer = appendUint16BE(w.buffer, uint16(wordCost))

	if flags&HasBaseform != 0 {
		if len([]rune(baseForm)) >= 16 {
			return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: base form %q length >= 16", baseForm)
		}
		srRunes := []rune(entry[0])
		bfRunes := []rune(baseForm)
		shared := sharedPrefixRunes(srRunes, bfRunes)
		suffix := len(bfRunes) - shared
		w.buffer = append(w.buffer, byte(shared<<4|suffix))
		for i := shared; i < len(bfRunes); i++ {
			w.buffer = appendUint16BE(w.buffer, uint16(bfRunes[i]))
		}
	}

	if flags&HasReading != 0 {
		rRunes := []rune(reading)
		if isKatakanaStr(rRunes) {
			w.buffer = append(w.buffer, byte(len(rRunes)<<1|1))
			w.buffer = writeKatakanaRunes(w.buffer, rRunes)
		} else {
			w.buffer = append(w.buffer, byte(len(rRunes)<<1))
			for _, ch := range rRunes {
				w.buffer = appendUint16BE(w.buffer, uint16(ch))
			}
		}
	}

	if flags&HasPronunciation != 0 {
		pRunes := []rune(pronunciation)
		if isKatakanaStr(pRunes) {
			w.buffer = append(w.buffer, byte(len(pRunes)<<1|1))
			w.buffer = writeKatakanaRunes(w.buffer, pRunes)
		} else {
			w.buffer = append(w.buffer, byte(len(pRunes)<<1))
			for _, ch := range pRunes {
				w.buffer = appendUint16BE(w.buffer, uint16(ch))
			}
		}
	}

	return wordID, nil
}

// --- helpers -----------------------------------------------------------------

func appendUint16BE(b []byte, v uint16) []byte {
	return append(b, byte(v>>8), byte(v))
}

func parseInt16(s string) (int16, error) {
	s = strings.TrimSpace(s)
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, err
	}
	return int16(n), nil
}

// isKatakanaStr reports whether all runes are in the katakana block [U+30A0..U+30FF].
func isKatakanaStr(r []rune) bool {
	for _, ch := range r {
		if ch < 0x30A0 || ch > 0x30FF {
			return false
		}
	}
	return true
}

// writeKatakanaRunes encodes katakana runes as single bytes (ch - 0x30A0).
func writeKatakanaRunes(b []byte, r []rune) []byte {
	for _, ch := range r {
		b = append(b, byte(ch-0x30A0))
	}
	return b
}

// toKatakanaEntry converts hiragana characters (U+3041..U+3096) to katakana
// by adding 0x60; other characters pass through unchanged.
func toKatakanaEntry(s string) string {
	runes := []rune(s)
	for i, ch := range runes {
		if ch > 0x3040 && ch < 0x3097 {
			runes[i] = ch + 0x60
		}
	}
	return string(runes)
}

// sharedPrefixRunes returns the length of the common rune prefix of left and right.
func sharedPrefixRunes(left, right []rune) int {
	n := len(left)
	if len(right) < n {
		n = len(right)
	}
	for i := 0; i < n; i++ {
		if left[i] != right[i] {
			return i
		}
	}
	return n
}

// csvQuoteEscape wraps s in double quotes and escapes inner double quotes by
// doubling them, mirroring Lucene's CSVUtil.quoteEscape.
func csvQuoteEscape(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
