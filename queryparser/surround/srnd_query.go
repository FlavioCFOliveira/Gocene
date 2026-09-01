package surround

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// SrndQuery is the base interface satisfied by every surround query node.
// It mirrors the abstract org.apache.lucene.queryparser.surround.query.SrndQuery
// class: nodes can be weighted, marked as field-substitutable, and rewritten
// into a search.Query targeted at a given field.
type SrndQuery interface {
	// MakeLuceneQueryField produces the Lucene Query equivalent of this node
	// applied to the supplied field.
	MakeLuceneQueryField(field string, factory *BasicQueryFactory) (search.Query, error)

	// IsFieldsSubQueryAcceptable reports whether this node is allowed to
	// appear inside a FieldsQuery — Lucene forbids nested FieldsQuery.
	IsFieldsSubQueryAcceptable() bool

	// GetWeight returns the boost factor stamped on this node.
	GetWeight() float32

	// SetWeight stamps a boost factor on this node.
	SetWeight(w float32)

	// String returns the string representation of the query.
	String() string
}

// SrndQueryBase is the embedding helper that supplies the boost handling and
// the default IsFieldsSubQueryAcceptable=true behaviour shared by every node.
type SrndQueryBase struct {
	weight    float32
	weightSet bool
}

// GetWeight returns the configured weight or 1.0 when none has been set.
func (b *SrndQueryBase) GetWeight() float32 {
	if !b.weightSet {
		return 1.0
	}
	return b.weight
}

// SetWeight stamps a boost factor on the node.
func (b *SrndQueryBase) SetWeight(w float32) {
	b.weight = w
	b.weightSet = true
}

// GetWeightString returns the string representation of the weight.
func (b *SrndQueryBase) GetWeightString() string {
	return fmt.Sprintf("%g", b.GetWeight())
}

// GetWeightOperator returns the weight operator.
func (b *SrndQueryBase) GetWeightOperator() string {
	return "^"
}

// WeightToString appends the weight part of a query to the supplied builder.
func (b *SrndQueryBase) WeightToString(sb *strings.Builder) {
	if b.IsWeighted() {
		sb.WriteString(b.GetWeightOperator())
		sb.WriteString(b.GetWeightString())
	}
}

// IsWeighted reports whether SetWeight has been called.
func (b *SrndQueryBase) IsWeighted() bool { return b.weightSet }

// WrapWithBoost wraps the supplied query in a BoostQuery if this node is weighted.
func (b *SrndQueryBase) WrapWithBoost(q search.Query) search.Query {
	if b.IsWeighted() {
		return search.NewBoostQuery(q, b.GetWeight())
	}
	return q
}

// IsFieldsSubQueryAcceptable defaults to true; FieldsQuery overrides this.
func (b *SrndQueryBase) IsFieldsSubQueryAcceptable() bool { return true }
// test
