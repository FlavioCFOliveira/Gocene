// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Auto-generated parametric description for Levenshtein degree 1 (no transpositions).
// Translated verbatim from org.apache.lucene.util.automaton.Lev1ParametricDescription
// in Apache Lucene 10.4.0; the upstream tables were produced by Lucene's
// moman/finenight tooling (MIT-licensed).

package automaton

type lev1ParametricDescription struct {
	baseParametricDescription
}

func newLev1ParametricDescription(w int) *lev1ParametricDescription {
	return &lev1ParametricDescription{
		baseParametricDescription: baseParametricDescription{
			w:         w,
			n:         1,
			minErrors: []int{0, 1, 0, -1, -1},
		},
	}
}

func (d *lev1ParametricDescription) size() int             { return d.baseParametricDescription.size() }
func (d *lev1ParametricDescription) isAccept(s int) bool   { return d.isAcceptInline(s) }
func (d *lev1ParametricDescription) getPosition(s int) int { return d.getPositionInline(s) }

func (d *lev1ParametricDescription) transition(absState, position, vector int) int {
	state := absState / (d.w + 1)
	offset := absState % (d.w + 1)

	switch {
	case position == d.w:
		if state < 2 {
			loc := vector*2 + state
			offset += unpack(lev1ToOffsetIncrs0, loc, 1)
			state = unpack(lev1ToStates0, loc, 2) - 1
		}
	case position == d.w-1:
		if state < 3 {
			loc := vector*3 + state
			offset += unpack(lev1ToOffsetIncrs1, loc, 1)
			state = unpack(lev1ToStates1, loc, 2) - 1
		}
	case position == d.w-2:
		if state < 5 {
			loc := vector*5 + state
			offset += unpack(lev1ToOffsetIncrs2, loc, 2)
			state = unpack(lev1ToStates2, loc, 3) - 1
		}
	default:
		if state < 5 {
			loc := vector*5 + state
			offset += unpack(lev1ToOffsetIncrs3, loc, 2)
			state = unpack(lev1ToStates3, loc, 3) - 1
		}
	}

	if state == -1 {
		return -1
	}
	return state*(d.w+1) + offset
}

// Parametric tables: position == w (2 bits per state value, 1 bit per offset).
var (
	lev1ToStates0      = []uint64{0x2}
	lev1ToOffsetIncrs0 = []uint64{0x0}
)

// Parametric tables: position == w-1.
var (
	lev1ToStates1      = []uint64{0xa43}
	lev1ToOffsetIncrs1 = []uint64{0x38}
)

// Parametric tables: position == w-2.
var (
	lev1ToStates2      = []uint64{0x4da292442420003}
	lev1ToOffsetIncrs2 = []uint64{0x5555528000}
)

// Parametric tables: 0 <= position <= w-3.
var (
	lev1ToStates3      = []uint64{0x14d0812112018003, 0xb1a29b46d48a49}
	lev1ToOffsetIncrs3 = []uint64{0x555555e80a0f0000, 0x5555}
)
