// Package document implements org.apache.lucene.misc.document.
package document

// LazyDocument loads field values on demand instead of materialising the whole
// stored document up-front. Mirrors org.apache.lucene.misc.document.LazyDocument.
type LazyDocument struct {
	DocID  int
	loader func(field string) (string, error)
	cache  map[string]string
}

// NewLazyDocument builds the wrapper.
func NewLazyDocument(docID int, loader func(field string) (string, error)) *LazyDocument {
	return &LazyDocument{DocID: docID, loader: loader, cache: make(map[string]string)}
}

// Get returns the value for field, lazily fetching from the supplied loader
// the first time the field is requested.
func (d *LazyDocument) Get(field string) (string, error) {
	if v, ok := d.cache[field]; ok {
		return v, nil
	}
	if d.loader == nil {
		return "", nil
	}
	v, err := d.loader(field)
	if err != nil {
		return "", err
	}
	d.cache[field] = v
	return v, nil
}
