// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

// This file adds the Lucene 10.4.0-compatible Document accessors that were
// missing from the pre-existing Gocene Document. The original Gocene API
// (Get/Size/IsEmpty/GetFieldsByName/...) remains unchanged for back-compat.

// GetField returns the first field with the given name, or nil.
// Mirrors Lucene's Document#getField(String).
//
// Note: this is functionally identical to the pre-existing Get(name); the
// alias is provided to satisfy callers porting Java code that read more
// naturally with the Lucene-canonical name.
func (d *Document) GetField(name string) IndexableField {
	return d.Get(name)
}

// GetString returns the string value of the first field with the given
// name, or empty string if no such field exists. Mirrors Lucene's
// Document#get(String).
func (d *Document) GetString(name string) string {
	if f := d.Get(name); f != nil {
		return f.StringValue()
	}
	return ""
}

// Iterate calls fn for every field in the document, in insertion order.
// Stops as soon as fn returns false. Mirrors Lucene's Iterable<IndexableField>
// idiomatically (Go does not have an Iterable interface).
func (d *Document) Iterate(fn func(IndexableField) bool) {
	for _, f := range d.fields {
		if !fn(f) {
			return
		}
	}
}

// Fields returns the underlying field slice as an immutable view (shallow
// copy). Mirrors Lucene's Document#getFields() which is an immutable List.
func (d *Document) Fields() []IndexableField {
	return d.GetAllFields()
}

// GetFieldsArray returns the slice of fields with the given name, or an
// empty slice when none exist. Mirrors Lucene's Document#getFields(String)
// which returns an array, never null.
func (d *Document) GetFieldsArray(name string) []IndexableField {
	if got := d.GetFieldsByName(name); got != nil {
		return got
	}
	return []IndexableField{}
}

// GetValuesArray returns every string value associated with the given name,
// or an empty slice. Mirrors Lucene's Document#getValues(String) — never nil.
func (d *Document) GetValuesArray(name string) []string {
	if v := d.GetValues(name); v != nil {
		return v
	}
	return []string{}
}

// GetBinaryValuesArray returns every binary value associated with the given
// name, or an empty slice. Mirrors Lucene's Document#getBinaryValues(String) —
// never nil.
func (d *Document) GetBinaryValuesArray(name string) [][]byte {
	if v := d.GetBinaryValues(name); v != nil {
		return v
	}
	return [][]byte{}
}
