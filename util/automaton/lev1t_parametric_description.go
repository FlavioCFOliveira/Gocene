// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Auto-generated parametric description for Levenshtein degree 1 with
// transpositions. Translated verbatim from Lucene 10.4.0's
// Lev1TParametricDescription (Apache 2.0; underlying tables MIT-licensed via
// moman/finenight).

package automaton

type lev1TParametricDescription struct {
	baseParametricDescription
}

func newLev1TParametricDescription(w int) *lev1TParametricDescription {
	return &lev1TParametricDescription{
		baseParametricDescription: baseParametricDescription{
			w:         w,
			n:         1,
			minErrors: []int{0, 1, 0, -1, -1, -1},
		},
	}
}

func (d *lev1TParametricDescription) size() int             { return d.baseParametricDescription.size() }
func (d *lev1TParametricDescription) isAccept(s int) bool   { return d.isAcceptInline(s) }
func (d *lev1TParametricDescription) getPosition(s int) int { return d.getPositionInline(s) }

func (d *lev1TParametricDescription) transition(absState, position, vector int) int {
	state := absState / (d.w + 1)
	offset := absState % (d.w + 1)

	switch {
	case position == d.w:
		if state < 2 {
			loc := vector*2 + state
			offset += unpack(lev1tToOffsetIncrs0, loc, 1)
			state = unpack(lev1tToStates0, loc, 2) - 1
		}
	case position == d.w-1:
		if state < 3 {
			loc := vector*3 + state
			offset += unpack(lev1tToOffsetIncrs1, loc, 1)
			state = unpack(lev1tToStates1, loc, 2) - 1
		}
	case position == d.w-2:
		if state < 6 {
			loc := vector*6 + state
			offset += unpack(lev1tToOffsetIncrs2, loc, 2)
			state = unpack(lev1tToStates2, loc, 3) - 1
		}
	default:
		if state < 6 {
			loc := vector*6 + state
			offset += unpack(lev1tToOffsetIncrs3, loc, 2)
			state = unpack(lev1tToStates3, loc, 3) - 1
		}
	}

	if state == -1 {
		return -1
	}
	return state*(d.w+1) + offset
}

var (
	lev1tToStates0      = []uint64{0x2}
	lev1tToOffsetIncrs0 = []uint64{0x0}

	lev1tToStates1      = []uint64{0xa43}
	lev1tToOffsetIncrs1 = []uint64{0x38}

	lev1tToStates2      = []uint64{0xb45a491412180003, 0x69}
	lev1tToOffsetIncrs2 = []uint64{0x5555558a0000}

	lev1tToStates3      = []uint64{0xa1904864900c0003, 0x5a6d196a45a49169, 0x9634}
	lev1tToOffsetIncrs3 = []uint64{0x5555ba08a0fc0000, 0x55555555}
)
