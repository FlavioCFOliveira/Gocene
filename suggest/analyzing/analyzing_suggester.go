// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

// Package analyzing implements the analyzing-suggester family from
// org.apache.lucene.search.suggest.analyzing.
package analyzing

// This file ports org.apache.lucene.search.suggest.analyzing.AnalyzingSuggester
// from Apache Lucene 10.4.0.
//
// Deviations from Java:
//   - Store/Load accept store.DataOutput / store.DataInput rather than Java's
//     DataOutput/DataInput (they are the same wire contract).
//   - The type parameter for FSTCompiler is *fst.Pair[int64, *util.BytesRef]
//     instead of Java's PairOutputs<Long,BytesRef>.
//   - TopNSearcher AcceptResult callback is set via struct field rather than
//     override of a virtual method.
//   - Sorting is performed in-memory (Go sort.SliceStable) rather than via
//     Lucene's OfflineSorter, producing the same logical order for small-to-
//     medium corpora; for very large corpora callers may wrap the iterator.

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/suggest"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
	fstp "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// ExactFirst instructs the suggester to always return the exact match
// first regardless of score. Mirrors AnalyzingSuggester.EXACT_FIRST.
const ExactFirst = 1

// PreserveSep instructs the suggester to preserve token separators when
// matching. Mirrors AnalyzingSuggester.PRESERVE_SEP.
const PreserveSep = 2

// sepLabel is the synthetic label used in place of the POS_SEP byte when
// PreserveSep is active. Mirrors AnalyzingSuggester.SEP_LABEL (0x001F).
const sepLabel = 0x001F

// endByte marks the end of the analyzed input and the start of the dedup byte.
// Mirrors AnalyzingSuggester.END_BYTE (0x0).
const endByte = 0x0

// payloadSep separates surface form from payload inside FST output2.
// Mirrors AnalyzingSuggester.PAYLOAD_SEP (0x001F).
const payloadSep = 0x001F

// defaultMaxSurfaceForms matches the Lucene default for
// maxSurfaceFormsPerAnalyzedForm (256).
const defaultMaxSurfaceForms = 256

// defaultMaxGraphExpansions matches the Lucene default (-1, no limit).
const defaultMaxGraphExpansions = -1

// pairOutputsType is the concrete type used for FST outputs in this suggester.
// Java: PairOutputs<Long, BytesRef> → Go: *fstp.Pair[int64, *util.BytesRef]
type pairOutputsType = *fstp.Pair[int64, *util.BytesRef]

// pairOutputs is the singleton Outputs implementation for this suggester.
var pairOutputs = fstp.NewPairOutputs[int64, *util.BytesRef](
	fstp.PositiveIntOutputs(),
	fstp.ByteSequenceOutputs(),
)

// weightComparator is the Comparator<Pair<Long,BytesRef>> used to rank
// paths by their encoded weight (output1). Lower encoded value = higher
// original weight (because weight is encoded as MAX_INT - weight).
// Mirrors AnalyzingSuggester.weightComparator.
func weightComparator(a, b pairOutputsType) int {
	switch {
	case a.Output1 < b.Output1:
		return -1
	case a.Output1 > b.Output1:
		return 1
	default:
		return 0
	}
}

// AnalyzingSuggester is the Go port of
// org.apache.lucene.search.suggest.analyzing.AnalyzingSuggester.
//
// It analyzes surface forms at index time, builds an FST keyed on the
// analyzed form with the original surface form as output, then at query
// time analyzes the key and intersects with the FST to find completions.
//
// The on-disk format produced by Store / consumed by Load is byte-for-byte
// identical to the Lucene 10.4.0 format.
type AnalyzingSuggester struct {
	indexAnalyzer      analysis.Analyzer
	queryAnalyzer      analysis.Analyzer
	exactFirst         bool
	preserveSep        bool
	maxSurfaceForms    int
	maxGraphExpansions int

	// fst is nil until Build is called.
	fst *fstp.FST[pairOutputsType]
	// count is the number of (analyzed-path, surface) pairs added.
	count int64
	// maxAnalyzedPathsForOneInput is serialised in Store.
	maxAnalyzedPathsForOneInput int
	hasPayloads                 bool
	preservePositionIncrements  bool

	// tempFileNamePrefix is used by the offline sorter.
	tempFileNamePrefix string
}

// NewAnalyzingSuggester creates an AnalyzingSuggester with default options
// (EXACT_FIRST | PRESERVE_SEP, maxSurfaceForms=256, maxGraphExpansions=-1,
// preservePositionIncrements=true).
// Mirrors the Java constructor
//
//	AnalyzingSuggester(Directory tempDir, String tempFileNamePrefix, Analyzer analyzer)
func NewAnalyzingSuggester(analyzer analysis.Analyzer, tempFileNamePrefix string) *AnalyzingSuggester {
	return NewAnalyzingSuggesterFull(analyzer, analyzer, ExactFirst|PreserveSep, defaultMaxSurfaceForms, defaultMaxGraphExpansions, true, tempFileNamePrefix)
}

// NewAnalyzingSuggesterFull creates an AnalyzingSuggester with full control
// over every parameter.
func NewAnalyzingSuggesterFull(
	indexAnalyzer, queryAnalyzer analysis.Analyzer,
	options int,
	maxSurfaceFormsPerAnalyzedForm int,
	maxGraphExpansions int,
	preservePositionIncrements bool,
	tempFileNamePrefix string,
) *AnalyzingSuggester {
	if (options & ^(ExactFirst | PreserveSep)) != 0 {
		panic(fmt.Sprintf("options should only contain ExactFirst and PreserveSep; got %d", options))
	}
	if maxSurfaceFormsPerAnalyzedForm <= 0 || maxSurfaceFormsPerAnalyzedForm > 256 {
		panic(fmt.Sprintf("maxSurfaceFormsPerAnalyzedForm must be > 0 and <= 256, got %d", maxSurfaceFormsPerAnalyzedForm))
	}
	if maxGraphExpansions < 1 && maxGraphExpansions != -1 {
		panic(fmt.Sprintf("maxGraphExpansions must be -1 or > 0, got %d", maxGraphExpansions))
	}
	return &AnalyzingSuggester{
		indexAnalyzer:              indexAnalyzer,
		queryAnalyzer:              queryAnalyzer,
		exactFirst:                 (options & ExactFirst) != 0,
		preserveSep:                (options & PreserveSep) != 0,
		maxSurfaceForms:            maxSurfaceFormsPerAnalyzedForm,
		maxGraphExpansions:         maxGraphExpansions,
		preservePositionIncrements: preservePositionIncrements,
		tempFileNamePrefix:         tempFileNamePrefix,
	}
}

// ----------------------- Build -----------------------

// Build indexes all entries from the InputIterator.
// It mirrors org.apache.lucene.search.suggest.analyzing.AnalyzingSuggester.build.
func (s *AnalyzingSuggester) Build(it suggest.InputIterator) error {
	if it.HasContexts() {
		return fmt.Errorf("AnalyzingSuggester does not support contexts")
	}
	s.hasPayloads = it.HasPayloads()

	ts2a := s.getTokenStreamToAutomaton()

	// Collect all encoded byte-sequence tuples in memory.
	var encoded [][]byte
	var newCount int64
	s.maxAnalyzedPathsForOneInput = 0

	for {
		surface, weight, payload, _, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		surfaceRef := &util.BytesRef{Bytes: surface, Offset: 0, Length: len(surface)}
		a, err := s.toAutomaton(surfaceRef, ts2a)
		if err != nil {
			return err
		}
		finiteIt, err := automaton.NewLimitedFiniteStringsIterator(a, s.maxGraphExpansions)
		if err != nil {
			return err
		}
		pathCount := 0
		scratchIntsBuilder := util.NewIntsRefBuilder()
		scratchBytesBuilder := util.NewBytesRefBuilder()
		for {
			intsRef, err2 := finiteIt.Next()
			if err2 != nil {
				return err2
			}
			if intsRef == nil {
				break
			}
			pathCount++
			newCount++
			// Convert IntsRef → BytesRef (each int is a byte label).
			analyzedBytes := fstp.ToBytesRef(intsRef, scratchBytesBuilder)
			analyzedLen := analyzedBytes.Length
			if analyzedLen > math.MaxInt16-2 {
				return fmt.Errorf("analyzed form too long: %d", analyzedLen)
			}
			surfaceLen := len(surface)
			if s.hasPayloads && surfaceLen > math.MaxInt16-2 {
				return fmt.Errorf("surface form too long: %d", surfaceLen)
			}
			if s.hasPayloads {
				for _, b := range surface {
					if b == payloadSep {
						return fmt.Errorf("surface form cannot contain unit separator 0x1F")
					}
				}
			}

			// Encode: [short analyzedLen][analyzed bytes][int cost][surface bytes]
			// (when hasPayloads: [short surfaceLen][surface bytes][payload bytes])
			requiredLen := 2 + analyzedLen + 4 + surfaceLen
			if s.hasPayloads {
				requiredLen += 2 + len(payload)
			}
			buf := make([]byte, requiredLen)
			out := store.NewByteArrayDataOutputAt(buf, 0)
			if err3 := out.WriteShort(int16(analyzedLen)); err3 != nil {
				return err3
			}
			if err3 := out.WriteBytes(analyzedBytes.Bytes[analyzedBytes.Offset : analyzedBytes.Offset+analyzedLen]); err3 != nil {
				return err3
			}
			if err3 := out.WriteInt(int32(encodeWeight(weight))); err3 != nil {
				return err3
			}
			if s.hasPayloads {
				if err3 := out.WriteShort(int16(surfaceLen)); err3 != nil {
					return err3
				}
				if err3 := out.WriteBytes(surface); err3 != nil {
					return err3
				}
				if err3 := out.WriteBytes(payload); err3 != nil {
					return err3
				}
			} else {
				if err3 := out.WriteBytes(surface); err3 != nil {
					return err3
				}
			}
			encoded = append(encoded, buf[:out.GetPosition()])
			_ = scratchIntsBuilder // used for FST add phase below
		}
		if pathCount > s.maxAnalyzedPathsForOneInput {
			s.maxAnalyzedPathsForOneInput = pathCount
		}
	}

	// Sort by (analyzedForm, cost, surface).
	sort.SliceStable(encoded, func(i, j int) bool {
		return analyzingComparator(encoded[i], encoded[j], s.hasPayloads) < 0
	})

	// Build FST from sorted entries.
	fstCompiler := fstp.NewFSTCompilerBuilder[pairOutputsType](
		fstp.InputTypeByte1, pairOutputs).Build()

	var previousAnalyzed []byte
	scratchInts := util.NewIntsRefBuilder()
	seenSurfaces := make(map[string]struct{})
	dedup := 0

	for _, entry := range encoded {
		in := store.NewByteArrayDataInput(entry)
		analyzedLenI, err := in.ReadShort()
		if err != nil {
			return err
		}
		analyzedLen := int(uint16(analyzedLenI))
		analyzed := make([]byte, analyzedLen)
		if err := in.ReadBytes(analyzed); err != nil {
			return err
		}
		costI, err := in.ReadInt()
		if err != nil {
			return err
		}
		cost := int64(uint32(costI))

		var surfaceBytes []byte
		var payloadStartPos int
		if s.hasPayloads {
			sfLenI, err := in.ReadShort()
			if err != nil {
				return err
			}
			sfLen := int(uint16(sfLenI))
			surfaceBytes = make([]byte, sfLen)
			if err := in.ReadBytes(surfaceBytes); err != nil {
				return err
			}
			payloadStartPos = in.GetPosition()
		} else {
			surfaceBytes = entry[in.GetPosition():]
		}

		// Dedup logic (same as Java).
		if previousAnalyzed == nil {
			previousAnalyzed = analyzed
			seenSurfaces[string(surfaceBytes)] = struct{}{}
		} else if bytes.Equal(analyzed, previousAnalyzed) {
			dedup++
			if dedup >= s.maxSurfaceForms {
				continue
			}
			if _, exists := seenSurfaces[string(surfaceBytes)]; exists {
				continue
			}
			seenSurfaces[string(surfaceBytes)] = struct{}{}
		} else {
			dedup = 0
			previousAnalyzed = analyzed
			seenSurfaces = map[string]struct{}{string(surfaceBytes): {}}
		}

		// Append END_BYTE + dedup byte to analyzed form.
		fstInput := make([]byte, analyzedLen+2)
		copy(fstInput, analyzed)
		fstInput[analyzedLen] = endByte
		fstInput[analyzedLen+1] = byte(dedup)

		fstIntsRef := &util.BytesRef{Bytes: fstInput, Offset: 0, Length: len(fstInput)}
		fstp.ToIntsRef(fstIntsRef, scratchInts)

		var outputPair pairOutputsType
		if !s.hasPayloads {
			outputPair = pairOutputs.NewPair(cost, &util.BytesRef{Bytes: surfaceBytes, Offset: 0, Length: len(surfaceBytes)})
		} else {
			// surface + PAYLOAD_SEP + payload
			payloadBytes := entry[payloadStartPos:]
			merged := make([]byte, len(surfaceBytes)+1+len(payloadBytes))
			copy(merged, surfaceBytes)
			merged[len(surfaceBytes)] = payloadSep
			copy(merged[len(surfaceBytes)+1:], payloadBytes)
			outputPair = pairOutputs.NewPair(cost, &util.BytesRef{Bytes: merged, Offset: 0, Length: len(merged)})
		}

		if err := fstCompiler.Add(scratchInts.Get(), outputPair); err != nil {
			return err
		}
	}

	fstMeta, err := fstCompiler.Compile()
	if err != nil {
		return err
	}
	fstReader := fstCompiler.GetFSTReader()
	s.fst, err = fstp.FromFSTReader(fstMeta, fstReader)
	if err != nil {
		return err
	}
	s.count = newCount
	return nil
}

// ----------------------- Store / Load -----------------------

// Store serialises the FST to output. Returns true on success, false if
// the FST was never built. Mirrors AnalyzingSuggester.store(DataOutput).
//
// Wire format (identical to Lucene 10.4.0):
//
//	writeVLong(count)
//	fst.save(output, output)
//	writeVInt(maxAnalyzedPathsForOneInput)
//	writeByte(hasPayloads ? 1 : 0)
func (s *AnalyzingSuggester) Store(output store.DataOutput) (bool, error) {
	if err := store.WriteVLong(output, s.count); err != nil {
		return false, err
	}
	if s.fst == nil {
		return false, nil
	}
	if err := s.fst.Save(output, output); err != nil {
		return false, err
	}
	if err := store.WriteVInt(output, int32(s.maxAnalyzedPathsForOneInput)); err != nil {
		return false, err
	}
	var hasPay byte
	if s.hasPayloads {
		hasPay = 1
	}
	if err := output.WriteByte(hasPay); err != nil {
		return false, err
	}
	return true, nil
}

// Load reads a serialised FST produced by Store (or Lucene's store()).
// Returns true on success. Mirrors AnalyzingSuggester.load(DataInput).
func (s *AnalyzingSuggester) Load(input store.DataInput) (bool, error) {
	cnt, err := store.ReadVLong(input)
	if err != nil {
		return false, err
	}
	s.count = cnt

	meta, err := fstp.ReadMetadata[pairOutputsType](input, pairOutputs)
	if err != nil {
		return false, err
	}
	s.fst, err = fstp.NewFSTFromDataInput(meta, input)
	if err != nil {
		return false, err
	}
	maxPaths, err := store.ReadVInt(input)
	if err != nil {
		return false, err
	}
	s.maxAnalyzedPathsForOneInput = int(maxPaths)
	hasPay, err := input.ReadByte()
	if err != nil {
		return false, err
	}
	s.hasPayloads = hasPay == 1
	return true, nil
}

// ----------------------- Lookup / LookupResults -----------------------

// LookupResults returns up to num completions for key.
// It mirrors AnalyzingSuggester.lookup(CharSequence, Set, boolean, int).
func (s *AnalyzingSuggester) LookupResults(key string, _ [][]byte, onlyMorePopular bool, num int) ([]*suggest.LookupResult, error) {
	if onlyMorePopular {
		return nil, fmt.Errorf("AnalyzingSuggester only works with onlyMorePopular=false")
	}
	if s.fst == nil {
		return nil, nil
	}
	for _, ch := range key {
		if ch == 0x1E {
			return nil, fmt.Errorf("lookup key cannot contain HOLE character U+001E")
		}
		if ch == 0x1F {
			return nil, fmt.Errorf("lookup key cannot contain unit separator U+001F")
		}
	}
	utf8Key := []byte(key)

	lookupAutomaton, err := s.toLookupAutomaton(key)
	if err != nil {
		return nil, err
	}

	bytesReader := s.fst.GetBytesReader()
	var scratchArc fstp.Arc[pairOutputsType]
	var results []*suggest.LookupResult

	prefixPaths, err := IntersectPrefixPaths(lookupAutomaton, s.fst)
	if err != nil {
		return nil, err
	}

	if s.exactFirst {
		count := 0
		for _, path := range prefixPaths {
			fa, err2 := s.fst.FindTargetArc(endByte, path.FSTNode, &scratchArc, bytesReader)
			if err2 != nil {
				return nil, err2
			}
			if fa != nil {
				count++
			}
		}
		if count > 0 {
			searcher := fstp.NewTopNSearcher(s.fst, count*s.maxSurfaceForms, count*s.maxSurfaceForms, weightComparator)
			for _, path := range prefixPaths {
				fa, err2 := s.fst.FindTargetArc(endByte, path.FSTNode, &scratchArc, bytesReader)
				if err2 != nil {
					return nil, err2
				}
				if fa != nil {
					arcCopy := new(fstp.Arc[pairOutputsType])
					arcCopy.CopyFrom(fa)
					if err2 := searcher.AddStartPaths(arcCopy, s.fst.Outputs().Add(path.Output, fa.Output()), false, path.Input); err2 != nil {
						return nil, err2
					}
				}
			}
			completions, err2 := searcher.Search()
			if err2 != nil {
				return nil, err2
			}
			for _, c := range completions.TopN {
				output2 := c.Output.Output2
				if sameSurfaceForm(utf8Key, output2, s.hasPayloads) {
					results = append(results, getLookupResult(c.Output.Output1, output2, s.hasPayloads))
					break
				}
			}
			if len(results) == num {
				return results, nil
			}
		}
	}

	remaining := num - len(results)
	seen := make(map[string]struct{})
	searcher := fstp.NewTopNSearcher(s.fst, remaining, remaining*s.maxAnalyzedPathsForOneInput, weightComparator)
	searcher.AcceptResult = func(input *util.IntsRef, output pairOutputsType) bool {
		key2 := string(output.Output2.Bytes[output.Output2.Offset : output.Output2.Offset+output.Output2.Length])
		if _, exists := seen[key2]; exists {
			return false
		}
		seen[key2] = struct{}{}
		if !s.exactFirst {
			return true
		}
		return !sameSurfaceForm(utf8Key, output.Output2, s.hasPayloads)
	}

	for _, path := range prefixPaths {
		if err2 := searcher.AddStartPaths(path.FSTNode, path.Output, true, path.Input); err2 != nil {
			return nil, err2
		}
	}
	completions, err := searcher.Search()
	if err != nil {
		return nil, err
	}
	for _, c := range completions.TopN {
		results = append(results, getLookupResult(c.Output.Output1, c.Output.Output2, s.hasPayloads))
		if len(results) == num {
			break
		}
	}
	return results, nil
}

// GetCount returns the number of indexed entries.
func (s *AnalyzingSuggester) GetCount() int64 { return s.count }

// ----------------------- Private helpers -----------------------

func getLookupResult(output1 int64, output2 *util.BytesRef, hasPayloads bool) *suggest.LookupResult {
	weight := decodeWeight(output1)
	if hasPayloads {
		// Find PAYLOAD_SEP.
		sepIdx := -1
		for i := 0; i < output2.Length; i++ {
			if output2.Bytes[output2.Offset+i] == payloadSep {
				sepIdx = i
				break
			}
		}
		if sepIdx < 0 {
			sepIdx = output2.Length
		}
		surface := string(output2.Bytes[output2.Offset : output2.Offset+sepIdx])
		payloadLen := output2.Length - sepIdx - 1
		var payload []byte
		if payloadLen > 0 {
			payload = make([]byte, payloadLen)
			copy(payload, output2.Bytes[output2.Offset+sepIdx+1:output2.Offset+output2.Length])
		}
		return &suggest.LookupResult{Key: surface, Value: int64(weight), Payload: payload}
	}
	surface := string(output2.Bytes[output2.Offset : output2.Offset+output2.Length])
	return &suggest.LookupResult{Key: surface, Value: int64(weight)}
}

func sameSurfaceForm(key []byte, output2 *util.BytesRef, hasPayloads bool) bool {
	if hasPayloads {
		if len(key) >= output2.Length {
			return false
		}
		for i := 0; i < len(key); i++ {
			if key[i] != output2.Bytes[output2.Offset+i] {
				return false
			}
		}
		return output2.Bytes[output2.Offset+len(key)] == payloadSep
	}
	if len(key) != output2.Length {
		return false
	}
	for i, b := range key {
		if b != output2.Bytes[output2.Offset+i] {
			return false
		}
	}
	return true
}

// encodeWeight maps weight → cost (MAX_INT - weight).
// Mirrors AnalyzingSuggester.encodeWeight.
func encodeWeight(value int64) int {
	if value < 0 || value > math.MaxInt32 {
		panic(fmt.Sprintf("cannot encode value: %d", value))
	}
	return math.MaxInt32 - int(value)
}

// decodeWeight maps cost → weight.
// Mirrors AnalyzingSuggester.decodeWeight.
func decodeWeight(encoded int64) int {
	return math.MaxInt32 - int(encoded)
}

// analyzingComparator is the Go equivalent of Java's AnalyzingComparator.
// It compares two byte-encoded entries by (analyzedForm, cost, surface).
func analyzingComparator(a, b []byte, hasPayloads bool) int {
	inA := store.NewByteArrayDataInput(a)
	inB := store.NewByteArrayDataInput(b)

	// Read analyzed lengths (little-endian shorts in Gocene's ByteArrayDataOutput).
	aLenI, _ := inA.ReadShort()
	bLenI, _ := inB.ReadShort()
	aLen := int(uint16(aLenI))
	bLen := int(uint16(bLenI))

	// Compare analyzed forms byte-by-byte.
	aBytes := a[inA.GetPosition() : inA.GetPosition()+aLen]
	bBytes := b[inB.GetPosition() : inB.GetPosition()+bLen]
	c := bytes.Compare(aBytes, bBytes)
	if c != 0 {
		return c
	}
	_ = inA.SetPosition(inA.GetPosition() + aLen)
	_ = inB.SetPosition(inB.GetPosition() + bLen)

	// Compare costs (int32, little-endian).
	aCostI, _ := inA.ReadInt()
	bCostI, _ := inB.ReadInt()
	aCost := int64(uint32(aCostI))
	bCost := int64(uint32(bCostI))
	if aCost < bCost {
		return -1
	} else if aCost > bCost {
		return 1
	}

	// Compare surface forms.
	var aSurf, bSurf []byte
	if hasPayloads {
		aSfLenI, _ := inA.ReadShort()
		bSfLenI, _ := inB.ReadShort()
		aSurf = a[inA.GetPosition() : inA.GetPosition()+int(uint16(aSfLenI))]
		bSurf = b[inB.GetPosition() : inB.GetPosition()+int(uint16(bSfLenI))]
	} else {
		aSurf = a[inA.GetPosition():]
		bSurf = b[inB.GetPosition():]
	}
	return bytes.Compare(aSurf, bSurf)
}

// toAutomaton converts a surface form to an automaton via the index analyzer.
// Mirrors AnalyzingSuggester.toAutomaton.
func (s *AnalyzingSuggester) toAutomaton(surfaceForm *util.BytesRef, ts2a *analysis.TokenStreamToAutomaton) (*automaton.Automaton, error) {
	ts, err := s.indexAnalyzer.TokenStream("", strings.NewReader(string(surfaceForm.Bytes[surfaceForm.Offset:surfaceForm.Offset+surfaceForm.Length])))
	if err != nil {
		return nil, err
	}
	a, err := ts2a.ToAutomaton(ts)
	if err != nil {
		return nil, err
	}
	if err := ts.Close(); err != nil {
		return nil, err
	}
	a = s.replaceSep(a)
	return a, nil
}

// toLookupAutomaton converts a query key to a deterministic automaton via
// the query analyzer. Mirrors AnalyzingSuggester.toLookupAutomaton.
func (s *AnalyzingSuggester) toLookupAutomaton(key string) (*automaton.Automaton, error) {
	ts, err := s.queryAnalyzer.TokenStream("", strings.NewReader(key))
	if err != nil {
		return nil, err
	}
	ts2a := s.getTokenStreamToAutomaton()
	a, err := ts2a.ToAutomaton(ts)
	if err != nil {
		return nil, err
	}
	if err := ts.Close(); err != nil {
		return nil, err
	}
	a = s.replaceSep(a)
	a, err = automaton.Determinize(a, automaton.DefaultDeterminizeWorkLimit)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// getTokenStreamToAutomaton builds the TokenStreamToAutomaton configured
// according to this suggester's settings.
func (s *AnalyzingSuggester) getTokenStreamToAutomaton() *analysis.TokenStreamToAutomaton {
	ts2a := analysis.NewTokenStreamToAutomaton()
	ts2a.SetPreservePositionIncrements(s.preservePositionIncrements)
	ts2a.SetFinalOffsetGapAsHole(true)
	return ts2a
}

// replaceSep replaces POS_SEP transitions with SEP_LABEL (when
// preserveSep is set) or with epsilon, and removes HOLE transitions.
// Mirrors AnalyzingSuggester.replaceSep.
func (s *AnalyzingSuggester) replaceSep(a *automaton.Automaton) *automaton.Automaton {
	numStates := a.NumStates()
	b := automaton.NewBuilderWithCapacity(numStates, a.NumTransitions())
	b.CopyStates(a)

	topoStates, err := automaton.TopoSortStates(a)
	if err != nil {
		// Cyclic input — return the original automaton unchanged.
		return a
	}

	var t automaton.Transition
	for i := 0; i < len(topoStates); i++ {
		state := topoStates[len(topoStates)-1-i]
		count := a.InitTransition(state, &t)
		for j := 0; j < count; j++ {
			a.GetNextTransition(&t)
			switch t.Min {
			case analysis.PosSep:
				if s.preserveSep {
					b.AddTransitionSingle(state, t.Dest, sepLabel)
				} else {
					b.AddEpsilon(state, t.Dest)
				}
			case analysis.Hole:
				b.AddEpsilon(state, t.Dest)
			default:
				b.AddTransition(state, t.Dest, t.Min, t.Max)
			}
		}
	}
	result := b.Finish()
	// Preserve accept states.
	for st := 0; st < numStates; st++ {
		if a.IsAccept(st) {
			result.SetAccept(st, true)
		}
	}
	return result
}

// ----------------------- Interface compliance -----------------------

var _ suggest.Lookup = (*AnalyzingSuggester)(nil)
