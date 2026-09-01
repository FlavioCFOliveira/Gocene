package components

// FieldValuesTabOperator is the operator for the FieldValues tab.
type FieldValuesTabOperator interface {
	ComponentOperator

	SetFields(fields []string)
	GetFieldsToLoad() []string
}
