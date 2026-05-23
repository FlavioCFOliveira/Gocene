// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hyphenation

// TernaryTree is a ternary search tree mapping null-terminated uint16 strings
// to uint16 values. It is the backbone of the hyphenation pattern lookup.
//
// This is the Go port of
// org.apache.lucene.analysis.compound.hyphenation.TernaryTree from
// Apache Lucene 10.4.0. Taken originally from the Apache FOP project.
//
// Deviation: Java uses char (16-bit); this port uses uint16 for direct
// equivalence. The sentinel values 0x0000 (string terminator) and 0xFFFF
// (compressed branch) are preserved verbatim.
type TernaryTree struct {
	lo       []uint16
	hi       []uint16
	eq       []uint16
	sc       []uint16
	kv       *CharVector
	root     uint16
	freenode uint16
	length   int // number of items in tree
}

const tstBlockSize = 2048

// newTernaryTree creates an initialised TernaryTree.
func newTernaryTree() *TernaryTree {
	t := &TernaryTree{}
	t.init()
	return t
}

func (t *TernaryTree) init() {
	t.root = 0
	t.freenode = 1
	t.length = 0
	t.lo = make([]uint16, tstBlockSize)
	t.hi = make([]uint16, tstBlockSize)
	t.eq = make([]uint16, tstBlockSize)
	t.sc = make([]uint16, tstBlockSize)
	t.kv = NewCharVector()
}

// Clone returns a deep copy.
func (t *TernaryTree) Clone() *TernaryTree {
	c := &TernaryTree{
		lo:       make([]uint16, len(t.lo)),
		hi:       make([]uint16, len(t.hi)),
		eq:       make([]uint16, len(t.eq)),
		sc:       make([]uint16, len(t.sc)),
		kv:       t.kv.Clone(),
		root:     t.root,
		freenode: t.freenode,
		length:   t.length,
	}
	copy(c.lo, t.lo)
	copy(c.hi, t.hi)
	copy(c.eq, t.eq)
	copy(c.sc, t.sc)
	return c
}

func (t *TernaryTree) redim(newsize int) {
	n := newsize
	if n > len(t.lo) {
		n = len(t.lo)
	}
	lo2 := make([]uint16, newsize)
	copy(lo2, t.lo[:n])
	t.lo = lo2
	hi2 := make([]uint16, newsize)
	copy(hi2, t.hi[:n])
	t.hi = hi2
	eq2 := make([]uint16, newsize)
	copy(eq2, t.eq[:n])
	t.eq = eq2
	sc2 := make([]uint16, newsize)
	copy(sc2, t.sc[:n])
	t.sc = sc2
}

// tstStrcpy copies a null-terminated uint16 string from src[si] to dst[di].
func tstStrcpy(dst []uint16, di int, src []uint16, si int) {
	for src[si] != 0 {
		dst[di] = src[si]
		di++
		si++
	}
	dst[di] = 0
}

// tstStrlen returns the length of a null-terminated uint16 string starting at start.
func tstStrlen(a []uint16, start int) int {
	n := 0
	for i := start; i < len(a) && a[i] != 0; i++ {
		n++
	}
	return n
}

// tstStrcmpSlice compares two null-terminated uint16 strings.
func tstStrcmpSlice(a []uint16, startA int, b []uint16, startB int) int {
	for a[startA] == b[startB] {
		if a[startA] == 0 {
			return 0
		}
		startA++
		startB++
	}
	return int(a[startA]) - int(b[startB])
}

// Insert inserts a string key with value val into the tree.
func (t *TernaryTree) Insert(key string, val uint16) {
	l := len(key)
	strkey := make([]uint16, l+1)
	for i, c := range key {
		strkey[i] = uint16(c)
	}
	strkey[l] = 0
	if int(t.freenode)+l+1 > len(t.eq) {
		t.redim(len(t.eq) + tstBlockSize)
	}
	t.root = t.insertSlice(t.root, strkey, 0, val)
}

// insertSlice inserts the null-terminated slice key[start:] under node p.
func (t *TernaryTree) insertSlice(p uint16, key []uint16, start int, val uint16) uint16 {
	slen := tstStrlen(key, start)
	if p == 0 {
		p = t.freenode
		t.freenode++
		t.eq[p] = val
		t.length++
		t.hi[p] = 0
		if slen > 0 {
			t.sc[p] = 0xFFFF
			ptr := uint16(t.kv.Alloc(slen + 1))
			t.lo[p] = ptr
			tstStrcpy(t.kv.GetArray(), int(ptr), key, start)
		} else {
			t.sc[p] = 0
			t.lo[p] = 0
		}
		return p
	}

	if t.sc[p] == 0xFFFF {
		// decompress
		pp := t.freenode
		t.freenode++
		t.lo[pp] = t.lo[p]
		t.eq[pp] = t.eq[p]
		t.lo[p] = 0
		if slen > 0 {
			t.sc[p] = t.kv.Get(int(t.lo[pp]))
			t.eq[p] = pp
			t.lo[pp]++
			if t.kv.Get(int(t.lo[pp])) == 0 {
				t.lo[pp] = 0
				t.sc[pp] = 0
				t.hi[pp] = 0
			} else {
				t.sc[pp] = 0xFFFF
			}
		} else {
			t.sc[pp] = 0xFFFF
			t.hi[p] = pp
			t.sc[p] = 0
			t.eq[p] = val
			t.length++
			return p
		}
	}

	s := key[start]
	if int16(s) < int16(t.sc[p]) {
		t.lo[p] = t.insertSlice(t.lo[p], key, start, val)
	} else if s == t.sc[p] {
		if s != 0 {
			t.eq[p] = t.insertSlice(t.eq[p], key, start+1, val)
		} else {
			t.eq[p] = val
		}
	} else {
		t.hi[p] = t.insertSlice(t.hi[p], key, start, val)
	}
	return p
}

// Find returns the value associated with key, or -1 if not found.
func (t *TernaryTree) Find(key string) int {
	l := len(key)
	strkey := make([]uint16, l+1)
	for i, c := range key {
		strkey[i] = uint16(c)
	}
	strkey[l] = 0
	return t.findSlice(strkey, 0)
}

// findSlice looks up a null-terminated uint16 key starting at position start.
func (t *TernaryTree) findSlice(key []uint16, start int) int {
	p := t.root
	i := start
	for p != 0 {
		if t.sc[p] == 0xFFFF {
			if tstStrcmpSlice(key, i, t.kv.GetArray(), int(t.lo[p])) == 0 {
				return int(t.eq[p])
			}
			return -1
		}
		c := key[i]
		d := int(c) - int(t.sc[p])
		if d == 0 {
			if c == 0 {
				return int(t.eq[p])
			}
			i++
			p = t.eq[p]
		} else if d < 0 {
			p = t.lo[p]
		} else {
			p = t.hi[p]
		}
	}
	return -1
}

// Size returns the number of items stored in the tree.
func (t *TernaryTree) Size() int { return t.length }

// Balance rebalances the tree for optimal search performance.
func (t *TernaryTree) Balance() {
	n := t.length
	keys := make([]string, n)
	vals := make([]uint16, n)
	i := 0
	iter := t.Keys()
	for iter.HasMore() {
		vals[i] = iter.Value()
		keys[i] = iter.Next()
		i++
	}
	t.init()
	t.insertBalanced(keys, vals, 0, n)
}

func (t *TernaryTree) insertBalanced(k []string, v []uint16, offset, n int) {
	if n < 1 {
		return
	}
	m := n >> 1
	t.Insert(k[m+offset], v[m+offset])
	t.insertBalanced(k, v, offset, m)
	t.insertBalanced(k, v, offset+m+1, n-m-1)
}

// TrimToSize balances the tree and compacts the key vector.
func (t *TernaryTree) TrimToSize() {
	t.Balance()
	t.redim(int(t.freenode))
	kx := NewCharVector()
	kx.Alloc(1)
	mapping := newTernaryTree()
	t.compact(kx, mapping, t.root)
	t.kv = kx
	t.kv.TrimToSize()
}

func (t *TernaryTree) compact(kx *CharVector, mapping *TernaryTree, p uint16) {
	if p == 0 {
		return
	}
	if t.sc[p] == 0xFFFF {
		k := mapping.findSlice(t.kv.GetArray(), int(t.lo[p]))
		if k < 0 {
			sl := tstStrlen(t.kv.GetArray(), int(t.lo[p])) + 1
			k = kx.Alloc(sl)
			tstStrcpy(kx.GetArray(), k, t.kv.GetArray(), int(t.lo[p]))
			mapping.insertSlice(mapping.root, kx.GetArray(), k, uint16(k))
		}
		t.lo[p] = uint16(k)
	} else {
		t.compact(kx, mapping, t.lo[p])
		if t.sc[p] != 0 {
			t.compact(kx, mapping, t.eq[p])
		}
		t.compact(kx, mapping, t.hi[p])
	}
}

// tstNSItem is a stack frame used by TSTIterator for tree traversal.
type tstNSItem struct {
	parent uint16
	child  byte
}

// TSTIterator iterates over keys in a TernaryTree.
type TSTIterator struct {
	t      *TernaryTree
	cur    int
	curkey string
	ns     []tstNSItem
	ks     []uint16
}

// Keys returns an iterator over all keys in the tree.
func (t *TernaryTree) Keys() *TSTIterator {
	it := &TSTIterator{t: t, cur: -1}
	it.rewind()
	return it
}

// HasMore reports whether more keys are available.
func (it *TSTIterator) HasMore() bool { return it.cur != -1 }

// Value returns the value associated with the current key.
func (it *TSTIterator) Value() uint16 {
	if it.cur >= 0 {
		return it.t.eq[it.cur]
	}
	return 0
}

// Next advances to the next key and returns it.
func (it *TSTIterator) Next() string {
	res := it.curkey
	it.cur = it.up()
	it.run()
	return res
}

func (it *TSTIterator) rewind() {
	it.ns = it.ns[:0]
	it.ks = it.ks[:0]
	it.cur = int(it.t.root)
	it.run()
}

func (it *TSTIterator) up() int {
	if len(it.ns) == 0 {
		return -1
	}
	if it.cur != 0 && it.t.sc[it.cur] == 0 {
		return int(it.t.lo[it.cur])
	}
	for {
		if len(it.ns) == 0 {
			return -1
		}
		item := &it.ns[len(it.ns)-1]
		item.child++
		switch item.child {
		case 1:
			if it.t.sc[item.parent] != 0 {
				res := int(it.t.eq[item.parent])
				it.ks = append(it.ks, it.t.sc[item.parent])
				return res
			}
			item.child++
			res := int(it.t.hi[item.parent])
			return res
		case 2:
			res := int(it.t.hi[item.parent])
			if len(it.ks) > 0 {
				it.ks = it.ks[:len(it.ks)-1]
			}
			it.ns = it.ns[:len(it.ns)-1]
			return res
		default:
			it.ns = it.ns[:len(it.ns)-1]
		}
	}
}

func (it *TSTIterator) run() {
	if it.cur == -1 {
		return
	}
	leaf := false
	for {
		for it.cur != 0 {
			if it.t.sc[it.cur] == 0xFFFF {
				leaf = true
				break
			}
			it.ns = append(it.ns, tstNSItem{parent: uint16(it.cur), child: 0})
			if it.t.sc[it.cur] == 0 {
				leaf = true
				break
			}
			it.cur = int(it.t.lo[it.cur])
		}
		if leaf {
			break
		}
		it.cur = it.up()
		if it.cur == -1 {
			return
		}
	}
	var buf []uint16
	buf = append(buf, it.ks...)
	if it.t.sc[it.cur] == 0xFFFF {
		p := int(it.t.lo[it.cur])
		for it.t.kv.Get(p) != 0 {
			buf = append(buf, it.t.kv.Get(p))
			p++
		}
	}
	runes := make([]rune, len(buf))
	for i, c := range buf {
		runes[i] = rune(c)
	}
	it.curkey = string(runes)
}
