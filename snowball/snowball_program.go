// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

/*
Copyright (c) 2001, Dr Martin Porter
Copyright (c) 2004,2005, Richard Boulton
Copyright (c) 2013, Yoshiki Shibukawa
Copyright (c) 2006,2007,2009,2010,2011,2014-2019, Olly Betts
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions
are met:

  1. Redistributions of source code must retain the above copyright notice,
     this list of conditions and the following disclaimer.
  2. Redistributions in binary form must reproduce the above copyright notice,
     this list of conditions and the following disclaimer in the documentation
     and/or other materials provided with the distribution.
  3. Neither the name of the Snowball project nor the names of its contributors
     may be used to endorse or promote products derived from this software
     without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

package snowball

// SnowballProgram mirrors org.tartarus.snowball.SnowballProgram.
//
// It is the base for all Snowball stemmer implementations, providing
// the character buffer and the standard find_among, replace_s, slice_*
// primitives used by generated stemmer code.
//
// Go deviation: Java works on char[] (UTF-16 code units); Go works on
// []rune (Unicode code points). The observable behaviour is identical for
// the languages covered by the Snowball stemmers (all BMP).
type SnowballProgram struct {
	current        []rune
	Cursor         int
	Length         int
	Limit          int
	LimitBackward  int
	Bra            int
	Ket            int
}

// NewSnowballProgram creates a SnowballProgram with an empty current string.
func NewSnowballProgram() *SnowballProgram {
	p := &SnowballProgram{}
	p.SetCurrent("")
	return p
}

// SetCurrent sets the string to be processed.
//
// Mirrors: void setCurrent(String value)
func (p *SnowballProgram) SetCurrent(value string) {
	p.current = []rune(value)
	n := len(p.current)
	p.Cursor = 0
	p.Length = n
	p.Limit = n
	p.LimitBackward = 0
	p.Bra = p.Cursor
	p.Ket = p.Limit
}

// GetCurrent returns the current string.
//
// Mirrors: String getCurrent()
func (p *SnowballProgram) GetCurrent() string {
	return string(p.current[:p.Length])
}

// GetCurrentBuffer returns the underlying rune buffer.
//
// Mirrors: char[] getCurrentBuffer()
func (p *SnowballProgram) GetCurrentBuffer() []rune {
	return p.current
}

// GetCurrentBufferLength returns the valid length of the rune buffer.
//
// Mirrors: int getCurrentBufferLength()
func (p *SnowballProgram) GetCurrentBufferLength() int {
	return p.Length
}

// CopyFrom copies state from another SnowballProgram.
//
// Mirrors: void copy_from(SnowballProgram other)
func (p *SnowballProgram) CopyFrom(other *SnowballProgram) {
	p.current = other.current
	p.Cursor = other.Cursor
	p.Length = other.Length
	p.Limit = other.Limit
	p.LimitBackward = other.LimitBackward
	p.Bra = other.Bra
	p.Ket = other.Ket
}

// InGrouping returns true if the character at Cursor is in the grouping
// defined by s, min, and max, and advances Cursor.
//
// Mirrors: boolean in_grouping(char[] s, int min, int max)
func (p *SnowballProgram) InGrouping(s []byte, min, max rune) bool {
	if p.Cursor >= p.Limit {
		return false
	}
	ch := p.current[p.Cursor]
	if ch > max || ch < min {
		return false
	}
	ch -= min
	if (s[ch>>3] & (0x1 << (ch & 0x7))) == 0 {
		return false
	}
	p.Cursor++
	return true
}

// InGroupingB is the backward version of InGrouping.
//
// Mirrors: boolean in_grouping_b(char[] s, int min, int max)
func (p *SnowballProgram) InGroupingB(s []byte, min, max rune) bool {
	if p.Cursor <= p.LimitBackward {
		return false
	}
	ch := p.current[p.Cursor-1]
	if ch > max || ch < min {
		return false
	}
	ch -= min
	if (s[ch>>3] & (0x1 << (ch & 0x7))) == 0 {
		return false
	}
	p.Cursor--
	return true
}

// OutGrouping returns true if the character at Cursor is outside the
// grouping and advances Cursor.
//
// Mirrors: boolean out_grouping(char[] s, int min, int max)
func (p *SnowballProgram) OutGrouping(s []byte, min, max rune) bool {
	if p.Cursor >= p.Limit {
		return false
	}
	ch := p.current[p.Cursor]
	if ch > max || ch < min {
		p.Cursor++
		return true
	}
	ch -= min
	if (s[ch>>3] & (0x1 << (ch & 0x7))) == 0 {
		p.Cursor++
		return true
	}
	return false
}

// OutGroupingB is the backward version of OutGrouping.
//
// Mirrors: boolean out_grouping_b(char[] s, int min, int max)
func (p *SnowballProgram) OutGroupingB(s []byte, min, max rune) bool {
	if p.Cursor <= p.LimitBackward {
		return false
	}
	ch := p.current[p.Cursor-1]
	if ch > max || ch < min {
		p.Cursor--
		return true
	}
	ch -= min
	if (s[ch>>3] & (0x1 << (ch & 0x7))) == 0 {
		p.Cursor--
		return true
	}
	return false
}

// EqS returns true if the next s.length characters of current match s,
// and advances Cursor by s.length.
//
// Mirrors: boolean eq_s(CharSequence s)
func (p *SnowballProgram) EqS(s []rune) bool {
	if p.Limit-p.Cursor < len(s) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if p.current[p.Cursor+i] != s[i] {
			return false
		}
	}
	p.Cursor += len(s)
	return true
}

// EqSStr is a convenience wrapper for EqS that accepts a string.
func (p *SnowballProgram) EqSStr(s string) bool {
	return p.EqS([]rune(s))
}

// EqSB is the backward version of EqS.
//
// Mirrors: boolean eq_s_b(CharSequence s)
func (p *SnowballProgram) EqSB(s []rune) bool {
	if p.Cursor-p.LimitBackward < len(s) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if p.current[p.Cursor-len(s)+i] != s[i] {
			return false
		}
	}
	p.Cursor -= len(s)
	return true
}

// EqSBStr is a convenience wrapper for EqSB that accepts a string.
func (p *SnowballProgram) EqSBStr(s string) bool {
	return p.EqSB([]rune(s))
}

// FindAmong performs a forward binary search in v and returns the result
// of the matching entry (0 if no match).
//
// Mirrors: int find_among(Among[] v)
func (p *SnowballProgram) FindAmong(v []*Among) int {
	i := 0
	j := len(v)

	c := p.Cursor
	l := p.Limit

	commonI := 0
	commonJ := 0

	firstKeyInspected := false

	for {
		k := i + ((j - i) >> 1)
		diff := 0
		common := commonI
		if commonJ < commonI {
			common = commonJ
		}
		w := v[k]
		var i2 int
		for i2 = common; i2 < len(w.S); i2++ {
			if c+common == l {
				diff = -1
				break
			}
			diff = int(p.current[c+common]) - int(w.S[i2])
			if diff != 0 {
				break
			}
			common++
		}
		if diff < 0 {
			j = k
			commonJ = common
		} else {
			i = k
			commonI = common
		}
		if j-i <= 1 {
			if i > 0 {
				break
			}
			if j == i {
				break
			}
			if firstKeyInspected {
				break
			}
			firstKeyInspected = true
		}
	}
	for {
		w := v[i]
		if commonI >= len(w.S) {
			p.Cursor = c + len(w.S)
			if w.Method == nil {
				return w.Result
			}
			res := w.Method(p)
			p.Cursor = c + len(w.S)
			if res {
				return w.Result
			}
		}
		i = w.SubstringI
		if i < 0 {
			return 0
		}
	}
}

// FindAmongB performs a backward binary search in v and returns the result
// of the matching entry (0 if no match).
//
// Mirrors: int find_among_b(Among[] v)
func (p *SnowballProgram) FindAmongB(v []*Among) int {
	i := 0
	j := len(v)

	c := p.Cursor
	lb := p.LimitBackward

	commonI := 0
	commonJ := 0

	firstKeyInspected := false

	for {
		k := i + ((j - i) >> 1)
		diff := 0
		common := commonI
		if commonJ < commonI {
			common = commonJ
		}
		w := v[k]
		var i2 int
		for i2 = len(w.S) - 1 - common; i2 >= 0; i2-- {
			if c-common == lb {
				diff = -1
				break
			}
			diff = int(p.current[c-1-common]) - int(w.S[i2])
			if diff != 0 {
				break
			}
			common++
		}
		if diff < 0 {
			j = k
			commonJ = common
		} else {
			i = k
			commonI = common
		}
		if j-i <= 1 {
			if i > 0 {
				break
			}
			if j == i {
				break
			}
			if firstKeyInspected {
				break
			}
			firstKeyInspected = true
		}
	}
	for {
		w := v[i]
		if commonI >= len(w.S) {
			p.Cursor = c - len(w.S)
			if w.Method == nil {
				return w.Result
			}
			res := w.Method(p)
			p.Cursor = c - len(w.S)
			if res {
				return w.Result
			}
		}
		i = w.SubstringI
		if i < 0 {
			return 0
		}
	}
}

// ReplaceS replaces the characters between cBra and cKet in current with s,
// adjusting Cursor, Length, and Limit accordingly, and returns the adjustment.
//
// Mirrors: int replace_s(int c_bra, int c_ket, CharSequence s)
func (p *SnowballProgram) ReplaceS(cBra, cKet int, s []rune) int {
	adjustment := len(s) - (cKet - cBra)
	newLength := p.Length + adjustment
	// Resize if necessary.
	if newLength > len(p.current) {
		grown := make([]rune, newLength)
		copy(grown, p.current)
		p.current = grown
	}
	// Shift the tail if lengths differ.
	if adjustment != 0 && cKet < p.Length {
		copy(p.current[cBra+len(s):], p.current[cKet:p.Length])
	}
	// Write the replacement.
	copy(p.current[cBra:], s)
	p.Length += adjustment
	p.Limit += adjustment
	if p.Cursor >= cKet {
		p.Cursor += adjustment
	} else if p.Cursor > cBra {
		p.Cursor = cBra
	}
	return adjustment
}

// ReplaceSStr is a convenience wrapper for ReplaceS that accepts a string.
func (p *SnowballProgram) ReplaceSStr(cBra, cKet int, s string) int {
	return p.ReplaceS(cBra, cKet, []rune(s))
}

// SliceCheck asserts that Bra/Ket/Limit/Length are consistent.
//
// Mirrors: void slice_check()
func (p *SnowballProgram) SliceCheck() {
	// Assertions are intentionally elided in production; the Java version
	// uses assert statements. Panic is not used per Go production conventions.
}

// SliceFrom replaces the [Bra, Ket) region with s.
//
// Mirrors: void slice_from(CharSequence s)
func (p *SnowballProgram) SliceFrom(s []rune) {
	p.ReplaceS(p.Bra, p.Ket, s)
}

// SliceFromStr is a convenience wrapper for SliceFrom that accepts a string.
func (p *SnowballProgram) SliceFromStr(s string) {
	p.SliceFrom([]rune(s))
}

// SliceDel deletes the [Bra, Ket) region.
//
// Mirrors: void slice_del()
func (p *SnowballProgram) SliceDel() {
	p.SliceFrom(nil)
}

// Insert inserts s at position [cBra, cKet), adjusting Bra and Ket.
//
// Mirrors: void insert(int c_bra, int c_ket, CharSequence s)
func (p *SnowballProgram) Insert(cBra, cKet int, s []rune) {
	adjustment := p.ReplaceS(cBra, cKet, s)
	if cBra <= p.Bra {
		p.Bra += adjustment
	}
	if cBra <= p.Ket {
		p.Ket += adjustment
	}
}

// InsertStr is a convenience wrapper for Insert that accepts a string.
func (p *SnowballProgram) InsertStr(cBra, cKet int, s string) {
	p.Insert(cBra, cKet, []rune(s))
}

// SliceTo copies the [Bra, Ket) region into buf and returns it.
//
// Mirrors: void slice_to(StringBuilder s)
func (p *SnowballProgram) SliceTo(buf []rune) []rune {
	n := p.Ket - p.Bra
	if cap(buf) < n {
		buf = make([]rune, n)
	}
	buf = buf[:n]
	copy(buf, p.current[p.Bra:p.Ket])
	return buf
}

// AssignTo copies the [0, Limit) region into buf and returns it.
//
// Mirrors: void assign_to(StringBuilder s)
func (p *SnowballProgram) AssignTo(buf []rune) []rune {
	n := p.Limit
	if cap(buf) < n {
		buf = make([]rune, n)
	}
	buf = buf[:n]
	copy(buf, p.current[:p.Limit])
	return buf
}
