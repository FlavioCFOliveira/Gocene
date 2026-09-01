package function

import (
	"testing"
)

func TestConstValueSource_Constants(t *testing.T) {
	tests := []struct {
		name      string
		val       float32
		wantInt   int32
		wantLong  int64
		wantFloat float32
		wantDouble float64
		wantBool  bool
	}{
		{"positive", 10.5, 10, 10, 10.5, 10.5, true},
		{"zero", 0.0, 0, 0, 0.0, 0.0, false},
		{"negative", -5.2, -5, -5, -5.2, -5.2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cvs := NewConstValueSource(tt.val)
			if got := cvs.GetInt(); got != tt.wantInt {
				t.Errorf("GetInt() = %v, want %v", got, tt.wantInt)
			}
			if got := cvs.GetLong(); got != tt.wantLong {
				t.Errorf("GetLong() = %v, want %v", got, tt.wantLong)
			}
			if got := cvs.GetFloat(); got != tt.wantFloat {
				t.Errorf("GetFloat() = %v, want %v", got, tt.wantFloat)
			}
			if got := cvs.GetDouble(); got != tt.wantDouble {
				t.Errorf("GetDouble() = %v, want %v", got, tt.wantDouble)
			}
			if got := cvs.GetBool(); got != tt.wantBool {
				t.Errorf("GetBool() = %v, want %v", got, tt.wantBool)
			}
		})
	}
}

func TestConstValueSource_Values(t *testing.T) {
	val := float32(42.7)
	cvs := NewConstValueSource(val)
	fv, err := cvs.GetValues(NewContext(), nil)
	if err != nil {
		t.Fatalf("GetValues failed: %v", err)
	}

	doc := 123
	if v, _ := fv.IntVal(doc); v != int32(val) {
		t.Errorf("IntVal(%d) = %v, want %v", doc, v, int32(val))
	}
	if v, _ := fv.LongVal(doc); v != int64(val) {
		t.Errorf("LongVal(%d) = %v, want %v", doc, v, int64(val))
	}
	if v, _ := fv.FloatVal(doc); v != val {
		t.Errorf("FloatVal(%d) = %v, want %v", doc, v, val)
	}
	if v, _ := fv.DoubleVal(doc); v != float64(val) {
		t.Errorf("DoubleVal(%d) = %v, want %v", doc, v, float64(val))
	}
	if v, _ := fv.BoolVal(doc); v != true {
		t.Errorf("BoolVal(%d) = %v, want true", doc, v)
	}
	if v, _ := fv.Exists(doc); v != true {
		t.Errorf("Exists(%d) = %v, want true", doc, v)
	}
}
