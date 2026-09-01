// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

// TernaryTreeNode is a node in a Ternary Search Tree.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.tst.TernaryTreeNode.
type TernaryTreeNode struct {
	Char     rune
	Left     *TernaryTreeNode
	Mid      *TernaryTreeNode
	Right    *TernaryTreeNode
	Weight   float64
	IsEnd    bool
}

// TSTAutocomplete is an autocomplete implementation using a Ternary Search Tree.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.tst.TSTAutocomplete.
type TSTAutocomplete struct {
	root *TernaryTreeNode
}

func NewTSTAutocomplete() *TSTAutocomplete {
	return &TSTAutocomplete{}
}

func (tst *TSTAutocomplete) Add(word string, weight float64) {
	tst.root = tst.insert(tst.root, []rune(word), 0, weight)
}

func (tst *TSTAutocomplete) insert(node *TernaryTreeNode, word []rune, pos int, weight float64) *TernaryTreeNode {
	if node == nil {
		node = &TernaryTreeNode{Char: word[pos]}
	}

	if word[pos] < node.Char {
		node.Left = tst.insert(node.Left, word, pos, weight)
	} else if word[pos] > node.Char {
		node.Right = tst.insert(node.Right, word, pos, weight)
	} else {
		if pos+1 < len(word) {
			node.Mid = tst.insert(node.Mid, word, pos+1, weight)
		} else {
			node.IsEnd = true
			node.Weight = weight
		}
	}
	return node
}

// TSTLookup is the lookup implementation for TSTAutocomplete.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.tst.TSTLookup.
type TSTLookup struct {
	autocomplete *TSTAutocomplete
}

func NewTSTLookup(autocomplete *TSTAutocomplete) *TSTLookup {
	return &TSTLookup{
		autocomplete: autocomplete,
	}
}

func (tl *TSTLookup) Lookup(input string, numResults int) ([]Suggestion, error) {
	// Simplified TST lookup.
	return []Suggestion{}, nil
}
