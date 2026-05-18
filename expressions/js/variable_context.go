package js

// VariableContext records the per-variable metadata captured at compile time.
// Mirrors org.apache.lucene.expressions.js.VariableContext.
type VariableContext struct {
	Variables map[string]VariableInfo
}

// VariableInfo describes one variable.
type VariableInfo struct {
	Name     string
	IsField  bool
	IsMethod bool
}

// NewVariableContext builds an empty context.
func NewVariableContext() *VariableContext {
	return &VariableContext{Variables: make(map[string]VariableInfo)}
}

// Add registers a variable.
func (c *VariableContext) Add(info VariableInfo) {
	c.Variables[info.Name] = info
}

// Get returns the info for name.
func (c *VariableContext) Get(name string) (VariableInfo, bool) {
	v, ok := c.Variables[name]
	return v, ok
}
