// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package egothor

import "sync"

// Diff generates a patch string that transforms one string into another.
//
// A patch string is a command sequence for a stemmer, encoding how to reduce
// a word to its root. For example, to reduce "teacher" to "teach" the patch
// string "Db" is generated (delete the last 2 characters).
//
// This is the Go port of org.egothor.stemmer.Diff (Lucene 10.4.0).
type Diff struct {
	mu      sync.Mutex
	sizex   int
	sizey   int
	net     [][]int
	way     [][]int
	INSERT  int
	DELETE  int
	REPLACE int
	NOOP    int
}

// NewDiff creates a Diff with default costs: INSERT=1, DELETE=1, REPLACE=1, NOOP=0.
func NewDiff() *Diff {
	return NewDiffWithCosts(1, 1, 1, 0)
}

// NewDiffWithCosts creates a Diff with explicit operation costs.
func NewDiffWithCosts(ins, del, rep, noop int) *Diff {
	return &Diff{INSERT: ins, DELETE: del, REPLACE: rep, NOOP: noop}
}

// Apply applies the given patch command string diff to dest in-place.
// Errors (index out of range) are silently ignored, mirroring the Java source.
func Apply(dest *[]rune, diff string) {
	if diff == "" || len(*dest) == 0 {
		return
	}
	pos := len(*dest) - 1
	if pos < 0 {
		return
	}
	// Mirrors Java Diff.apply, which catches StringIndexOutOfBoundsException and
	// ArrayIndexOutOfBoundsException; recoverBounds swallows only out-of-range
	// panics and re-panics anything else.
	defer recoverBounds()
	for i := 0; i < len(diff)/2; i++ {
		cmd := diff[2*i]
		param := diff[2*i+1]
		parNum := int(param-'a') + 1
		switch cmd {
		case '-':
			pos = pos - parNum + 1
		case 'R':
			(*dest)[pos] = rune(param)
		case 'D':
			o := pos
			pos -= parNum - 1
			// delete chars from pos..o (inclusive)
			*dest = append((*dest)[:pos], (*dest)[o+1:]...)
		case 'I':
			pos++
			ins := make([]rune, len(*dest)+1)
			copy(ins, (*dest)[:pos])
			ins[pos] = rune(param)
			copy(ins[pos+1:], (*dest)[pos:])
			*dest = ins
		}
		pos--
	}
}

// ApplyToBuilder applies the patch string diff to sb (a string builder backed
// by a rune slice). This is the primary API used by StempelStemmer.
func ApplyToBuilder(sb *[]rune, diff string) {
	Apply(sb, diff)
}

// Exec constructs a patch string that transforms string a into string b.
// Returns "" if either is empty.
func (d *Diff) Exec(a, b string) string {
	if a == "" || b == "" {
		return ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	ra := []rune(a)
	rb := []rune(b)

	const (
		X = 1 // delete from a
		Y = 2 // insert from b
		R = 3 // replace
		D = 0 // diagonal / noop
	)

	maxx := len(ra) + 1
	maxy := len(rb) + 1

	// grow matrices if needed
	if maxx >= d.sizex || maxy >= d.sizey {
		d.sizex = maxx + 8
		d.sizey = maxy + 8
		d.net = make([][]int, d.sizex)
		d.way = make([][]int, d.sizex)
		for i := range d.net {
			d.net[i] = make([]int, d.sizey)
			d.way[i] = make([]int, d.sizey)
		}
	}

	// clear
	for x := 0; x < maxx; x++ {
		for y := 0; y < maxy; y++ {
			d.net[x][y] = 0
		}
	}

	// base cases
	for x := 1; x < maxx; x++ {
		d.net[x][0] = x
		d.way[x][0] = X
	}
	for y := 1; y < maxy; y++ {
		d.net[0][y] = y
		d.way[0][y] = Y
	}

	for x := 1; x < maxx; x++ {
		for y := 1; y < maxy; y++ {
			go_ := [4]int{}
			go_[X] = d.net[x-1][y] + d.DELETE
			go_[Y] = d.net[x][y-1] + d.INSERT
			go_[R] = d.net[x-1][y-1] + d.REPLACE
			if ra[x-1] == rb[y-1] {
				go_[D] = d.net[x-1][y-1] + d.NOOP
			} else {
				go_[D] = d.net[x-1][y-1] + 100
			}
			var min int = D
			if go_[min] >= go_[X] {
				min = X
			}
			if go_[min] > go_[Y] {
				min = Y
			}
			if go_[min] > go_[R] {
				min = R
			}
			d.way[x][y] = min
			d.net[x][y] = go_[min]
		}
	}

	// read back
	var result []rune
	const base = 'a' - 1
	deletes := rune(base)
	equals := rune(base)

	x, y := maxx-1, maxy-1
	for x+y != 0 {
		switch d.way[x][y] {
		case X: // delete
			if equals != base {
				result = append(result, '-', equals)
				equals = base
			}
			deletes++
			x--
		case Y: // insert
			if deletes != base {
				result = append(result, 'D', deletes)
				deletes = base
			}
			if equals != base {
				result = append(result, '-', equals)
				equals = base
			}
			y--
			result = append(result, 'I', rb[y])
		case R: // replace
			if deletes != base {
				result = append(result, 'D', deletes)
				deletes = base
			}
			if equals != base {
				result = append(result, '-', equals)
				equals = base
			}
			y--
			result = append(result, 'R', rb[y])
			x--
		case D: // no change
			if deletes != base {
				result = append(result, 'D', deletes)
				deletes = base
			}
			equals++
			x--
			y--
		}
	}
	if deletes != base {
		result = append(result, 'D', deletes)
	}
	return string(result)
}
