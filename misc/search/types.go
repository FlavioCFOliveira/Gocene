// Package search implements org.apache.lucene.misc.search.
package search

import "sort"

// DiversifiedTopDocsCollector keeps the top-N hits but ensures no more than
// MaxPerKey hits share the same diversification key. Mirrors
// org.apache.lucene.misc.search.DiversifiedTopDocsCollector.
type DiversifiedTopDocsCollector struct {
	NumHits    int
	MaxPerKey  int
	KeyForDoc  func(docID int) string
	hits       []Hit
	keyCounts  map[string]int
}

// Hit is the (doc, score) tuple this collector accumulates.
type Hit struct {
	Doc   int
	Score float32
	Key   string
}

// NewDiversifiedTopDocsCollector builds the collector.
func NewDiversifiedTopDocsCollector(numHits, maxPerKey int, keyFn func(int) string) *DiversifiedTopDocsCollector {
	if numHits < 1 {
		numHits = 10
	}
	if maxPerKey < 1 {
		maxPerKey = 1
	}
	return &DiversifiedTopDocsCollector{
		NumHits:   numHits,
		MaxPerKey: maxPerKey,
		KeyForDoc: keyFn,
		keyCounts: make(map[string]int),
	}
}

// Collect records a docID with score, honouring the per-key cap.
func (c *DiversifiedTopDocsCollector) Collect(docID int, score float32) {
	key := ""
	if c.KeyForDoc != nil {
		key = c.KeyForDoc(docID)
	}
	if c.keyCounts[key] >= c.MaxPerKey {
		return
	}
	c.hits = append(c.hits, Hit{Doc: docID, Score: score, Key: key})
	c.keyCounts[key]++
	sort.SliceStable(c.hits, func(i, j int) bool { return c.hits[i].Score > c.hits[j].Score })
	if len(c.hits) > c.NumHits {
		evicted := c.hits[c.NumHits]
		c.hits = c.hits[:c.NumHits]
		c.keyCounts[evicted.Key]--
	}
}

// Hits returns a copy of the collected hits.
func (c *DiversifiedTopDocsCollector) Hits() []Hit {
	out := make([]Hit, len(c.hits))
	copy(out, c.hits)
	return out
}

// DocValuesStats captures min/max/sum/count summaries for a DocValues field.
// Mirrors org.apache.lucene.misc.search.DocValuesStats.
type DocValuesStats struct {
	Min   int64
	Max   int64
	Sum   int64
	Count int64
}

// Add folds value into the stats.
func (s *DocValuesStats) Add(value int64) {
	if s.Count == 0 {
		s.Min, s.Max = value, value
	} else {
		if value < s.Min {
			s.Min = value
		}
		if value > s.Max {
			s.Max = value
		}
	}
	s.Sum += value
	s.Count++
}

// Mean returns Sum/Count as a float64.
func (s *DocValuesStats) Mean() float64 {
	if s.Count == 0 {
		return 0
	}
	return float64(s.Sum) / float64(s.Count)
}

// DocValuesStatsCollector accumulates DocValuesStats over a stream of values.
// Mirrors org.apache.lucene.misc.search.DocValuesStatsCollector.
type DocValuesStatsCollector struct {
	Stats *DocValuesStats
}

// NewDocValuesStatsCollector builds the collector.
func NewDocValuesStatsCollector() *DocValuesStatsCollector {
	return &DocValuesStatsCollector{Stats: &DocValuesStats{}}
}

// Collect folds value into the running stats.
func (c *DocValuesStatsCollector) Collect(value int64) { c.Stats.Add(value) }

// HumanReadableQuery wraps a query with a debug-friendly label. Mirrors
// org.apache.lucene.misc.search.HumanReadableQuery.
type HumanReadableQuery struct {
	Description string
	Inner       any
}

// NewHumanReadableQuery builds the wrapper.
func NewHumanReadableQuery(desc string, inner any) *HumanReadableQuery {
	return &HumanReadableQuery{Description: desc, Inner: inner}
}

// MemoryAccountingBitsetCollector retains documents in a bitset while
// charging against a memory budget. Mirrors
// org.apache.lucene.misc.search.MemoryAccountingBitsetCollector.
type MemoryAccountingBitsetCollector struct {
	bits       map[int]bool
	MaxBytes   int64
	used       int64
	BytesPerDoc int64
}

// NewMemoryAccountingBitsetCollector builds the collector. Each docID
// counts as bytesPerDoc against the budget.
func NewMemoryAccountingBitsetCollector(maxBytes, bytesPerDoc int64) *MemoryAccountingBitsetCollector {
	if bytesPerDoc < 1 {
		bytesPerDoc = 8
	}
	return &MemoryAccountingBitsetCollector{bits: make(map[int]bool), MaxBytes: maxBytes, BytesPerDoc: bytesPerDoc}
}

// Add records docID, returning false when the budget would be exceeded.
func (c *MemoryAccountingBitsetCollector) Add(docID int) bool {
	if c.bits[docID] {
		return true
	}
	if c.MaxBytes > 0 && c.used+c.BytesPerDoc > c.MaxBytes {
		return false
	}
	c.bits[docID] = true
	c.used += c.BytesPerDoc
	return true
}

// Has reports whether docID is in the bitset.
func (c *MemoryAccountingBitsetCollector) Has(docID int) bool { return c.bits[docID] }

// Used returns the current budget consumption.
func (c *MemoryAccountingBitsetCollector) Used() int64 { return c.used }
