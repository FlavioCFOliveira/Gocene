package automaton

// Terms is an interface for accessing terms in a field.
type Terms interface {
	Iterator() TermsEnum
}

// TermsEnum is an interface for iterating over terms.
type TermsEnum interface {
	SeekExact(term string) bool
	DocFreq() int
	TotalTermFreq() int64
}

// CompiledAutomaton is an optimized version of an Automaton for term enumeration.
type CompiledAutomaton struct {
	automaton *Automaton
}

func NewCompiledAutomaton(a *Automaton, binary bool, sorted bool, isBinary bool) *CompiledAutomaton {
	return &CompiledAutomaton{
		automaton: a,
	}
}

func (ca *CompiledAutomaton) GetTermsEnum(terms Terms) TermsEnum {
	// Placeholder: should return a TermsEnum that iterates terms accepted by the automaton.
	return nil
}

func (ca *CompiledAutomaton) HashCode() int {
	return 0
}

func (ca *CompiledAutomaton) Equals(other *CompiledAutomaton) bool {
	return ca.automaton == other.automaton
}

func (ca *CompiledAutomaton) RamBytesUsed() int64 {
	return 200
}

func (ca *CompiledAutomaton) Visit(visitor interface{}, query interface{}, field string) {
	// Placeholder
}
