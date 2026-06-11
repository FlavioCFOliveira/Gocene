package tst

import (
	"sort"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/suggest"
)

// Bit masks for TST serialisation, matching the Java original.
const (
	maskLoKid    = 0x01
	maskEqKid    = 0x02
	maskHiKid    = 0x04
	maskHasToken = 0x08
	maskHasValue = 0x10
)

// TSTLookup is the suggest.Lookup-compliant front-end for the TST tree.
// Mirrors org.apache.lucene.search.suggest.tst.TSTLookup.
type TSTLookup struct {
	tree  *TSTAutocomplete
	count int64
}

// NewTSTLookup builds an empty lookup.
func NewTSTLookup() *TSTLookup { return &TSTLookup{tree: NewTSTAutocomplete()} }

// Build ingests an InputIterator.
func (l *TSTLookup) Build(it suggest.InputIterator) error {
	for {
		t, w, _, _, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		l.tree.Insert(string(t), w)
		l.count++
	}
}

// LookupResults returns up to num completions for key sorted by descending
// weight.
func (l *TSTLookup) LookupResults(key string, _ [][]byte, _ bool, num int) ([]*suggest.LookupResult, error) {
	if num < 1 {
		num = 10
	}
	entries := l.tree.PrefixCompletion(key)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Val > entries[j].Val })
	if len(entries) > num {
		entries = entries[:num]
	}
	out := make([]*suggest.LookupResult, len(entries))
	for i, e := range entries {
		out[i] = suggest.NewLookupResult(e.Token, e.Val)
	}
	return out, nil
}

// GetCount returns the number of tokens stored.
func (l *TSTLookup) GetCount() int64 { return l.count }

// Store serialises the TST to output using pre-order traversal.
// Mirrors TSTLookup.store(DataOutput) from Lucene 10.4.0.
//
// Wire format:
//
//	writeVLong(count)
//	for each node (pre-order):
//	  writeString(splitchar as 1-char string)
//	  writeByte(mask)
//	  if HAS_TOKEN: writeString(token)
//	  if HAS_VALUE: writeLong(val)
//	  recurse into loKid, eqKid, hiKid (if present per mask)
func (l *TSTLookup) Store(output store.DataOutput) (bool, error) {
	if err := store.WriteVLong(output, l.count); err != nil {
		return false, err
	}
	if l.tree.root != nil {
		if err := writeTSTNode(output, l.tree.root); err != nil {
			return false, err
		}
	}
	return true, nil
}

// Load reads a serialised TST produced by Store (or Lucene's store()).
// Returns true on success. Mirrors TSTLookup.load(DataInput).
func (l *TSTLookup) Load(input store.DataInput) (bool, error) {
	cnt, err := store.ReadVLong(input)
	if err != nil {
		return false, err
	}
	l.count = cnt
	l.tree = NewTSTAutocomplete()
	if cnt > 0 {
		node, err := readTSTNode(input)
		if err != nil {
			return false, err
		}
		l.tree.root = node
	}
	return true, nil
}

// writeTSTNode recursively serialises a node and its children (pre-order).
func writeTSTNode(output store.DataOutput, node *TernaryTreeNode) error {
	if err := output.WriteString(string(node.SplitChar)); err != nil {
		return err
	}
	mask := byte(0)
	if node.Loword != nil {
		mask |= maskLoKid
	}
	if node.Eqkid != nil {
		mask |= maskEqKid
	}
	if node.Hiword != nil {
		mask |= maskHiKid
	}
	if node.Token != "" {
		mask |= maskHasToken
		mask |= maskHasValue // val is always set alongside token
	}
	if err := output.WriteByte(mask); err != nil {
		return err
	}
	if node.Token != "" {
		if err := output.WriteString(node.Token); err != nil {
			return err
		}
		if err := output.WriteLong(node.Val); err != nil {
			return err
		}
	}
	if node.Loword != nil {
		if err := writeTSTNode(output, node.Loword); err != nil {
			return err
		}
	}
	if node.Eqkid != nil {
		if err := writeTSTNode(output, node.Eqkid); err != nil {
			return err
		}
	}
	if node.Hiword != nil {
		if err := writeTSTNode(output, node.Hiword); err != nil {
			return err
		}
	}
	return nil
}

// readTSTNode recursively deserialises a node and its children (pre-order).
func readTSTNode(input store.DataInput) (*TernaryTreeNode, error) {
	splitStr, err := input.ReadString()
	if err != nil {
		return nil, err
	}
	var splitChar rune
	if len(splitStr) > 0 {
		splitChar = rune(splitStr[0])
	}
	node := NewTernaryTreeNode(splitChar)
	mask, err := input.ReadByte()
	if err != nil {
		return nil, err
	}
	if (mask & maskHasToken) != 0 {
		node.Token, err = input.ReadString()
		if err != nil {
			return nil, err
		}
	}
	if (mask & maskHasValue) != 0 {
		node.Val, err = input.ReadLong()
		if err != nil {
			return nil, err
		}
	}
	if (mask & maskLoKid) != 0 {
		node.Loword, err = readTSTNode(input)
		if err != nil {
			return nil, err
		}
	}
	if (mask & maskEqKid) != 0 {
		node.Eqkid, err = readTSTNode(input)
		if err != nil {
			return nil, err
		}
	}
	if (mask & maskHiKid) != 0 {
		node.Hiword, err = readTSTNode(input)
		if err != nil {
			return nil, err
		}
	}
	return node, nil
}

var _ suggest.Lookup = (*TSTLookup)(nil)
