// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"errors"
	"math"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// ErrInvalidRangeBound is returned by [parseRangeBound] when an explicit
// bound cannot be parsed as a float32.
var ErrInvalidRangeBound = errors.New("function: invalid range bound")

// Context is the per-search bag passed from [ValueSource.CreateWeight] to
// [ValueSource.GetValues]. It mirrors the
// java.util.Map<Object,Object> used by Lucene. The keys are arbitrary
// (Lucene uses identity-hash semantics); Go callers should rely on
// type-safe wrappers built on top instead of stuffing untyped values
// directly.
//
// Gocene deviation: Lucene's IdentityHashMap pivots on object identity;
// Go's reference-equality model already lets callers achieve that via
// pointer keys. A map[any]any is therefore a sufficient mirror; callers
// must keep keys consistent (typed comparable values or pointers).
type Context map[any]any

// NewContext returns a new empty context. The caller may seed it via
// [Context.Put] before passing it to [ValueSource.GetValues].
func NewContext() Context { return make(Context) }

// Put stores value under key.
func (c Context) Put(key, value any) { c[key] = value }

// Get retrieves the value stored under key.
func (c Context) Get(key any) (any, bool) { v, ok := c[key]; return v, ok }

// SearcherKey is the conventional key under which the originating
// IndexSearcher (or its lightweight Gocene substitute) is stashed in the
// context. The concrete searcher type is intentionally `any` to avoid an
// import cycle with the search package.
const SearcherKey = "searcher"

// ScorerKey is the conventional key under which the current Scorer (or
// any scorable adapter) is stashed in the context.
const ScorerKey = "scorer"

// ValueSource instantiates [FunctionValues] for a particular reader. It is
// the abstract base for the entire org.apache.lucene.queries.function
// hierarchy. Implementations are expected to be MT-safe and lightweight so
// that they can serve as cache keys for [FunctionQuery] and friends.
//
// Concrete implementations must override GetValues, Description, Equals
// and HashCode. They may optionally override CreateWeight to pre-compute
// per-query state in the supplied [Context].
type ValueSource interface {
	// GetValues returns the per-doc FunctionValues for readerContext.
	// The context is the one previously seeded by [ValueSource.CreateWeight].
	// Returned values must be consumed in forward docID order; a new call
	// is required to re-iterate.
	GetValues(ctx Context, readerContext *index.LeafReaderContext) (FunctionValues, error)
	// Equals reports value-equality (Lucene's Object.equals contract).
	Equals(other ValueSource) bool
	// HashCode returns a stable hash that agrees with Equals.
	HashCode() int32
	// Description renders a short description used by explain() output and
	// satisfies the implicit toString() contract.
	Description() string
	// CreateWeight lets the implementation precompute per-searcher state
	// and stash it in the context. The default base implementation is a
	// no-op; concrete types embedding [BaseValueSource] inherit that.
	CreateWeight(ctx Context, searcher any) error
}

// BaseValueSource is an embeddable struct that supplies the default
// [ValueSource.CreateWeight] (no-op) and serves as a marker type.
//
// Embedding this struct lets concrete value sources avoid stuttering the
// no-op implementation while still allowing them to override CreateWeight
// when needed.
type BaseValueSource struct{}

// CreateWeight is a no-op by default.
func (BaseValueSource) CreateWeight(_ Context, _ any) error { return nil }

// String implements fmt.Stringer for ValueSource implementations that
// embed BaseValueSource and forward through this helper. Concrete types
// that need a custom String() should declare their own method.
func (BaseValueSource) String() string { return "ValueSource" }

// parseRangeBound parses the textual bound used by [FunctionValues.GetRangeScorer].
// An empty bound maps to ±Inf depending on the `isLower` flag, matching
// Lucene's Float.parseFloat semantics on null inputs.
func parseRangeBound(bound string, isLower bool) (float32, error) {
	if bound == "" {
		if isLower {
			return float32(math.Inf(-1)), nil
		}
		return float32(math.Inf(1)), nil
	}
	v, err := strconv.ParseFloat(bound, 32)
	if err != nil {
		return 0, ErrInvalidRangeBound
	}
	return float32(v), nil
}
