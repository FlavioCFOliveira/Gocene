// Package histograms implements
// org.apache.lucene.sandbox.facet.plain.histograms.
package histograms

// HistogramCollectorManager builds histograms over a numeric field. Mirrors
// org.apache.lucene.sandbox.facet.plain.histograms.HistogramCollectorManager.
type HistogramCollectorManager struct {
	Field    string
	Interval int64
}

// NewHistogramCollectorManager builds the manager with the supplied interval.
func NewHistogramCollectorManager(field string, interval int64) *HistogramCollectorManager {
	if interval < 1 {
		interval = 1
	}
	return &HistogramCollectorManager{Field: field, Interval: interval}
}
