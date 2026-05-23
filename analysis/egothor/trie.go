// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

import (
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf16"
)

// Trie stores a dictionary of words and their associated patch commands.
//
// A trie can be forward (keys read left to right) or backward (keys read
// right to left). This property varies depending on the language.
//
// This is the Go port of org.egothor.stemmer.Trie (Lucene 10.4.0).
type Trie struct {
	rows    []*Row
	cmds    []string
	root    int
	forward bool
}

// NewTrieFromReader deserialises a Trie from a Java DataInput-compatible stream.
func NewTrieFromReader(r io.Reader) (*Trie, error) {
	t := &Trie{}
	var fwd uint8
	if err := binary.Read(r, binary.BigEndian, &fwd); err != nil {
		return nil, err
	}
	t.forward = fwd != 0

	var root int32
	if err := binary.Read(r, binary.BigEndian, &root); err != nil {
		return nil, err
	}
	t.root = int(root)

	var nCmds int32
	if err := binary.Read(r, binary.BigEndian, &nCmds); err != nil {
		return nil, err
	}
	t.cmds = make([]string, nCmds)
	for i := int32(0); i < nCmds; i++ {
		s, err := readUTF(r)
		if err != nil {
			return nil, err
		}
		t.cmds[i] = s
	}

	var nRows int32
	if err := binary.Read(r, binary.BigEndian, &nRows); err != nil {
		return nil, err
	}
	t.rows = make([]*Row, nRows)
	for i := int32(0); i < nRows; i++ {
		row, err := newRowFromReader(r)
		if err != nil {
			return nil, err
		}
		t.rows[i] = row
	}
	return t, nil
}

// NewTrie creates an empty forward or backward Trie.
func NewTrie(forward bool) *Trie {
	t := &Trie{
		forward: forward,
		root:    0,
	}
	t.rows = append(t.rows, newRow())
	return t
}

// newTrieWithData constructs a Trie from pre-built rows and cmds.
func newTrieWithData(forward bool, root int, cmds []string, rows []*Row) *Trie {
	return &Trie{rows: rows, cmds: cmds, root: root, forward: forward}
}

// GetAll returns all patch commands found along the path for key.
func (t *Trie) GetAll(key []rune) []string {
	res := make([]int, len(key))
	resc := 0
	now := t.getRow(t.root)
	e := newStrEnum(key, t.forward)

	br := false
	for i := 0; i < len(key)-1; i++ {
		ch := e.next()
		w := now.getCmd(ch)
		if w >= 0 {
			n := w
			dup := false
			for j := 0; j < resc; j++ {
				if n == res[j] {
					dup = true
					break
				}
			}
			if !dup {
				res[resc] = n
				resc++
			}
		}
		w = now.getRef(ch)
		if w >= 0 {
			now = t.getRow(w)
		} else {
			br = true
			break
		}
	}
	if !br {
		w := now.getCmd(e.next())
		if w >= 0 {
			n := w
			dup := false
			for j := 0; j < resc; j++ {
				if n == res[j] {
					dup = true
					break
				}
			}
			if !dup {
				res[resc] = n
				resc++
			}
		}
	}

	if resc < 1 {
		return nil
	}
	R := make([]string, resc)
	for j := 0; j < resc; j++ {
		R[j] = t.cmds[res[j]]
	}
	return R
}

// GetCells returns the total number of active cells across all rows.
func (t *Trie) GetCells() int {
	size := 0
	for _, row := range t.rows {
		size += row.getCells()
	}
	return size
}

// GetCellsPnt returns the total number of reference cells across all rows.
func (t *Trie) GetCellsPnt() int {
	size := 0
	for _, row := range t.rows {
		size += row.getCellsPnt()
	}
	return size
}

// GetCellsVal returns the total number of patch-command cells across all rows.
func (t *Trie) GetCellsVal() int {
	size := 0
	for _, row := range t.rows {
		size += row.getCellsVal()
	}
	return size
}

// GetFully returns the element stored in the cell exactly associated with key.
func (t *Trie) GetFully(key []rune) string {
	now := t.getRow(t.root)
	e := newStrEnum(key, t.forward)
	cmd := -1

	for i := 0; i < len(key); {
		ch := e.next()
		i++

		c := now.at(ch)
		if c == nil {
			return ""
		}
		cmd = c.cmd

		for skip := c.skip; skip > 0; skip-- {
			if i < len(key) {
				e.next()
			} else {
				return ""
			}
			i++
		}

		w := now.getRef(ch)
		if w >= 0 {
			now = t.getRow(w)
		} else if i < len(key) {
			return ""
		}
	}
	if cmd == -1 {
		return ""
	}
	return t.cmds[cmd]
}

// GetLastOnPath returns the patch command stored last on the path for key.
func (t *Trie) GetLastOnPath(key []rune) string {
	now := t.getRow(t.root)
	last := ""
	e := newStrEnum(key, t.forward)

	for i := 0; i < len(key)-1; i++ {
		ch := e.next()
		w := now.getCmd(ch)
		if w >= 0 {
			last = t.cmds[w]
		}
		w = now.getRef(ch)
		if w >= 0 {
			now = t.getRow(w)
		} else {
			return last
		}
	}
	w := now.getCmd(e.next())
	if w >= 0 {
		return t.cmds[w]
	}
	return last
}

// getRow returns the row at index, or nil if out of range.
func (t *Trie) getRow(index int) *Row {
	if index < 0 || index >= len(t.rows) {
		return nil
	}
	return t.rows[index]
}

// Store serialises this Trie using Java DataOutput wire format.
func (t *Trie) Store(w io.Writer) error {
	var fwd uint8
	if t.forward {
		fwd = 1
	}
	if err := binary.Write(w, binary.BigEndian, fwd); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, int32(t.root)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, int32(len(t.cmds))); err != nil {
		return err
	}
	for _, cmd := range t.cmds {
		if err := writeUTF(w, cmd); err != nil {
			return err
		}
	}
	if err := binary.Write(w, binary.BigEndian, int32(len(t.rows))); err != nil {
		return err
	}
	for _, row := range t.rows {
		if err := row.store(w); err != nil {
			return err
		}
	}
	return nil
}

// Add associates key with the given patch command cmd.
// Does nothing if key or cmd is empty.
func (t *Trie) Add(key []rune, cmd string) {
	if len(key) == 0 || len(cmd) == 0 {
		return
	}
	idCmd := -1
	for i, c := range t.cmds {
		if c == cmd {
			idCmd = i
			break
		}
	}
	if idCmd == -1 {
		idCmd = len(t.cmds)
		t.cmds = append(t.cmds, cmd)
	}

	node := t.root
	r := t.getRow(node)
	e := newStrEnum(key, t.forward)

	for i := 0; i < len(key)-1; i++ {
		ch := e.next()
		node = r.getRef(ch)
		if node >= 0 {
			r = t.getRow(node)
		} else {
			node = len(t.rows)
			n := newRow()
			t.rows = append(t.rows, n)
			r.setRef(ch, node)
			r = n
		}
	}
	r.setCmd(e.next(), idCmd)
}

// Reduce removes empty rows using the given Reduce strategy.
func (t *Trie) Reduce(by Reducer) *Trie {
	return by.Optimize(t)
}

// PrintInfo writes diagnostic info to w.
func (t *Trie) PrintInfo(w io.Writer, prefix string) {
	fmt.Fprintf(w, "%snds %d cmds %d cells %d valcells %d pntcells %d\n",
		prefix, len(t.rows), len(t.cmds),
		t.GetCells(), t.GetCellsVal(), t.GetCellsPnt())
}

// strEnum iterates over a rune slice forward or backward.
type strEnum struct {
	s    []rune
	from int
	by   int
}

func newStrEnum(s []rune, up bool) *strEnum {
	if up {
		return &strEnum{s: s, from: 0, by: 1}
	}
	return &strEnum{s: s, from: len(s) - 1, by: -1}
}

func (e *strEnum) next() rune {
	ch := e.s[e.from]
	e.from += e.by
	return ch
}

// readUTF reads a Java modified-UTF-8 string (2-byte length prefix).
func readUTF(r io.Reader) (string, error) {
	var length uint16
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return decodeModifiedUTF8(buf), nil
}

// writeUTF writes a Java modified-UTF-8 string (2-byte length prefix).
func writeUTF(w io.Writer, s string) error {
	encoded := encodeModifiedUTF8(s)
	if err := binary.Write(w, binary.BigEndian, uint16(len(encoded))); err != nil {
		return err
	}
	_, err := w.Write(encoded)
	return err
}

// decodeModifiedUTF8 converts Java modified-UTF-8 bytes to a Go string.
// Handles the supplementary character encoding (two 3-byte sequences) and
// the null-byte encoding (\xC0\x80 -> \x00).
func decodeModifiedUTF8(b []byte) string {
	runes := make([]rune, 0, len(b))
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case c&0x80 == 0: // 1-byte ASCII
			runes = append(runes, rune(c))
			i++
		case c&0xE0 == 0xC0 && i+1 < len(b): // 2-byte
			r := rune(c&0x1F)<<6 | rune(b[i+1]&0x3F)
			runes = append(runes, r)
			i += 2
		case c&0xF0 == 0xE0 && i+2 < len(b): // 3-byte
			r := rune(c&0x0F)<<12 | rune(b[i+1]&0x3F)<<6 | rune(b[i+2]&0x3F)
			// Decode supplementary characters encoded as surrogate pairs.
			if r >= 0xD800 && r <= 0xDBFF && i+5 < len(b) {
				r2 := rune(b[i+3]&0x0F)<<12 | rune(b[i+4]&0x3F)<<6 | rune(b[i+5]&0x3F)
				if r2 >= 0xDC00 && r2 <= 0xDFFF {
					runes = append(runes, utf16.DecodeRune(r, r2))
					i += 6
					continue
				}
			}
			runes = append(runes, r)
			i += 3
		default:
			runes = append(runes, rune(c))
			i++
		}
	}
	return string(runes)
}

// encodeModifiedUTF8 converts a Go string to Java modified-UTF-8 bytes.
func encodeModifiedUTF8(s string) []byte {
	var out []byte
	for _, r := range s {
		switch {
		case r == 0: // null encoded as \xC0\x80
			out = append(out, 0xC0, 0x80)
		case r < 0x80:
			out = append(out, byte(r))
		case r < 0x800:
			out = append(out, byte(0xC0|r>>6), byte(0x80|r&0x3F))
		case r < 0x10000:
			out = append(out, byte(0xE0|r>>12), byte(0x80|(r>>6)&0x3F), byte(0x80|r&0x3F))
		default:
			// Encode supplementary as two 3-byte surrogate sequences.
			r1, r2 := utf16.EncodeRune(r)
			out = append(out,
				byte(0xE0|r1>>12), byte(0x80|(r1>>6)&0x3F), byte(0x80|r1&0x3F),
				byte(0xE0|r2>>12), byte(0x80|(r2>>6)&0x3F), byte(0x80|r2&0x3F),
			)
		}
	}
	return out
}
