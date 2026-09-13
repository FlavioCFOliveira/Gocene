// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// emptyTermStateType is the concrete type behind emptyTermState. The unused
// byte gives every instance a distinct address so that the identity comparison
// Lucene performs against EMPTY_TERMSTATE keeps its meaning in Go.
type emptyTermStateType struct{ _ byte }

// CopyFrom does nothing, mirroring the anonymous TermState assigned to
// TermStates.EMPTY_TERMSTATE.
func (*emptyTermStateType) CopyFrom(TermState) error { return nil }

// emptyTermState mirrors the private constant TermStates.EMPTY_TERMSTATE: the
// marker stored in states[ord] to record that the term is known to be absent
// from that leaf.
var emptyTermState TermState = &emptyTermStateType{}

// TermStates maintains an IndexReader TermState view over IndexReader
// instances containing a single term. The TermStates doesn't track if the given
// TermState objects are valid, neither if the TermState instances refer to the
// same terms in the associated readers.
//
// Mirrors org.apache.lucene.index.TermStates from Apache Lucene 10.5.0.
type TermStates struct {
	// topReaderContextIdentity is Lucene's `Object topReaderContextIdentity`.
	// Important: do NOT keep hard references to index readers.
	topReaderContextIdentity any

	// states is indexed by per-leaf ord; nil entries mean the TermState has
	// not been resolved yet, and emptyTermState means the term is absent from
	// that leaf.
	states []TermState

	// term is nil if stats are to be used, mirroring Lucene's `Term term`.
	term *Term

	// docFreq is the accumulated document frequency.
	docFreq int

	// totalTermFreq is the accumulated total term frequency.
	totalTermFreq int64
}

// newTermStates mirrors the private constructor TermStates(Term, IndexReaderContext).
// Lucene asserts the context is non-nil and top level; Leaves() reports the
// non-top-level case as an error.
func newTermStates(term *Term, context IndexReaderContext) (*TermStates, error) {
	leaves, err := context.Leaves()
	if err != nil {
		return nil, err
	}
	return &TermStates{
		topReaderContextIdentity: context.ID(),
		docFreq:                  0,
		totalTermFreq:            0,
		states:                   make([]TermState, len(leaves)),
		term:                     term,
	}, nil
}

// NewTermStatesForContext creates an empty TermStates from an IndexReaderContext,
// mirroring the public constructor TermStates(IndexReaderContext).
func NewTermStatesForContext(context IndexReaderContext) (*TermStates, error) {
	return newTermStates(nil, context)
}

// NewTermStates allocates an empty TermStates sized for leafCount leaves and
// owned by the given cache key.
//
// PORT NOTE: Lucene derives both the leaf count and the identity token from the
// IndexReaderContext handed to its constructor. This form takes them apart so
// that callers holding only a cache key can build a TermStates; it is the
// Gocene spelling that predates the port of TermStates.build, and it is kept so
// those callers continue to work. Callers that hold a context should prefer
// NewTermStatesForContext, which is the exact Lucene constructor.
func NewTermStates(owner *spi.CacheKey, leafCount int) *TermStates {
	return &TermStates{
		topReaderContextIdentity: owner,
		states:                   make([]TermState, leafCount),
	}
}

// pendingTermLookup mirrors the private record
// TermStates.PendingTermLookup(TermsEnum, IOBooleanSupplier).
type pendingTermLookup struct {
	termsEnum TermsEnum
	supplier  util.IOBooleanSupplier
}

// BuildTermStates creates a TermStates from a top-level IndexReaderContext and
// the given Term. This method will lookup the given term in all context's leaf
// readers and register each of the readers containing the term in the returned
// TermStates using the leaf reader's ordinal.
//
// Note: the given context must be a top-level context.
//
// needsStats: if true then all leaf contexts will be visited up-front to
// collect term statistics. Otherwise, the TermState objects will be built only
// when requested.
//
// Mirrors the static TermStates.build(IndexSearcher, Term, boolean).
func BuildTermStates(indexSearcher spi.IndexSearcher, term *Term, needsStats bool) (*TermStates, error) {
	context := indexSearcher.GetTopReaderContext()
	var lazyTerm *Term
	if !needsStats {
		lazyTerm = term
	}
	perReaderTermState, err := newTermStates(lazyTerm, context)
	if err != nil {
		return nil, err
	}
	if needsStats {
		leaves, err := context.Leaves()
		if err != nil {
			return nil, err
		}
		pendingTermLookups := make([]*pendingTermLookup, 0)
		for _, ctx := range leaves {
			terms, err := GetTerms(ctx.LeafReader(), term.Field)
			if err != nil {
				return nil, err
			}
			termsEnum, err := terms.GetIterator()
			if err != nil {
				return nil, err
			}
			if termsEnum == nil {
				continue
			}
			// Schedule the I/O in the terms dictionary in the background.
			termExistsSupplier, err := PrepareSeekExactDelegated(termsEnum, term)
			if err != nil {
				return nil, err
			}
			if termExistsSupplier != nil {
				pendingTermLookups = growPendingTermLookups(pendingTermLookups, ctx.Ord+1)
				pendingTermLookups[ctx.Ord] = &pendingTermLookup{termsEnum: termsEnum, supplier: termExistsSupplier}
			}
		}
		for ord := 0; ord < len(pendingTermLookups); ord++ {
			lookup := pendingTermLookups[ord]
			if lookup == nil {
				continue
			}
			exists, err := lookup.supplier()
			if err != nil {
				return nil, err
			}
			if !exists {
				continue
			}
			termsEnum := lookup.termsEnum
			state, err := TermStateDelegated(termsEnum)
			if err != nil {
				return nil, err
			}
			docFreq, err := termsEnum.DocFreq()
			if err != nil {
				return nil, err
			}
			totalTermFreq, err := termsEnum.TotalTermFreq()
			if err != nil {
				return nil, err
			}
			perReaderTermState.Register(ord, state, docFreq, totalTermFreq)
		}
	}
	return perReaderTermState, nil
}

// growPendingTermLookups mirrors ArrayUtil.grow(T[], int) applied to the
// pendingTermLookups array in TermStates.build.
func growPendingTermLookups(array []*pendingTermLookup, minSize int) []*pendingTermLookup {
	if len(array) < minSize {
		return util.GrowExact(array, util.Oversize(minSize, util.NumBytesObjectRef))
	}
	return array
}

// WasBuiltFor returns whether this TermStates was built for the given
// IndexReaderContext. This is typically used for assertions.
//
// Mirrors TermStates.wasBuiltFor(IndexReaderContext).
func (ts *TermStates) WasBuiltFor(context IndexReaderContext) bool {
	return ts.topReaderContextIdentity == context.ID()
}

// Clear clears the TermStates internal state and removes all registered
// TermStates. Mirrors TermStates.clear().
func (ts *TermStates) Clear() {
	ts.docFreq = 0
	ts.totalTermFreq = 0
	for i := range ts.states {
		ts.states[i] = nil
	}
}

// Register records a TermState for the given leaf ord and accumulates its term
// statistics. The leaf ordinal should be derived from an IndexReaderContext's
// leaf ord.
//
// Mirrors TermStates.register(TermState, int, int, long); Gocene keeps the ord
// first to match the Gocene call sites that predate this port.
func (ts *TermStates) Register(ord int, state TermState, docFreq int, totalTermFreq int64) {
	ts.RegisterState(ord, state)
	ts.AccumulateStatistics(docFreq, totalTermFreq)
}

// RegisterState records a TermState for the given leaf ord without updating
// term statistics. The leaf ordinal should be derived from an
// IndexReaderContext's leaf ord.
//
// Mirrors the expert overload TermStates.register(TermState, int).
func (ts *TermStates) RegisterState(ord int, state TermState) {
	ts.states[ord] = state
}

// AccumulateStatistics accumulates term statistics.
//
// Mirrors TermStates.accumulateStatistics(int, long).
func (ts *TermStates) AccumulateStatistics(docFreq int, totalTermFreq int64) {
	ts.docFreq += docFreq
	ts.totalTermFreq += totalTermFreq
}

// Get returns a supplier for a TermState for the given LeafReaderContext. This
// may return nil if some cheap checks help figure out that this term doesn't
// exist in this leaf. The supplier may then also return a nil TermState if the
// term doesn't exist.
//
// Calling this method typically schedules some I/O in the background, so it is
// recommended to retrieve suppliers across all required terms first before
// calling them, so that the I/O for these terms can be performed in parallel.
//
// Mirrors TermStates.get(LeafReaderContext).
func (ts *TermStates) Get(ctx *LeafReaderContext) (util.IOSupplier[TermState], error) {
	if ts.term == nil {
		if ts.states[ctx.Ord] == nil {
			return nil, nil
		}
		state := ts.states[ctx.Ord]
		return func() (TermState, error) { return state, nil }, nil
	}
	if ts.states[ctx.Ord] == nil {
		terms, err := ctx.LeafReader().Terms(ts.term.Field)
		if err != nil {
			return nil, err
		}
		if terms == nil {
			ts.states[ctx.Ord] = emptyTermState
			return nil, nil
		}
		termsEnum, err := terms.GetIterator()
		if err != nil {
			return nil, err
		}
		if termsEnum == nil {
			ts.states[ctx.Ord] = emptyTermState
			return nil, nil
		}
		termExistsSupplier, err := PrepareSeekExactDelegated(termsEnum, ts.term)
		if err != nil {
			return nil, err
		}
		if termExistsSupplier == nil {
			ts.states[ctx.Ord] = emptyTermState
			return nil, nil
		}
		return func() (TermState, error) {
			if ts.states[ctx.Ord] == nil {
				exists, err := termExistsSupplier()
				if err != nil {
					return nil, err
				}
				if exists {
					state, err := TermStateDelegated(termsEnum)
					if err != nil {
						return nil, err
					}
					ts.states[ctx.Ord] = state
				} else {
					ts.states[ctx.Ord] = emptyTermState
				}
			}
			state := ts.states[ctx.Ord]
			if state == emptyTermState {
				return nil, nil
			}
			return state, nil
		}, nil
	}
	state := ts.states[ctx.Ord]
	if state == emptyTermState {
		return nil, nil
	}
	return func() (TermState, error) { return state, nil }, nil
}

// DocFreq returns the accumulated document frequency of all TermState
// instances passed to Register.
//
// Mirrors TermStates.docFreq(), which throws IllegalStateException when the
// TermStates was built with needsStats=false; Go renders that unchecked
// exception as a panic.
func (ts *TermStates) DocFreq() int {
	if ts.term != nil {
		panic("Cannot call docFreq() when needsStats=false")
	}
	return ts.docFreq
}

// TotalTermFreq returns the accumulated term frequency of all TermState
// instances passed to Register.
//
// Mirrors TermStates.totalTermFreq(), which throws IllegalStateException when
// the TermStates was built with needsStats=false; Go renders that unchecked
// exception as a panic.
func (ts *TermStates) TotalTermFreq() int64 {
	if ts.term != nil {
		panic("Cannot call totalTermFreq() when needsStats=false")
	}
	return ts.totalTermFreq
}

// String mirrors TermStates.toString().
func (ts *TermStates) String() string {
	var sb strings.Builder
	sb.WriteString("TermStates\n")
	for _, termState := range ts.states {
		sb.WriteString("  state=")
		sb.WriteString(termStateString(termState))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// termStateString renders a TermState the way Java's StringBuilder.append(Object)
// does, including the "null" spelling for an absent state.
func termStateString(state TermState) string {
	if state == nil {
		return "null"
	}
	if s, ok := state.(interface{ String() string }); ok {
		return s.String()
	}
	return "TermState"
}
