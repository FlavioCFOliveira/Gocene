package tst

// TSTAutocomplete is the ternary-search-tree backbone used by TSTLookup.
// Mirrors org.apache.lucene.search.suggest.tst.TSTAutocomplete.
type TSTAutocomplete struct {
	root *TernaryTreeNode
}

// NewTSTAutocomplete builds an empty tree.
func NewTSTAutocomplete() *TSTAutocomplete { return &TSTAutocomplete{} }

// Insert stores token with the supplied value.
func (t *TSTAutocomplete) Insert(token string, val int64) {
	t.root = insert(t.root, []rune(token), 0, token, val)
}

func insert(node *TernaryTreeNode, chars []rune, pos int, token string, val int64) *TernaryTreeNode {
	if len(chars) == 0 {
		return node
	}
	c := chars[pos]
	if node == nil {
		node = NewTernaryTreeNode(c)
	}
	switch {
	case c < node.SplitChar:
		node.Loword = insert(node.Loword, chars, pos, token, val)
	case c > node.SplitChar:
		node.Hiword = insert(node.Hiword, chars, pos, token, val)
	default:
		if pos < len(chars)-1 {
			node.Eqkid = insert(node.Eqkid, chars, pos+1, token, val)
		} else {
			node.Token = token
			node.Val = val
		}
	}
	return node
}

// PrefixCompletion returns every (token, value) pair whose key starts with
// prefix. Results are unordered; callers may sort by value.
type CompletionEntry struct {
	Token string
	Val   int64
}

// PrefixCompletion returns matching entries for prefix.
func (t *TSTAutocomplete) PrefixCompletion(prefix string) []CompletionEntry {
	chars := []rune(prefix)
	node := find(t.root, chars, 0)
	var out []CompletionEntry
	if node == nil {
		return out
	}
	// node points to the node matching the last prefix character; the
	// completion subtree hangs off node.Eqkid (children continuing the prefix)
	// and node itself records the matching token if any.
	if node.Token != "" {
		out = append(out, CompletionEntry{Token: node.Token, Val: node.Val})
	}
	collect(node.Eqkid, &out)
	return out
}

func find(node *TernaryTreeNode, chars []rune, pos int) *TernaryTreeNode {
	if node == nil {
		return nil
	}
	c := chars[pos]
	switch {
	case c < node.SplitChar:
		return find(node.Loword, chars, pos)
	case c > node.SplitChar:
		return find(node.Hiword, chars, pos)
	default:
		if pos == len(chars)-1 {
			return node
		}
		return find(node.Eqkid, chars, pos+1)
	}
}

func collect(node *TernaryTreeNode, out *[]CompletionEntry) {
	if node == nil {
		return
	}
	collect(node.Loword, out)
	if node.Token != "" {
		*out = append(*out, CompletionEntry{Token: node.Token, Val: node.Val})
	}
	collect(node.Eqkid, out)
	collect(node.Hiword, out)
}
