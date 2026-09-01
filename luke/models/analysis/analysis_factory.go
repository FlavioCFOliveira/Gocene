package analysis

// AnalysisFactory is a factory of Analysis.
type AnalysisFactory struct{}

// NewAnalysisFactory creates a new instance of AnalysisFactory.
func NewAnalysisFactory() *AnalysisFactory {
	return &AnalysisFactory{}
}

// NewInstance creates a new instance of Analysis.
func (f *AnalysisFactory) NewInstance() Analysis {
	return NewAnalysisImpl()
}
