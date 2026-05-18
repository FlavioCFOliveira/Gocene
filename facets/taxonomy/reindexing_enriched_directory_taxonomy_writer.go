package taxonomy

// ReindexingEnrichedDirectoryTaxonomyWriter is a DirectoryTaxonomyWriter
// variant that captures additional metadata while reindexing a taxonomy.
// Mirrors org.apache.lucene.facet.taxonomy.directory.ReindexingEnrichedDirectoryTaxonomyWriter.
//
// The Go port surfaces the API contract: it wraps an existing
// DirectoryTaxonomyWriter equivalent (delegate) and stores per-ordinal
// metadata supplied by the caller. The merge / commit operations stay
// transparent so the wrapper composes with whatever writer the project
// already exposes.
type ReindexingEnrichedDirectoryTaxonomyWriter struct {
	delegate Writer
	metadata map[int][]byte
}

// Writer is the minimum interface the reindexing wrapper needs from the
// underlying taxonomy writer. Implementations live in the existing
// directory_taxonomy_writer.go file.
type Writer interface {
	AddCategory(dim string, path []string) (int, error)
	Commit() error
	Close() error
}

// NewReindexingEnrichedDirectoryTaxonomyWriter builds a wrapper that captures
// extra metadata.
func NewReindexingEnrichedDirectoryTaxonomyWriter(delegate Writer) *ReindexingEnrichedDirectoryTaxonomyWriter {
	return &ReindexingEnrichedDirectoryTaxonomyWriter{
		delegate: delegate,
		metadata: make(map[int][]byte),
	}
}

// AddCategory delegates and records empty metadata for the ordinal.
func (w *ReindexingEnrichedDirectoryTaxonomyWriter) AddCategory(dim string, path []string) (int, error) {
	ord, err := w.delegate.AddCategory(dim, path)
	if err != nil {
		return ord, err
	}
	if _, ok := w.metadata[ord]; !ok {
		w.metadata[ord] = nil
	}
	return ord, nil
}

// PutMetadata stores opaque metadata bytes for ord.
func (w *ReindexingEnrichedDirectoryTaxonomyWriter) PutMetadata(ord int, payload []byte) {
	clone := make([]byte, len(payload))
	copy(clone, payload)
	w.metadata[ord] = clone
}

// GetMetadata returns the stored metadata for ord (nil when absent).
func (w *ReindexingEnrichedDirectoryTaxonomyWriter) GetMetadata(ord int) []byte {
	return w.metadata[ord]
}

// Commit forwards to the delegate.
func (w *ReindexingEnrichedDirectoryTaxonomyWriter) Commit() error { return w.delegate.Commit() }

// Close forwards to the delegate.
func (w *ReindexingEnrichedDirectoryTaxonomyWriter) Close() error { return w.delegate.Close() }
