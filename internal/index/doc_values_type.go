package index

// DocValuesType describes the type of DocValues.
type DocValuesType int

const (
	DocValuesTypeNone DocValuesType = iota
	DocValuesTypeNumeric
	DocValuesTypeBinary
	DocValuesTypeSorted
	DocValuesTypeSortedNumeric
	DocValuesTypeSortedSet
)
