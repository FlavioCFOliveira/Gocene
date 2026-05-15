// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Auto-generated parametric description for Levenshtein degree 2 (no
// transpositions). Translated verbatim from Lucene 10.4.0's
// Lev2ParametricDescription (Apache 2.0; underlying tables MIT-licensed).

package automaton

type lev2ParametricDescription struct {
	baseParametricDescription
}

func newLev2ParametricDescription(w int) *lev2ParametricDescription {
	return &lev2ParametricDescription{
		baseParametricDescription: baseParametricDescription{
			w: w,
			n: 2,
			minErrors: []int{
				0, 1, 2, 0, 1, -1, 0, -1, 0, -1, 0, -1, -1, -1, -1, -2,
				-1, -2, -1, -2, -1, -2, -2, -2, -2, -2, -2, -2, -2, -2,
			},
		},
	}
}

func (d *lev2ParametricDescription) size() int             { return d.baseParametricDescription.size() }
func (d *lev2ParametricDescription) isAccept(s int) bool   { return d.isAcceptInline(s) }
func (d *lev2ParametricDescription) getPosition(s int) int { return d.getPositionInline(s) }

func (d *lev2ParametricDescription) transition(absState, position, vector int) int {
	state := absState / (d.w + 1)
	offset := absState % (d.w + 1)

	switch {
	case position == d.w:
		if state < 3 {
			loc := vector*3 + state
			offset += unpack(lev2ToOffsetIncrs0, loc, 1)
			state = unpack(lev2ToStates0, loc, 2) - 1
		}
	case position == d.w-1:
		if state < 5 {
			loc := vector*5 + state
			offset += unpack(lev2ToOffsetIncrs1, loc, 1)
			state = unpack(lev2ToStates1, loc, 3) - 1
		}
	case position == d.w-2:
		if state < 11 {
			loc := vector*11 + state
			offset += unpack(lev2ToOffsetIncrs2, loc, 2)
			state = unpack(lev2ToStates2, loc, 4) - 1
		}
	case position == d.w-3:
		if state < 21 {
			loc := vector*21 + state
			offset += unpack(lev2ToOffsetIncrs3, loc, 2)
			state = unpack(lev2ToStates3, loc, 5) - 1
		}
	case position == d.w-4:
		if state < 30 {
			loc := vector*30 + state
			offset += unpack(lev2ToOffsetIncrs4, loc, 3)
			state = unpack(lev2ToStates4, loc, 5) - 1
		}
	default:
		if state < 30 {
			loc := vector*30 + state
			offset += unpack(lev2ToOffsetIncrs5, loc, 3)
			state = unpack(lev2ToStates5, loc, 5) - 1
		}
	}

	if state == -1 {
		return -1
	}
	return state*(d.w+1) + offset
}

var (
	lev2ToStates0      = []uint64{0xe}
	lev2ToOffsetIncrs0 = []uint64{0x0}

	lev2ToStates1      = []uint64{0x1a688a2c}
	lev2ToOffsetIncrs1 = []uint64{0x3e0}

	lev2ToStates2 = []uint64{
		0x3a07603570707054, 0x522323232103773a, 0x352254543213,
	}
	lev2ToOffsetIncrs2 = []uint64{0x5555520880080000, 0x555555}

	lev2ToStates3 = []uint64{
		0x7000a560180380a4, 0xc015a0180a0194a, 0x8032c58318a301c0, 0x9d8350d403980318,
		0x3006028ca73a8602, 0xc51462640b21a807, 0x2310c4100c62194e, 0xce35884218ce248d,
		0xa9285a0691882358, 0x1046b5a86b1252b5, 0x2110a33892521483, 0xe62906208d63394e,
		0xd6a29c4921d6a4a0, 0x1a,
	}
	lev2ToOffsetIncrs3 = []uint64{
		0xf0c000c8c0080000,
		0xca808822003f303,
		0x5555553fa02f0880,
		0x5555555555555555,
		0x5555555555555555,
		0x5555,
	}

	lev2ToStates4 = []uint64{
		0x7000a560180380a4, 0xa000000280e0294a, 0x6c0b00e029000000, 0x8c4350c59cdc6039,
		0x600ad00c03380601, 0x2962c18c5180e00, 0x18c4000c6028c4, 0x8a314603801802b4,
		0x6328c4520c59c5, 0x60d43500e600c651, 0x280e339cea180a7, 0x4039800000a318c6,
		0xd57be96039ec3d0d, 0xc0338d6358c4352, 0x28c4c81643500e60, 0x3194a028c4339d8a,
		0x590d403980018c4, 0xc4522d57b68e3132, 0xc4100c6510d6538, 0x9884218ce248d231,
		0x318ce318c6398d83, 0xa3609c370c431046, 0xea3ad6958568f7be, 0x2d0348c411d47560,
		0x9ad43989295ad494, 0x3104635ad431ad63, 0x8f73a6b5250b40d2, 0x57350eab9d693956,
		0x8ce24948520c411d, 0x294a398d85608442, 0x5694831046318ce5, 0x958460f7b623609c,
		0xc411d475616258d6, 0x9243ad4941cc520, 0x5ad4529ce39ad456, 0xb525073148310463,
		0x27656939460f7358, 0x1d573516,
	}
	lev2ToOffsetIncrs4 = []uint64{
		0x610600010000000, 0x2040000000001000, 0x1044209245200, 0x80d86d86006d80c0,
		0x2001b6030000006d, 0x8200011b6237237, 0x12490612400410, 0x2449001040208000,
		0x4d80820001044925, 0x6da4906da400, 0x9252369001360208, 0x24924924924911b6,
		0x9249249249249249, 0x4924924924924924, 0x2492492492492492, 0x9249249249249249,
		0x4924924924924924, 0x2492492492492492, 0x9249249249249249, 0x4924924924924924,
		0x2492492492492492, 0x9249249249249249, 0x24924924,
	}

	lev2ToStates5 = []uint64{
		0x7000a560180380a4, 0xa000000280e0294a, 0x580600e029000000, 0x80e0600e529c0029,
		0x380a418c6388c631, 0x316737180e5b02c0, 0x300ce01806310d4, 0xc60396c0b00e0290,
		0xca328c4350c59cd, 0x80e00600ad194656, 0x28c402962c18c51, 0x802b40018c4000c6,
		0xe58b06314603801, 0x8d6b48c6b580e348, 0x28c5180e00600ad1, 0x18ca31148316716,
		0x3801802b4031944, 0xc4520c59c58a3146, 0xe61956748cab38, 0x39cea180a760d435,
		0xa318c60280e3, 0x6029d8350d403980, 0x6b5a80e060d873a8, 0xf43500e618c638d,
		0x10d4b55efa580e7b, 0x3980300ce358d63, 0x57be96039ec3d0d4, 0x4656567598c4352d,
		0x8c4c81643500e619, 0x194a028c4339d8a2, 0x590d403980018c43, 0xe348d87628a31320,
		0xe618d6b4d6b1880, 0x5eda38c4c8164350, 0x19443594e31148b5, 0x31320590d4039803,
		0x7160c4522d57b68e, 0xd2310c41195674d6, 0x8d839884218ce248, 0x1046318ce318c639,
		0x2108633892348c43, 0xdebfbdef0f63b0f6, 0xd8270dc310c41f7b, 0x8eb5a5615a3defa8,
		0x70c43104751d583a, 0x58568f7bea3609c3, 0x41f77ddb7bbeed69, 0x9295ad4942d0348c,
		0xad431ad639ad4398, 0x5250b40d23104635, 0xce0f6bd0f624a56b, 0x348c41f7b9cd7bd,
		0xe55a3dce9ad4942d, 0x4755cd43aae75a4, 0x73a6b5250b40d231, 0xbd7bbcdd6939568f,
		0xe24948520c41f779, 0x4a398d856084428c, 0x14831046318ce529, 0xb16c2110a3389252,
		0x1f7bdebe739c8f63, 0xed88d82715a520c4, 0x58589635a561183d, 0x9c569483104751d,
		0xc56958460f7b6236, 0x520c41f77ddb6719, 0x45609243ad4941cc, 0x4635ad4529ce39ad,
		0x90eb525073148310, 0xd6737b8f6bd16c24, 0x941cc520c41f7b9c, 0x95a4e5183dcd62d4,
		0x483104755cd4589d, 0x460f7358b5250731, 0xf779bd6717b56939,
	}
	lev2ToOffsetIncrs5 = []uint64{
		0x610600010000000, 0x40000000001000, 0xb6d56da184180, 0x824914800810000,
		0x2002040000000411, 0xc0000b2c5659245, 0x6d80d86d86006d8, 0x1b61801b60300000,
		0x6d80c0000b5b76b6, 0x46d88dc8dc800, 0x6372372001b60300, 0x400410082000b1b7,
		0x2080000012490612, 0x6d49241849001040, 0x912400410082000b, 0x402080004112494,
		0xb2c49252449001, 0x4906da4004d80820, 0x136020800006da, 0x82000b5b69241b69,
		0x6da4948da4004d80, 0x3690013602080004, 0x49249249b1b69252, 0x2492492492492492,
		0x9249249249249249, 0x4924924924924924, 0x2492492492492492, 0x9249249249249249,
		0x4924924924924924, 0x2492492492492492, 0x9249249249249249, 0x4924924924924924,
		0x2492492492492492, 0x9249249249249249, 0x4924924924924924, 0x2492492492492492,
		0x9249249249249249, 0x4924924924924924, 0x2492492492492492, 0x9249249249249249,
		0x4924924924924924, 0x2492492492492492, 0x9249249249249249, 0x4924924924924924,
		0x2492492492492492,
	}
)
