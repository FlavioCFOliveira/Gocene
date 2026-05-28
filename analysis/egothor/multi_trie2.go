// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

import (
	"io"
)

// MultiTrie2 is a Trie of Tries that handles patch commands broken into their
// constituent parts delimited by the skip command ('-').
//
// This is the Go port of org.egothor.stemmer.MultiTrie2 (Lucene 10.4.0).
type MultiTrie2 struct {
	MultiTrie
}

// NewMultiTrie2FromReader deserialises a MultiTrie2 from a Java DataInput stream.
func NewMultiTrie2FromReader(r io.Reader) (*MultiTrie2, error) {
	m, err := NewMultiTrieFromReader(r)
	if err != nil {
		return nil, err
	}
	return &MultiTrie2{MultiTrie: *m}, nil
}

// NewMultiTrie2 creates an empty MultiTrie2.
func NewMultiTrie2(forward bool) *MultiTrie2 {
	return &MultiTrie2{MultiTrie: *NewMultiTrie(forward)}
}

// GetFully returns the patch command stored in the cell exactly associated with key.
func (m *MultiTrie2) GetFully(key []rune) string {
	var result []rune
	func() {
		// Mirrors Java MultiTrie2.getFully, which catches IndexOutOfBoundsException;
		// recoverBounds swallows only out-of-range panics and re-panics anything else.
		defer recoverBounds()
		lastkey := key
		p := make([]string, len(m.tries))
		lastch := ' '
		for i, t := range m.tries {
			r := t.GetFully(lastkey)
			if r == "" || (len(r) == 1 && rune(r[0]) == eom) {
				return
			}
			rr := []rune(r)
			if m2cannotFollow(rune(lastch), rr[0]) {
				return
			}
			lastch = rune(rr[len(rr)-2])
			p[i] = r
			if rr[0] == '-' {
				if i > 0 {
					key = m2skip(key, m.forward, m2lengthPP([]rune(p[i-1])))
				}
				key = m2skip(key, m.forward, m2lengthPP(rr))
			}
			result = append(result, rr...)
			if len(key) != 0 {
				lastkey = key
			}
		}
	}()
	return string(result)
}

// GetLastOnPath returns the patch command stored last on the path for key.
func (m *MultiTrie2) GetLastOnPath(key []rune) string {
	var result []rune
	func() {
		// Mirrors Java MultiTrie2.getLastOnPath, which catches IndexOutOfBoundsException;
		// recoverBounds swallows only out-of-range panics and re-panics anything else.
		defer recoverBounds()
		lastkey := key
		p := make([]string, len(m.tries))
		lastch := ' '
		for i, t := range m.tries {
			r := t.GetLastOnPath(lastkey)
			if r == "" || (len(r) == 1 && rune(r[0]) == eom) {
				return
			}
			rr := []rune(r)
			if m2cannotFollow(rune(lastch), rr[0]) {
				return
			}
			lastch = rune(rr[len(rr)-2])
			p[i] = r
			if rr[0] == '-' {
				if i > 0 {
					key = m2skip(key, m.forward, m2lengthPP([]rune(p[i-1])))
				}
				key = m2skip(key, m.forward, m2lengthPP(rr))
			}
			result = append(result, rr...)
			if len(key) != 0 {
				lastkey = key
			}
		}
	}()
	return string(result)
}

// Store serialises this MultiTrie2 (same wire format as MultiTrie).
func (m *MultiTrie2) Store(w io.Writer) error {
	return m.MultiTrie.Store(w)
}

// Add associates key with the given patch command cmd.
func (m *MultiTrie2) Add(key []rune, cmd string) {
	if len(cmd) == 0 {
		return
	}
	p := m2decompose([]rune(cmd))
	levels := len(p)
	for levels >= len(m.tries) {
		m.tries = append(m.tries, NewTrie(m.forward))
	}
	lastkey := key
	for i, part := range p {
		if len(key) > 0 {
			m.tries[i].Add(key, string(part))
			lastkey = key
		} else {
			m.tries[i].Add(lastkey, string(part))
		}
		if len(part) > 0 && part[0] == '-' {
			if i > 0 {
				key = m2skip(key, m.forward, m2lengthPP(p[i-1]))
			}
			key = m2skip(key, m.forward, m2lengthPP(part))
		}
	}
	if len(key) > 0 {
		m.tries[levels].Add(key, eomStr)
	} else {
		m.tries[levels].Add(lastkey, eomStr)
	}
}

// Reduce removes empty rows using by and returns a new MultiTrie2.
func (m *MultiTrie2) Reduce(by Reducer) *MultiTrie2 {
	result := make([]*Trie, len(m.tries))
	for i, t := range m.tries {
		result[i] = t.Reduce(by)
	}
	nm := NewMultiTrie2(m.forward)
	nm.tries = result
	return nm
}

// m2decompose breaks cmd into parts delimited by '-' (NOOP commands).
func m2decompose(cmd []rune) [][]rune {
	parts := 0
	for i := 0; 0 <= i && i < len(cmd); {
		next := m2dashEven(cmd, i)
		if i == next {
			parts++
			i = next + 2
		} else {
			parts++
			i = next
		}
	}
	part := make([][]rune, parts)
	x := 0
	for i := 0; 0 <= i && i < len(cmd); {
		next := m2dashEven(cmd, i)
		if i == next {
			part[x] = cmd[i : i+2]
			x++
			i = next + 2
		} else {
			if next < 0 {
				part[x] = cmd[i:]
			} else {
				part[x] = cmd[i:next]
			}
			x++
			i = next
		}
	}
	return part
}

func m2cannotFollow(after, goes rune) bool {
	return (after == '-' || after == 'D') && after == goes
}

func m2skip(in []rune, forward bool, count int) []rune {
	if forward {
		if count >= len(in) {
			return nil
		}
		return in[count:]
	}
	if count >= len(in) {
		return nil
	}
	return in[:len(in)-count]
}

func m2dashEven(in []rune, from int) int {
	for from < len(in) {
		if in[from] == '-' {
			return from
		}
		from += 2
	}
	return -1
}

func m2lengthPP(cmd []rune) int {
	length := 0
	for i := 0; i < len(cmd); i++ {
		switch cmd[i] {
		case '-', 'D':
			i++
			if i < len(cmd) {
				length += int(cmd[i]-'a') + 1
			}
		case 'R':
			length++
			i++ // skip param (intentional fallthrough in Java)
		case 'I':
			i++ // skip param
		}
	}
	return length
}
