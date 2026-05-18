package ext

import (
	"fmt"
	"strings"
)

// DefaultExtensionFieldDelimiter is the character that separates the base
// field from the extension key inside an extended field name (e.g. "foo:bar"
// where ':' is the extension delimiter).
const DefaultExtensionFieldDelimiter = ':'

// Extensions is the registry of named ParserExtension handlers used by
// ExtendableQueryParser. It mirrors org.apache.lucene.queryparser.ext.Extensions.
type Extensions struct {
	extensions     map[string]ParserExtension
	fieldDelimiter rune
}

// NewExtensions returns an Extensions registry that uses the default ':'
// delimiter between the field name and the extension key.
func NewExtensions() *Extensions {
	return NewExtensionsWithDelimiter(DefaultExtensionFieldDelimiter)
}

// NewExtensionsWithDelimiter returns an Extensions registry that uses the
// supplied rune as the extension delimiter.
func NewExtensionsWithDelimiter(delim rune) *Extensions {
	return &Extensions{
		extensions:     make(map[string]ParserExtension),
		fieldDelimiter: delim,
	}
}

// Add registers a ParserExtension under the given key. A subsequent Add with
// the same key replaces the previously registered extension.
func (e *Extensions) Add(key string, extension ParserExtension) {
	e.extensions[key] = extension
}

// GetExtension returns the ParserExtension registered for key, or nil if none.
func (e *Extensions) GetExtension(key string) ParserExtension {
	return e.extensions[key]
}

// GetExtensionFieldDelimiter returns the character used to separate the
// extension key from the base field.
func (e *Extensions) GetExtensionFieldDelimiter() rune { return e.fieldDelimiter }

// SplitExtensionField parses an extended field name of the form
// "<field><delim><key>" and returns the base field and extension key. When the
// supplied field does not contain the delimiter, defaultField is returned as
// the field and an empty key is returned.
func (e *Extensions) SplitExtensionField(defaultField, fieldOrKey string) (string, string) {
	delim := string(e.fieldDelimiter)
	idx := strings.Index(fieldOrKey, delim)
	if idx < 0 {
		return defaultField, ""
	}
	return fieldOrKey[:idx], fieldOrKey[idx+len(delim):]
}

// BuildExtensionField composes an extended field name from the base field and
// the extension key. When field is empty the bare key is returned (matching
// Lucene's behaviour where the parser would later substitute the default field).
func (e *Extensions) BuildExtensionField(key, field string) string {
	if field == "" {
		return key
	}
	return fmt.Sprintf("%s%c%s", field, e.fieldDelimiter, key)
}

// EscapeExtensionField inserts an escape character before every occurrence of
// the extension delimiter so the resulting string can survive a parse round
// trip without being interpreted as an extension.
func (e *Extensions) EscapeExtensionField(extField string) string {
	delim := e.fieldDelimiter
	var b strings.Builder
	b.Grow(len(extField) + 4)
	for _, r := range extField {
		if r == delim {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
