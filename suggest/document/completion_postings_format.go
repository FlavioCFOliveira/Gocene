// Package document implements
// org.apache.lucene.search.suggest.document: the index-side suggester family.
package document

// CompletionPostingsFormat is the contract every per-version completion
// postings format implements. Mirrors
// org.apache.lucene.search.suggest.document.CompletionPostingsFormat.
type CompletionPostingsFormat interface {
	Name() string
	Version() string
}

// baseCompletionPostingsFormat captures the name + version shared by every
// concrete subtype.
type baseCompletionPostingsFormat struct {
	name    string
	version string
}

func (b baseCompletionPostingsFormat) Name() string    { return b.name }
func (b baseCompletionPostingsFormat) Version() string { return b.version }

// Completion50PostingsFormat is the Lucene 5.0 completion postings format.
type Completion50PostingsFormat struct{ baseCompletionPostingsFormat }

// NewCompletion50PostingsFormat builds the format.
func NewCompletion50PostingsFormat() *Completion50PostingsFormat {
	return &Completion50PostingsFormat{baseCompletionPostingsFormat{name: "Completion50", version: "5.0"}}
}

// Completion84PostingsFormat is the Lucene 8.4 variant.
type Completion84PostingsFormat struct{ baseCompletionPostingsFormat }

// NewCompletion84PostingsFormat builds the format.
func NewCompletion84PostingsFormat() *Completion84PostingsFormat {
	return &Completion84PostingsFormat{baseCompletionPostingsFormat{name: "Completion84", version: "8.4"}}
}

// Completion90PostingsFormat is the Lucene 9.0 variant.
type Completion90PostingsFormat struct{ baseCompletionPostingsFormat }

// NewCompletion90PostingsFormat builds the format.
func NewCompletion90PostingsFormat() *Completion90PostingsFormat {
	return &Completion90PostingsFormat{baseCompletionPostingsFormat{name: "Completion90", version: "9.0"}}
}

// Completion912PostingsFormat is the Lucene 9.12 variant.
type Completion912PostingsFormat struct{ baseCompletionPostingsFormat }

// NewCompletion912PostingsFormat builds the format.
func NewCompletion912PostingsFormat() *Completion912PostingsFormat {
	return &Completion912PostingsFormat{baseCompletionPostingsFormat{name: "Completion912", version: "9.12"}}
}

// Completion99PostingsFormat is the Lucene 9.9 variant.
type Completion99PostingsFormat struct{ baseCompletionPostingsFormat }

// NewCompletion99PostingsFormat builds the format.
func NewCompletion99PostingsFormat() *Completion99PostingsFormat {
	return &Completion99PostingsFormat{baseCompletionPostingsFormat{name: "Completion99", version: "9.9"}}
}

// Completion101PostingsFormat is the Lucene 10.1 variant.
type Completion101PostingsFormat struct{ baseCompletionPostingsFormat }

// NewCompletion101PostingsFormat builds the format.
func NewCompletion101PostingsFormat() *Completion101PostingsFormat {
	return &Completion101PostingsFormat{baseCompletionPostingsFormat{name: "Completion101", version: "10.1"}}
}

// Completion104PostingsFormat is the Lucene 10.4 variant.
type Completion104PostingsFormat struct{ baseCompletionPostingsFormat }

// NewCompletion104PostingsFormat builds the format.
func NewCompletion104PostingsFormat() *Completion104PostingsFormat {
	return &Completion104PostingsFormat{baseCompletionPostingsFormat{name: "Completion104", version: "10.4"}}
}

var (
	_ CompletionPostingsFormat = (*Completion50PostingsFormat)(nil)
	_ CompletionPostingsFormat = (*Completion84PostingsFormat)(nil)
	_ CompletionPostingsFormat = (*Completion90PostingsFormat)(nil)
	_ CompletionPostingsFormat = (*Completion912PostingsFormat)(nil)
	_ CompletionPostingsFormat = (*Completion99PostingsFormat)(nil)
	_ CompletionPostingsFormat = (*Completion101PostingsFormat)(nil)
	_ CompletionPostingsFormat = (*Completion104PostingsFormat)(nil)
)
