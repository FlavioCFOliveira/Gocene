// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.util.automaton.UTF32ToUTF8 from Apache Lucene
// 10.4.0 (Apache License 2.0).

package automaton

// UTF8 lead/trail boundary tables. Matching Lucene's MASK values:
// MASKS[1] = (2<<0)-1 = 1, MASKS[2] = 3, ... MASKS[6] = 63.
var utf8Masks = [8]byte{0, 1, 3, 7, 15, 31, 63, 127}

// startCodes/endCodes mark the inclusive code point ranges encoded by N-byte
// UTF-8 sequences for N=1..4 (in Lucene's order).
var utf8StartCodes = [4]int{0, 128, 2048, 65536}
var utf8EndCodes = [4]int{127, 2047, 65535, 1114111}

// utf8Byte represents one byte of a multi-byte UTF-8 sequence: value is the
// raw byte and bits is the number of payload bits at this position.
type utf8Byte struct {
	value byte
	bits  byte
}

// utf8Sequence buffers a single Unicode code point as 1-4 UTF-8 bytes.
type utf8Sequence struct {
	bytes [4]utf8Byte
	len   int
}

func (s *utf8Sequence) byteAt(i int) int  { return int(s.bytes[i].value) & 0xFF }
func (s *utf8Sequence) numBits(i int) int { return int(s.bytes[i].bits) }

// set encodes c into bytes/len.
func (s *utf8Sequence) set(c int) {
	switch {
	case c < 128:
		s.bytes[0].value = byte(c)
		s.bytes[0].bits = 7
		s.len = 1
	case c < 2048:
		s.bytes[0].value = byte((6 << 5) | (c >> 6))
		s.bytes[0].bits = 5
		s.setRest(c, 1)
		s.len = 2
	case c < 65536:
		s.bytes[0].value = byte((14 << 4) | (c >> 12))
		s.bytes[0].bits = 4
		s.setRest(c, 2)
		s.len = 3
	default:
		s.bytes[0].value = byte((30 << 3) | (c >> 18))
		s.bytes[0].bits = 3
		s.setRest(c, 3)
		s.len = 4
	}
}

// setFirstByte writes only bytes[0] and len, used to derive boundaries.
func (s *utf8Sequence) setFirstByte(c int) {
	switch {
	case c < 128:
		s.bytes[0].value = byte(c)
		s.len = 1
	case c < 2048:
		s.bytes[0].value = byte((6 << 5) | (c >> 6))
		s.len = 2
	case c < 65536:
		s.bytes[0].value = byte((14 << 4) | (c >> 12))
		s.len = 3
	default:
		s.bytes[0].value = byte((30 << 3) | (c >> 18))
		s.len = 4
	}
}

func (s *utf8Sequence) setRest(c, numBytes int) {
	for i := 0; i < numBytes; i++ {
		s.bytes[numBytes-i].value = byte(128 | (c & int(utf8Masks[6])))
		s.bytes[numBytes-i].bits = 6
		c >>= 6
	}
}

// UTF32ToUTF8 converts code-point automata to their UTF-8 byte equivalents.
// The conversion may produce a non-deterministic automaton; callers requiring
// a DFA should determinize the result.
type UTF32ToUTF8 struct {
	startUTF8 utf8Sequence
	endUTF8   utf8Sequence
	tmpUTF8a  utf8Sequence
	tmpUTF8b  utf8Sequence
	out       *Builder
}

// NewUTF32ToUTF8 constructs an empty converter.
func NewUTF32ToUTF8() *UTF32ToUTF8 { return &UTF32ToUTF8{} }

// Convert returns a new automaton with the same language as utf32, but with
// transitions labelled by UTF-8 bytes.
func (u *UTF32ToUTF8) Convert(utf32 *Automaton) *Automaton {
	if utf32.NumStates() == 0 {
		return utf32
	}
	u.out = NewBuilder()
	srcStates := utf32.NumStates()
	mapping := make([]int, srcStates)
	for i := range mapping {
		mapping[i] = -1
	}
	pending := []int{0}
	utf8State := u.out.CreateState()
	if utf32.IsAccept(0) {
		u.out.SetAccept(utf8State, true)
	}
	mapping[0] = utf8State

	t := NewTransition()
	for len(pending) > 0 {
		utf32State := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		myUTF8 := mapping[utf32State]
		n := utf32.InitTransition(utf32State, t)
		for i := 0; i < n; i++ {
			utf32.GetNextTransition(t)
			destUTF32 := t.Dest
			destUTF8 := mapping[destUTF32]
			if destUTF8 == -1 {
				destUTF8 = u.out.CreateState()
				if utf32.IsAccept(destUTF32) {
					u.out.SetAccept(destUTF8, true)
				}
				mapping[destUTF32] = destUTF8
				pending = append(pending, destUTF32)
			}
			u.convertOneEdge(myUTF8, destUTF8, t.Min, t.Max)
		}
	}
	return u.out.Finish()
}

func (u *UTF32ToUTF8) convertOneEdge(start, end, startCodePoint, endCodePoint int) {
	u.startUTF8.set(startCodePoint)
	u.endUTF8.set(endCodePoint)
	u.build(start, end, &u.startUTF8, &u.endUTF8, 0)
}

func (u *UTF32ToUTF8) build(start, end int, s, e *utf8Sequence, upto int) {
	switch {
	case s.byteAt(upto) == e.byteAt(upto):
		if upto == s.len-1 && upto == e.len-1 {
			u.out.AddTransition(start, end, s.byteAt(upto), e.byteAt(upto))
			return
		}
		n := u.out.CreateState()
		u.out.AddTransitionSingle(start, n, s.byteAt(upto))
		u.build(n, end, s, e, upto+1)
	case s.len == e.len:
		if upto == s.len-1 {
			u.out.AddTransition(start, end, s.byteAt(upto), e.byteAt(upto))
			return
		}
		u.start(start, end, s, upto, false)
		if e.byteAt(upto)-s.byteAt(upto) > 1 {
			u.all(start, end, s.byteAt(upto)+1, e.byteAt(upto)-1, s.len-upto-1)
		}
		u.end(start, end, e, upto, false)
	default:
		u.start(start, end, s, upto, true)
		byteCount := 1 + s.len - upto
		limit := e.len - upto
		for byteCount < limit {
			u.tmpUTF8a.setFirstByte(utf8StartCodes[byteCount-1])
			u.tmpUTF8b.setFirstByte(utf8EndCodes[byteCount-1])
			u.all(start, end, u.tmpUTF8a.byteAt(0), u.tmpUTF8b.byteAt(0), u.tmpUTF8a.len-1)
			byteCount++
		}
		u.end(start, end, e, upto, true)
	}
}

func (u *UTF32ToUTF8) start(start, end int, s *utf8Sequence, upto int, doAll bool) {
	if upto == s.len-1 {
		u.out.AddTransition(start, end, s.byteAt(upto), s.byteAt(upto)|int(utf8Masks[s.numBits(upto)]))
		return
	}
	n := u.out.CreateState()
	u.out.AddTransitionSingle(start, n, s.byteAt(upto))
	u.start(n, end, s, upto+1, true)
	endCode := s.byteAt(upto) | int(utf8Masks[s.numBits(upto)])
	if doAll && s.byteAt(upto) != endCode {
		u.all(start, end, s.byteAt(upto)+1, endCode, s.len-upto-1)
	}
}

func (u *UTF32ToUTF8) end(start, end int, e *utf8Sequence, upto int, doAll bool) {
	if upto == e.len-1 {
		u.out.AddTransition(start, end, e.byteAt(upto)&(^int(utf8Masks[e.numBits(upto)])), e.byteAt(upto))
		return
	}
	var startCode int
	// Lucene GH-12472 special cases for the first byte of each length class.
	switch {
	case e.len == 2:
		startCode = 0xC2
	case e.len == 3 && upto == 1 && e.byteAt(0) == 0xE0:
		startCode = 0xA0
	case e.len == 4 && upto == 1 && e.byteAt(0) == 0xF0:
		startCode = 0x90
	default:
		startCode = e.byteAt(upto) & (^int(utf8Masks[e.numBits(upto)]))
	}
	if doAll && e.byteAt(upto) != startCode {
		u.all(start, end, startCode, e.byteAt(upto)-1, e.len-upto-1)
	}
	n := u.out.CreateState()
	u.out.AddTransitionSingle(start, n, e.byteAt(upto))
	u.end(n, end, e, upto+1, true)
}

func (u *UTF32ToUTF8) all(start, end, startCode, endCode, left int) {
	if left == 0 {
		u.out.AddTransition(start, end, startCode, endCode)
		return
	}
	lastN := u.out.CreateState()
	u.out.AddTransition(start, lastN, startCode, endCode)
	for left > 1 {
		n := u.out.CreateState()
		u.out.AddTransition(lastN, n, 128, 191)
		left--
		lastN = n
	}
	u.out.AddTransition(lastN, end, 128, 191)
}
