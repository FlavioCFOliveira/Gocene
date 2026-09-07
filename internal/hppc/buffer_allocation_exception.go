package hppc

import "fmt"

// BufferAllocationException is thrown when a buffer cannot be allocated.
type BufferAllocationException struct {
	Message string
}

func (e *BufferAllocationException) Error() string {
	return e.Message
}

func NewBufferAllocationException(message string, args ...interface{}) *BufferAllocationException {
	return &BufferAllocationException{
		Message: fmt.Sprintf(message, args...),
	}
}
