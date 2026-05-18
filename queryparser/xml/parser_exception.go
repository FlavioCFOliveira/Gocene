// Package xml implements Lucene's XML query parser, allowing queries to be
// expressed as XML documents and parsed into search.Query instances via a
// registry of named QueryBuilders.
//
// This is the Go port of org.apache.lucene.queryparser.xml.
package xml

// ParserException is returned when an XML query document cannot be parsed
// into a search.Query (malformed structure, unknown element, missing
// required attribute, etc.). It mirrors
// org.apache.lucene.queryparser.xml.ParserException.
type ParserException struct {
	Message string
	Cause   error
}

func (e *ParserException) Error() string {
	if e.Message == "" {
		if e.Cause != nil {
			return e.Cause.Error()
		}
		return "xml parser error"
	}
	return e.Message
}

func (e *ParserException) Unwrap() error { return e.Cause }

// NewParserException creates a ParserException with the given message.
func NewParserException(message string) *ParserException {
	return &ParserException{Message: message}
}

// NewParserExceptionWithCause wraps an underlying error in a ParserException.
func NewParserExceptionWithCause(message string, cause error) *ParserException {
	return &ParserException{Message: message, Cause: cause}
}
