package uniformsplit

import "testing"

func TestBlockHeader_New(t *testing.T) {
	h := NewBlockHeader(10, 0, []byte("first_term"))
	if h == nil {
		t.Fatal("NewBlockHeader returned nil")
	}
}

func TestBlockLine_New(t *testing.T) {
	l := NewBlockLine([]byte("term"), []byte("state"))
	if l == nil {
		t.Fatal("NewBlockLine returned nil")
	}
}

func TestFieldMetadata_New(t *testing.T) {
	m := NewFieldMetadata(100, 50)
	if m == nil {
		t.Fatal("NewFieldMetadata returned nil")
	}
}

func TestBlockWriter_New(t *testing.T) {
	w := NewBlockWriter()
	if w == nil {
		t.Fatal("NewBlockWriter returned nil")
	}
}
