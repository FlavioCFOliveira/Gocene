// Package labels implements org.apache.lucene.sandbox.facet.labels.
package labels

// LabelToOrd maps a label to its ordinal. Mirrors
// org.apache.lucene.sandbox.facet.labels.LabelToOrd.
type LabelToOrd interface {
	GetOrd(label string) int
}

// OrdToLabel is the reverse mapping. Mirrors
// org.apache.lucene.sandbox.facet.labels.OrdToLabel.
type OrdToLabel interface {
	GetLabel(ord int) string
}

// RangeOrdToLabel maps an ordinal to the bracketed string of a range.
type RangeOrdToLabel struct {
	Ranges []string
}

// NewRangeOrdToLabel builds the mapper.
func NewRangeOrdToLabel(ranges []string) *RangeOrdToLabel {
	return &RangeOrdToLabel{Ranges: append([]string(nil), ranges...)}
}

// GetLabel returns the label for ord or "".
func (r *RangeOrdToLabel) GetLabel(ord int) string {
	if ord < 0 || ord >= len(r.Ranges) {
		return ""
	}
	return r.Ranges[ord]
}

var _ OrdToLabel = (*RangeOrdToLabel)(nil)

// TaxonomyOrdLabelBiMap is the in-memory bidirectional map between
// taxonomy ordinals and labels.
type TaxonomyOrdLabelBiMap struct {
	ToLabel map[int]string
	ToOrd   map[string]int
}

// NewTaxonomyOrdLabelBiMap builds an empty bimap.
func NewTaxonomyOrdLabelBiMap() *TaxonomyOrdLabelBiMap {
	return &TaxonomyOrdLabelBiMap{ToLabel: make(map[int]string), ToOrd: make(map[string]int)}
}

// Add registers (ord, label).
func (b *TaxonomyOrdLabelBiMap) Add(ord int, label string) {
	b.ToLabel[ord] = label
	b.ToOrd[label] = ord
}

// GetOrd returns the ordinal for label or -1.
func (b *TaxonomyOrdLabelBiMap) GetOrd(label string) int {
	if ord, ok := b.ToOrd[label]; ok {
		return ord
	}
	return -1
}

// GetLabel returns the label for ord or "".
func (b *TaxonomyOrdLabelBiMap) GetLabel(ord int) string { return b.ToLabel[ord] }

var (
	_ LabelToOrd = (*TaxonomyOrdLabelBiMap)(nil)
	_ OrdToLabel = (*TaxonomyOrdLabelBiMap)(nil)
)
