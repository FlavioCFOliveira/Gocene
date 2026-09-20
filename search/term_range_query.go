package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// TermRangeQuery matches documents within a range of terms.
//
// This query matches the documents looking for terms that fall into the supplied range according
// to BytesRef.CompareTo.
//
// NOTE: TermRangeQuery performs significantly slower than PointRangeQuery
// point-based ranges as it needs to visit all terms that match the range and merges their matches.
//
// This query uses the ConstantScoreBlendedRewrite rewrite method.
type TermRangeQuery struct {
	*AutomatonQuery
	lowerTerm    *util.BytesRef
	upperTerm    *util.BytesRef
	includeLower bool
	includeUpper bool
}

// NewTermRangeQuery constructs a query selecting all terms greater/equal than lowerTerm but less/equal
// than upperTerm.
//
// If an endpoint is nil, it is said to be "open". Either or both endpoints may be open.
// Open endpoints may not be exclusive (you can't select all but the first or last term without
// explicitly specifying the term to exclude.)
func NewTermRangeQuery(
	field string,
	lowerTerm *util.BytesRef,
	upperTerm *util.BytesRef,
	includeLower bool,
	includeUpper bool,
) *TermRangeQuery {
	return NewTermRangeQueryWithRewriteMethod(field, lowerTerm, upperTerm, includeLower, includeUpper, ConstantScoreBlendedRewrite)
}

// NewTermRangeQueryWithRewriteMethod constructs a query selecting all terms greater/equal than lowerTerm
// but less/equal than upperTerm with a specific rewrite method.
func NewTermRangeQueryWithRewriteMethod(
	field string,
	lowerTerm *util.BytesRef,
	upperTerm *util.BytesRef,
	includeLower bool,
	includeUpper bool,
	rewriteMethod RewriteMethod,
) *TermRangeQuery {
	auto, err := TermRangeQueryToAutomaton(lowerTerm, upperTerm, includeLower, includeUpper)
	if err != nil {
		// In Java, toAutomaton is static and called in constructor.
		// If it fails, it's a programmer error or invalid range.
		// We'll return an empty automaton to match the failed state.
		auto = automaton.MakeEmpty()
	}

	term := index.NewTermFromBytesRef(field, lowerTerm)
	return &TermRangeQuery{
		AutomatonQuery: NewAutomatonQuery(term, auto, true, rewriteMethod),
		lowerTerm:      lowerTerm,
		upperTerm:      upperTerm,
		includeLower:   includeLower,
		includeUpper:   includeUpper,
	}
}

// TermRangeQueryToAutomaton creates an automaton matching the requested binary
// interval.
//
// Mirrors the static method TermRangeQuery.toAutomaton(BytesRef, BytesRef,
// boolean, boolean).
func TermRangeQueryToAutomaton(lowerTerm *util.BytesRef, upperTerm *util.BytesRef, includeLower, includeUpper bool) (*automaton.Automaton, error) {
	if lowerTerm == nil {
		includeLower = true
	}

	if upperTerm == nil {
		includeUpper = true
	}

	return automaton.MakeBinaryInterval(lowerTerm, includeLower, upperTerm, includeUpper)
}

// NewStringRange is a factory that creates a new TermRangeQuery using strings for term text.
func NewStringRange(
	field string,
	lowerTerm string,
	upperTerm string,
	includeLower bool,
	includeUpper bool,
) *TermRangeQuery {
	return NewStringRangeWithRewriteMethod(field, lowerTerm, upperTerm, includeLower, includeUpper, ConstantScoreBlendedRewrite)
}

// NewStringRangeWithRewriteMethod is a factory that creates a new TermRangeQuery using strings for term text
// with a specific rewrite method.
func NewStringRangeWithRewriteMethod(
	field string,
	lowerTerm string,
	upperTerm string,
	includeLower bool,
	includeUpper bool,
	rewriteMethod RewriteMethod,
) *TermRangeQuery {
	var lower *util.BytesRef
	if lowerTerm != "" {
		lower = util.NewBytesRef([]byte(lowerTerm))
	}

	var upper *util.BytesRef
	if upperTerm != "" {
		upper = util.NewBytesRef([]byte(upperTerm))
	}

	return NewTermRangeQueryWithRewriteMethod(field, lower, upper, includeLower, includeUpper, rewriteMethod)
}

func (q *TermRangeQuery) GetLowerTerm() *util.BytesRef {
	return q.lowerTerm
}

func (q *TermRangeQuery) GetUpperTerm() *util.BytesRef {
	return q.upperTerm
}

func (q *TermRangeQuery) IncludesLower() bool {
	return q.includeLower
}

func (q *TermRangeQuery) IncludesUpper() bool {
	return q.includeUpper
}

func (q *TermRangeQuery) ToString(field string) string {
	var sb strings.Builder
	if q.GetField() != field {
		sb.WriteString(q.GetField())
		sb.WriteString(":")
	}

	if q.includeLower {
		sb.WriteByte('[')
	} else {
		sb.WriteByte('{')
	}

	if q.lowerTerm != nil {
		s := q.lowerTerm.String()
		if s == "*" {
			sb.WriteString("\\*")
		} else {
			sb.WriteString(s)
		}
	} else {
		sb.WriteString("*")
	}

	sb.WriteString(" TO ")

	if q.upperTerm != nil {
		s := q.upperTerm.String()
		if s == "*" {
			sb.WriteString("\\*")
		} else {
			sb.WriteString(s)
		}
	} else {
		sb.WriteString("*")
	}

	if q.includeUpper {
		sb.WriteByte(']')
	} else {
		sb.WriteByte('}')
	}

	return sb.String()
}

func (q *TermRangeQuery) HashCode() int {
	prime := 31
	result := q.AutomatonQuery.HashCode()
	if q.includeLower {
		result = prime*result + 1231
	} else {
		result = prime*result + 1237
	}
	if q.includeUpper {
		result = prime*result + 1231
	} else {
		result = prime*result + 1237
	}

	if q.lowerTerm == nil {
		result = prime*result + 0
	} else {
		result = prime*result + q.lowerTerm.HashCode()
	}

	if q.upperTerm == nil {
		result = prime*result + 0
	} else {
		result = prime*result + q.upperTerm.HashCode()
	}

	return result
}

func (q *TermRangeQuery) Equals(other spi.Query) bool {
	if q == other {
		return true
	}
	if !q.AutomatonQuery.Equals(other) {
		return false
	}
	otherQuery, ok := other.(*TermRangeQuery)
	if !ok {
		return false
	}
	if q.includeLower != otherQuery.includeLower {
		return false
	}
	if q.includeUpper != otherQuery.includeUpper {
		return false
	}
	if q.lowerTerm == nil {
		if otherQuery.lowerTerm != nil {
			return false
		}
	} else if !util.BytesRefEquals(q.lowerTerm, otherQuery.lowerTerm) {
		return false
	}
	if q.upperTerm == nil {
		if otherQuery.upperTerm != nil {
			return false
		}
	} else if !util.BytesRefEquals(q.upperTerm, otherQuery.upperTerm) {
		return false
	}
	return true
}
