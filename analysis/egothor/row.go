// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Row represents a row in a matrix representation of a trie.
//
// This is the Go port of org.egothor.stemmer.Row (Lucene 10.4.0).
type Row struct {
	cells      map[rune]*cell
	uniformCnt  int
	uniformSkip int
}

// newRow creates an empty Row.
func newRow() *Row {
	return &Row{cells: make(map[rune]*cell)}
}

// newRowFrom creates a Row sharing the cells map of old.
func newRowFrom(old *Row) *Row {
	return &Row{cells: old.cells}
}

// newRowFromReader deserialises a Row from a DataInput-compatible stream.
//
// Wire format mirrors Java DataInput: ints are big-endian, chars are BE uint16.
func newRowFromReader(r io.Reader) (*Row, error) {
	row := &Row{cells: make(map[rune]*cell)}
	var count int32
	if err := binary.Read(r, binary.BigEndian, &count); err != nil {
		return nil, err
	}
	for i := int32(0); i < count; i++ {
		var ch uint16
		if err := binary.Read(r, binary.BigEndian, &ch); err != nil {
			return nil, err
		}
		c := newCell()
		var cmd, cnt, ref, skip int32
		if err := binary.Read(r, binary.BigEndian, &cmd); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.BigEndian, &cnt); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.BigEndian, &ref); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.BigEndian, &skip); err != nil {
			return nil, err
		}
		c.cmd = int(cmd)
		c.cnt = int(cnt)
		c.ref = int(ref)
		c.skip = int(skip)
		row.cells[rune(ch)] = c
	}
	return row, nil
}

// setCmd sets the command for the cell at way.
func (row *Row) setCmd(way rune, cmd int) {
	c := row.at(way)
	if c == nil {
		c = newCell()
		c.cmd = cmd
		row.cells[way] = c
	} else {
		c.cmd = cmd
	}
	if cmd >= 0 {
		c.cnt = 1
	} else {
		c.cnt = 0
	}
}

// setRef sets the reference to the next row for the cell at way.
func (row *Row) setRef(way rune, ref int) {
	c := row.at(way)
	if c == nil {
		c = newCell()
		c.ref = ref
		row.cells[way] = c
	} else {
		c.ref = ref
	}
}

// getCells returns the number of active cells (cmd>=0 or ref>=0).
func (row *Row) getCells() int {
	size := 0
	for _, e := range row.cells {
		if e.cmd >= 0 || e.ref >= 0 {
			size++
		}
	}
	return size
}

// getCellsPnt returns the number of cells that are references.
func (row *Row) getCellsPnt() int {
	size := 0
	for _, e := range row.cells {
		if e.ref >= 0 {
			size++
		}
	}
	return size
}

// getCellsVal returns the number of cells that carry patch commands.
func (row *Row) getCellsVal() int {
	size := 0
	for _, e := range row.cells {
		if e.cmd >= 0 {
			size++
		}
	}
	return size
}

// getCmd returns the command in the cell at way, or -1 if absent.
func (row *Row) getCmd(way rune) int {
	c := row.at(way)
	if c == nil {
		return -1
	}
	return c.cmd
}

// getCnt returns the count in the cell at way, or -1 if absent.
func (row *Row) getCnt(way rune) int {
	c := row.at(way)
	if c == nil {
		return -1
	}
	return c.cnt
}

// getRef returns the reference in the cell at way, or -1 if absent.
func (row *Row) getRef(way rune) int {
	c := row.at(way)
	if c == nil {
		return -1
	}
	return c.ref
}

// store serialises this Row to the writer using Java DataOutput wire format.
func (row *Row) store(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, int32(len(row.cells))); err != nil {
		return err
	}
	for ch, e := range row.cells {
		if e.cmd < 0 && e.ref < 0 {
			continue
		}
		if err := binary.Write(w, binary.BigEndian, uint16(ch)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, int32(e.cmd)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, int32(e.cnt)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, int32(e.ref)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, int32(e.skip)); err != nil {
			return err
		}
	}
	return nil
}

// uniformCmd returns the single command that all cells agree on, or -1 if
// there are multiple different commands or any references. It also populates
// uniformCnt and uniformSkip.
func (row *Row) uniformCmd(eqSkip bool) int {
	ret := -1
	row.uniformCnt = 1
	row.uniformSkip = 0
	for _, c := range row.cells {
		if c.ref >= 0 {
			return -1
		}
		if c.cmd >= 0 {
			if ret < 0 {
				ret = c.cmd
				row.uniformSkip = c.skip
			} else if ret == c.cmd {
				if eqSkip {
					if row.uniformSkip == c.skip {
						row.uniformCnt++
					} else {
						return -1
					}
				} else {
					row.uniformCnt++
				}
			} else {
				return -1
			}
		}
	}
	return ret
}

// print writes a debugging line to w.
func (row *Row) print(w io.Writer) {
	for ch, c := range row.cells {
		fmt.Fprintf(w, "[%c:%s]", ch, c)
	}
	fmt.Fprintln(w)
}

// at returns the cell at index, or nil.
func (row *Row) at(index rune) *cell {
	return row.cells[index]
}
