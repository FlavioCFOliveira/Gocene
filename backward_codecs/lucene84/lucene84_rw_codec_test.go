package lucene84

import "testing"

func TestForDeltaUtil_New(t *testing.T) {
	u := NewForDeltaUtil("1.0")
	if u == nil {
		t.Fatal("NewForDeltaUtil returned nil")
	}
	if u.Name != "ForDeltaUtil" {
		t.Fatalf("Name=%q", u.Name)
	}
}

func TestPForUtil_New(t *testing.T) {
	u := NewPForUtil("1.0")
	if u == nil {
		t.Fatal("NewPForUtil returned nil")
	}
	if u.Name != "PForUtil" {
		t.Fatalf("Name=%q", u.Name)
	}
}

func TestLucene84PostingsFormat_New(t *testing.T) {
	f := &Lucene84PostingsFormat{Name: "Lucene84", Version: "1.0"}
	if f.Name != "Lucene84" {
		t.Fatalf("Name=%q", f.Name)
	}
}
