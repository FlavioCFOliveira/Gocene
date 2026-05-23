package matchhighlight

// Port of org.apache.lucene.search.matchhighlight.IndexBuilder.
//
// In Java this class builds a ByteBuffersDirectory-backed index and hands a
// DirectoryReader to a consumer block.  The Go port provides an equivalent
// in-memory document builder that stores field values in plain maps, which is
// all that the Go matchhighlight tests need.

import "testing"

// FldID is the auto-generated document id field name.
const FldID = "id"

// FldSortOrder is the sort-key field used for deterministic traversal.
const FldSortOrder = "id_order"

// testDocument is a simple document representation: a map from field name to
// ordered list of string values.
type testDocument struct {
	ID     int
	Fields map[string][]string
}

// IndexBuilder accumulates test documents and provides a fluent API for
// adding fields. Mirrors
// org.apache.lucene.search.matchhighlight.IndexBuilder.
type IndexBuilder struct {
	documents []*testDocument
	seq       int
}

// NewIndexBuilder creates a new IndexBuilder.
func NewIndexBuilder() *IndexBuilder {
	return &IndexBuilder{}
}

// Doc adds a new document with the given field and values.
func (b *IndexBuilder) Doc(field string, values ...string) *IndexBuilder {
	doc := &testDocument{
		ID:     b.seq,
		Fields: make(map[string][]string),
	}
	b.seq++
	if len(values) > 0 {
		doc.Fields[field] = append(doc.Fields[field], values...)
	}
	b.documents = append(b.documents, doc)
	return b
}

// DocWith adds a document whose fields are populated by the provided function.
func (b *IndexBuilder) DocWith(populate func(doc *testDocument)) *IndexBuilder {
	doc := &testDocument{
		ID:     b.seq,
		Fields: make(map[string][]string),
	}
	b.seq++
	populate(doc)
	b.documents = append(b.documents, doc)
	return b
}

// Documents returns a snapshot of all accumulated documents in insertion order.
func (b *IndexBuilder) Documents() []*testDocument {
	out := make([]*testDocument, len(b.documents))
	copy(out, b.documents)
	return out
}

// AddField appends a value to the named field of a testDocument.
func (d *testDocument) AddField(field string, values ...string) {
	d.Fields[field] = append(d.Fields[field], values...)
}

// GetValues returns the stored values for field, or nil if the field is absent.
func (d *testDocument) GetValues(field string) []string {
	return d.Fields[field]
}

// -- tests -------------------------------------------------------------------

func TestIndexBuilder_BasicDoc(t *testing.T) {
	b := NewIndexBuilder()
	b.Doc("title", "hello world")
	b.Doc("title", "foo bar")

	docs := b.Documents()
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}
	if docs[0].ID != 0 || docs[1].ID != 1 {
		t.Errorf("unexpected IDs: %d, %d", docs[0].ID, docs[1].ID)
	}
	if v := docs[0].GetValues("title"); len(v) != 1 || v[0] != "hello world" {
		t.Errorf("doc[0].title: want [%q], got %v", "hello world", v)
	}
}

func TestIndexBuilder_MultiValueField(t *testing.T) {
	b := NewIndexBuilder()
	b.Doc("body", "first value", "second value")

	docs := b.Documents()
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	vals := docs[0].GetValues("body")
	if len(vals) != 2 || vals[0] != "first value" || vals[1] != "second value" {
		t.Errorf("unexpected multi-value field: %v", vals)
	}
}

func TestIndexBuilder_DocWith(t *testing.T) {
	b := NewIndexBuilder()
	b.DocWith(func(doc *testDocument) {
		doc.AddField("f1", "alpha")
		doc.AddField("f2", "beta", "gamma")
	})

	docs := b.Documents()
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	d := docs[0]
	if v := d.GetValues("f1"); len(v) != 1 || v[0] != "alpha" {
		t.Errorf("f1: want [alpha], got %v", v)
	}
	if v := d.GetValues("f2"); len(v) != 2 {
		t.Errorf("f2: want 2 values, got %v", v)
	}
}

func TestIndexBuilder_IDsAreSequential(t *testing.T) {
	b := NewIndexBuilder()
	for i := 0; i < 5; i++ {
		b.Doc("f", "val")
	}
	for i, d := range b.Documents() {
		if d.ID != i {
			t.Errorf("doc[%d].ID = %d, want %d", i, d.ID, i)
		}
	}
}

func TestIndexBuilder_EmptyField(t *testing.T) {
	b := NewIndexBuilder()
	b.Doc("title")
	docs := b.Documents()
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if v := docs[0].GetValues("title"); len(v) != 0 {
		t.Errorf("expected empty title, got %v", v)
	}
}
