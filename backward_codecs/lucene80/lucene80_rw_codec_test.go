package lucene80

import "testing"

func TestLucene80Codec_New(t *testing.T) {
	c := NewLucene80Codec()
	if c == nil {
		t.Fatal("NewLucene80Codec returned nil")
	}
	if c.Name() == "" {
		t.Error("codec name should not be empty")
	}
}

func TestLucene80NormsFormat_New(t *testing.T) {
	f := NewLucene80NormsFormat()
	if f == nil {
		t.Fatal("NewLucene80NormsFormat returned nil")
	}
}
