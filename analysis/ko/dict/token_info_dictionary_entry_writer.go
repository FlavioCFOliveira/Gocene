// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// idLimit is the maximum permitted left connection ID.
const idLimit = 8192

// TokenInfoDictionaryEntryWriter packs mecab-ko-dic CSV entries into the
// compact binary format consumed by TokenInfoMorphData.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.dict.TokenInfoDictionaryEntryWriter from
// Apache Lucene 10.4.0.
//
// mecab-ko-dic column layout:
//
//	0   surface
//	1   left cost
//	2   right cost
//	3   word cost
//	4   part of speech (e.g. NNG,*,T,*,*,*,*)
//	5   semantic class
//	6   T/F (coda present)
//	7   reading
//	8   POS type (*, Compound, Inflect, Preanalysis)
//	9   left POS
//	10  right POS
//	11  expression
//
// Binary layout per entry:
//
//	[0..1]  uint16 BE  — (leftID << 2) | posType.ordinal()
//	[2..3]  uint16 BE  — (rightID << 2) | flags
//	[4..5]  int16  BE  — word cost
//	then optional fields depending on posType/flags
type TokenInfoDictionaryEntryWriter struct {
	buffer  []byte
	posDict []string
}

// NewTokenInfoDictionaryEntryWriter creates an entry writer with the given
// initial buffer capacity hint.
func NewTokenInfoDictionaryEntryWriter(size int) *TokenInfoDictionaryEntryWriter {
	return &TokenInfoDictionaryEntryWriter{
		buffer: make([]byte, 0, size),
	}
}

// CurrentPosition returns the current write position (i.e., the byte offset
// that will become the word ID of the next entry).
func (w *TokenInfoDictionaryEntryWriter) CurrentPosition() int { return len(w.buffer) }

// Buffer returns the packed binary buffer (callers must not mutate).
func (w *TokenInfoDictionaryEntryWriter) Buffer() []byte { return w.buffer }

// POSDict returns the POS dictionary built so far (callers must not mutate).
func (w *TokenInfoDictionaryEntryWriter) POSDict() []string { return w.posDict }

// PutEntry packs one CSV entry into the buffer and returns the byte offset of
// the entry (its word ID).
//
// entry must have at least 12 columns matching the mecab-ko-dic layout
// described on the type doc.
func (w *TokenInfoDictionaryEntryWriter) PutEntry(entry []string) (int, error) {
	if len(entry) < 12 {
		return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: expected ≥12 columns, got %d", len(entry))
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

	if int(leftID) >= idLimit {
		return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: leftID %d >= %d", leftID, idLimit)
	}

	posType := ResolveTypeByName(entry[8])

	var leftPOS, rightPOS POSTag
	if posType == POSTypeMorpheme || posType == POSTypeCompound || entry[9] == "*" {
		leftPOS = ResolveTagByName(entry[4])
		rightPOS = leftPOS
	} else {
		leftPOS = ResolveTagByName(entry[9])
		rightPOS = ResolveTagByName(entry[10])
	}

	reading := ""
	if entry[7] != "*" && entry[0] != entry[7] {
		reading = entry[7]
	}

	expression := ""
	if entry[11] != "*" {
		expression = entry[11]
	}

	// Grow posDict to accommodate leftID.
	id := int(leftID)
	for len(w.posDict) <= id {
		w.posDict = append(w.posDict, "")
	}
	fullPOSData := leftPOS.String() + "," + entry[5]
	if existing := w.posDict[id]; existing != "" && existing != fullPOSData {
		return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: conflicting POS for leftID=%d: %q vs %q",
			id, existing, fullPOSData)
	}
	w.posDict[id] = fullPOSData

	// Build morphemes list.
	type morphemeEntry struct {
		posTag      POSTag
		surfaceForm string
	}
	var morphemes []morphemeEntry
	hasSinglePOS := (leftPOS == rightPOS)
	if posType != POSTypeMorpheme && expression != "" {
		for _, exprToken := range strings.Split(expression, "+") {
			parts := strings.Split(exprToken, "/")
			if len(parts) < 3 {
				continue
			}
			sf := strings.TrimSpace(parts[0])
			if sf == "" {
				continue
			}
			exprTag := ResolveTagByName(parts[1])
			morphemes = append(morphemes, morphemeEntry{posTag: exprTag, surfaceForm: sf})
			if leftPOS != exprTag {
				hasSinglePOS = false
			}
		}
	}

	// Compute flags.
	flags := uint16(0)
	if hasSinglePOS {
		flags |= HasSinglePOS
	}
	if posType == POSTypeMorpheme && len(reading) > 0 {
		flags |= HasReading
	}

	if posType.ordinalInt() >= 4 {
		return 0, fmt.Errorf("tokenInfoDictionaryEntryWriter: posType ordinal >= 4: %v", posType)
	}

	wordID := len(w.buffer)

	// Header: [leftID<<2 | posType.ordinal(), rightID<<2 | flags, wordCost]
	w.buffer = appendUint16BE(w.buffer, uint16(int(leftID)<<2|posType.ordinalInt()))
	w.buffer = appendUint16BE(w.buffer, uint16(int(rightID)<<2)|flags)
	w.buffer = appendInt16BE(w.buffer, wordCost)

	if posType == POSTypeMorpheme {
		if len(reading) > 0 {
			w.writeString(reading)
		}
	} else {
		if !hasSinglePOS {
			w.buffer = append(w.buffer, byte(rightPOS))
		}
		w.buffer = append(w.buffer, byte(len(morphemes)))
		for _, m := range morphemes {
			if !hasSinglePOS {
				w.buffer = append(w.buffer, byte(m.posTag))
			}
			if posType != POSTypeInflect {
				w.buffer = append(w.buffer, byte(len([]rune(m.surfaceForm))))
			} else {
				w.writeString(m.surfaceForm)
			}
		}
	}

	return wordID, nil
}

// BuildPOSTagTable converts the posDict into a []POSTag slice indexed by
// left connection ID, as expected by TokenInfoMorphData.
func (w *TokenInfoDictionaryEntryWriter) BuildPOSTagTable() []POSTag {
	result := make([]POSTag, len(w.posDict))
	for i, s := range w.posDict {
		if s == "" {
			result[i] = POSTagUNKNOWN
			continue
		}
		fields := analysis.CSVParse(s)
		if len(fields) >= 1 {
			result[i] = ResolveTagByName(fields[0])
		} else {
			result[i] = POSTagUNKNOWN
		}
	}
	return result
}

// writeString writes a length-prefixed UTF-16 big-endian string into buffer.
func (w *TokenInfoDictionaryEntryWriter) writeString(s string) {
	runes := []rune(s)
	w.buffer = append(w.buffer, byte(len(runes)))
	for _, ch := range runes {
		var b [2]byte
		binary.BigEndian.PutUint16(b[:], uint16(ch))
		w.buffer = append(w.buffer, b[:]...)
	}
}

// ordinalInt returns the ordinal of a POSType as int.
func (t POSType) ordinalInt() int { return int(t) }

func appendUint16BE(b []byte, v uint16) []byte {
	return append(b, byte(v>>8), byte(v))
}

func appendInt16BE(b []byte, v int16) []byte {
	return appendUint16BE(b, uint16(v))
}

func parseInt16(s string) (int16, error) {
	s = strings.TrimSpace(s)
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, err
	}
	return int16(n), nil
}
