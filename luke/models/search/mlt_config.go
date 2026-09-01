package search

// MLTConfig holds configurations for MoreLikeThis query.
type MLTConfig struct {
	Fields      []string
	MaxDocFreq  int
	MinDocFreq  int
	MinTermFreq int
}

func NewMLTConfig() *MLTConfig {
	return &MLTConfig{
		MaxDocFreq: 1000, // default
		MinDocFreq: 1,    // default
		MinTermFreq: 1,    // default
	}
}
