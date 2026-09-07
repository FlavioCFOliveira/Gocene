package store

// IOContext is an interface for providing context to I/O operations.
type IOContext interface {
	// GetID returns a unique identifier for this context.
	GetID() string
}

// DefaultIOContext is the default context used for I/O operations.
var DefaultIOContext = &defaultIOContext{}

type defaultIOContext struct{}

func (c *defaultIOContext) GetID() string { return "default" }
