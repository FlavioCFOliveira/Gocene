// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package cjk

import (
	"io"
	"unicode/utf8"
)

const (
	hwKatakanaVoicedMark     = 0xff9e
	hwKatakanaSemiVoicedMark = 0xff9f
)

// CJKWidthCharFilter is a character-level filter that normalises CJK width
// differences:
//   - Folds fullwidth ASCII variants (0xFF01–0xFF5E) into basic latin.
//   - Folds halfwidth Katakana variants (0xFF65–0xFF9F) into standard kana.
//
// It is the stream-level counterpart of CJKWidthFilter.
//
// This is the Go port of
// org.apache.lucene.analysis.cjk.CJKWidthCharFilter from
// Apache Lucene 10.4.0.
//
// Deviation: Lucene's BaseCharFilter provides a per-position offset correction
// map (addOffCorrectMap). Gocene's analysis.CharFilter tracks only a cumulative
// delta. This implementation accumulates the same corrections cumulatively,
// which is semantically identical for typical use.
type CJKWidthCharFilter struct {
	input io.Reader

	// prevRune is the buffered output rune (-1 = none pending).
	prevRune rune
	hasPrev  bool

	// inputOff tracks runes consumed from input (used for offset correction).
	inputOff int

	// cumulativeDiff mirrors Lucene BaseCharFilter.cumulativeDiff.
	cumulativeDiff int

	// offsets maps input rune position → correction delta (compact encoding).
	// We use a slice of [pos, delta] pairs, kept sorted by pos.
	offsets []offEntry

	// readBuf buffers encoded bytes for pending runes.
	readBuf [utf8.UTFMax * 2]byte
	readLen int
	readPos int
}

type offEntry struct {
	pos   int
	delta int
}

// NewCJKWidthCharFilter creates a CJKWidthCharFilter wrapping r.
func NewCJKWidthCharFilter(r io.Reader) *CJKWidthCharFilter {
	return &CJKWidthCharFilter{
		input:    r,
		prevRune: -1,
	}
}

// addOffCorrectMap mirrors BaseCharFilter.addOffCorrectMap.
func (f *CJKWidthCharFilter) addOffCorrectMap(pos, delta int) {
	f.offsets = append(f.offsets, offEntry{pos, delta})
	f.cumulativeDiff = delta
}

// getLastCumulativeDiff mirrors BaseCharFilter.getLastCumulativeDiff.
func (f *CJKWidthCharFilter) getLastCumulativeDiff() int {
	return f.cumulativeDiff
}

// CorrectOffset adjusts an output offset back to the original input offset.
func (f *CJKWidthCharFilter) CorrectOffset(off int) int {
	// Walk the offset map to find cumulative correction up to off.
	delta := 0
	for _, e := range f.offsets {
		if e.pos > off {
			break
		}
		delta = e.delta
	}
	return off + delta
}

// nextRune reads one decoded rune from the underlying reader.
func (f *CJKWidthCharFilter) nextRune() (rune, error) {
	// Read the first byte to determine the UTF-8 sequence length.
	var buf [utf8.UTFMax]byte
	n, err := io.ReadFull(f.input, buf[:1])
	if n == 0 {
		if err == io.ErrUnexpectedEOF {
			err = io.EOF
		}
		return 0, err
	}

	b0 := buf[0]
	// Determine expected sequence length from first byte.
	var seqLen int
	switch {
	case b0&0x80 == 0:
		seqLen = 1
	case b0&0xe0 == 0xc0:
		seqLen = 2
	case b0&0xf0 == 0xe0:
		seqLen = 3
	case b0&0xf8 == 0xf0:
		seqLen = 4
	default:
		// invalid lead byte
		return utf8.RuneError, nil
	}

	if seqLen == 1 {
		return rune(b0), nil
	}

	// Read continuation bytes.
	n2, e2 := io.ReadFull(f.input, buf[1:seqLen])
	if n2 < seqLen-1 {
		if e2 == io.ErrUnexpectedEOF {
			e2 = io.EOF
		}
		return utf8.RuneError, e2
	}

	r, _ := utf8.DecodeRune(buf[:seqLen])
	return r, nil
}

// nextOutputRune returns the next normalised rune and advances state.
// Returns (0, io.EOF) at end of stream.
func (f *CJKWidthCharFilter) nextOutputRune() (rune, error) {
	for {
		ch, err := f.nextRune()
		if err == io.EOF {
			// flush prevRune if any
			ret := f.prevRune
			hasPrev := f.hasPrev
			f.prevRune = -1
			f.hasPrev = false
			if hasPrev {
				return ret, nil
			}
			return 0, io.EOF
		}
		if err != nil {
			return 0, err
		}

		f.inputOff++

		// If current rune is a voice mark, try to combine with prevRune.
		if ch == hwKatakanaVoicedMark || ch == hwKatakanaSemiVoicedMark {
			if f.hasPrev {
				combined := f.combineVoiceMark(f.prevRune, ch)
				if combined != f.prevRune {
					// successfully combined: emit combined char immediately
					f.prevRune = -1
					f.hasPrev = false
					prevCumDiff := f.getLastCumulativeDiff()
					f.addOffCorrectMap(f.inputOff-1-prevCumDiff, prevCumDiff+1)
					return combined, nil
				}
			}
		}

		// Emit the buffered prevRune (if any).
		var ret rune
		hasRet := false
		if f.hasPrev {
			ret = f.prevRune
			hasRet = true
		}

		// Normalise ch and store as new prevRune.
		if ch >= 0xff01 && ch <= 0xff5e {
			f.prevRune = ch - 0xfee0
		} else if ch >= 0xff65 && ch <= 0xff9f {
			f.prevRune = kanaNorm[ch-0xff65]
		} else {
			f.prevRune = ch
		}
		f.hasPrev = true

		if hasRet {
			return ret, nil
		}
		// loop to get another rune to return
	}
}

// Read implements io.Reader.
func (f *CJKWidthCharFilter) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		// If we have buffered encoded bytes, drain them first.
		if f.readPos < f.readLen {
			copied := copy(p[n:], f.readBuf[f.readPos:f.readLen])
			f.readPos += copied
			n += copied
			if f.readPos == f.readLen {
				f.readLen = 0
				f.readPos = 0
			}
			continue
		}

		r, err := f.nextOutputRune()
		if err == io.EOF {
			if n > 0 {
				return n, nil
			}
			return 0, io.EOF
		}
		if err != nil {
			return n, err
		}

		// Encode r into readBuf.
		sz := utf8.EncodeRune(f.readBuf[:], r)
		f.readLen = sz
		f.readPos = 0
	}
	return n, nil
}

// combineVoiceMark tries to combine a halfwidth voice mark with ch.
// Returns the combined rune or ch unchanged.
func (f *CJKWidthCharFilter) combineVoiceMark(ch, voiceMark rune) rune {
	if ch >= 0x30a6 && ch <= 0x30fd {
		var delta int8
		if voiceMark == hwKatakanaSemiVoicedMark {
			delta = kanaCombineHalfVoiced[ch-0x30a6]
		} else {
			delta = kanaCombineVoiced[ch-0x30a6]
		}
		return ch + rune(delta)
	}
	return ch
}

// CJKWidthCharFilterFactory creates CJKWidthCharFilter instances.
//
// This is the Go port of
// org.apache.lucene.analysis.cjk.CJKWidthCharFilterFactory from
// Apache Lucene 10.4.0.
type CJKWidthCharFilterFactory struct{}

// NewCJKWidthCharFilterFactory creates a CJKWidthCharFilterFactory.
func NewCJKWidthCharFilterFactory() *CJKWidthCharFilterFactory {
	return &CJKWidthCharFilterFactory{}
}

// Create creates a CJKWidthCharFilter wrapping r.
func (f *CJKWidthCharFilterFactory) Create(r io.Reader) io.Reader {
	return NewCJKWidthCharFilter(r)
}
