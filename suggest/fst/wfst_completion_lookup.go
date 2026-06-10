// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package fst

import (
	"bytes"
	"fmt"
	"math"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/suggest"
	"github.com/FlavioCFOliveira/Gocene/util"
	fstp "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// WFSTCompletionLookup is the weighted variant that returns completions
// sorted by descending weight rather than bucket. Mirrors
// org.apache.lucene.search.suggest.fst.WFSTCompletionLookup.
type WFSTCompletionLookup struct {
	fst        *fstp.FST[int64]
	count      int64
	exactFirst bool
}

// NewWFSTCompletionLookup builds an empty lookup with exactFirst=true.
func NewWFSTCompletionLookup() *WFSTCompletionLookup {
	return &WFSTCompletionLookup{exactFirst: true}
}

// Build ingests an InputIterator, sorts and deduplicates entries, and compiles
// an FST with PositiveIntOutputs where the stored value is the encoded cost
// (Integer.MAX_VALUE - weight).
func (l *WFSTCompletionLookup) Build(it suggest.InputIterator) error {
	if it.HasPayloads() {
		return fmt.Errorf("WFSTCompletionLookup does not support payloads")
	}
	if it.HasContexts() {
		return fmt.Errorf("WFSTCompletionLookup does not support contexts")
	}

	// Buffer all entries.
	buf, err := suggest.NewBufferedInputIterator(it)
	if err != nil {
		return err
	}

	// Sort by term ascending, then by weight descending so that the best
	// weight is kept when we deduplicate.
	type entry struct {
		term   []byte
		weight int64
	}
	entries := make([]entry, buf.Count())
	for i := 0; i < buf.Count(); i++ {
		t, w, _, _ := buf.At(i)
		entries[i] = entry{term: append([]byte(nil), t...), weight: w}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		cmp := bytes.Compare(entries[i].term, entries[j].term)
		if cmp != 0 {
			return cmp < 0
		}
		return entries[i].weight > entries[j].weight
	})

	// Deduplicate by term (keep the first, which has the highest weight).
	var deduped []entry
	for _, e := range entries {
		if len(deduped) == 0 || !bytes.Equal(deduped[len(deduped)-1].term, e.term) {
			deduped = append(deduped, e)
		}
	}

	// Compile FST.
	if len(deduped) == 0 {
		l.fst = nil
		l.count = 0
		return nil
	}

	builder := fstp.NewFSTCompilerBuilder[int64](fstp.InputTypeByte1, fstp.PositiveIntOutputs())
	builder.AllowFixedLengthArcs(true)
	compiler := builder.Build()
	scratchInts := util.NewIntsRefBuilder()
	for _, e := range deduped {
		cost := int64(encodeWeight(e.weight))
		scratchInts.Grow(len(e.term))
		for i, b := range e.term {
			scratchInts.SetIntAt(i, int(b)&0xff)
		}
		scratchInts.SetLength(len(e.term))
		if err := compiler.Add(scratchInts.Get(), cost); err != nil {
			return err
		}
	}

	meta, err := compiler.Compile()
	if err != nil {
		return err
	}
	l.fst, err = fstp.FromFSTReader[int64](meta, compiler.GetFSTReader())
	if err != nil {
		return err
	}
	l.count = int64(len(deduped))
	return nil
}

// LookupResults returns up to num completions for key.
func (l *WFSTCompletionLookup) LookupResults(key string, _ [][]byte, _ bool, num int) ([]*suggest.LookupResult, error) {
	if num < 1 {
		num = 10
	}
	if l.fst == nil {
		return nil, nil
	}

	scratch := []byte(key)
	prefixLen := len(scratch)
	var arc fstp.Arc[int64]
	prefixOutput, ok, err := l.lookupPrefix(scratch, &arc)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	results := make([]*suggest.LookupResult, 0, num)
	if l.exactFirst && arc.IsFinal() {
		results = append(results, suggest.NewLookupResult(key, decodeWeight(prefixOutput+arc.NextFinalOutput())))
		if len(results) == num {
			return results, nil
		}
	}

	completions, err := fstp.ShortestPaths(l.fst, &arc, prefixOutput, int64Comparator, num-len(results), !l.exactFirst)
	if err != nil {
		return nil, err
	}

	for _, c := range completions.TopN {
		suffix := fstp.ToBytesRef(c.Input, util.NewBytesRefBuilder())
		fullTerm := append(append([]byte(nil), scratch[:prefixLen]...), suffix.Bytes[suffix.Offset:suffix.Offset+suffix.Length]...)
		results = append(results, suggest.NewLookupResult(string(fullTerm), decodeWeight(c.Output)))
		if len(results) == num {
			break
		}
	}
	return results, nil
}

// lookupPrefix walks the FST along the bytes in scratch and returns the
// accumulated output. The arc is left positioned at the last matching node.
func (l *WFSTCompletionLookup) lookupPrefix(scratch []byte, arc *fstp.Arc[int64]) (int64, bool, error) {
	output := int64(0)
	bytesReader := l.fst.GetBytesReader()
	l.fst.GetFirstArc(arc)
	for _, b := range scratch {
		fa, err := l.fst.FindTargetArc(int(b)&0xff, arc, arc, bytesReader)
		if err != nil {
			return 0, false, err
		}
		if fa == nil {
			return 0, false, nil
		}
		output += fa.Output()
	}
	return output, true, nil
}

// Store writes count and the FST to output.
func (l *WFSTCompletionLookup) Store(output store.DataOutput) (bool, error) {
	if err := store.WriteVLong(output, l.count); err != nil {
		return false, err
	}
	if l.fst == nil {
		return false, nil
	}
	if err := l.fst.Save(output, output); err != nil {
		return false, err
	}
	return true, nil
}

// Load reads count and the FST from input.
func (l *WFSTCompletionLookup) Load(input store.DataInput) (bool, error) {
	cnt, err := store.ReadVLong(input)
	if err != nil {
		return false, err
	}
	l.count = cnt
	if cnt == 0 {
		l.fst = nil
		return false, nil
	}
	meta, err := fstp.ReadMetadata[int64](input, fstp.PositiveIntOutputs())
	if err != nil {
		return false, err
	}
	l.fst, err = fstp.NewFSTFromDataInput[int64](meta, input)
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetCount returns the indexed term count.
func (l *WFSTCompletionLookup) GetCount() int64 { return l.count }

// encodeWeight converts a weight to a cost suitable for PositiveIntOutputs.
func encodeWeight(weight int64) int {
	if weight < 0 || weight > math.MaxInt32 {
		panic(fmt.Sprintf("cannot encode weight: %d", weight))
	}
	return math.MaxInt32 - int(weight)
}

// decodeWeight converts a cost back to the original weight.
func decodeWeight(cost int64) int64 {
	return int64(math.MaxInt32 - int(cost))
}

func int64Comparator(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

var _ suggest.Lookup = (*WFSTCompletionLookup)(nil)
