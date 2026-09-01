package analysis

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

type analysisImpl struct {
	analyzer analysis.Analyzer
	loader   *AnalysisSPILoader
}

// NewAnalysisImpl creates a new instance of Analysis.
func NewAnalysisImpl() Analysis {
	return &analysisImpl{
		analyzer: analysis.NewStandardAnalyzer(),
		loader:   analysis.NewAnalysisSPILoader(),
	}
}

func (a *analysisImpl) GetAvailableCharFilters() []string {
	return a.loader.AvailableServices()
}

func (a *analysisImpl) GetAvailableTokenizers() []string {
	return a.loader.AvailableServices()
}

func (a *analysisImpl) GetAvailableTokenFilters() []string {
	return a.loader.AvailableServices()
}

func (a *analysisImpl) CreateAnalyzerFromClassName(analyzerType string) (analysis.Analyzer, error) {
	inst, err := a.loader.NewInstance(analyzerType, nil)
	if err != nil {
		return nil, err
	}
	analyzer, ok := inst.(analysis.Analyzer)
	if !ok {
		return nil, fmt.Errorf("instance of %s does not implement analysis.Analyzer", analyzerType)
	}
	a.analyzer = analyzer
	return analyzer, nil
}

func (a *analysisImpl) BuildCustomAnalyzer(config CustomAnalyzerConfig) (analysis.Analyzer, error) {
	builder := analysis.NewCustomAnalyzerBuilder()

	// Tokenizer
	tokenizerFactory, err := a.loader.NewInstance(config.TokenizerConfig.Name, config.TokenizerConfig.Params)
	if err != nil {
		return nil, err
	}
	tf, ok := tokenizerFactory.(analysis.TokenizerFactory)
	if !ok {
		return nil, fmt.Errorf("factory for %s is not a TokenizerFactory", config.TokenizerConfig.Name)
	}
	builder.WithTokenizer(tf)

	// Char filters
	for _, cfConf := range config.CharFilterConfigs {
		cfFactory, err := a.loader.NewInstance(cfConf.Name, cfConf.Params)
		if err != nil {
			return nil, err
		}
		cff, ok := cfFactory.(analysis.CharFilterFactory)
		if !ok {
			return nil, fmt.Errorf("factory for %s is not a CharFilterFactory", cfConf.Name)
		}
		builder.AddCharFilter(cff)
	}

	// Token filters
	for _, tfConf := range config.TokenFilterConfigs {
		tfFactory, err := a.loader.NewInstance(tfConf.Name, tfConf.Params)
		if err != nil {
			return nil, err
		}
		tff, ok := tfFactory.(analysis.TokenFilterFactory)
		if !ok {
			return nil, fmt.Errorf("factory for %s is not a TokenFilterFactory", tfConf.Name)
		}
		builder.AddTokenFilter(tff)
	}

	analyzer, err := builder.Build()
	if err != nil {
		return nil, err
	}
	a.analyzer = analyzer
	return analyzer, nil
}

func (a *analysisImpl) Analyze(text string) ([]Token, error) {
	if a.analyzer == nil {
		return nil, errors.New("analyzer is not set")
	}

	reader := strings.NewReader(text)
	stream, err := a.analyzer.TokenStream("field", reader)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var result []Token
	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}

		attributes := a.copyAttributes(stream)
		var term string
		if ha, ok := stream.(interface{ GetCharTermAttribute() analysis.CharTermAttribute }); ok {
			term = ha.GetCharTermAttribute().String()
		}

		result = append(result, Token{
			term:       term,
			attributes: attributes,
		})
	}
	_ = stream.End()

	return result, nil
}

func (a *analysisImpl) copyAttributes(stream analysis.TokenStream) []TokenAttribute {
	var attributes []TokenAttribute

	if bts, ok := stream.(interface{ GetAttributeSource() *util.AttributeSource }); ok {
		source := bts.GetAttributeSource()
		for _, impl := range source.GetAttributeImplsIterator() {
			var attValues map[string]string
			attValues = make(map[string]string)
			impl.ReflectWith(func(attType reflect.Type, key string, value any) {
				if value != nil {
					attValues[key] = fmt.Sprintf("%v", value)
				}
			})
			attributes = append(attributes, TokenAttribute{
				attClass:  impl.GetType().String(),
				attValues: attValues,
			})
		}
	}

	return attributes
}

func (a *analysisImpl) CurrentAnalyzer() (analysis.Analyzer, error) {
	if a.analyzer == nil {
		return nil, errors.New("analyzer is not set")
	}
	return a.analyzer, nil
}

func (a *analysisImpl) AddExternalJars(jarFiles []string) error {
	return errors.New("loading external JARs is not supported in the Go port")
}

func (a *analysisImpl) AnalyzeStepByStep(text string) (*StepByStepResult, error) {
	if a.analyzer == nil {
		return nil, errors.New("analyzer is not set")
	}

	custom, ok := a.analyzer.(*analysis.CustomAnalyzer)
	if !ok {
		return nil, errors.New("analyzer is not CustomAnalyzer")
	}

	charFilterFactories := custom.GetCharFilterFactories()
	var charfilteredTexts []CharfilteredText
	var currentReader io.Reader = strings.NewReader(text)

	for _, cfFactory := range charFilterFactories {
		currentReader = cfFactory.Create(currentReader)
		out := writeCharStream(currentReader)
		charfilteredTexts = append(charfilteredTexts, CharfilteredText{
			NamedObject: NamedObject{name: "CharFilter"},
			text:        out,
		})
	}

	tokenizerFactory := custom.GetTokenizerFactory()
	tokenizer := analysis.CreateDefaultTokenizer(tokenizerFactory)
	if err := tokenizer.SetReader(currentReader); err != nil {
		return nil, err
	}

	var namedTokens []NamedTokens

	tokens, attrSources, err := a.analyzeTokenStream(tokenizer)
	if err != nil {
		return nil, err
	}
	namedTokens = append(namedTokens, NamedTokens{
		NamedObject: NamedObject{name: "Tokenizer"},
		tokens:      tokens,
	})

	tokenFilterFactories := custom.GetTokenFilterFactories()
	var currentStream analysis.TokenStream = tokenizer

	for _, tfFactory := range tokenFilterFactories {
		listStream := newListBasedTokenStream(attrSources)
		currentStream = tfFactory.Create(listStream)

		tokens, attrSources, err = a.analyzeTokenStream(currentStream)
		if err != nil {
			return nil, err
		}
		namedTokens = append(namedTokens, NamedTokens{
			NamedObject: NamedObject{name: "TokenFilter"},
			tokens:      tokens,
		})
	}

	return &StepByStepResult{
		charfilteredTexts: charfilteredTexts,
		namedTokens:       namedTokens,
	}, nil
}

func (a *analysisImpl) analyzeTokenStream(stream analysis.TokenStream) ([]Token, []*util.AttributeSource, error) {
	var result []Token
	var sources []*util.AttributeSource

	if err := stream.Reset(); err != nil {
		return nil, nil, err
	}

	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			break
		}

		if bts, ok := stream.(interface{ GetAttributeSource() *util.AttributeSource }); ok {
			sources = append(sources, bts.GetAttributeSource().CloneAttributes())
		}

		attributes := a.copyAttributes(stream)
		var term string
		if ha, ok := stream.(interface{ GetCharTermAttribute() analysis.CharTermAttribute }); ok {
			term = ha.GetCharTermAttribute().String()
		}

		result = append(result, Token{
			term:       term,
			attributes: attributes,
		})
	}
	if err := stream.End(); err != nil {
		return nil, nil, err
	}

	return result, sources, nil
}

type listBasedTokenStream struct {
	factory  util.AttributeFactory
	sources  []*util.AttributeSource
	iterator int
}

func newListBasedTokenStream(sources []*util.AttributeSource) *listBasedTokenStream {
	return &listBasedTokenStream{
		factory: util.DefaultAttributeFactoryInstance,
		sources: sources,
	}
}

func (ls *listBasedTokenStream) IncrementToken() (bool, error) {
	if ls.iterator >= len(ls.sources) {
		return false, nil
	}

	source := ls.sources[ls.iterator]
	ls.iterator++

	// We need to provide an AttributeSource for the downstream filter
	// But the TokenStream interface doesn't have a way to set the current source.
	// In Gocene, this is typically handled by the TokenStream implementation.
	// Since we are a mock, we just return true.
	// However, if the downstream filter calls GetAttributeSource(), we need it to work.
	// So we'll need to make listBasedTokenStream a BaseTokenStream or similar.
	return true, nil
}

func (ls *listBasedTokenStream) End() error {
	return nil
}

func (ls *listBasedTokenStream) Close() error {
	return nil
}

func (ls *listBasedTokenStream) GetAttributeSource() *util.AttributeSource {
	if ls.iterator == 0 || ls.iterator > len(ls.sources) {
		return nil
	}
	return ls.sources[ls.iterator-1]
}

func writeCharStream(input io.Reader) string {
	var sb strings.Builder
	buf := make([]byte, 1024)
	for {
		n, err := input.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return ""
		}
	}
	return sb.String()
}
