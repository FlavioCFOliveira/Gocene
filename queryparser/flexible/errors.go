// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import "fmt"

// Message is the interface for structured, potentially localizable messages.
// This is the Go equivalent of Lucene's Message.
type Message interface {
	// GetKey returns the message key (e.g. a constant from QueryParserMessages).
	GetKey() string
	// GetInserts returns the variable substitution arguments.
	GetInserts() []interface{}
	// GetLocalizedMessage returns the fully formatted message string.
	GetLocalizedMessage() string
}

// MessageImpl is a concrete implementation of Message.
// This is the Go equivalent of Lucene's MessageImpl.
type MessageImpl struct {
	key     string
	inserts []interface{}
}

// NewMessageImpl creates a new MessageImpl with key and optional substitution arguments.
func NewMessageImpl(key string, inserts ...interface{}) *MessageImpl {
	args := make([]interface{}, len(inserts))
	copy(args, inserts)
	return &MessageImpl{key: key, inserts: args}
}

// GetKey returns the message key.
func (m *MessageImpl) GetKey() string { return m.key }

// GetInserts returns the substitution arguments.
func (m *MessageImpl) GetInserts() []interface{} {
	out := make([]interface{}, len(m.inserts))
	copy(out, m.inserts)
	return out
}

// GetLocalizedMessage formats the key with the inserts using fmt.Sprintf semantics.
// If there are no inserts the key is returned as-is.
func (m *MessageImpl) GetLocalizedMessage() string {
	if len(m.inserts) == 0 {
		return m.key
	}
	return fmt.Sprintf(m.key, m.inserts...)
}

// String returns the formatted message.
func (m *MessageImpl) String() string { return m.GetLocalizedMessage() }

// NLS provides minimal internationalisation support.
// In Lucene, this class loads ResourceBundles for locale-specific messages.
// Gocene does not ship resource bundles; NLS returns message keys as-is.
// This is the Go equivalent of Lucene's NLS.
type NLS struct{}

// GetLocalizedMessage returns the message formatted with its inserts.
// locale is accepted but ignored (Gocene is English-only).
func (NLS) GetLocalizedMessage(msg Message) string {
	if msg == nil {
		return ""
	}
	return msg.GetLocalizedMessage()
}

// NLSException is an error that carries a structured Message for localisation.
// This is the Go equivalent of Lucene's NLSException.
type NLSException struct {
	message Message
	cause   error
}

// NewNLSException creates a new NLSException.
func NewNLSException(message Message, cause error) *NLSException {
	return &NLSException{message: message, cause: cause}
}

// Error implements error.
func (e *NLSException) Error() string {
	if e.message != nil {
		return e.message.GetLocalizedMessage()
	}
	return ""
}

// Unwrap returns the underlying cause.
func (e *NLSException) Unwrap() error { return e.cause }

// GetMessage returns the structured Message.
func (e *NLSException) GetMessage() Message { return e.message }

// QueryNodeError represents a non-recoverable error in the query node framework.
// In Java Lucene this extends Error (not Exception); in Go it is a regular error.
// This is the Go equivalent of Lucene's QueryNodeError.
type QueryNodeError struct {
	message Message
	cause   error
}

// NewQueryNodeError creates a new QueryNodeError.
func NewQueryNodeError(message Message) *QueryNodeError {
	return &QueryNodeError{message: message}
}

// NewQueryNodeErrorWithCause creates a QueryNodeError wrapping a cause.
func NewQueryNodeErrorWithCause(message Message, cause error) *QueryNodeError {
	return &QueryNodeError{message: message, cause: cause}
}

// Error implements error.
func (e *QueryNodeError) Error() string {
	if e.message != nil {
		return e.message.GetLocalizedMessage()
	}
	return MsgQueryNodeError
}

// Unwrap returns the cause.
func (e *QueryNodeError) Unwrap() error { return e.cause }

// GetMessage returns the structured Message.
func (e *QueryNodeError) GetMessage() Message { return e.message }

// QueryNodeException is a recoverable exception in the query node framework.
// This is the Go equivalent of Lucene's QueryNodeException.
type QueryNodeException struct {
	message Message
	cause   error
}

// NewQueryNodeException creates a new QueryNodeException.
func NewQueryNodeException(message Message) *QueryNodeException {
	return &QueryNodeException{message: message}
}

// NewQueryNodeExceptionWithCause creates a QueryNodeException wrapping a cause.
func NewQueryNodeExceptionWithCause(message Message, cause error) *QueryNodeException {
	return &QueryNodeException{message: message, cause: cause}
}

// Error implements error.
func (e *QueryNodeException) Error() string {
	if e.message != nil {
		return e.message.GetLocalizedMessage()
	}
	return MsgQueryNodeError
}

// Unwrap returns the cause.
func (e *QueryNodeException) Unwrap() error { return e.cause }

// GetMessage returns the structured Message.
func (e *QueryNodeException) GetMessage() Message { return e.message }

// QueryNodeParseException is thrown when the syntax parser encounters an error
// in the query string. It specialises QueryNodeException.
// This is the Go equivalent of Lucene's QueryNodeParseException.
type QueryNodeParseException struct {
	*QueryNodeException
	query  string
	line   int
	column int
}

// NewQueryNodeParseException creates a new QueryNodeParseException.
func NewQueryNodeParseException(message Message) *QueryNodeParseException {
	return &QueryNodeParseException{QueryNodeException: NewQueryNodeException(message)}
}

// NewQueryNodeParseExceptionWithLocation creates a QueryNodeParseException with position.
func NewQueryNodeParseExceptionWithLocation(message Message, query string, line, column int) *QueryNodeParseException {
	e := &QueryNodeParseException{
		QueryNodeException: NewQueryNodeException(message),
		query:              query,
		line:               line,
		column:             column,
	}
	return e
}

// GetQuery returns the query string that triggered the error.
func (e *QueryNodeParseException) GetQuery() string { return e.query }

// GetBeginLine returns the line number of the error.
func (e *QueryNodeParseException) GetBeginLine() int { return e.line }

// GetBeginColumn returns the column number of the error.
func (e *QueryNodeParseException) GetBeginColumn() int { return e.column }
