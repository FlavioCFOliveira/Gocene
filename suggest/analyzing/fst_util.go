// Package analyzing implements
// org.apache.lucene.search.suggest.analyzing: the analyzing-suggester family
// and the small FST-related helpers it relies on.
package analyzing

// Path is the (input, output) pair the suggester records on the FST.
// Mirrors org.apache.lucene.search.suggest.analyzing.FSTUtil.Path.
type Path struct {
	Input  []byte
	Output []byte
}

// FSTUtil offers the small FST helpers the analyzing suggester uses.
// Mirrors org.apache.lucene.search.suggest.analyzing.FSTUtil.

// IntersectPrefixPaths returns every Path whose input starts with prefix.
// The Go port operates on an in-memory slice; concrete FST traversal lives
// inside util/fst.
func IntersectPrefixPaths(paths []Path, prefix []byte) []Path {
	var out []Path
	for _, p := range paths {
		if hasPrefix(p.Input, prefix) {
			out = append(out, p)
		}
	}
	return out
}

func hasPrefix(s, prefix []byte) bool {
	if len(prefix) > len(s) {
		return false
	}
	for i := range prefix {
		if s[i] != prefix[i] {
			return false
		}
	}
	return true
}
