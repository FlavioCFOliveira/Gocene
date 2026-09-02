package index

// DocValuesSkipIndexType describes options for skip indexes on doc values.
type DocValuesSkipIndexType int

const (
	DocValuesSkipIndexTypeNone DocValuesSkipIndexType = iota
	DocValuesSkipIndexTypeRange
)

// IsCompatibleWith returns true if the skip index type is compatible with the doc values type.
func (ds DocValuesSkipIndexType) IsCompatibleWith(dv DocValuesType) bool {
	switch ds {
	case DocValuesSkipIndexTypeNone:
		return true
	case DocValuesSkipIndexTypeRange:
		return dv == DocValuesTypeNumeric ||
			dv == DocValuesTypeSortedNumeric ||
			dv == DocValuesTypeSorted ||
			dv == DocValuesTypeSortedSet
	default:
		return false
	}
}
