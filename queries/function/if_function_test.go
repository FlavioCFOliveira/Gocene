package function

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// mockValueSource implements ValueSource for testing.
type mockValueSource struct {
	description string
	vals        mockFunctionValues
}

func (m *mockValueSource) GetValues(ctx Context, readerContext *index.LeafReaderContext) (FunctionValues, error) {
	return &m.vals, nil
}

func (m *mockValueSource) Equals(other ValueSource) bool {
	o, ok := other.(*mockValueSource)
	return ok && m.description == o.description
}

func (m *mockValueSource) HashCode() int32 {
	return int32(len(m.description))
}

func (m *mockValueSource) Description() string {
	return m.description
}

func (m *mockValueSource) CreateWeight(ctx Context, searcher any) error {
	return nil
}

// mockFunctionValues implements FunctionValues for testing.
type mockFunctionValues struct {
	bools   []bool
	floats  []float32
	ints    []int32
	strings []string
}

func (m *mockFunctionValues) ByteVal(doc int) (int8, error) { return 0, ErrUnsupportedValue }
func (m *mockFunctionValues) ShortVal(doc int) (int16, error) { return 0, ErrUnsupportedValue }
func (m *mockFunctionValues) FloatVal(doc int) (float32, error) {
	if doc >= len(m.floats) {
		return 0, nil
	}
	return m.floats[doc], nil
}
func (m *mockFunctionValues) IntVal(doc int) (int32, error) {
	if doc >= len(m.ints) {
		return 0, nil
	}
	return m.ints[doc], nil
}
func (m *mockFunctionValues) LongVal(doc int) (int64, error) { return 0, ErrUnsupportedValue }
func (m *mockFunctionValues) DoubleVal(doc int) (float64, error) { return 0, ErrUnsupportedValue }
func (m *mockFunctionValues) StrVal(doc int) (string, error) {
	if doc >= len(m.strings) {
		return "", nil
	}
	return m.strings[doc], nil
}
func (m *mockFunctionValues) BoolVal(doc int) (bool, error) {
	if doc >= len(m.bools) {
		return false, nil
	}
	return m.bools[doc], nil
}
func (m *mockFunctionValues) FloatVectorVal(_ int) ([]float32, error) { return nil, ErrUnsupportedValue }
func (m *mockFunctionValues) ByteVectorVal(_ int) ([]byte, error) { return nil, ErrUnsupportedValue }
func (m *mockFunctionValues) BytesVal(_ int, _ *[]byte) (bool, error) { return false, ErrUnsupportedValue }
func (m *mockFunctionValues) ObjectVal(_ int) (any, error) { return nil, ErrUnsupportedValue }
func (m *mockFunctionValues) Exists(_ int) (bool, error) { return true, nil }
func (m *mockFunctionValues) OrdVal(_ int) (int, error) { return 0, ErrUnsupportedValue }
func (m *mockFunctionValues) NumOrd() (int, error) { return 0, nil }
func (m *mockFunctionValues) Cost() float32 { return 100 }
func (m *mockFunctionValues) ToString(doc int) (string, error) {
	return "mock", nil
}
func (m *mockFunctionValues) GetValueFiller() ValueFiller { return nil }
func (m *mockFunctionValues) ByteValMulti(_ int, _ []int8) error { return ErrUnsupportedValue }
func (m *mockFunctionValues) ShortValMulti(_ int, _ []int16) error { return ErrUnsupportedValue }
func (m *mockFunctionValues) FloatValMulti(_ int, _ []float32) error { return ErrUnsupportedValue }
func (m *mockFunctionValues) IntValMulti(_ int, _ []int32) error { return ErrUnsupportedValue }
func (m *mockFunctionValues) LongValMulti(_ int, _ []int64) error { return ErrUnsupportedValue }
func (m *mockFunctionValues) DoubleValMulti(_ int, _ []float64) error { return ErrUnsupportedValue }
func (m *mockFunctionValues) StrValMulti(_ int, _ []string) error { return ErrUnsupportedValue }
func (m *mockFunctionValues) Explain(_ int) (string, error) { return "mock", nil }
func (m *mockFunctionValues) GetScorer(_ *index.LeafReaderContext) ValueSourceScorer { return nil }
func (m *mockFunctionValues) GetRangeScorer(_ *index.LeafReaderContext, _, _ string, _, _ bool) (ValueSourceScorer, error) {
	return nil, ErrUnsupportedValue
}

func TestIfFunction(t *testing.T) {
	ifSrc := &mockValueSource{
		description: "if",
		vals: mockFunctionValues{
			bools: []bool{true, false, true},
		},
	}
	trueSrc := &mockValueSource{
		description: "true",
		vals: mockFunctionValues{
			floats: []float32{1.1, 1.2, 1.3},
			ints:    []int32{10, 11, 12},
			strings: []string{"t1", "t2", "t3"},
		},
	}
	falseSrc := &mockValueSource{
		description: "false",
		vals: mockFunctionValues{
			floats: []float32{2.1, 2.2, 2.3},
			ints:    []int32{20, 21, 22},
			strings: []string{"f1", "f2", "f3"},
		},
	}

	ifFunc := NewIfFunction(ifSrc, trueSrc, falseSrc)
	vals, err := ifFunc.GetValues(NewContext(), nil)
	if err != nil {
		t.Fatalf("GetValues failed: %v", err)
	}

	tests := []struct {
		doc      int
		wantFloat float32
		wantInt   int32
		wantStr   string
	}{
		{0, 1.1, 10, "t1"}, // true
		{1, 2.2, 21, "f2"}, // false
		{2, 1.3, 12, "t3"}, // true
	}

	for _, tt := range tests {
		f, err := vals.FloatVal(tt.doc)
		if err != nil || f != tt.wantFloat {
			t.Errorf("doc %d FloatVal = %f, want %f (err: %v)", tt.doc, f, tt.wantFloat, err)
		}
		i, err := vals.IntVal(tt.doc)
		if err != nil || i != tt.wantInt {
			t.Errorf("doc %d IntVal = %d, want %d (err: %v)", tt.doc, i, tt.wantInt, err, err)
		}
		s, err := vals.StrVal(tt.doc)
		if err != nil || s != tt.wantStr {
			t.Errorf("doc %d StrVal = %q, want %q (err: %v)", tt.doc, s, tt.wantStr, err)
		}
	}
}

func TestIfFunction_Description(t *testing.T) {
	ifSrc := &mockValueSource{description: "if"}
	trueSrc := &mockValueSource{description: "true"}
	falseSrc := &mockValueSource{description: "false"}
	ifFunc := NewIfFunction(ifSrc, trueSrc, falseSrc)
	want := "if(if,true,false)"
	if got := ifFunc.Description(); got != want {
		t.Errorf("Description = %q, want %q", got, want)
	}
}

func TestIfFunction_Equals(t *testing.T) {
	ifSrc := &mockValueSource{description: "if"}
	trueSrc := &mockValueSource{description: "true"}
	falseSrc := &mockValueSource{description: "false"}
	f1 := NewIfFunction(ifSrc, trueSrc, falseSrc)
	f2 := NewIfFunction(ifSrc, trueSrc, falseSrc)
	if !f1.Equals(f2) {
		t.Error("f1 should equal f2")
	}
	f3 := NewIfFunction(ifSrc, trueSrc, &mockValueSource{description: "other"})
	if f1.Equals(f3) {
		t.Error("f1 should not equal f3")
	}
}

func TestIfFunction_HashCode(t *testing.T) {
	ifSrc := &mockValueSource{description: "if"}
	trueSrc := &mockValueSource{description: "true"}
	falseSrc := &mockValueSource{description: "false"}
	f1 := NewIfFunction(ifSrc, trueSrc, falseSrc)
	f2 := NewIfFunction(ifSrc, trueSrc, falseSrc)
	if f1.HashCode() != f2.HashCode() {
		t.Errorf("f1 HashCode = %d, f2 HashCode = %d", f1.HashCode(), f2.HashCode())
	}
}
