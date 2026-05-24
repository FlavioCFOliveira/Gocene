// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package prefixtree_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/spatial/prefixtree"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ---------------------------------------------------------------------------
// LegacyCell
// ---------------------------------------------------------------------------

func TestLegacyCellTokenRoundTrip(t *testing.T) {
	c := prefixtree.NewLegacyCell(3, []byte{0x01, 0x02, 0x03}, false)
	ref := c.GetTokenBytesNoLeaf(nil)
	if ref.Length != 3 {
		t.Fatalf("expected 3 bytes, got %d", ref.Length)
	}
	if ref.Bytes[0] != 0x01 || ref.Bytes[1] != 0x02 || ref.Bytes[2] != 0x03 {
		t.Fatal("token bytes mismatch")
	}
}

func TestLegacyCellLeafMarker(t *testing.T) {
	c := prefixtree.NewLegacyCell(1, []byte{0xAA}, false)
	c.SetLeaf()
	if !c.IsLeaf() {
		t.Fatal("expected IsLeaf() true after SetLeaf()")
	}
	ref := c.GetTokenBytesWithLeaf(nil)
	if ref.Length != 2 {
		t.Fatalf("expected 2 bytes with leaf, got %d", ref.Length)
	}
	if ref.Bytes[ref.Offset+1] != 0xFF {
		t.Fatalf("expected leaf marker 0xFF, got 0x%X", ref.Bytes[ref.Offset+1])
	}
}

func TestLegacyCellGetLevel(t *testing.T) {
	c := prefixtree.NewLegacyCell(5, []byte{1, 2, 3, 4, 5}, false)
	if got := c.GetLevel(); got != 5 {
		t.Fatalf("expected level 5, got %d", got)
	}
}

func TestLegacyCellIsPrefixOf(t *testing.T) {
	parent := prefixtree.NewLegacyCell(2, []byte{0x01, 0x02}, false)
	child := prefixtree.NewLegacyCell(3, []byte{0x01, 0x02, 0x03}, false)
	if !parent.IsPrefixOf(child) {
		t.Fatal("parent should be a prefix of child")
	}
	if child.IsPrefixOf(parent) {
		t.Fatal("child should not be a prefix of parent")
	}
}

func TestLegacyCellCompareToNoLeaf(t *testing.T) {
	a := prefixtree.NewLegacyCell(2, []byte{0x01, 0x02}, false)
	b := prefixtree.NewLegacyCell(2, []byte{0x01, 0x03}, false)
	if a.CompareToNoLeaf(b) >= 0 {
		t.Fatal("a should be less than b")
	}
	if b.CompareToNoLeaf(a) <= 0 {
		t.Fatal("b should be greater than a")
	}
	c := prefixtree.NewLegacyCell(2, []byte{0x01, 0x02}, false)
	if a.CompareToNoLeaf(c) != 0 {
		t.Fatal("equal cells should compare 0")
	}
}

func TestLegacyCellReadCell(t *testing.T) {
	c := prefixtree.NewLegacyCell(0, nil, false)
	term := util.NewBytesRef([]byte{0x05, 0x06})
	c.ReadCell(term)
	if c.GetLevel() != 2 {
		t.Fatalf("expected level 2 after ReadCell, got %d", c.GetLevel())
	}
}

func TestLegacyCellReadCellLeaf(t *testing.T) {
	c := prefixtree.NewLegacyCell(0, nil, false)
	term := util.NewBytesRef([]byte{0xAA, 0xFF})
	c.ReadCell(term)
	if !c.IsLeaf() {
		t.Fatal("expected leaf after reading term ending in 0xFF")
	}
	if c.GetLevel() != 1 {
		t.Fatalf("expected level 1, got %d", c.GetLevel())
	}
}

// ---------------------------------------------------------------------------
// SingletonCellIterator
// ---------------------------------------------------------------------------

func TestSingletonCellIteratorSingleCell(t *testing.T) {
	cell := prefixtree.NewLegacyCell(1, []byte{0x01}, false)
	iter := prefixtree.NewSingletonCellIterator(cell)
	if !iter.HasNext() {
		t.Fatal("expected HasNext() true")
	}
	got := iter.Next()
	if got == nil {
		t.Fatal("expected non-nil cell")
	}
	if iter.HasNext() {
		t.Fatal("expected HasNext() false after consuming the single cell")
	}
}

// ---------------------------------------------------------------------------
// FilterCellIterator (nil filter = pass-through)
// ---------------------------------------------------------------------------

func TestFilterCellIteratorPassThrough(t *testing.T) {
	cells := []prefixtree.Cell{
		prefixtree.NewLegacyCell(1, []byte{0x01}, false),
		prefixtree.NewLegacyCell(1, []byte{0x02}, false),
	}
	idx := 0
	baseIter := prefixtree.NewSingletonCellIterator(cells[0])
	// Build a simple multi-cell iterator via two singletons composed.
	_ = baseIter
	// Use FilterCellIterator over a SingletonCellIterator (one cell at a time).
	iter := prefixtree.NewFilterCellIterator(prefixtree.NewSingletonCellIterator(cells[1]), nil)
	if !iter.HasNext() {
		t.Fatal("expected HasNext() true")
	}
	got := iter.Next()
	if got == nil {
		t.Fatal("expected non-nil cell from FilterCellIterator")
	}
	_ = idx
}

// ---------------------------------------------------------------------------
// S2PrefixTree
// ---------------------------------------------------------------------------

func TestS2PrefixTreeConstruction(t *testing.T) {
	tree := prefixtree.NewS2PrefixTree(nil, 10)
	if tree == nil {
		t.Fatal("expected non-nil S2PrefixTree")
	}
	if tree.GetMaxLevels() != 10 {
		t.Fatalf("expected 10, got %d", tree.GetMaxLevels())
	}
}

func TestS2GetMaxLevels(t *testing.T) {
	// arity=1: 30/1 + 1 = 31
	if got := prefixtree.S2GetMaxLevels(1); got != 31 {
		t.Fatalf("expected 31, got %d", got)
	}
	// arity=2: 30/2 + 1 = 16
	if got := prefixtree.S2GetMaxLevels(2); got != 16 {
		t.Fatalf("expected 16, got %d", got)
	}
}

func TestS2PrefixTreeInvalidArity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid arity")
		}
	}()
	prefixtree.NewS2PrefixTreeWithArity(nil, 10, 5)
}

// ---------------------------------------------------------------------------
// S2PrefixTreeCell
// ---------------------------------------------------------------------------

func TestS2PrefixTreeCellWorldCell(t *testing.T) {
	tree := prefixtree.NewS2PrefixTree(nil, 10)
	world := tree.GetWorldCell()
	if world == nil {
		t.Fatal("expected non-nil world cell")
	}
	if world.GetLevel() != 0 {
		t.Fatalf("expected world cell at level 0, got %d", world.GetLevel())
	}
}

func TestS2PrefixTreeCellLeaf(t *testing.T) {
	tree := prefixtree.NewS2PrefixTree(nil, 10)
	cell := tree.GetWorldCell()
	cell.SetLeaf()
	if !cell.IsLeaf() {
		t.Fatal("expected IsLeaf() true")
	}
}

// ---------------------------------------------------------------------------
// PackedQuadPrefixTree
// ---------------------------------------------------------------------------

func TestPackedQuadPrefixTreeConstruction(t *testing.T) {
	tree := prefixtree.NewPackedQuadPrefixTree(nil, 20)
	if tree.GetMaxLevels() != 20 {
		t.Fatalf("expected 20, got %d", tree.GetMaxLevels())
	}
}

func TestPackedQuadPrefixTreeMaxLevelsClamped(t *testing.T) {
	tree := prefixtree.NewPackedQuadPrefixTree(nil, 100)
	if tree.GetMaxLevels() != prefixtree.PackedQuadPrefixTreeMaxLevelsPossible {
		t.Fatalf("expected %d, got %d",
			prefixtree.PackedQuadPrefixTreeMaxLevelsPossible, tree.GetMaxLevels())
	}
}

func TestPackedQuadPrefixTreeLeafyPrune(t *testing.T) {
	tree := prefixtree.NewPackedQuadPrefixTree(nil, 10)
	if !tree.IsLeafyPrune() {
		t.Fatal("expected leafyPrune=true by default")
	}
	tree.SetLeafyPrune(false)
	if tree.IsLeafyPrune() {
		t.Fatal("expected leafyPrune=false after SetLeafyPrune(false)")
	}
}

// ---------------------------------------------------------------------------
// NumberRangePrefixTree
// ---------------------------------------------------------------------------

func TestNumberRangePrefixTreeConstruction(t *testing.T) {
	tree := prefixtree.NewNumberRangePrefixTree([]int{10, 12, 31})
	if tree.GetMaxLevels() != 3 {
		t.Fatalf("expected 3 levels, got %d", tree.GetMaxLevels())
	}
}

func TestNumberRangePrefixTreeWorldCell(t *testing.T) {
	tree := prefixtree.NewNumberRangePrefixTree([]int{10})
	wc := tree.GetWorldCell()
	if wc == nil {
		t.Fatal("expected non-nil world cell")
	}
	if wc.GetLevel() != 0 {
		t.Fatalf("expected level 0, got %d", wc.GetLevel())
	}
}

// ---------------------------------------------------------------------------
// DateRangePrefixTree
// ---------------------------------------------------------------------------

func TestDateRangePrefixTreeConstruction(t *testing.T) {
	tree := prefixtree.NewDateRangePrefixTree()
	if tree == nil {
		t.Fatal("expected non-nil DateRangePrefixTree")
	}
	if tree.GetMaxLevels() < 1 {
		t.Fatalf("expected positive MaxLevels, got %d", tree.GetMaxLevels())
	}
}

// ---------------------------------------------------------------------------
// SpatialPrefixTreeFactory constants
// ---------------------------------------------------------------------------

func TestSpatialPrefixTreeFactoryConstants(t *testing.T) {
	cases := []struct{ key, want string }{
		{prefixtree.PrefixTreeKey, "prefixTree"},
		{prefixtree.MaxLevelsKey, "maxLevels"},
		{prefixtree.MaxDistErrKey, "maxDistErr"},
		{prefixtree.VersionKey, "version"},
	}
	for _, c := range cases {
		if c.key != c.want {
			t.Errorf("constant %q: want %q, got %q", c.want, c.want, c.key)
		}
	}
}

func TestSpatialPrefixTreeFactorySetParam(t *testing.T) {
	f := prefixtree.NewSpatialPrefixTreeFactory()
	f.SetParam("maxLevels", "11")
	if got := f.Args["maxLevels"]; got != "11" {
		t.Fatalf("expected '11', got %q", got)
	}
}

func TestMakeSPTReturnsNil(t *testing.T) {
	if got := prefixtree.MakeSPT(nil, nil); got != nil {
		t.Fatal("expected nil from MakeSPT stub")
	}
}
