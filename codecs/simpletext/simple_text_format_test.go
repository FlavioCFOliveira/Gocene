package simpletext

import "testing"

func TestSimpleTextCodec_Name(t *testing.T) {
	c := NewSimpleTextCodec()
	if c == nil {
		t.Fatal("NewSimpleTextCodec returned nil")
	}
	if c.Name() == "" {
		t.Error("SimpleText codec name should not be empty")
	}
}

func TestSimpleTextCompoundFormat_New(t *testing.T) {
	f := NewSimpleTextCompoundFormat()
	if f == nil {
		t.Fatal("NewSimpleTextCompoundFormat returned nil")
	}
}

func TestSimpleTextNormsFormat_New(t *testing.T) {
	f := NewSimpleTextNormsFormat()
	if f == nil {
		t.Fatal("NewSimpleTextNormsFormat returned nil")
	}
}

func TestSimpleTextPointsFormat_New(t *testing.T) {
	f := NewSimpleTextPointsFormat()
	if f == nil {
		t.Fatal("NewSimpleTextPointsFormat returned nil")
	}
}
