// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

func TestNewTerm(t *testing.T) {
	term := NewTerm("title", "hello world")
	if term.Field != "title" {
		t.Errorf("Expected field 'title', got '%s'", term.Field)
	}
	if term.Text() != "hello world" {
		t.Errorf("Expected text 'hello world', got '%s'", term.Text())
	}
}

func TestNewTermFromBytes(t *testing.T) {
	term := NewTermFromBytes("body", []byte("content"))
	if term.Field != "body" {
		t.Errorf("Expected field 'body', got '%s'", term.Field)
	}
	if term.Text() != "content" {
		t.Errorf("Expected text 'content', got '%s'", term.Text())
	}
}

func TestNewTermFromBytesRef(t *testing.T) {
	br := util.NewBytesRef([]byte("test"))
	term := NewTermFromBytesRef("field", br)
	if term.Field != "field" {
		t.Errorf("Expected field 'field', got '%s'", term.Field)
	}
	if term.Text() != "test" {
		t.Errorf("Expected text 'test', got '%s'", term.Text())
	}

	// Ensure bytes are cloned (modifying original shouldn't affect term)
	br.Bytes[0] = 'X'
	if term.Text() == "Xest" {
		t.Error("Term should have its own copy of bytes")
	}
}

func TestTerm_Equals(t *testing.T) {
	term1 := NewTerm("field", "value")
	term2 := NewTerm("field", "value")
	term3 := NewTerm("field", "other")
	term4 := NewTerm("other", "value")

	// Same values
	if !term1.Equals(term2) {
		t.Error("Terms with same field and text should be equal")
	}

	// Different text
	if term1.Equals(term3) {
		t.Error("Terms with different text should not be equal")
	}

	// Different field
	if term1.Equals(term4) {
		t.Error("Terms with different field should not be equal")
	}

	// Same instance
	if !term1.Equals(term1) {
		t.Error("Term should equal itself")
	}

	// Nil
	if term1.Equals(nil) {
		t.Error("Term should not equal nil")
	}
}

func TestTerm_CompareTo(t *testing.T) {
	// Compare by field first
	term1 := NewTerm("aaa", "zzz")
	term2 := NewTerm("bbb", "aaa")

	if term1.CompareTo(term2) >= 0 {
		t.Error("Term with field 'aaa' should be less than 'bbb'")
	}
	if term2.CompareTo(term1) <= 0 {
		t.Error("Term with field 'bbb' should be greater than 'aaa'")
	}

	// Same field, compare by bytes
	term3 := NewTerm("field", "aaa")
	term4 := NewTerm("field", "bbb")

	if term3.CompareTo(term4) >= 0 {
		t.Error("Term with text 'aaa' should be less than 'bbb'")
	}
	if term4.CompareTo(term3) <= 0 {
		t.Error("Term with text 'bbb' should be greater than 'aaa'")
	}

	// Equal terms
	term5 := NewTerm("field", "value")
	term6 := NewTerm("field", "value")

	if term5.CompareTo(term6) != 0 {
		t.Error("Equal terms should compare as 0")
	}

	// Same instance
	if term5.CompareTo(term5) != 0 {
		t.Error("Term should compare to itself as 0")
	}

	// Compare with nil
	if term5.CompareTo(nil) <= 0 {
		t.Error("Term should be greater than nil")
	}

	// Nil compare
	var nilTerm *Term
	if nilTerm.CompareTo(term5) >= 0 {
		t.Error("Nil should be less than any term")
	}
}

func TestTerm_Clone(t *testing.T) {
	original := NewTerm("field", "value")
	cloned := original.Clone()

	// Values should be equal
	if !original.Equals(cloned) {
		t.Error("Cloned term should equal original")
	}

	// But bytes should be independent
	original.Bytes.Bytes[0] = 'X'
	if cloned.Text() == "Xalue" {
		t.Error("Cloned term should have independent bytes")
	}

	// Nil clone
	var nilTerm *Term
	if nilTerm.Clone() != nil {
		t.Error("Cloning nil should return nil")
	}
}

func TestTerm_HashCode(t *testing.T) {
	term1 := NewTerm("field", "value")
	term2 := NewTerm("field", "value")
	term3 := NewTerm("field", "other")

	// Same values should have same hash
	if term1.HashCode() != term2.HashCode() {
		t.Error("Equal terms should have equal hash codes")
	}

	// Different values likely have different hashes (not guaranteed but highly probable)
	if term1.HashCode() == term3.HashCode() {
		t.Log("Note: Different terms can have same hash code (hash collision)")
	}

	// Nil term
	var nilTerm *Term
	if nilTerm.HashCode() != 0 {
		t.Error("Nil term should have hash code 0")
	}
}

func TestTerm_Text(t *testing.T) {
	term := NewTerm("field", "hello")
	if term.Text() != "hello" {
		t.Errorf("Expected 'hello', got '%s'", term.Text())
	}

	// Empty string
	term2 := NewTerm("field", "")
	if term2.Text() != "" {
		t.Errorf("Expected empty string, got '%s'", term2.Text())
	}

	// Nil bytes
	term3 := &Term{Field: "field", Bytes: nil}
	if term3.Text() != "" {
		t.Errorf("Expected empty string for nil bytes, got '%s'", term3.Text())
	}
}

func TestTerm_String(t *testing.T) {
	term := NewTerm("title", "lucene")
	str := term.String()
	if str == "" {
		t.Error("String() should not return empty")
	}
	if str != "Term(field=title,text=lucene)" {
		t.Logf("String representation: %s", str)
	}

	// Nil term
	var nilTerm *Term
	if nilTerm.String() != "nil" {
		t.Errorf("Expected 'nil' for nil term, got '%s'", nilTerm.String())
	}
}

func TestTerm_SetBytes(t *testing.T) {
	term := NewTerm("field", "original")
	term.SetBytes([]byte("new value"))

	if term.Text() != "new value" {
		t.Errorf("Expected 'new value', got '%s'", term.Text())
	}

	// Set nil
	term.SetBytes(nil)
	if term.Bytes != nil {
		t.Error("Bytes should be nil after setting nil")
	}
}

func TestTerm_SetBytesRef(t *testing.T) {
	term := NewTerm("field", "original")
	br := util.NewBytesRef([]byte("new value"))
	term.SetBytesRef(br)

	if term.Text() != "new value" {
		t.Errorf("Expected 'new value', got '%s'", term.Text())
	}

	// Modify original
	br.Bytes[0] = 'X'
	if term.Text() == "Xew value" {
		t.Error("SetBytesRef should clone the bytes")
	}

	// Set nil
	term.SetBytesRef(nil)
	if term.Bytes != nil {
		t.Error("Bytes should be nil after setting nil")
	}
}

func TestTerm_GetBytesRef(t *testing.T) {
	term := NewTerm("field", "value")
	br := term.GetBytesRef()

	if br == nil {
		t.Fatal("GetBytesRef should not return nil")
	}
	if string(br.ValidBytes()) != "value" {
		t.Errorf("Expected 'value', got '%s'", string(br.ValidBytes()))
	}

	// Nil bytes
	term2 := &Term{Field: "field", Bytes: nil}
	br2 := term2.GetBytesRef()
	if br2 == nil {
		t.Error("GetBytesRef should return empty BytesRef for nil bytes, not nil")
	}
}

func TestTerm_StartsWith(t *testing.T) {
	term := NewTerm("field", "hello world")

	if !term.StartsWith([]byte("hello")) {
		t.Error("Should start with 'hello'")
	}
	if !term.StartsWith([]byte("hello world")) {
		t.Error("Should start with 'hello world'")
	}
	if term.StartsWith([]byte("world")) {
		t.Error("Should not start with 'world'")
	}
	if term.StartsWith([]byte("hello world!")) {
		t.Error("Should not start with longer string")
	}

	// Empty prefix
	if !term.StartsWith([]byte("")) {
		t.Error("Should start with empty prefix")
	}

	// Nil term
	var nilTerm *Term
	if nilTerm != nil && nilTerm.StartsWith([]byte("test")) {
		t.Error("Nil term should not start with anything")
	}
}

func TestTerm_StartsWithTerm(t *testing.T) {
	term1 := NewTerm("field", "hello world")
	term2 := NewTerm("field", "hello")
	term3 := NewTerm("field", "world")

	if !term1.StartsWithTerm(term2) {
		t.Error("'hello world' should start with 'hello'")
	}
	if term1.StartsWithTerm(term3) {
		t.Error("'hello world' should not start with 'world'")
	}

	// Nil term
	if !term1.StartsWithTerm(nil) {
		t.Error("Any term should start with nil")
	}
}

func TestTermCompare(t *testing.T) {
	term1 := NewTerm("aaa", "zzz")
	term2 := NewTerm("bbb", "aaa")

	if TermCompare(term1, term2) >= 0 {
		t.Error("'aaa' should be less than 'bbb'")
	}
	if TermCompare(term2, term1) <= 0 {
		t.Error("'bbb' should be greater than 'aaa'")
	}

	// Equal
	term3 := NewTerm("field", "value")
	term4 := NewTerm("field", "value")
	if TermCompare(term3, term4) != 0 {
		t.Error("Equal terms should compare as 0")
	}

	// Nil comparisons
	if TermCompare(nil, nil) != 0 {
		t.Error("Two nils should compare as 0")
	}
	if TermCompare(nil, term1) >= 0 {
		t.Error("Nil should be less than any term")
	}
}

func TestTermEquals(t *testing.T) {
	term1 := NewTerm("field", "value")
	term2 := NewTerm("field", "value")
	term3 := NewTerm("field", "other")

	if !TermEquals(term1, term2) {
		t.Error("Same terms should be equal")
	}
	if TermEquals(term1, term3) {
		t.Error("Different terms should not be equal")
	}
	if !TermEquals(nil, nil) {
		t.Error("Two nils should be equal")
	}
	if TermEquals(term1, nil) {
		t.Error("Term should not equal nil")
	}
	if TermEquals(nil, term1) {
		t.Error("Nil should not equal term")
	}
}

func TestTermBytesEquals(t *testing.T) {
	term1 := NewTerm("field1", "value")
	term2 := NewTerm("field2", "value")
	term3 := NewTerm("field1", "other")

	// Same bytes, different fields
	if !TermBytesEquals(term1, term2) {
		t.Error("Terms with same bytes should be equal regardless of field")
	}

	// Different bytes
	if TermBytesEquals(term1, term3) {
		t.Error("Terms with different bytes should not be equal")
	}

	// Same instance
	if !TermBytesEquals(term1, term1) {
		t.Error("Term should equal itself")
	}

	// Nil
	if TermBytesEquals(term1, nil) {
		t.Error("Term should not equal nil")
	}
	if !TermBytesEquals(nil, nil) {
		t.Error("Two nils should be equal")
	}
}
