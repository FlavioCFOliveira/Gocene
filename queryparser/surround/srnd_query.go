package surround

import "github.com/FlavioCFOliveira/Gocene/search"

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

// IsWeighted reports whether SetWeight has been called.
func (b *SrndQueryBase) IsWeighted() bool { return b.weightSet }

// IsFieldsSubQueryAcceptable defaults to true; FieldsQuery overrides this.
func (b *SrndQueryBase) IsFieldsSubQueryAcceptable() bool { return true }
