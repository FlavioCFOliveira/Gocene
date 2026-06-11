package expressions

import "testing"

func TestConstantDoubleValues_New(t *testing.T) {
	v := NewConstantDoubleValues(3.14)
	if v == nil {
		t.Fatal("NewConstantDoubleValues returned nil")
	}
}

func TestExpressionRescorer_New(t *testing.T) {
	r := NewExpressionRescorer(nil, nil, nil)
	if r == nil {
		t.Fatal("NewExpressionRescorer returned nil")
	}
}
