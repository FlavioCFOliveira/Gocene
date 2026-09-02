package codecs

// TermStats holds per-term statistics.
type TermStats struct {
	DocFreq       int32
	TotalTermFreq int64
}
