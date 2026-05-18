// Package tst implements org.apache.lucene.search.suggest.tst: the
// ternary-search-tree autocomplete suggester.
package tst

// TernaryTreeNode is one node of the TST. Mirrors
// org.apache.lucene.search.suggest.tst.TernaryTreeNode.
type TernaryTreeNode struct {
	SplitChar rune
	Loword    *TernaryTreeNode
	Eqkid     *TernaryTreeNode
	Hiword    *TernaryTreeNode
	Token     string
	Val       int64
}

// NewTernaryTreeNode builds an empty node with the supplied split character.
func NewTernaryTreeNode(splitChar rune) *TernaryTreeNode {
	return &TernaryTreeNode{SplitChar: splitChar}
}
