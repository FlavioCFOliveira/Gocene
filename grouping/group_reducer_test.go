package grouping

import "testing"

func TestLongRange_New(t *testing.T) {
	r := NewLongRange("low", 0, 100)
	if r == nil {
		t.Fatal("NewLongRange returned nil")
	}
}

func TestLongRangeFactory_New(t *testing.T) {
	r1 := NewLongRange("a", 0, 10)
	r2 := NewLongRange("b", 11, 20)
	f := NewLongRangeFactory(r1, r2)
	if f == nil {
		t.Fatal("NewLongRangeFactory returned nil")
	}
}

func TestCollectedSearchGroup_New(t *testing.T) {
	g := NewCollectedSearchGroup("value", nil, 0, 0)
	if g == nil {
		t.Fatal("NewCollectedSearchGroup returned nil")
	}
}
