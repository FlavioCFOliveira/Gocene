package codecs

// TermStats holds per-term statistics.
type TermStats struct {
	Field         string
	TermText      []byte
	DocFreq       int32
	TotalTermFreq int64
}
