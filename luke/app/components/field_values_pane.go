package components

// FieldValuesPane implements FieldValuesTabOperator.
type FieldValuesPane struct {
	fields []string
}

func NewFieldValuesPane() *FieldValuesPane {
	return &FieldValuesPane{}
}

func (p *FieldValuesPane) SetFields(fields []string) {
	p.fields = fields
}

func (p *FieldValuesPane) GetFieldsToLoad() []string {
	return p.fields
}
