// Package payloads implements org.apache.lucene.payloads.
package payloads

// PayloadSpanCollector collects (term, span, payload) tuples produced by a
// span query. Mirrors org.apache.lucene.payloads.PayloadSpanCollector.
type PayloadSpanCollector struct {
	Payloads [][]byte
}

// NewPayloadSpanCollector builds the collector.
func NewPayloadSpanCollector() *PayloadSpanCollector { return &PayloadSpanCollector{} }

// CollectLeaf records payload for one span.
func (c *PayloadSpanCollector) CollectLeaf(payload []byte) {
	c.Payloads = append(c.Payloads, append([]byte(nil), payload...))
}

// Reset clears the collected payloads.
func (c *PayloadSpanCollector) Reset() { c.Payloads = c.Payloads[:0] }

// PayloadSpanUtil hosts the bundled helpers Lucene exposes for payloads.
// Mirrors org.apache.lucene.payloads.PayloadSpanUtil.
type PayloadSpanUtil struct{}

// GetPayloads returns the payloads recorded by the supplied collector,
// flattening into a single slice.
func (PayloadSpanUtil) GetPayloads(c *PayloadSpanCollector) [][]byte {
	if c == nil {
		return nil
	}
	out := make([][]byte, len(c.Payloads))
	copy(out, c.Payloads)
	return out
}
