package expressions

// Bindings resolves named expression variables to a value source. Mirrors
// org.apache.lucene.expressions.Bindings.
type Bindings interface {
	// GetValueSource returns the value source registered for name.
	GetValueSource(name string) (ValueSource, bool)
}

// ValueSource produces a per-document numeric value.
type ValueSource interface {
	DoubleValueAt(docID int) (float64, error)
}

// SimpleBindings is the map-backed Bindings used by tests and by callers
// wiring expressions to known sources. Mirrors
// org.apache.lucene.expressions.SimpleBindings.
type SimpleBindings struct {
	sources map[string]ValueSource
}

// NewSimpleBindings builds an empty registry.
func NewSimpleBindings() *SimpleBindings {
	return &SimpleBindings{sources: make(map[string]ValueSource)}
}

// Add registers source under name.
func (b *SimpleBindings) Add(name string, source ValueSource) {
	b.sources[name] = source
}

// GetValueSource returns the value source registered for name.
func (b *SimpleBindings) GetValueSource(name string) (ValueSource, bool) {
	src, ok := b.sources[name]
	return src, ok
}

var _ Bindings = (*SimpleBindings)(nil)
