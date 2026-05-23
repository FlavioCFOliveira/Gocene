// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// ConvTable holds an ICONV or OCONV replacement table compiled into an FST for
// efficient longest-match replacement.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.ConvTable from Apache Lucene 10.4.0.
type ConvTable struct {
	fstTable        *fst.FST[*util.CharsRef]
	firstCharHashes *firstCharBitSet
	mod             int
}

// firstCharBitSet is a probabilistic first-character bloom filter that avoids
// invoking the FST for characters that can never be the start of a match.
type firstCharBitSet struct {
	bits []uint64
	mod  int
}

func newFirstCharBitSet(mod int) *firstCharBitSet {
	words := (mod + 63) / 64
	return &firstCharBitSet{bits: make([]uint64, words), mod: mod}
}

func (f *firstCharBitSet) set(c rune) {
	idx := int(c) % f.mod
	f.bits[idx>>6] |= 1 << (idx & 63)
}

func (f *firstCharBitSet) get(c rune) bool {
	idx := int(c) % f.mod
	return f.bits[idx>>6]>>uint(idx&63)&1 != 0
}

// NewConvTable builds a ConvTable from a sorted map of pattern → replacement.
func NewConvTable(mappings map[string]string) (*ConvTable, error) {
	// Sort keys for deterministic FST construction.
	keys := make([]string, 0, len(mappings))
	for k := range mappings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// nextPow2 ≥ max(256, 2*len)
	mod := 256
	for mod < len(mappings)*2 {
		mod <<= 1
	}

	hashes := newFirstCharBitSet(mod)

	outputs := fst.CharSequenceOutputs()
	builder := fst.NewFSTCompilerBuilder[*util.CharsRef](fst.InputTypeByte2, outputs).Build()

	scratch := util.NewIntsRefBuilder()

	for _, key := range keys {
		r := []rune(key)
		if len(r) == 0 {
			continue
		}
		hashes.set(r[0])

		// Build IntsRef for UTF-16-style rune sequence.
		scratch.Clear()
		for _, ch := range r {
			scratch.Append(int(ch))
		}
		val := util.NewCharsRefFromString(mappings[key])
		if err := builder.Add(scratch.Get(), val); err != nil {
			return nil, err
		}
	}

	meta, err := builder.Compile()
	if err != nil {
		return nil, err
	}
	compiled, err := fst.FromFSTReader[*util.CharsRef](meta, builder.GetFSTReader())
	if err != nil {
		return nil, err
	}

	return &ConvTable{
		fstTable:        compiled,
		firstCharHashes: hashes,
		mod:             mod,
	}, nil
}

// ApplyMappings applies all conversions in-place to the given string builder.
func (ct *ConvTable) ApplyMappings(sb *strings.Builder) {
	s := sb.String()
	if len(s) == 0 {
		return
	}

	type replacement struct {
		start  int
		end    int // exclusive rune position
		output string
	}

	runes := []rune(s)
	var replacements []replacement

	bytesReader := ct.fstTable.GetBytesReader()
	firstArc := ct.fstTable.GetFirstArc(new(fst.Arc[*util.CharsRef]))

	for i := 0; i < len(runes); i++ {
		if !ct.firstCharHashes.get(runes[i]) {
			continue
		}

		arc := new(fst.Arc[*util.CharsRef])
		arc.CopyFrom(firstArc)
		output := ct.fstTable.Outputs().GetNoOutput()
		longestMatch := -1
		var longestOutput *util.CharsRef

		for j := i; j < len(runes); j++ {
			ch := int(runes[j])
			next, err := ct.fstTable.FindTargetArc(ch, arc, arc, bytesReader)
			if err != nil || next == nil {
				break
			}
			output = ct.fstTable.Outputs().Add(output, arc.Output())
			if arc.IsFinal() {
				longestOutput = ct.fstTable.Outputs().Add(output, arc.NextFinalOutput())
				longestMatch = j
			}
		}

		if longestMatch >= 0 {
			replacements = append(replacements, replacement{
				start:  i,
				end:    longestMatch + 1,
				output: longestOutput.String(),
			})
			i = longestMatch // advance past matched range
		}
	}

	if len(replacements) == 0 {
		return
	}

	// Rebuild string with replacements applied.
	var result strings.Builder
	result.Grow(len(s))
	pos := 0
	for _, rep := range replacements {
		result.WriteString(string(runes[pos:rep.start]))
		result.WriteString(rep.output)
		pos = rep.end
	}
	result.WriteString(string(runes[pos:]))

	sb.Reset()
	sb.WriteString(result.String())
}

// MightReplaceChar reports whether the given rune could be the start of any
// conversion in this table.
func (ct *ConvTable) MightReplaceChar(c rune) bool {
	return ct.firstCharHashes.get(c)
}
