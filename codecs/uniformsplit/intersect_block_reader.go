// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"errors"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// BlockIteration is the block iteration order: whether to move to the next
// block, jump to a block away, or end the iteration.
//
// Mirrors the nested enum
// org.apache.lucene.codecs.uniformsplit.IntersectBlockReader.BlockIteration
// (IntersectBlockReader.java:53). Go has no nested types, so it is declared
// beside the type that owns it; Java's `protected` has no Go equivalent, so it
// is exported, as subclasses of IntersectBlockReader live in other packages.
type BlockIteration int

const (
	// BlockIterationNext renders BlockIteration.NEXT.
	BlockIterationNext BlockIteration = iota
	// BlockIterationSeek renders BlockIteration.SEEK.
	BlockIterationSeek
	// BlockIterationEnd renders BlockIteration.END.
	BlockIterationEnd
)

// errIntersectBlockReaderUnsupported renders the
// UnsupportedOperationException thrown by the seek methods of
// IntersectBlockReader (IntersectBlockReader.java:346-364).
var errIntersectBlockReaderUnsupported = errors.New("IntersectBlockReader: unsupported operation")

// errIntersectBlockReaderUnsupportedBlockIteration renders the
// UnsupportedOperationException of the default branch of the nextBlock switch
// (IntersectBlockReader.java:336).
var errIntersectBlockReaderUnsupportedBlockIteration = errors.New("IntersectBlockReader: unsupported BlockIteration")

// IntersectBlockReader is the "intersect" TermsEnum response to
// UniformSplitTerms.Intersect, intersecting the terms with an automaton.
//
// By design of the UniformSplit block keys, it is less efficient than
// org.apache.lucene.backward_codecs.lucene40.blocktree.IntersectTermsEnum for
// search.FuzzyQuery (-37%). It is slightly slower for search.WildcardQuery
// (-5%) and slightly faster for search.PrefixQuery (+5%).
//
// Mirrors org.apache.lucene.codecs.uniformsplit.IntersectBlockReader from
// Apache Lucene 10.5.0, which extends BlockReader; Go has no inheritance, so
// the superclass is embedded.
type IntersectBlockReader struct {
	*BlockReader

	// numConsecutivelyRejectedTermsThreshold is the threshold that controls
	// when to attempt to jump to a block away.
	//
	// This counter is 0 when entering a block. It is incremented each time a
	// term is rejected by the automaton. When the counter is greater than or
	// equal to this threshold, then we compute the next term accepted by the
	// automaton, with AutomatonNextTermCalculator, and we jump to a block away
	// if the next term accepted is greater than the immediate next term in the
	// block.
	//
	// A low value, for example 1, improves the performance of automatons
	// requiring many jumps, for example search.FuzzyQuery and most
	// search.WildcardQuery. A higher value improves the performance of
	// automatons with less or no jump, for example search.PrefixQuery. A
	// threshold of 4 seems to be a good balance.
	//
	// Mirrors the protected final instance field
	// NUM_CONSECUTIVELY_REJECTED_TERMS_THRESHOLD
	// (IntersectBlockReader.java:73).
	numConsecutivelyRejectedTermsThreshold int

	transitionAccessor automaton.TransitionAccessor
	byteRunnable       automaton.ByteRunnable
	finite             bool
	// commonSuffix may be nil.
	commonSuffix         *util.BytesRef
	minTermLength        int
	nextStringCalculator *AutomatonNextTermCalculator

	// seekTerm is set when the current mode is seeking to this term; it is set
	// to nil after.
	seekTerm *util.BytesRef

	// numMatchedBytes is the number of bytes accepted by the automaton when
	// validating the current term.
	numMatchedBytes int

	// states holds the automaton states reached when validating the current
	// term, from 0 to numMatchedBytes - 1.
	states []int

	// blockIteration is the block iteration order determined when scanning the
	// terms in the current block.
	blockIteration BlockIteration

	// numConsecutivelyRejectedTerms counts the consecutively rejected terms.
	// Depending on numConsecutivelyRejectedTermsThreshold, this may trigger a
	// jump to a block away.
	numConsecutivelyRejectedTerms int
}

// NewIntersectBlockReader mirrors the protected IntersectBlockReader
// constructor (IntersectBlockReader.java:103).
func NewIntersectBlockReader(
	compiled *automaton.CompiledAutomaton,
	startTerm *util.BytesRef,
	dictionaryBrowserSupplier IndexDictionaryBrowserSupplier,
	blockInput store.IndexInput,
	postingsReader codecs.PostingsReaderBase,
	fieldMetadata *FieldMetadata,
	blockDecoder BlockDecoder,
) (*IntersectBlockReader, error) {
	blockReader, err := NewBlockReader(dictionaryBrowserSupplier, blockInput, postingsReader, fieldMetadata, blockDecoder)
	if err != nil {
		return nil, err
	}
	r := &IntersectBlockReader{
		BlockReader: blockReader,
		// The Java instance initialiser of
		// NUM_CONSECUTIVELY_REJECTED_TERMS_THRESHOLD runs before the
		// constructor body.
		numConsecutivelyRejectedTermsThreshold: 4,
	}
	r.byteRunnable = compiled.GetByteRunnable()
	r.transitionAccessor = compiled.GetTransitionAccessor()
	r.finite = compiled.Finite
	r.commonSuffix = compiled.CommonSuffixRef
	r.minTermLength = r.getMinTermLength()
	r.nextStringCalculator = newAutomatonNextTermCalculator(r, compiled)
	r.seekTerm = startTerm
	return r, nil
}

// getMinTermLength computes the minimal length of the terms accepted by the
// automaton. This speeds up the term scanning for automatons accepting a finite
// language.
//
// Mirrors IntersectBlockReader.getMinTermLength
// (IntersectBlockReader.java:126).
func (r *IntersectBlockReader) getMinTermLength() int {
	// Automatons accepting infinite language (e.g. PrefixQuery and
	// WildcardQuery) do not benefit much from min term length while it takes
	// time to compute it. More precisely, by skipping this computation
	// PrefixQuery is significantly boosted while WildcardQuery might be
	// slightly degraded on average. This min term length mainly boosts
	// FuzzyQuery.
	commonSuffixLength := 0
	if r.commonSuffix != nil {
		commonSuffixLength = r.commonSuffix.Length
	}
	if !r.finite {
		return commonSuffixLength
	}
	// Since we are only dealing with finite language, there is no loop to
	// detect.
	commonPrefixLength := 0
	state := 0
	var t *automaton.Transition
	for {
		if r.byteRunnable.IsAccept(state) {
			// The common prefix reaches a final state. So common prefix and
			// common suffix overlap. Min term length is the max between common
			// prefix and common suffix lengths.
			return max(commonPrefixLength, commonSuffixLength)
		}
		if r.transitionAccessor.GetNumTransitions(state) == 1 {
			if t == nil {
				t = automaton.NewTransition()
			}
			r.transitionAccessor.GetTransition(state, 0, t)
			if t.Min == t.Max {
				state = t.Dest
				commonPrefixLength++
				continue
			}
		}
		break
	}
	// Min term length is the sum of common prefix and common suffix lengths.
	return commonPrefixLength + commonSuffixLength
}

// Next mirrors IntersectBlockReader.next (IntersectBlockReader.java:165),
// adapted to the Gocene SPI, which carries the field name alongside the term
// bytes in *spi.Term where Java returns a bare BytesRef.
func (r *IntersectBlockReader) Next() (*spi.Term, error) {
	if r.blockHeader == nil {
		found, err := r.seekFirstBlock()
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, nil
		}
		r.states = make([]int, 32)
		r.blockIteration = BlockIterationNext
	}
	r.termState = nil
	for {
		term, err := r.nextTermInBlockMatching()
		if err != nil {
			return nil, err
		}
		if term != nil {
			return spi.NewTermFromBytesRef(r.fieldMetadata.GetFieldInfo().Name(), term), nil
		}
		more, err := r.nextBlock()
		if err != nil {
			return nil, err
		}
		if !more {
			return nil, nil
		}
	}
}

// seekFirstBlock mirrors IntersectBlockReader.seekFirstBlock
// (IntersectBlockReader.java:184).
func (r *IntersectBlockReader) seekFirstBlock() (bool, error) {
	r.seekTerm = r.nextStringCalculator.nextSeekTerm(r.seekTerm)
	if r.seekTerm == nil {
		return false, nil
	}
	browser, err := r.getOrCreateDictionaryBrowser()
	if err != nil {
		return false, err
	}
	blockStartFP, err := browser.SeekBlock(r.seekTerm)
	if err != nil {
		return false, err
	}
	if blockStartFP == -1 {
		blockStartFP = r.fieldMetadata.GetFirstBlockStartFP()
	} else if r.isBeyondLastTerm(r.seekTerm, blockStartFP) {
		return false, nil
	}
	if err := r.initializeHeader(r.seekTerm, blockStartFP); err != nil {
		return false, err
	}
	return r.blockHeader != nil, nil
}

// nextTermInBlockMatching finds the next block line that matches (accepted by
// the automaton), or nil when at end of block.
//
// Returns the next term in the current block that is accepted by the automaton;
// or nil if none.
//
// Mirrors IntersectBlockReader.nextTermInBlockMatching
// (IntersectBlockReader.java:205).
func (r *IntersectBlockReader) nextTermInBlockMatching() (*util.BytesRef, error) {
	if r.seekTerm == nil {
		line, err := r.readLineInBlock()
		if err != nil {
			return nil, err
		}
		if line == nil {
			return nil, nil
		}
	} else {
		seekStatus, err := r.seekInBlock(r.seekTerm)
		if err != nil {
			return nil, err
		}
		r.seekTerm = nil
		if seekStatus == spi.SeekStatusEnd {
			return nil, nil
		}
		// assert numMatchedBytes == 0;
		// assert numConsecutivelyRejectedTerms == 0;
	}
	for {
		lineTermBytes := r.blockLine.GetTermBytes()
		lineTerm := lineTermBytes.GetTerm()
		// assert lineTerm.offset == 0;
		if len(r.states) <= lineTerm.Length {
			r.states = util.GrowExact(r.states, util.Oversize(lineTerm.Length+1, 1))
		}
		// Since terms are delta encoded, we may start the automaton steps from
		// the last state reached by the previous term.
		idx := min(lineTermBytes.GetSuffixOffset(), r.numMatchedBytes)
		// Skip this term early if it is shorter than the min term length, or if
		// it does not end with the common suffix accepted by the automaton.
		if lineTerm.Length >= r.minTermLength &&
			(r.commonSuffix == nil || r.endsWithCommonSuffix(lineTerm.Bytes, lineTerm.Length)) {
			state := r.states[idx]
			for {
				if idx == lineTerm.Length {
					if r.byteRunnable.IsAccept(state) {
						// The automaton accepts the current term. Record the
						// number of matched bytes and return the term.
						// assert byteRunnable.run(lineTerm.bytes, 0, lineTerm.length);
						r.numMatchedBytes = idx
						if r.numConsecutivelyRejectedTerms > 0 {
							r.numConsecutivelyRejectedTerms = 0
						}
						// assert blockIteration == BlockIteration.NEXT;
						return lineTerm, nil
					}
					break
				}
				state = r.byteRunnable.Step(state, int(lineTerm.Bytes[idx]))
				if state == -1 {
					// The automaton rejects the current term.
					break
				}
				// Record the reached automaton state.
				idx++
				r.states[idx] = state
			}
		}
		// The current term is not accepted by the automaton.
		// Still record the reached automaton state to start the next term steps
		// from there.
		// assert !byteRunnable.run(lineTerm.bytes, 0, lineTerm.length);
		r.numMatchedBytes = idx
		// If the number of consecutively rejected terms reaches the threshold,
		// then determine whether it is worthwhile to jump to a block away.
		r.numConsecutivelyRejectedTerms++
		if r.numConsecutivelyRejectedTerms >= r.numConsecutivelyRejectedTermsThreshold &&
			r.lineIndexInBlock < r.blockHeader.LinesCount()-1 &&
			!r.nextStringCalculator.isLinearState(lineTerm) {
			// Compute the next term accepted by the automaton after the current
			// term.
			r.seekTerm = r.nextStringCalculator.nextSeekTerm(lineTerm)
			if r.seekTerm == nil {
				r.blockIteration = BlockIterationEnd
				return nil, nil
			}
			// It is worthwhile to jump to a block away if the next term
			// accepted is after the next term in the block. Actually the block
			// away may be the current block, but this is a good heuristic.
			if _, err := r.readLineInBlock(); err != nil {
				return nil, err
			}
			if util.BytesRefCompare(r.seekTerm, r.blockLine.GetTermBytes().GetTerm()) > 0 {
				// Stop scanning this block terms and set the iteration order to
				// jump to a block away by seeking seekTerm.
				r.blockIteration = BlockIterationSeek
				return nil, nil
			}
			r.seekTerm = nil
			// If it is not worthwhile to jump to a block away, do not attempt
			// anymore for the current block.
			r.numConsecutivelyRejectedTerms = math.MinInt32
		} else {
			line, err := r.readLineInBlock()
			if err != nil {
				return nil, err
			}
			if line == nil {
				// No more terms in the block. The iteration order is to open
				// the very next block.
				// assert blockIteration == BlockIteration.NEXT;
				return nil, nil
			}
		}
	}
}

// endsWithCommonSuffix indicates whether the given term ends with the automaton
// common suffix. This allows to quickly skip terms that the automaton would
// reject eventually.
//
// Mirrors IntersectBlockReader.endsWithCommonSuffix
// (IntersectBlockReader.java:299).
func (r *IntersectBlockReader) endsWithCommonSuffix(termBytes []byte, termLength int) bool {
	suffixBytes := r.commonSuffix.Bytes
	suffixLength := r.commonSuffix.Length
	offset := termLength - suffixLength
	// assert offset >= 0; // We already checked minTermLength.
	for i := 0; i < suffixLength; i++ {
		if termBytes[offset+i] != suffixBytes[i] {
			return false
		}
	}
	return true
}

// nextBlock opens the next block. Depending on the blockIteration order, it may
// be the very next block, or a block away that may contain seekTerm.
//
// Returns true if the next block is opened; false if there is no block anymore
// and the iteration is over.
//
// Mirrors IntersectBlockReader.nextBlock (IntersectBlockReader.java:319).
func (r *IntersectBlockReader) nextBlock() (bool, error) {
	var blockStartFP int64
	switch r.blockIteration {
	case BlockIterationNext:
		// assert seekTerm == null;
		blockStartFP = r.blockInput.GetFilePointer()
	case BlockIterationSeek:
		// assert seekTerm != null;
		browser, err := r.getOrCreateDictionaryBrowser()
		if err != nil {
			return false, err
		}
		blockStartFP, err = browser.SeekBlock(r.seekTerm)
		if err != nil {
			return false, err
		}
		if r.isBeyondLastTerm(r.seekTerm, blockStartFP) {
			return false, nil
		}
		r.blockIteration = BlockIterationNext
	case BlockIterationEnd:
		return false, nil
	default:
		return false, errIntersectBlockReaderUnsupportedBlockIteration
	}
	r.numMatchedBytes = 0
	r.numConsecutivelyRejectedTerms = 0
	if err := r.initializeHeader(r.seekTerm, blockStartFP); err != nil {
		return false, err
	}
	return r.blockHeader != nil, nil
}

// SeekExact mirrors IntersectBlockReader.seekExact(BytesRef)
// (IntersectBlockReader.java:346), whose body is
// `throw new UnsupportedOperationException()`.
func (r *IntersectBlockReader) SeekExact(text *spi.Term) (bool, error) {
	return false, errIntersectBlockReaderUnsupported
}

// SeekExactOrd mirrors IntersectBlockReader.seekExact(long)
// (IntersectBlockReader.java:352), whose body is
// `throw new UnsupportedOperationException()`.
func (r *IntersectBlockReader) SeekExactOrd(ord int64) error {
	return errIntersectBlockReaderUnsupported
}

// SeekExactWithState mirrors IntersectBlockReader.seekExact(BytesRef,
// TermState) (IntersectBlockReader.java:357), whose body is
// `throw new UnsupportedOperationException()`.
func (r *IntersectBlockReader) SeekExactWithState(term *spi.Term, state index.TermState) error {
	return errIntersectBlockReaderUnsupported
}

// SeekCeil mirrors IntersectBlockReader.seekCeil(BytesRef)
// (IntersectBlockReader.java:362), whose body is
// `throw new UnsupportedOperationException()`.
func (r *IntersectBlockReader) SeekCeil(text *spi.Term) (*spi.Term, error) {
	return nil, errIntersectBlockReaderUnsupported
}

var _ spi.TermsEnum = (*IntersectBlockReader)(nil)

// AutomatonNextTermCalculator is mostly a copy of AutomatonTermsEnum. Since
// it's an inner class, the outer class can call methods that ATE does not
// expose. It'd be nice if ATE's logic could be more extensible.
//
// Mirrors the nested class
// org.apache.lucene.codecs.uniformsplit.IntersectBlockReader.AutomatonNextTermCalculator
// (IntersectBlockReader.java:370). Go has no inner classes, so the implicit
// IntersectBlockReader.this becomes the explicit parent back-pointer.
type AutomatonNextTermCalculator struct {
	// parent renders the implicit IntersectBlockReader.this of the Java inner
	// class, through which byteRunnable, transitionAccessor and finite are
	// reached.
	parent *IntersectBlockReader

	// visited is the path tracking: each int16 records the generation when we
	// last visited the state; we use generations to avoid having to clear.
	visited []int16
	curGen  int16
	// seekBytesRef is the reference used for seeking forwards through the term
	// dictionary.
	seekBytesRef *util.BytesRefBuilder
	// linear is true if we are enumerating an infinite portion of the DFA. In
	// this case it is faster to drive the query based on the terms dictionary.
	// When this is true, linearUpperBound indicates the end of range of terms
	// where we should simply do sequential reads instead.
	linear           bool
	linearUpperBound *util.BytesRef
	transition       *automaton.Transition
	savedStates      *util.IntsRefBuilder
}

// newAutomatonNextTermCalculator mirrors the protected
// AutomatonNextTermCalculator(CompiledAutomaton) constructor
// (IntersectBlockReader.java:386).
func newAutomatonNextTermCalculator(parent *IntersectBlockReader, compiled *automaton.CompiledAutomaton) *AutomatonNextTermCalculator {
	c := &AutomatonNextTermCalculator{
		parent:           parent,
		seekBytesRef:     util.NewBytesRefBuilder(),
		linearUpperBound: util.NewBytesRefEmpty(),
		transition:       automaton.NewTransition(),
		savedStates:      util.NewIntsRefBuilder(),
	}
	if !compiled.Finite {
		c.visited = make([]int16, parent.byteRunnable.GetSize())
	}
	return c
}

// setVisited records the given state has been visited. Mirrors the private
// AutomatonNextTermCalculator.setVisited (IntersectBlockReader.java:391).
//
// Java's ArrayUtil.grow(short[], int) has no Gocene counterpart, so its body —
// growExact(array, oversize(minSize, Short.BYTES)) when the array is too small
// — is inlined here.
func (c *AutomatonNextTermCalculator) setVisited(state int) {
	if !c.parent.finite {
		if state >= len(c.visited) {
			c.visited = util.GrowExactInt16(c.visited, util.Oversize(state+1, 2))
		}
		c.visited[state] = c.curGen
	}
}

// isVisited indicates whether the given state has been visited. Mirrors the
// private AutomatonNextTermCalculator.isVisited
// (IntersectBlockReader.java:401).
func (c *AutomatonNextTermCalculator) isVisited(state int) bool {
	return !c.parent.finite && state < len(c.visited) && c.visited[state] == c.curGen
}

// isLinearState reports whether the current state of the automaton is best
// iterated linearly (without seeking). Mirrors
// AutomatonNextTermCalculator.isLinearState (IntersectBlockReader.java:406).
func (c *AutomatonNextTermCalculator) isLinearState(term *util.BytesRef) bool {
	return c.linear && util.BytesRefCompare(term, c.linearUpperBound) < 0
}

// nextSeekTerm mirrors AutomatonNextTermCalculator.nextSeekTerm(BytesRef)
// (IntersectBlockReader.java:413); see
// org.apache.lucene.index.FilteredTermsEnum#nextSeekTerm(BytesRef).
func (c *AutomatonNextTermCalculator) nextSeekTerm(term *util.BytesRef) *util.BytesRef {
	if term == nil {
		// assert seekBytesRef.length() == 0;
		// return the empty term, as it's valid
		if c.parent.byteRunnable.IsAccept(0) {
			return c.seekBytesRef.Get()
		}
	} else {
		c.seekBytesRef.CopyBytesRef(term)
	}

	// seek to the next possible string;
	if c.nextString() {
		return c.seekBytesRef.Get() // reposition
	}
	return nil // no more possible strings can match
}

// setLinear sets the enum to operate in linear fashion, as we have found a
// looping transition at position: we set an upper bound and act like a
// TermRangeQuery for this portion of the term space.
//
// Mirrors AutomatonNextTermCalculator.setLinear
// (IntersectBlockReader.java:438).
func (c *AutomatonNextTermCalculator) setLinear(position int) {
	// assert linear == false;

	state := 0
	maxInterval := 0xff
	for i := 0; i < position; i++ {
		state = c.parent.byteRunnable.Step(state, int(c.seekBytesRef.ByteAt(i)))
		// assert state >= 0 : "state=" + state;
	}
	numTransitions := c.parent.transitionAccessor.GetNumTransitions(state)
	c.parent.transitionAccessor.InitTransition(state, c.transition)
	for i := 0; i < numTransitions; i++ {
		c.parent.transitionAccessor.GetNextTransition(c.transition)
		if c.transition.Min <= int(c.seekBytesRef.ByteAt(position)) &&
			int(c.seekBytesRef.ByteAt(position)) <= c.transition.Max {
			maxInterval = c.transition.Max
			break
		}
	}
	// 0xff terms don't get the optimization... not worth the trouble.
	if maxInterval != 0xff {
		maxInterval++
	}
	length := position + 1 /* position + maxTransition */
	if len(c.linearUpperBound.Bytes) < length {
		c.linearUpperBound.Bytes = make([]byte, util.Oversize(length, 1))
	}
	copy(c.linearUpperBound.Bytes[:position], c.seekBytesRef.Bytes()[:position])
	c.linearUpperBound.Bytes[position] = byte(maxInterval)
	c.linearUpperBound.Length = length

	c.linear = true
}

// nextString increments the byte buffer to the next String in binary order
// after s that will not put the machine into a reject state. If such a string
// does not exist, returns false.
//
// The correctness of this method depends upon the automaton being
// deterministic, and having no transitions to dead states.
//
// Returns true if more possible solutions exist for the DFA.
//
// Mirrors the no-argument AutomatonNextTermCalculator.nextString()
// (IntersectBlockReader.java:480).
func (c *AutomatonNextTermCalculator) nextString() bool {
	var state int
	pos := 0
	c.savedStates.Grow(c.seekBytesRef.Length() + 1)
	c.savedStates.SetIntAt(0, 0)

	for {
		if !c.parent.finite {
			c.curGen++
			if c.curGen == 0 {
				// Clear the visited states every time curGen wraps (so very
				// infrequently to not impact average perf).
				for i := range c.visited {
					c.visited[i] = -1
				}
			}
		}
		c.linear = false
		// walk the automaton until a character is rejected.
		for state = c.savedStates.IntAt(pos); pos < c.seekBytesRef.Length(); pos++ {
			c.setVisited(state)
			nextState := c.parent.byteRunnable.Step(state, int(c.seekBytesRef.ByteAt(pos)))
			if nextState == -1 {
				break
			}
			c.savedStates.SetIntAt(pos+1, nextState)
			// we found a loop, record it for faster enumeration
			if !c.linear && c.isVisited(nextState) {
				c.setLinear(pos)
			}
			state = nextState
		}

		// take the useful portion, and the last non-reject state, and attempt
		// to append characters that will match.
		if c.nextStringFrom(state, pos) {
			return true
		}
		/* no more solutions exist from this useful portion, backtrack */
		pos = c.backtrack(pos)
		if pos < 0 {
			/* no more solutions at all */
			return false
		}
		newState := c.parent.byteRunnable.Step(c.savedStates.IntAt(pos), int(c.seekBytesRef.ByteAt(pos)))
		if newState >= 0 && c.parent.byteRunnable.IsAccept(newState) {
			/* String is good to go as-is */
			return true
		}
		/* else advance further */
		// paranoia? if we backtrack thru an infinite DFA, the loop detection is
		// important! for now, restart from scratch for all infinite DFAs
		if !c.parent.finite {
			pos = 0
		}
	}
}

// nextStringFrom returns the next String in lexicographic order that will not
// put the machine into a reject state.
//
// This method traverses the DFA from the given position in the String, starting
// at the given state.
//
// If this cannot satisfy the machine, returns false. This method will walk the
// minimal path, in lexicographic order, as long as possible.
//
// If this method returns false, then there might still be more solutions, it is
// necessary to backtrack to find out.
//
// state is the current non-reject state; position is the useful portion of the
// string. Returns true if more possible solutions exist for the DFA from this
// position.
//
// Mirrors the two-argument AutomatonNextTermCalculator.nextString(int, int)
// (IntersectBlockReader.java:543). Go has no overloading, so the two nextString
// members are distinguished by name.
func (c *AutomatonNextTermCalculator) nextStringFrom(state, position int) bool {
	/*
	 * the next lexicographic character must be greater than the existing
	 * character, if it exists.
	 */
	ch := 0
	if position < c.seekBytesRef.Length() {
		ch = int(c.seekBytesRef.ByteAt(position))
		// if the next byte is 0xff and is not part of the useful portion,
		// then by definition it puts us in a reject state, and therefore this
		// path is dead. there cannot be any higher transitions. backtrack.
		if ch == 0xff {
			return false
		}
		ch++
	}

	c.seekBytesRef.SetLength(position)
	c.setVisited(state)

	numTransitions := c.parent.transitionAccessor.GetNumTransitions(state)
	c.parent.transitionAccessor.InitTransition(state, c.transition)
	// find the minimal path (lexicographic order) that is >= c

	for i := 0; i < numTransitions; i++ {
		c.parent.transitionAccessor.GetNextTransition(c.transition)
		if c.transition.Max >= ch {
			nextChar := max(ch, c.transition.Min)
			// append either the next sequential char, or the minimum transition
			c.seekBytesRef.AppendByte(byte(nextChar))
			state = c.transition.Dest
			/*
			 * as long as is possible, continue down the minimal path in
			 * lexicographic order. if a loop or accept state is encountered, stop.
			 */
			for !c.isVisited(state) && !c.parent.byteRunnable.IsAccept(state) {
				c.setVisited(state)
				/*
				 * Note: we work with a DFA with no transitions to dead states.
				 * so the below is ok, if it is not an accept state,
				 * then there MUST be at least one transition.
				 */
				c.parent.transitionAccessor.InitTransition(state, c.transition)
				c.parent.transitionAccessor.GetNextTransition(c.transition)
				state = c.transition.Dest

				// append the minimum transition
				c.seekBytesRef.AppendByte(byte(c.transition.Min))

				// we found a loop, record it for faster enumeration
				if !c.linear && c.isVisited(state) {
					c.setLinear(c.seekBytesRef.Length() - 1)
				}
			}
			return true
		}
	}
	return false
}

// backtrack attempts to backtrack thru the string after encountering a dead end
// at some given position. Returns -1 if no more possible strings can match.
//
// position is the current position in the input String; a returned
// `position >= 0` means more possible solutions exist for the DFA.
//
// Mirrors AutomatonNextTermCalculator.backtrack (IntersectBlockReader.java:607).
func (c *AutomatonNextTermCalculator) backtrack(position int) int {
	for position > 0 {
		position--
		nextChar := int(c.seekBytesRef.ByteAt(position))
		// if a character is 0xff it's a dead-end too,
		// because there is no higher character in binary sort order.
		if nextChar != 0xff {
			nextChar++
			c.seekBytesRef.SetByteAt(position, byte(nextChar))
			c.seekBytesRef.SetLength(position + 1)
			return position
		}
	}
	return -1 /* all solutions exhausted */
}
