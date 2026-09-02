package uhighlight

// PassageFormatter creates a formatted snippet from the top passages.
type PassageFormatter interface {
	// Format formats the top passages from content into a human-readable text snippet.
	// passages are sorted in the order that they appear in the document for convenience.
	Format(passages []*Passage, content string) interface{}
}
