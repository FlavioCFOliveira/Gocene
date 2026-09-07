package document

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// CompletionsTermsReader is a holder for suggester and field-level info for a suggest field.
// Mirrors org.apache.lucene.search.suggest.document.CompletionsTermsReader.
type CompletionsTermsReader struct {
	minWeight int64
	maxWeight int64
	fieldType byte
	dictIn    store.IndexInput
	offset    int64
	mu        sync.Mutex
	suggester *NRTSuggester
}

// NewCompletionsTermsReader creates a new CompletionsTermsReader.
func NewCompletionsTermsReader(dictIn store.IndexInput, offset, minWeight, maxWeight int64, fieldType byte) *CompletionsTermsReader {
	return &CompletionsTermsReader{
		dictIn:    dictIn,
		offset:    offset,
		minWeight: minWeight,
		maxWeight: maxWeight,
		fieldType: fieldType,
	}
}

// Suggester returns the suggester for a field, loading it lazily from the index.
// Mirrors CompletionsTermsReader.suggester().
func (c *CompletionsTermsReader) Suggester() (*NRTSuggester, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.suggester == nil {
		// slice the dictionary to the suggester's data
		sliceIn := c.dictIn.Slice("NRTSuggester", c.offset, c.dictIn.Length()-c.offset)
		s, err := Load(sliceIn)
		if err != nil {
			return nil, fmt.Errorf("completions terms reader: load suggester: %w", err)
		}
		c.suggester = s
	}
	return c.suggester, nil
}

// MinWeight returns the minimum stored weight for this field.
func (c *CompletionsTermsReader) MinWeight() int64 { return c.minWeight }

// MaxWeight returns the maximum stored weight for this field.
func (c *CompletionsTermsReader) MaxWeight() int64 { return c.maxWeight }

// FieldType returns the byte type tag for this completion field.
func (c *CompletionsTermsReader) FieldType() byte { return c.fieldType }

// RAMBytesUsed returns an estimate of RAM used by this reader.
func (c *CompletionsTermsReader) RAMBytesUsed() int64 {
	if c.suggester != nil {
		// NRTSuggester doesn't have RAMBytesUsed yet, but we can estimate it or add it.
		return 0
	}
	return 64 // basic struct overhead
}
