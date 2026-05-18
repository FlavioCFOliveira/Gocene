package misc

// IndexMergeTool is the command-line driver that merges several indexes into
// one. Mirrors org.apache.lucene.misc.IndexMergeTool. The Go port exposes a
// reusable function so callers can embed the merge step in their own tools.
type IndexMergeTool struct {
	Inputs []string
	Output string
}

// NewIndexMergeTool builds the tool.
func NewIndexMergeTool(output string, inputs []string) *IndexMergeTool {
	return &IndexMergeTool{Output: output, Inputs: append([]string(nil), inputs...)}
}

// Run is the public entry point. Concrete merging delegates to the
// index/IndexWriter routines and is left to the caller; the Go port
// focuses on the input/output bookkeeping shared by every invocation.
func (t *IndexMergeTool) Run(merge func(output string, inputs []string) error) error {
	if merge == nil {
		return nil
	}
	return merge(t.Output, t.Inputs)
}
