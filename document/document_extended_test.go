// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TestDocument_BinaryField tests binary field storage and retrieval.
// Source: TestDocument.testBinaryField()
// Purpose: Tests storing and retrieving binary field values
func TestDocument_BinaryField(t *testing.T) {
	binaryVal := "this text will be stored as a byte array in the index"
	binaryVal2 := "this text will be also stored as a byte array in the index"

	doc := NewDocument()

	// Create a stored field type
	ft := NewFieldType()
	ft.SetStored(true)
	ft.Freeze()

	stringFld, err := NewField("string", binaryVal, ft)
	if err != nil {
		t.Fatalf("Failed to create string field: %v", err)
	}

	binaryFld, err := NewStoredFieldFromBytes("binary", []byte(binaryVal))
	if err != nil {
		t.Fatalf("Failed to create binary field: %v", err)
	}

	binaryFld2, err := NewStoredFieldFromBytes("binary", []byte(binaryVal2))
	if err != nil {
		t.Fatalf("Failed to create binary field 2: %v", err)
	}

	doc.Add(stringFld)
	doc.Add(binaryFld)

	if doc.Size() != 2 {
		t.Errorf("Expected 2 fields, got %d", doc.Size())
	}

	// Check binary field properties
	if binaryFld.BinaryValue() == nil {
		t.Error("Expected non-nil binary value")
	}
	if !binaryFld.FieldType().Stored {
		t.Error("Expected binary field to be stored")
	}
	if binaryFld.FieldType().IndexOptions != index.IndexOptionsNone {
		t.Errorf("Expected IndexOptionsNone, got %v", binaryFld.FieldType().IndexOptions)
	}

	// Test GetBinaryValue
	binaryTest := doc.GetBinaryValue("binary")
	if binaryTest == nil {
		t.Fatal("Expected non-nil binary value from document")
	}
	if string(binaryTest) != binaryVal {
		t.Errorf("Expected '%s', got '%s'", binaryVal, string(binaryTest))
	}

	// Test Get on string field
	stringTest := doc.Get("string")
	if stringTest == nil {
		t.Fatal("Expected non-nil string field from document")
	}
	if stringTest.StringValue() != binaryVal {
		t.Errorf("Expected '%s', got '%s'", binaryVal, stringTest.StringValue())
	}

	// Add second binary field
	doc.Add(binaryFld2)

	if doc.Size() != 3 {
		t.Errorf("Expected 3 fields, got %d", doc.Size())
	}

	// Test GetBinaryValues
	binaryTests := doc.GetBinaryValues("binary")
	if len(binaryTests) != 2 {
		t.Errorf("Expected 2 binary values, got %d", len(binaryTests))
	}

	binaryTest1 := string(binaryTests[0])
	binaryTest2 := string(binaryTests[1])

	if binaryTest1 == binaryTest2 {
		t.Error("Expected different binary values")
	}

	if binaryTest1 != binaryVal {
		t.Errorf("Expected '%s', got '%s'", binaryVal, binaryTest1)
	}
	if binaryTest2 != binaryVal2 {
		t.Errorf("Expected '%s', got '%s'", binaryVal2, binaryTest2)
	}

	// Test RemoveField (removes first occurrence)
	doc.RemoveField("string")
	if doc.Size() != 2 {
		t.Errorf("Expected 2 fields after removeField, got %d", doc.Size())
	}

	// Test RemoveFields (removes all occurrences)
	doc.RemoveFields("binary")
	if !doc.IsEmpty() {
		t.Errorf("Expected empty document, got %d fields", doc.Size())
	}
}

// TestDocument_RemoveForNewDocument tests field removal for a new document.
// Source: TestDocument.testRemoveForNewDocument()
// Purpose: Tests removing fields from a document that has not been indexed yet
func TestDocument_RemoveForNewDocument(t *testing.T) {
	doc := makeDocumentWithFields()

	if doc.Size() != 10 {
		t.Errorf("Expected 10 fields, got %d", doc.Size())
	}

	// Remove all "keyword" fields (2 of them)
	doc.RemoveFields("keyword")
	if doc.Size() != 8 {
		t.Errorf("Expected 8 fields after removing keyword fields, got %d", doc.Size())
	}

	// Removing non-existing fields is silently ignored
	doc.RemoveFields("doesnotexist")
	if doc.Size() != 8 {
		t.Errorf("Expected 8 fields after removing non-existent field, got %d", doc.Size())
	}

	// Removing a field more than once
	doc.RemoveFields("keyword")
	if doc.Size() != 8 {
		t.Errorf("Expected 8 fields after removing keyword again, got %d", doc.Size())
	}

	// Remove single "text" field (3 times)
	doc.RemoveField("text")
	if doc.Size() != 7 {
		t.Errorf("Expected 7 fields after first text removal, got %d", doc.Size())
	}

	doc.RemoveField("text")
	if doc.Size() != 6 {
		t.Errorf("Expected 6 fields after second text removal, got %d", doc.Size())
	}

	doc.RemoveField("text")
	if doc.Size() != 6 {
		t.Errorf("Expected 6 fields after third text removal (no more text fields), got %d", doc.Size())
	}

	// Removing non-existing field is silently ignored
	doc.RemoveField("doesnotexist")
	if doc.Size() != 6 {
		t.Errorf("Expected 6 fields after removing non-existent field, got %d", doc.Size())
	}

	// Remove "unindexed" fields (2 of them)
	doc.RemoveFields("unindexed")
	if doc.Size() != 4 {
		t.Errorf("Expected 4 fields after removing unindexed fields, got %d", doc.Size())
	}

	// Remove "unstored" fields (2 of them)
	doc.RemoveFields("unstored")
	if doc.Size() != 2 {
		t.Errorf("Expected 2 fields after removing unstored fields, got %d", doc.Size())
	}

	// Removing non-existing fields is silently ignored
	doc.RemoveFields("doesnotexist")
	if doc.Size() != 2 {
		t.Errorf("Expected 2 fields after removing non-existent field, got %d", doc.Size())
	}

	// Remove "indexed_not_tokenized" fields (2 of them)
	doc.RemoveFields("indexed_not_tokenized")
	if !doc.IsEmpty() {
		t.Errorf("Expected empty document, got %d fields", doc.Size())
	}
}

// TestDocument_Clear tests clearing all fields from a document.
// Source: TestDocument.testClearDocument()
// Purpose: Tests the Clear method removes all fields
func TestDocument_Clear(t *testing.T) {
	doc := makeDocumentWithFields()

	if doc.Size() != 10 {
		t.Errorf("Expected 10 fields, got %d", doc.Size())
	}

	doc.Clear()

	if !doc.IsEmpty() {
		t.Errorf("Expected empty document after clear, got %d fields", doc.Size())
	}
}

// TestDocument_GetValuesForNewDocument tests getting values from a new document.
// Source: TestDocument.testGetValuesForNewDocument()
// Purpose: Tests retrieving field values from a document that has not been indexed
func TestDocument_GetValuesForNewDocument(t *testing.T) {
	doc := makeDocumentWithFields()
	doAssert(t, doc, false)
}

// TestDocument_GetValues tests the GetValues method.
// Source: TestDocument.testGetValues()
// Purpose: Tests retrieving multiple values for a field name
func TestDocument_GetValues(t *testing.T) {
	doc := makeDocumentWithFields()

	keywordValues := doc.GetValues("keyword")
	if len(keywordValues) != 2 {
		t.Errorf("Expected 2 keyword values, got %d", len(keywordValues))
	}
	if keywordValues[0] != "test1" || keywordValues[1] != "test2" {
		t.Errorf("Expected ['test1', 'test2'], got %v", keywordValues)
	}

	textValues := doc.GetValues("text")
	if len(textValues) != 2 {
		t.Errorf("Expected 2 text values, got %d", len(textValues))
	}
	if textValues[0] != "test1" || textValues[1] != "test2" {
		t.Errorf("Expected ['test1', 'test2'], got %v", textValues)
	}

	unindexedValues := doc.GetValues("unindexed")
	if len(unindexedValues) != 2 {
		t.Errorf("Expected 2 unindexed values, got %d", len(unindexedValues))
	}
	if unindexedValues[0] != "test1" || unindexedValues[1] != "test2" {
		t.Errorf("Expected ['test1', 'test2'], got %v", unindexedValues)
	}

	// Non-existent field returns empty slice
	nopeValues := doc.GetValues("nope")
	if len(nopeValues) != 0 {
		t.Errorf("Expected 0 values for non-existent field, got %d", len(nopeValues))
	}
}

// TestDocument_NumericFieldAsString tests numeric fields retrieved as strings.
// Source: TestDocument.testNumericFieldAsString()
// Purpose: Tests that numeric fields can be retrieved as string values
func TestDocument_NumericFieldAsString(t *testing.T) {
	doc := NewDocument()

	// Add numeric field
	intField, err := NewStoredFieldFromInt("int", 5)
	if err != nil {
		t.Fatalf("Failed to create int field: %v", err)
	}
	doc.Add(intField)

	// Get as string
	values := doc.GetValues("int")
	if len(values) != 1 {
		t.Errorf("Expected 1 value, got %d", len(values))
	}
	if values[0] != "5" {
		t.Errorf("Expected '5', got '%s'", values[0])
	}

	// Non-existent field returns empty slice
	somethingElse := doc.GetValues("somethingElse")
	if len(somethingElse) != 0 {
		t.Errorf("Expected 0 values for non-existent field, got %d", len(somethingElse))
	}

	// Add another int field
	intField2, err := NewStoredFieldFromInt("int", 4)
	if err != nil {
		t.Fatalf("Failed to create int field 2: %v", err)
	}
	doc.Add(intField2)

	// Get multiple values
	values = doc.GetValues("int")
	if len(values) != 2 {
		t.Errorf("Expected 2 values, got %d", len(values))
	}
	if values[0] != "5" || values[1] != "4" {
		t.Errorf("Expected ['5', '4'], got %v", values)
	}
}

// TestDocument_GetBinaryValue tests getting a single binary value.
// Purpose: Tests retrieving the first binary value for a field name
func TestDocument_GetBinaryValue(t *testing.T) {
	doc := NewDocument()

	// Add binary fields
	binary1, err := NewStoredFieldFromBytes("data", []byte("binary1"))
	if err != nil {
		t.Fatalf("Failed to create binary field: %v", err)
	}
	binary2, err := NewStoredFieldFromBytes("data", []byte("binary2"))
	if err != nil {
		t.Fatalf("Failed to create binary field 2: %v", err)
	}

	doc.Add(binary1)
	doc.Add(binary2)

	// GetBinaryValue should return the first one
	val := doc.GetBinaryValue("data")
	if val == nil {
		t.Fatal("Expected non-nil binary value")
	}
	if !bytes.Equal(val, []byte("binary1")) {
		t.Errorf("Expected 'binary1', got '%s'", string(val))
	}

	// GetBinaryValues should return all
	vals := doc.GetBinaryValues("data")
	if len(vals) != 2 {
		t.Errorf("Expected 2 binary values, got %d", len(vals))
	}

	// Non-existent field returns nil
	nonExistent := doc.GetBinaryValue("nonexistent")
	if nonExistent != nil {
		t.Error("Expected nil for non-existent field")
	}
}

// TestDocument_GetBinaryValueFromStringField tests GetBinaryValue on string fields.
// Purpose: Tests that string fields can also be retrieved as binary values
func TestDocument_GetBinaryValueFromStringField(t *testing.T) {
	doc := NewDocument()

	// Add a string field
	field, err := NewStoredField("text", "hello world")
	if err != nil {
		t.Fatalf("Failed to create field: %v", err)
	}
	doc.Add(field)

	// String fields can be retrieved as binary
	binaryVal := doc.GetBinaryValue("text")
	if binaryVal == nil {
		t.Error("Expected non-nil binary value from string field")
	}
	if string(binaryVal) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", string(binaryVal))
	}
}

// TestDocument_FieldRemovalOrder tests that field removal maintains correct order.
// Purpose: Tests that RemoveField removes the first occurrence and RemoveFields removes all
func TestDocument_FieldRemovalOrder(t *testing.T) {
	doc := NewDocument()

	// Add fields in specific order
	f1, _ := NewStoredField("field", "value1")
	f2, _ := NewStoredField("field", "value2")
	f3, _ := NewStoredField("field", "value3")

	doc.Add(f1)
	doc.Add(f2)
	doc.Add(f3)

	if doc.Size() != 3 {
		t.Fatalf("Expected 3 fields, got %d", doc.Size())
	}

	// RemoveField should remove the first one
	doc.RemoveField("field")

	values := doc.GetValues("field")
	if len(values) != 2 {
		t.Errorf("Expected 2 values after RemoveField, got %d", len(values))
	}
	if values[0] != "value2" || values[1] != "value3" {
		t.Errorf("Expected ['value2', 'value3'], got %v", values)
	}

	// RemoveFields should remove all remaining
	doc.RemoveFields("field")
	if !doc.IsEmpty() {
		t.Errorf("Expected empty document, got %d fields", doc.Size())
	}
}

// TestDocument_MixedFieldTypes tests documents with mixed field types.
// Purpose: Tests documents containing various field types work correctly
func TestDocument_MixedFieldTypes(t *testing.T) {
	doc := NewDocument()

	// Add different field types
	textField, _ := NewTextField("content", "hello world", true)
	stringField, _ := NewStringField("id", "doc-123", true)
	storedField, _ := NewStoredField("metadata", "some data")
	binaryField, _ := NewStoredFieldFromBytes("binary", []byte{0x01, 0x02, 0x03})

	doc.Add(textField)
	doc.Add(stringField)
	doc.Add(storedField)
	doc.Add(binaryField)

	if doc.Size() != 4 {
		t.Errorf("Expected 4 fields, got %d", doc.Size())
	}

	// Verify field types
	text := doc.Get("content")
	if text == nil {
		t.Error("Expected to find text field")
	} else if !text.FieldType().Tokenized {
		t.Error("Expected text field to be tokenized")
	}

	id := doc.Get("id")
	if id == nil {
		t.Error("Expected to find id field")
	} else if id.FieldType().Tokenized {
		t.Error("Expected string field to not be tokenized")
	}

	meta := doc.Get("metadata")
	if meta == nil {
		t.Error("Expected to find metadata field")
	} else if meta.FieldType().Indexed {
		t.Error("Expected stored field to not be indexed")
	}

	bin := doc.GetBinaryValue("binary")
	if bin == nil {
		t.Error("Expected to find binary field")
	}
	if !bytes.Equal(bin, []byte{0x01, 0x02, 0x03}) {
		t.Errorf("Expected binary data [1 2 3], got %v", bin)
	}
}

// TestDocument_EmptyDocumentOperations tests operations on empty documents.
// Purpose: Tests that operations on empty documents behave correctly
func TestDocument_EmptyDocumentOperations(t *testing.T) {
	doc := NewDocument()

	// Operations on empty document should not panic
	if !doc.IsEmpty() {
		t.Error("New document should be empty")
	}

	// Get on empty document
	if doc.Get("anything") != nil {
		t.Error("Get on empty document should return nil")
	}

	// GetValues on empty document
	values := doc.GetValues("anything")
	if len(values) != 0 {
		t.Error("GetValues on empty document should return empty slice")
	}

	// GetBinaryValue on empty document
	if doc.GetBinaryValue("anything") != nil {
		t.Error("GetBinaryValue on empty document should return nil")
	}

	// RemoveField on empty document
	if doc.RemoveField("anything") {
		t.Error("RemoveField on empty document should return false")
	}

	// RemoveFields on empty document
	if doc.RemoveFields("anything") != 0 {
		t.Error("RemoveFields on empty document should return 0")
	}

	// Clear on empty document should not panic
	doc.Clear()
	if !doc.IsEmpty() {
		t.Error("Document should still be empty after clear")
	}
}

// TestDocument_FieldCountConsistency tests that field count methods are consistent.
// Purpose: Tests that Size, GetFieldCount, and len(GetFieldsByName) are consistent
func TestDocument_FieldCountConsistency(t *testing.T) {
	doc := NewDocument()

	// Add multiple fields with same name
	doc.AddField("tag", "a", StringFieldTypeStored)
	doc.AddField("tag", "b", StringFieldTypeStored)
	doc.AddField("tag", "c", StringFieldTypeStored)
	doc.AddField("other", "x", StringFieldTypeStored)

	// Size should be total fields
	if doc.Size() != 4 {
		t.Errorf("Expected Size() = 4, got %d", doc.Size())
	}

	// GetFieldCount should return count for specific name
	if doc.GetFieldCount("tag") != 3 {
		t.Errorf("Expected GetFieldCount('tag') = 3, got %d", doc.GetFieldCount("tag"))
	}

	if doc.GetFieldCount("other") != 1 {
		t.Errorf("Expected GetFieldCount('other') = 1, got %d", doc.GetFieldCount("other"))
	}

	if doc.GetFieldCount("nonexistent") != 0 {
		t.Errorf("Expected GetFieldCount('nonexistent') = 0, got %d", doc.GetFieldCount("nonexistent"))
	}

	// GetFieldsByName should return same count
	if len(doc.GetFieldsByName("tag")) != 3 {
		t.Errorf("Expected len(GetFieldsByName('tag')) = 3, got %d", len(doc.GetFieldsByName("tag")))
	}
}

// TestDocument_GetAllFieldsIndependence tests that GetAllFields returns an independent copy.
// Purpose: Tests that modifying the returned slice doesn't affect the document
func TestDocument_GetAllFieldsIndependence(t *testing.T) {
	doc := NewDocument()
	doc.AddField("field", "value", StringFieldTypeStored)

	fields := doc.GetAllFields()
	if len(fields) != 1 {
		t.Fatalf("Expected 1 field, got %d", len(fields))
	}

	// Modify the returned slice
	fields[0] = nil

	// Document should be unchanged
	if doc.Size() != 1 {
		t.Error("Document should still have 1 field after modifying returned slice")
	}

	if doc.Get("field") == nil {
		t.Error("Document field should still be accessible")
	}
}

// makeDocumentWithFields creates a document with various field types for testing.
// This mirrors the makeDocumentWithFields() helper method in Java's TestDocument.
func makeDocumentWithFields() *Document {
	doc := NewDocument()

	// Create field types
	stored := NewFieldType()
	stored.SetStored(true)
	stored.Freeze()

	indexedNotTokenized := NewFieldType()
	indexedNotTokenized.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
	indexedNotTokenized.SetTokenized(false)
	indexedNotTokenized.Freeze()

	// Add fields (10 total)
	// 2 keyword fields (StringField, stored)
	doc.AddField("keyword", "test1", StringFieldTypeStored)
	doc.AddField("keyword", "test2", StringFieldTypeStored)

	// 2 text fields (TextField, stored)
	doc.AddField("text", "test1", TextFieldTypeStored)
	doc.AddField("text", "test2", TextFieldTypeStored)

	// 2 unindexed fields (stored but not indexed)
	doc.AddField("unindexed", "test1", stored)
	doc.AddField("unindexed", "test2", stored)

	// 2 unstored fields (TextField, not stored)
	doc.AddField("unstored", "test1", TextFieldTypeNotStored)
	doc.AddField("unstored", "test2", TextFieldTypeNotStored)

	// 2 indexed_not_tokenized fields
	doc.AddField("indexed_not_tokenized", "test1", indexedNotTokenized)
	doc.AddField("indexed_not_tokenized", "test2", indexedNotTokenized)

	return doc
}

// doAssert performs assertions on document field values.
// This mirrors the doAssert() helper method in Java's TestDocument.
func doAssert(t *testing.T, doc *Document, fromIndex bool) {
	keywordFieldValues := doc.GetFieldsByName("keyword")
	textFieldValues := doc.GetFieldsByName("text")
	unindexedFieldValues := doc.GetFieldsByName("unindexed")
	unstoredFieldValues := doc.GetFieldsByName("unstored")

	if len(keywordFieldValues) != 2 {
		t.Errorf("Expected 2 keyword fields, got %d", len(keywordFieldValues))
	}
	if len(textFieldValues) != 2 {
		t.Errorf("Expected 2 text fields, got %d", len(textFieldValues))
	}
	if len(unindexedFieldValues) != 2 {
		t.Errorf("Expected 2 unindexed fields, got %d", len(unindexedFieldValues))
	}

	// This test cannot work for documents retrieved from the index
	// since unstored fields will obviously not be returned
	if !fromIndex {
		if len(unstoredFieldValues) != 2 {
			t.Errorf("Expected 2 unstored fields, got %d", len(unstoredFieldValues))
		}
	}

	if keywordFieldValues[0].StringValue() != "test1" {
		t.Errorf("Expected keyword[0] = 'test1', got '%s'", keywordFieldValues[0].StringValue())
	}
	if keywordFieldValues[1].StringValue() != "test2" {
		t.Errorf("Expected keyword[1] = 'test2', got '%s'", keywordFieldValues[1].StringValue())
	}
	if textFieldValues[0].StringValue() != "test1" {
		t.Errorf("Expected text[0] = 'test1', got '%s'", textFieldValues[0].StringValue())
	}
	if textFieldValues[1].StringValue() != "test2" {
		t.Errorf("Expected text[1] = 'test2', got '%s'", textFieldValues[1].StringValue())
	}
	if unindexedFieldValues[0].StringValue() != "test1" {
		t.Errorf("Expected unindexed[0] = 'test1', got '%s'", unindexedFieldValues[0].StringValue())
	}
	if unindexedFieldValues[1].StringValue() != "test2" {
		t.Errorf("Expected unindexed[1] = 'test2', got '%s'", unindexedFieldValues[1].StringValue())
	}

	// This test cannot work for documents retrieved from the index
	if !fromIndex {
		if unstoredFieldValues[0].StringValue() != "test1" {
			t.Errorf("Expected unstored[0] = 'test1', got '%s'", unstoredFieldValues[0].StringValue())
		}
		if unstoredFieldValues[1].StringValue() != "test2" {
			t.Errorf("Expected unstored[1] = 'test2', got '%s'", unstoredFieldValues[1].StringValue())
		}
	}
}
