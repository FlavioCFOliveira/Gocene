// Command kg-inventory measures the working tree of the Gocene Go module and
// writes the knowledge-graph inventory for the Gocene tier (as defined in
// knowledge-model.md) to /tmp/gocene-kg-inventory.
//
// The program is a pure measurement tool: it walks the tree, parses every Go
// file with go/parser, and emits TSV dumps (one row per node or edge, with a
// header row) plus a populations.json carrying the per-label statistics the
// model document needs. It never guesses: a file that fails to parse is
// reported as an anomaly and emitted with zero declarations.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const outDir = "/tmp/gocene-kg-inventory"

var (
	spaceRe    = regexp.MustCompile(`[\t\r\n]+`)
	pkgClause  = regexp.MustCompile(`(?m)^package\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?://.*)?$`)
	buildTagRe = regexp.MustCompile(`unix|windows|posix`)
)

var modulePath string

// ---------------------------------------------------------------------------
// records
// ---------------------------------------------------------------------------

type FileRec struct {
	Path       string
	Language   string
	Role       string
	ImportPath string
	Generated  bool
	Generator  string // generator path(s), comma-joined; empty when not generated
	Lines      int

	pkgName  string // declared Go package name; "" when unknown
	src      []byte
	parseErr string
	platform bool // a //go:build constraint mentions unix/windows/posix
}

type PkgRec struct {
	ImportPath string
	Name       string
	Dir        string
	Depth      int
	Test       bool
	TestOnly   bool
}

type TypeRec struct {
	File        string
	IP          string // declaring package importPath
	Nat         string // natural qn: IP.Name
	Name        string
	Kind        string // struct | interface | defined | alias
	DefinedFrom string
	TypeParams  int
	Line        int
	QN          string // final identity
	FileOrd     int    // ordinal among the file's type declarations
}

type FuncRec struct {
	File    string
	IP      string
	Nat     string
	Name    string
	Kind    string
	Line    int
	QN      string
	FileOrd int // ordinal among the file's package-level functions
}

type MethodRec struct {
	File        string
	IP          string
	NatSig      string
	Name        string
	Receiver    string // exact source text; "" for interface specs
	RecvQN      string // computed base QN; "" for interface specs
	RecvPtr     bool
	IsInterface bool
	IfaceQN     string
	Line        int
	ParamRecv   bool
	Defect      bool // concrete method whose recvQN resolves to no declared type
	OwnerList   []string
	Sig         string // final identity
	FileOrd     int    // ordinal among the file's method declarations (concrete + specs)
}

type FieldRec struct {
	File     string
	Owner    *TypeRec
	Name     string
	Type     string
	Embedded bool
	Line     int
	NatKey   string // Owner.QN + "." + name-part (set after type disambiguation)
	Key      string // final identity
	FileOrd  int    // ordinal among the file's field declarations
}

type CVRec struct { // const or var
	File    string
	IP      string
	Nat     string
	Name    string
	Type    string
	Line    int
	QN      string
	FileOrd int // ordinal among the file's const (or var) names
}

type ImpRec struct {
	File  string
	Path  string
	Blank bool
	Dot   bool
}

type EmbedEdge struct {
	From string // owner type final QN
	To   string // target type final QN, or external reference text
	Kind string // "type" | "external"
}

type ImportEdge struct {
	File  string
	To    string
	Kind  string // "package" | "external" | "missing"
	Blank bool
	Dot   bool
}

type ContainsFileEdge struct {
	From string // package importPath, or "." for the module
	To   string // file path
}

type GenEdge struct {
	File      string
	Generator string
	Self      bool
}

// ---------------------------------------------------------------------------
// collections
// ---------------------------------------------------------------------------

var (
	files   []*FileRec
	pkgs    []*PkgRec
	types   []*TypeRec
	funcs   []*FuncRec
	methods []*MethodRec
	fields  []*FieldRec
	consts  []*CVRec
	vars    []*CVRec
	imps    []ImpRec
)

func main() {
	root, err := os.Getwd()
	must(err)

	mp, goVersion, err := readGoMod(filepath.Join(root, "go.mod"))
	must(err)
	modulePath = mp

	files, err = walk(root)
	must(err)

	fset := token.NewFileSet()
	var parseErrs [][2]string
	for _, f := range files {
		if f.Language != "go" {
			f.assignRole()
			continue
		}
		f.parseErr = parseFile(fset, f)
		if f.parseErr != "" {
			parseErrs = append(parseErrs, [2]string{f.Path, f.parseErr})
		}
	}

	buildPackages()

	dirProd := map[string]*PkgRec{}
	for _, p := range pkgs {
		if !strings.HasSuffix(p.Name, "_test") {
			dirProd[p.Dir] = p // at most one production package per directory
		}
	}
	for _, f := range files {
		switch {
		case f.Language == "go":
			if f.pkgName != "" {
				f.ImportPath = pkgIndex[pkgKey{dirOf(f.Path), f.pkgName}].ImportPath
			}
		default:
			if p, ok := dirProd[dirOf(f.Path)]; ok {
				f.ImportPath = p.ImportPath
			}
		}
	}

	// ---- type identity first (field keys depend on it) ----
	assignFinals(types,
		func(t *TypeRec) string { return t.Nat },
		func(t *TypeRec) string { return t.File },
		func(t *TypeRec) int { return t.FileOrd },
		func(t *TypeRec, s string) { t.QN = s },
		func(t *TypeRec) bool { return false })

	typesByNat := map[string][]*TypeRec{}
	for _, t := range types {
		typesByNat[t.Nat] = append(typesByNat[t.Nat], t)
	}

	// ---- method owner resolution ----
	for _, m := range methods {
		if m.IsInterface {
			if c := typesByNat[m.IfaceQN]; len(c) > 0 {
				m.OwnerList = finalOf(c)
			}
			continue
		}
		if c := typesByNat[m.RecvQN]; len(c) > 0 {
			m.OwnerList = finalOf(c)
		} else {
			m.Defect = true
		}
	}

	// ---- field natural keys, then field identity ----
	for _, fd := range fields {
		fd.NatKey = fd.Owner.QN + "." + fd.Name
	}
	assignFinals(fields,
		func(f *FieldRec) string { return f.NatKey },
		func(f *FieldRec) string { return f.File },
		func(f *FieldRec) int { return f.FileOrd },
		func(f *FieldRec, s string) { f.Key = s },
		func(f *FieldRec) bool { return false })

	assignFinals(funcs,
		func(f *FuncRec) string { return f.Nat },
		func(f *FuncRec) string { return f.File },
		func(f *FuncRec) int { return f.FileOrd },
		func(f *FuncRec, s string) { f.QN = s },
		func(f *FuncRec) bool { return f.Kind == "init" })

	assignFinals(methods,
		func(m *MethodRec) string { return m.NatSig },
		func(m *MethodRec) string { return m.File },
		func(m *MethodRec) int { return m.FileOrd },
		func(m *MethodRec, s string) { m.Sig = s },
		func(m *MethodRec) bool { return false })

	assignFinals(consts,
		func(c *CVRec) string { return c.Nat },
		func(c *CVRec) string { return c.File },
		func(c *CVRec) int { return c.FileOrd },
		func(c *CVRec, s string) { c.QN = s },
		func(c *CVRec) bool { return false })

	assignFinals(vars,
		func(v *CVRec) string { return v.Nat },
		func(v *CVRec) string { return v.File },
		func(v *CVRec) int { return v.FileOrd },
		func(v *CVRec, s string) { v.QN = s },
		func(v *CVRec) bool { return false })

	checkUnique()

	// ---- embeds ----
	pkgNames := map[string][]string{} // package name -> importPaths
	for _, p := range pkgs {
		pkgNames[p.Name] = append(pkgNames[p.Name], p.ImportPath)
	}
	var embeds []EmbedEdge
	extTypes := map[string]string{} // reference text -> origin
	for _, fd := range fields {
		if !fd.Embedded {
			continue
		}
		ref := fd.Type
		// A leading * marks a pointer embed and type arguments are erased; the
		// embedded type's reference is the bare type name (the * and [..] are
		// kept only in the field's `type` property and in the external
		// reference text). The first '[' always opens the argument list — an
		// identifier or selector cannot contain one — so the arguments are
		// stripped before the package qualifier is looked for.
		base := strings.TrimSpace(strings.TrimPrefix(ref, "*"))
		if j := strings.IndexByte(base, '['); j >= 0 {
			base = strings.TrimSpace(base[:j])
		}
		var cands []*TypeRec
		if i := strings.IndexByte(base, '.'); i < 0 {
			cands = typesByNat[fd.Owner.IP+"."+base]
		} else {
			pkg, typ := base[:i], base[i+1:]
			for _, ip := range pkgNames[pkg] {
				cands = append(cands, typesByNat[ip+"."+typ]...)
			}
		}
		if len(cands) == 1 {
			embeds = append(embeds, EmbedEdge{From: fd.Owner.QN, To: cands[0].QN, Kind: "type"})
		} else {
			origin := "unresolved"
			if strings.Contains(ref, ".") {
				origin = "qualified"
			}
			extTypes[ref] = origin
			embeds = append(embeds, EmbedEdge{From: fd.Owner.QN, To: ref, Kind: "external"})
		}
	}

	// ---- imports ----
	extPkgs := map[string]bool{}
	missingPkgs := map[string]bool{}
	var importEdges []ImportEdge
	for _, im := range imps {
		if im.Path == modulePath || strings.HasPrefix(im.Path, modulePath+"/") {
			dir := "."
			if im.Path != modulePath {
				dir = im.Path[len(modulePath)+1:]
			}
			if p, ok := dirProd[dir]; ok {
				importEdges = append(importEdges, ImportEdge{File: im.File, To: p.ImportPath, Kind: "package", Blank: im.Blank, Dot: im.Dot})
			} else {
				missingPkgs[im.Path] = true
				importEdges = append(importEdges, ImportEdge{File: im.File, To: im.Path, Kind: "missing", Blank: im.Blank, Dot: im.Dot})
			}
		} else {
			extPkgs[im.Path] = true
			importEdges = append(importEdges, ImportEdge{File: im.File, To: im.Path, Kind: "external", Blank: im.Blank, Dot: im.Dot})
		}
	}

	// ---- contains_file (exactly one edge per file) ----
	var containsFile []ContainsFileEdge
	for _, f := range files {
		if f.Language == "go" && f.pkgName != "" {
			containsFile = append(containsFile, ContainsFileEdge{From: f.ImportPath, To: f.Path})
			continue
		}
		if p, ok := dirProd[dirOf(f.Path)]; ok {
			containsFile = append(containsFile, ContainsFileEdge{From: p.ImportPath, To: f.Path})
			continue
		}
		containsFile = append(containsFile, ContainsFileEdge{From: ".", To: f.Path})
	}

	// ---- generated_from ----
	var goFiles, genFiles []*FileRec
	for _, f := range files {
		if f.Language == "go" {
			goFiles = append(goFiles, f)
		}
		if f.Generated {
			genFiles = append(genFiles, f)
		}
	}
	var genEdges []GenEdge
	for _, f := range genFiles {
		var gens []*FileRec
		for _, g := range goFiles {
			if g != f && bytes.Contains(g.src, []byte(f.Path)) {
				gens = append(gens, g)
			}
		}
		if len(gens) == 0 && bytes.Contains(f.src, []byte(f.Path)) {
			gens = append(gens, f) // self-edge: the known detector over-match
		}
		if len(gens) == 0 {
			continue // no generator found: reported as dangling in populations.json
		}
		paths := make([]string, 0, len(gens))
		for _, g := range gens {
			genEdges = append(genEdges, GenEdge{File: f.Path, Generator: g.Path, Self: g == f})
			paths = append(paths, g.Path)
		}
		sort.Strings(paths)
		f.Generator = strings.Join(paths, ",")
	}

	must(os.MkdirAll(outDir, 0o755))

	writeTSV("module.tsv", []string{"modulePath", "goVersion"},
		[][]string{{mp, goVersion}})
	writeTSV("packages.tsv", []string{"importPath", "name", "dir", "depth", "test", "testOnly"},
		pkgRows())
	writeTSV("files.tsv", []string{"path", "language", "role", "importPath", "generated", "generator", "lines"},
		fileRows())
	writeTSV("types.tsv", []string{"qn", "name", "kind", "definedFrom", "typeParams", "line"},
		typeRows())
	writeTSV("functions.tsv", []string{"qn", "name", "kind", "line"},
		funcRows())
	writeTSV("methods.tsv", []string{"sig", "name", "receiver", "recvQN", "recvPtr", "interface", "ifaceQN", "line"},
		methodRows())
	writeTSV("fields.tsv", []string{"key", "name", "type", "embedded", "line"},
		fieldRows())
	writeTSV("constants.tsv", []string{"qn", "name", "type", "line"},
		cvRows(consts))
	writeTSV("variables.tsv", []string{"qn", "name", "type", "line"},
		cvRows(vars))

	extPkgRows := make([][]string, 0, len(extPkgs))
	for p := range extPkgs {
		extPkgRows = append(extPkgRows, []string{p})
	}
	extTypeRows := make([][]string, 0, len(extTypes))
	for q, o := range extTypes {
		extTypeRows = append(extTypeRows, []string{q, o})
	}
	missingRows := make([][]string, 0, len(missingPkgs))
	for m := range missingPkgs {
		missingRows = append(missingRows, []string{m})
	}
	writeTSV("ext_packages.tsv", []string{"importPath"}, extPkgRows)
	writeTSV("ext_types.tsv", []string{"qn", "origin"}, extTypeRows)
	writeTSV("missing_packages.tsv", []string{"importPath"}, missingRows)

	cpRows := make([][]string, 0, len(pkgs))
	for _, p := range pkgs {
		cpRows = append(cpRows, []string{p.ImportPath})
	}
	writeTSV("edges_contains_package.tsv", []string{"importPath"}, cpRows)

	cfRows := make([][]string, 0, len(containsFile))
	for _, e := range containsFile {
		cfRows = append(cfRows, []string{e.From, e.To})
	}
	writeTSV("edges_contains_file.tsv", []string{"from", "to"}, cfRows)

	dtRows := make([][]string, 0, len(types))
	for _, t := range types {
		dtRows = append(dtRows, []string{t.File, t.QN})
	}
	dfRows := make([][]string, 0, len(funcs))
	for _, f := range funcs {
		dfRows = append(dfRows, []string{f.File, f.QN})
	}
	dcRows := make([][]string, 0, len(consts))
	for _, c := range consts {
		dcRows = append(dcRows, []string{c.File, c.QN})
	}
	dvRows := make([][]string, 0, len(vars))
	for _, v := range vars {
		dvRows = append(dvRows, []string{v.File, v.QN})
	}
	dmfRows := make([][]string, 0, len(methods))
	dmtRows := make([][]string, 0, len(methods))
	for _, m := range methods {
		dmfRows = append(dmfRows, []string{m.File, m.Sig})
		for _, o := range m.OwnerList {
			dmtRows = append(dmtRows, []string{o, m.Sig})
		}
	}
	dflRows := make([][]string, 0, len(fields))
	for _, fd := range fields {
		dflRows = append(dflRows, []string{fd.Owner.QN, fd.Key})
	}
	emRows := make([][]string, 0, len(embeds))
	for _, e := range embeds {
		emRows = append(emRows, []string{e.From, e.To, e.Kind})
	}
	iiRows := make([][]string, 0, len(importEdges))
	for _, e := range importEdges {
		iiRows = append(iiRows, []string{e.File, e.To, e.Kind, strconv.FormatBool(e.Blank)})
	}
	geRows := make([][]string, 0, len(genEdges))
	for _, e := range genEdges {
		geRows = append(geRows, []string{e.File, e.Generator})
	}
	peRows := make([][]string, 0, len(parseErrs))
	for _, pe := range parseErrs {
		peRows = append(peRows, []string{pe[0], pe[1]})
	}

	writeTSV("edges_declares_type.tsv", []string{"file", "qn"}, dtRows)
	writeTSV("edges_declares_function.tsv", []string{"file", "qn"}, dfRows)
	writeTSV("edges_declares_constant.tsv", []string{"file", "qn"}, dcRows)
	writeTSV("edges_declares_variable.tsv", []string{"file", "qn"}, dvRows)
	writeTSV("edges_declares_method_file.tsv", []string{"file", "sig"}, dmfRows)
	writeTSV("edges_declares_method_type.tsv", []string{"typeQN", "sig"}, dmtRows)
	writeTSV("edges_declares_field.tsv", []string{"typeQN", "key"}, dflRows)
	writeTSV("edges_embeds.tsv", []string{"fromQN", "toQN", "toKind"}, emRows)
	writeTSV("edges_imports.tsv", []string{"file", "target", "targetKind", "blank"}, iiRows)
	writeTSV("edges_generated_from.tsv", []string{"file", "generator"}, geRows)
	writeTSV("parse_errors.tsv", []string{"path", "error"}, peRows)

	writePopulations(goVersion, parseErrs, extPkgs, extTypes, missingPkgs,
		importEdges, embeds, genEdges, containsFile)

	fmt.Println("kg-inventory: done; dumps in", outDir)
}

// ---------------------------------------------------------------------------
// walking, parsing, extraction
// ---------------------------------------------------------------------------

func walk(root string) ([]*FileRec, error) {
	var out []*FileRec
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if p != root && filepath.Base(p) == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		lang := languageOf(filepath.Base(rel))
		if lang == "" {
			return nil
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out = append(out, &FileRec{
			Path:      rel,
			Language:  lang,
			Lines:     countLines(src),
			Generated: isGenerated(src),
			src:       src,
			platform:  platformBuildTag(src),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func languageOf(base string) string {
	switch base {
	case "Makefile", "makefile":
		return "makefile"
	case "go.mod":
		return "gomod"
	}
	switch filepath.Ext(base) {
	case ".go":
		return "go"
	case ".java":
		return "java"
	case ".py":
		return "python"
	case ".sh":
		return "shell"
	case ".mk":
		return "makefile"
	}
	return ""
}

func countLines(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	n := bytes.Count(b, []byte{'\n'})
	if b[len(b)-1] != '\n' {
		n++
	}
	return n
}

func isGenerated(src []byte) bool {
	line := 0
	for _, l := range bytes.Split(src, []byte{'\n'}) {
		if line >= 10 {
			break
		}
		if strings.Contains(string(l), "Code generated by") && strings.Contains(string(l), "DO NOT EDIT") {
			return true
		}
		line++
	}
	return false
}

func platformBuildTag(src []byte) bool {
	for _, l := range bytes.Split(src, []byte{'\n'}) {
		s := strings.TrimSpace(string(l))
		if strings.HasPrefix(s, "//go:build") && buildTagRe.MatchString(s) {
			return true
		}
	}
	return false
}

// parseFile parses f and extracts its declarations. It returns "" on success
// and the one-line error on failure; the file is then kept with zero
// declarations, and its package clause is recovered by regex when possible
// (the declarations themselves are never guessed).
func parseFile(fset *token.FileSet, f *FileRec) string {
	astf, err := parser.ParseFile(fset, f.Path, f.src, parser.ParseComments)
	if err != nil {
		e := oneLine(err.Error())
		if m := pkgClause.FindSubmatch(f.src); m != nil {
			f.pkgName = string(m[1])
		}
		f.assignRole()
		return e
	}
	f.pkgName = astf.Name.Name
	f.assignRole()
	extract(f, astf, fset)
	return ""
}

func (f *FileRec) assignRole() {
	switch {
	case f.Language == "go":
		switch {
		case strings.HasSuffix(f.Path, "_test.go") || strings.HasPrefix(f.Path, "tests/"):
			f.Role = "test"
		case strings.Contains(f.Path, "testdata/"):
			// testdata directories hold test fixtures; the go tool
			// never builds them into production
			f.Role = "test"
		case strings.HasPrefix(f.Path, "examples/"):
			f.Role = "example"
		case f.pkgName == "main":
			f.Role = "tool"
		default:
			f.Role = "production"
		}
	case f.Language == "makefile" || f.Language == "gomod":
		f.Role = "build"
	default:
		f.Role = "tool"
	}
}

func extract(f *FileRec, astf *ast.File, fset *token.FileSet) {
	ip := f.importPath()
	src := f.src
	line := func(p token.Pos) int { return fset.Position(p).Line }
	text := func(e ast.Expr) string {
		if e == nil {
			return ""
		}
		a := fset.Position(e.Pos()).Offset
		b := fset.Position(e.End()).Offset
		return strings.TrimSpace(spaceRe.ReplaceAllString(string(src[a:b]), " "))
	}

	var typeOrd, funcOrd, methodOrd, fieldOrd, constOrd, varOrd int

	for _, decl := range astf.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				funcOrd++
				funcs = append(funcs, &FuncRec{
					File: f.Path, IP: ip, Nat: ip + "." + d.Name.Name,
					Name: d.Name.Name, Kind: funcKind(d.Name.Name),
					Line: line(d.Name.Pos()), FileOrd: funcOrd,
				})
				continue
			}
			methodOrd++
			recv := text(d.Recv.List[0].Type)
			recvPtr := strings.HasPrefix(recv, "*")
			base := strings.TrimSpace(strings.TrimPrefix(recv, "*"))
			param := false
			if i := strings.IndexByte(base, '['); i >= 0 {
				param = true
				base = strings.TrimSpace(base[:i])
			}
			methods = append(methods, &MethodRec{
				File: f.Path, IP: ip,
				NatSig:    ip + "." + base + "." + d.Name.Name,
				Name:      d.Name.Name,
				Receiver:  recv,
				RecvQN:    ip + "." + base,
				RecvPtr:   recvPtr,
				Line:      line(d.Name.Pos()),
				ParamRecv: param,
				FileOrd:   methodOrd,
			})

		case *ast.GenDecl:
			switch d.Tok {
			case token.TYPE:
				for _, s := range d.Specs {
					ts := s.(*ast.TypeSpec)
					typeOrd++
					t := &TypeRec{
						File: f.Path, IP: ip, Nat: ip + "." + ts.Name.Name,
						Name: ts.Name.Name, Line: line(ts.Name.Pos()), FileOrd: typeOrd,
					}
					switch rhs := ts.Type.(type) {
					case *ast.StructType:
						t.Kind = "struct"
						for _, fld := range rhs.Fields.List {
							typ := text(fld.Type)
							if len(fld.Names) == 0 {
								fieldOrd++
								fields = append(fields, &FieldRec{
									File: f.Path, Owner: t,
									Name: typ, Type: typ, Embedded: true,
									Line: line(fld.Type.Pos()), FileOrd: fieldOrd,
								})
							} else {
								for _, n := range fld.Names {
									fieldOrd++
									fields = append(fields, &FieldRec{
										File: f.Path, Owner: t,
										Name: n.Name, Type: typ,
										Line: line(n.Pos()), FileOrd: fieldOrd,
									})
								}
							}
						}
					case *ast.InterfaceType:
						t.Kind = "interface"
						for _, fld := range rhs.Methods.List {
							if _, ok := fld.Type.(*ast.FuncType); !ok {
								continue // embedded interface, not a method spec
							}
							if len(fld.Names) == 0 || fld.Names[0] == nil || fld.Names[0].Name == "" {
								continue // malformed spec
							}
							methodOrd++
							methods = append(methods, &MethodRec{
								File: f.Path, IP: ip,
								NatSig:      ip + "." + ts.Name.Name + "." + fld.Names[0].Name,
								Name:        fld.Names[0].Name,
								IsInterface: true,
								IfaceQN:     ip + "." + ts.Name.Name,
								Line:        line(fld.Names[0].Pos()),
								FileOrd:     methodOrd,
							})
						}
					default:
						if ts.Assign.IsValid() {
							t.Kind = "alias"
						} else {
							t.Kind = "defined"
							t.DefinedFrom = text(ts.Type)
						}
					}
					if ts.TypeParams != nil {
						t.TypeParams = len(ts.TypeParams.List)
					}
					types = append(types, t)
				}

			case token.CONST:
				for _, s := range d.Specs {
					vs := s.(*ast.ValueSpec)
					typ := text(vs.Type)
					for _, n := range vs.Names {
						constOrd++
						consts = append(consts, &CVRec{
							File: f.Path, IP: ip, Nat: ip + "." + n.Name,
							Name: n.Name, Type: typ, Line: line(n.Pos()), FileOrd: constOrd,
						})
					}
				}

			case token.VAR:
				for _, s := range d.Specs {
					vs := s.(*ast.ValueSpec)
					typ := text(vs.Type)
					for _, n := range vs.Names {
						varOrd++
						vars = append(vars, &CVRec{
							File: f.Path, IP: ip, Nat: ip + "." + n.Name,
							Name: n.Name, Type: typ, Line: line(n.Pos()), FileOrd: varOrd,
						})
					}
				}

			case token.IMPORT:
				for _, s := range d.Specs {
					im := s.(*ast.ImportSpec)
					name := ""
					if im.Name != nil {
						name = im.Name.Name
					}
					imps = append(imps, ImpRec{
						File:  f.Path,
						Path:  strings.Trim(im.Path.Value, `"`),
						Blank: name == "_",
						Dot:   name == ".",
					})
				}
			}
		}
	}
}

// importPath computes the file's declared-package importPath from the module
// path and the file's own directory and package name.
func (f *FileRec) importPath() string {
	if f.pkgName == "" {
		return ""
	}
	dir := dirOf(f.Path)
	ip := modulePath
	if dir != "." {
		ip = modulePath + "/" + dir
	}
	if strings.HasSuffix(f.pkgName, "_test") {
		ip += "_test"
	}
	return ip
}

func funcKind(name string) string {
	switch {
	case name == "init":
		return "init"
	case strings.HasPrefix(name, "Test"):
		return "test"
	case strings.HasPrefix(name, "Benchmark"):
		return "benchmark"
	case strings.HasPrefix(name, "Fuzz"):
		return "fuzz"
	case strings.HasPrefix(name, "Example"):
		return "example"
	default:
		return "function"
	}
}

// ---------------------------------------------------------------------------
// packages
// ---------------------------------------------------------------------------

type pkgKey struct{ dir, name string }

var pkgIndex = map[pkgKey]*PkgRec{}

func buildPackages() {
	nonTest := map[pkgKey]int{}
	for _, f := range files {
		if f.Language != "go" || f.pkgName == "" {
			continue
		}
		k := pkgKey{dirOf(f.Path), f.pkgName}
		if _, ok := pkgIndex[k]; !ok {
			p := &PkgRec{
				ImportPath: f.importPath(),
				Name:       k.name,
				Dir:        k.dir,
				Depth:      strings.Count(k.dir, "/"),
			}
			p.Test = strings.HasSuffix(p.ImportPath, "_test")
			pkgIndex[k] = p
			pkgs = append(pkgs, p)
		}
		if f.Role != "test" {
			nonTest[k]++
		}
	}
	for k, p := range pkgIndex {
		p.TestOnly = nonTest[k] == 0
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ImportPath < pkgs[j].ImportPath })
}

func dirOf(path string) string {
	d := filepath.ToSlash(filepath.Dir(path))
	if d == "." {
		return "."
	}
	return d
}

// ---------------------------------------------------------------------------
// identity disambiguation
// ---------------------------------------------------------------------------

// assignFinals applies the #<file>#<n> disambiguator to every record whose
// natural key has more than one record in the whole tree, plus every record
// for which forced is true (init functions, always suffixed).
func assignFinals[T any](
	recs []T,
	nat func(T) string,
	file func(T) string,
	ord func(T) int,
	set func(T, string),
	forced func(T) bool,
) {
	group := map[string][]T{}
	var order []string
	for _, r := range recs {
		k := nat(r)
		if _, ok := group[k]; !ok {
			order = append(order, k)
		}
		group[k] = append(group[k], r)
	}
	for _, k := range order {
		recs := group[k]
		suffixed := len(recs) > 1
		for _, r := range recs {
			if suffixed || forced(r) {
				set(r, k+"#"+file(r)+"#"+strconv.Itoa(ord(r)))
			} else {
				set(r, k)
			}
		}
	}
}

func checkUnique() {
	seen := map[string]bool{}
	bad := func(label, id string) {
		k := label + "\x00" + id
		if seen[k] {
			panic("duplicate identity in " + label + ": " + id)
		}
		seen[k] = true
	}
	for _, t := range types {
		bad("GoceneType", t.QN)
	}
	for _, f := range funcs {
		bad("GoceneFunction", f.QN)
	}
	for _, m := range methods {
		bad("GoceneMethod", m.Sig)
	}
	for _, f := range fields {
		bad("GoceneField", f.Key)
	}
	for _, c := range consts {
		bad("GoceneConstant", c.QN)
	}
	for _, v := range vars {
		bad("GoceneVariable", v.QN)
	}
	for _, p := range pkgs {
		bad("GocenePackage", p.ImportPath)
	}
	for _, f := range files {
		bad("GoceneFile", f.Path)
	}
}

func finalOf(cands []*TypeRec) []string {
	out := make([]string, 0, len(cands))
	for _, c := range cands {
		out = append(out, c.QN)
	}
	return out
}

// ---------------------------------------------------------------------------
// row builders
// ---------------------------------------------------------------------------

func pkgRows() [][]string {
	rows := make([][]string, 0, len(pkgs))
	for _, p := range pkgs {
		rows = append(rows, []string{p.ImportPath, p.Name, p.Dir, strconv.Itoa(p.Depth),
			strconv.FormatBool(p.Test), strconv.FormatBool(p.TestOnly)})
	}
	return rows
}

func fileRows() [][]string {
	rows := make([][]string, 0, len(files))
	for _, f := range files {
		rows = append(rows, []string{f.Path, f.Language, f.Role, f.ImportPath,
			strconv.FormatBool(f.Generated), f.Generator, strconv.Itoa(f.Lines)})
	}
	return rows
}

func typeRows() [][]string {
	rows := make([][]string, 0, len(types))
	for _, t := range types {
		tp := ""
		if t.TypeParams > 0 {
			tp = strconv.Itoa(t.TypeParams)
		}
		rows = append(rows, []string{t.QN, t.Name, t.Kind, t.DefinedFrom, tp, strconv.Itoa(t.Line)})
	}
	return rows
}

func funcRows() [][]string {
	rows := make([][]string, 0, len(funcs))
	for _, f := range funcs {
		rows = append(rows, []string{f.QN, f.Name, f.Kind, strconv.Itoa(f.Line)})
	}
	return rows
}

func methodRows() [][]string {
	rows := make([][]string, 0, len(methods))
	for _, m := range methods {
		recvQN := ""
		if !m.IsInterface && !m.Defect {
			recvQN = m.RecvQN
		}
		rows = append(rows, []string{m.Sig, m.Name, m.Receiver, recvQN,
			strconv.FormatBool(m.RecvPtr), strconv.FormatBool(m.IsInterface), m.IfaceQN, strconv.Itoa(m.Line)})
	}
	return rows
}

func fieldRows() [][]string {
	rows := make([][]string, 0, len(fields))
	for _, f := range fields {
		rows = append(rows, []string{f.Key, f.Name, f.Type,
			strconv.FormatBool(f.Embedded), strconv.Itoa(f.Line)})
	}
	return rows
}

func cvRows(recs []*CVRec) [][]string {
	rows := make([][]string, 0, len(recs))
	for _, c := range recs {
		rows = append(rows, []string{c.QN, c.Name, c.Type, strconv.Itoa(c.Line)})
	}
	return rows
}

// ---------------------------------------------------------------------------
// output
// ---------------------------------------------------------------------------

func writeTSV(name string, header []string, rows [][]string) {
	sort.SliceStable(rows, func(i, j int) bool {
		for c := 0; c < len(rows[i]); c++ {
			if rows[i][c] != rows[j][c] {
				return rows[i][c] < rows[j][c]
			}
		}
		return false
	})
	var b bytes.Buffer
	b.WriteString(strings.Join(header, "\t") + "\n")
	for _, r := range rows {
		for i, c := range r {
			if i > 0 {
				b.WriteByte('\t')
			}
			b.WriteString(sanitize(c))
		}
		b.WriteByte('\n')
	}
	must(os.WriteFile(filepath.Join(outDir, name), b.Bytes(), 0o644))
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func oneLine(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

func readGoMod(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	modulePath, goVersion := "", ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if i := strings.Index(line, " //"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if modulePath == "" && strings.HasPrefix(line, "module ") {
			modulePath = strings.TrimSpace(strings.TrimPrefix(line, "module"))
		} else if goVersion == "" && strings.HasPrefix(line, "go ") {
			goVersion = strings.TrimSpace(strings.TrimPrefix(line, "go"))
		}
		if modulePath != "" && goVersion != "" {
			break
		}
	}
	if modulePath == "" || goVersion == "" {
		return "", "", fmt.Errorf("go.mod: module or go directive not found")
	}
	return modulePath, goVersion, nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// ---------------------------------------------------------------------------
// populations.json
// ---------------------------------------------------------------------------

type collisionStats struct {
	Groups        int `json:"groups"`
	Records       int `json:"records"`
	BuildTagPairs int `json:"buildTagPairs"`
}

func writePopulations(goVersion string, parseErrs [][2]string,
	extPkgs map[string]bool, extTypes map[string]string, missingPkgs map[string]bool,
	importEdges []ImportEdge, embeds []EmbedEdge, genEdges []GenEdge,
	containsFile []ContainsFileEdge) {

	byLang := map[string]int{}
	byRole := map[string]int{}
	for _, f := range files {
		byLang[f.Language]++
		byRole[f.Role]++
	}
	var genList []string
	for _, f := range files {
		if f.Generated {
			genList = append(genList, f.Path)
		}
	}

	distinct := func(names []string) int {
		s := map[string]bool{}
		for _, n := range names {
			s[n] = true
		}
		return len(s)
	}
	typeNames := make([]string, 0, len(types))
	funcNames := make([]string, 0, len(funcs))
	methodNames := make([]string, 0, len(methods))
	fieldNames := make([]string, 0, len(fields))
	constNames := make([]string, 0, len(consts))
	varNames := make([]string, 0, len(vars))
	pkgNameList := make([]string, 0, len(pkgs))
	for _, t := range types {
		typeNames = append(typeNames, t.Name)
	}
	for _, f := range funcs {
		funcNames = append(funcNames, f.Name)
	}
	for _, m := range methods {
		methodNames = append(methodNames, m.Name)
	}
	for _, f := range fields {
		fieldNames = append(fieldNames, f.Name)
	}
	for _, c := range consts {
		constNames = append(constNames, c.Name)
	}
	for _, v := range vars {
		varNames = append(varNames, v.Name)
	}
	for _, p := range pkgs {
		pkgNameList = append(pkgNameList, p.Name)
	}

	byKind := map[string]int{}
	withDefFrom, withTypeParams := 0, 0
	for _, t := range types {
		byKind[t.Kind]++
		if t.DefinedFrom != "" {
			withDefFrom++
		}
		if t.TypeParams > 0 {
			withTypeParams++
		}
	}
	funcKinds := map[string]int{}
	initByFile := map[string]int{}
	for _, f := range funcs {
		funcKinds[f.Kind]++
		if f.Kind == "init" {
			initByFile[f.File]++
		}
	}
	initCount := 0
	for _, n := range initByFile {
		initCount += n
	}
	multiInit := []map[string]any{}
	for file, n := range initByFile {
		if n >= 2 {
			multiInit = append(multiInit, map[string]any{"file": file, "inits": n})
		}
	}
	sort.Slice(multiInit, func(i, j int) bool {
		return multiInit[i]["file"].(string) < multiInit[j]["file"].(string)
	})

	concrete, iface, recvPtr, paramRecv := 0, 0, 0, 0
	defectRows := []map[string]any{}
	for _, m := range methods {
		if m.IsInterface {
			iface++
			continue
		}
		concrete++
		if m.RecvPtr {
			recvPtr++
		}
		if m.ParamRecv {
			paramRecv++
		}
		if m.Defect {
			defectRows = append(defectRows, map[string]any{
				"file": m.File, "line": m.Line, "name": m.Name,
				"receiver": m.Receiver, "recvQN": m.RecvQN,
			})
		}
	}
	sort.Slice(defectRows, func(i, j int) bool {
		if defectRows[i]["file"] != defectRows[j]["file"] {
			return defectRows[i]["file"].(string) < defectRows[j]["file"].(string)
		}
		return defectRows[i]["line"].(int) < defectRows[j]["line"].(int)
	})

	embedded := 0
	for _, f := range fields {
		if f.Embedded {
			embedded++
		}
	}
	constTyped, varTyped, blankVars := 0, 0, 0
	for _, c := range consts {
		if c.Type != "" {
			constTyped++
		}
	}
	for _, v := range vars {
		if v.Type != "" {
			varTyped++
		}
		if v.Name == "_" {
			blankVars++
		}
	}
	blankVarGroups := 0
	{
		g := map[string]int{}
		for _, v := range vars {
			if v.Name == "_" {
				g[v.Nat]++
			}
		}
		for _, n := range g {
			if n > 1 {
				blankVarGroups++
			}
		}
	}

	extPkgList := make([]string, 0, len(extPkgs))
	for p := range extPkgs {
		extPkgList = append(extPkgList, p)
	}
	sort.Strings(extPkgList)
	stdlib := 0
	nonStdlib := []string{}
	for _, p := range extPkgList {
		first := p
		if i := strings.IndexByte(p, '/'); i >= 0 {
			first = p[:i]
		}
		if strings.Contains(first, ".") {
			nonStdlib = append(nonStdlib, p)
		} else {
			stdlib++
		}
	}

	extTypeTexts := make([]string, 0, len(extTypes))
	extTypeOrigin := map[string]int{}
	for q, o := range extTypes {
		extTypeTexts = append(extTypeTexts, q)
		extTypeOrigin[o]++
	}
	sort.Strings(extTypeTexts)

	missingList := make([]string, 0, len(missingPkgs))
	for m := range missingPkgs {
		missingList = append(missingList, m)
	}
	sort.Strings(missingList)

	testCount, testOnlyCount := 0, 0
	testOnlyList := []string{}
	for _, p := range pkgs {
		if p.Test {
			testCount++
		}
		if p.TestOnly {
			testOnlyCount++
			testOnlyList = append(testOnlyList, p.ImportPath)
		}
	}

	impByKind := map[string]int{}
	blankImp, dotImp := 0, 0
	dotList := []map[string]any{}
	for _, e := range importEdges {
		impByKind[e.Kind]++
		if e.Blank {
			blankImp++
		}
		if e.Dot {
			dotImp++
			dotList = append(dotList, map[string]any{"file": e.File, "path": e.To})
		}
	}
	embedByKind := map[string]int{}
	for _, e := range embeds {
		embedByKind[e.Kind]++
	}
	cfPackage, cfModule := 0, 0
	for _, e := range containsFile {
		if e.From == "." {
			cfModule++
		} else {
			cfPackage++
		}
	}
	declaresMethodType := 0
	for _, m := range methods {
		declaresMethodType += len(m.OwnerList)
	}

	// ---- collision statistics, per label ----
	type natRef struct {
		nat  string
		file string
		ord  int
	}
	collisions := map[string]collisionStats{}
	topGroups := map[string][]map[string]any{}
	platformByPath := map[string]bool{}
	for _, f := range files {
		platformByPath[f.Path] = f.platform
	}
	doCollisions := func(label string, refs []natRef) {
		g := map[string][]natRef{}
		var order []string
		for _, r := range refs {
			if _, ok := g[r.nat]; !ok {
				order = append(order, r.nat)
			}
			g[r.nat] = append(g[r.nat], r)
		}
		c := collisionStats{}
		type grp struct {
			key  string
			size int
			fl   []string
			pair bool
		}
		var grps []grp
		for _, k := range order {
			recs := g[k]
			if len(recs) < 2 {
				continue
			}
			c.Groups++
			c.Records += len(recs)
			fset := map[string]bool{}
			for _, r := range recs {
				fset[r.file] = true
			}
			fl := make([]string, 0, len(fset))
			for f := range fset {
				fl = append(fl, f)
			}
			sort.Strings(fl)
			pair := len(fl) == 2 && platformByPath[fl[0]] && platformByPath[fl[1]]
			if pair {
				c.BuildTagPairs++
			}
			grps = append(grps, grp{k, len(recs), fl, pair})
		}
		collisions[label] = c
		sort.Slice(grps, func(i, j int) bool {
			if grps[i].size != grps[j].size {
				return grps[i].size > grps[j].size
			}
			return grps[i].key < grps[j].key
		})
		if len(grps) > 10 {
			grps = grps[:10]
		}
		var topRows []map[string]any
		for _, gr := range grps {
			topRows = append(topRows, map[string]any{
				"key":          gr.key,
				"size":         gr.size,
				"files":        gr.fl,
				"buildTagPair": gr.pair,
			})
		}
		topGroups[label] = topRows
	}
	doCollisions("GocenePackage", func() []natRef {
		out := make([]natRef, 0, len(pkgs))
		for _, p := range pkgs {
			out = append(out, natRef{nat: p.ImportPath, file: p.Dir})
		}
		return out
	}())
	doCollisions("GoceneFile", func() []natRef {
		out := make([]natRef, 0, len(files))
		for _, f := range files {
			out = append(out, natRef{nat: f.Path, file: f.Path})
		}
		return out
	}())
	doCollisions("GoceneType", func() []natRef {
		out := make([]natRef, 0, len(types))
		for _, t := range types {
			out = append(out, natRef{nat: t.Nat, file: t.File, ord: t.FileOrd})
		}
		return out
	}())
	doCollisions("GoceneFunction", func() []natRef {
		out := make([]natRef, 0, len(funcs))
		for _, f := range funcs {
			out = append(out, natRef{nat: f.Nat, file: f.File, ord: f.FileOrd})
		}
		return out
	}())
	doCollisions("GoceneMethod", func() []natRef {
		out := make([]natRef, 0, len(methods))
		for _, m := range methods {
			out = append(out, natRef{nat: m.NatSig, file: m.File, ord: m.FileOrd})
		}
		return out
	}())
	doCollisions("GoceneField", func() []natRef {
		out := make([]natRef, 0, len(fields))
		for _, f := range fields {
			out = append(out, natRef{nat: f.NatKey, file: f.File, ord: f.FileOrd})
		}
		return out
	}())
	doCollisions("GoceneConstant", func() []natRef {
		out := make([]natRef, 0, len(consts))
		for _, c := range consts {
			out = append(out, natRef{nat: c.Nat, file: c.File, ord: c.FileOrd})
		}
		return out
	}())
	doCollisions("GoceneVariable", func() []natRef {
		out := make([]natRef, 0, len(vars))
		for _, v := range vars {
			out = append(out, natRef{nat: v.Nat, file: v.File, ord: v.FileOrd})
		}
		return out
	}())

	goFileTotal, nonGoResolving := 0, 0
	for _, f := range files {
		if f.Language == "go" {
			goFileTotal++
		} else if f.ImportPath != "" {
			nonGoResolving++
		}
	}

	parseErrRows := []map[string]any{}
	for _, pe := range parseErrs {
		parseErrRows = append(parseErrRows, map[string]any{"file": pe[0], "error": pe[1]})
	}
	genEdgeRows := []map[string]any{}
	for _, e := range genEdges {
		genEdgeRows = append(genEdgeRows, map[string]any{
			"file": e.File, "generator": e.Generator, "self": e.Self,
		})
	}
	genFileList := []string{}
	for _, f := range files {
		if f.Generated {
			genFileList = append(genFileList, f.Path)
		}
	}

	doc := map[string]any{
		"module": map[string]any{"modulePath": modulePath, "goVersion": goVersion},
		"labels": map[string]int{
			"GoceneModule":          1,
			"GocenePackage":         len(pkgs),
			"GoceneFile":            len(files),
			"GoceneType":            len(types),
			"GoceneFunction":        len(funcs),
			"GoceneMethod":          len(methods),
			"GoceneField":           len(fields),
			"GoceneConstant":        len(consts),
			"GoceneVariable":        len(vars),
			"GoceneExternalPackage": len(extPkgs),
			"GoceneExternalType":    len(extTypes),
			"GoceneMissingPackage":  len(missingPkgs),
		},
		"GocenePackage": map[string]any{
			"total":         len(pkgs),
			"distinctNames": distinct(pkgNameList),
			"production":    len(pkgs) - testCount,
			"test":          testCount,
			"testOnly":      testOnlyCount,
			"testOnlyList":  testOnlyList,
		},
		"GoceneFile": map[string]any{
			"total":                             len(files),
			"byLanguage":                        byLang,
			"byRole":                            byRole,
			"generated":                         len(genList),
			"generatedList":                     genList,
			"nonGoResolvingToProductionPackage": nonGoResolving,
		},
		"GoceneType": map[string]any{
			"total":           len(types),
			"distinctNames":   distinct(typeNames),
			"byKind":          byKind,
			"withDefinedFrom": withDefFrom,
			"withTypeParams":  withTypeParams,
		},
		"GoceneFunction": map[string]any{
			"total":              len(funcs),
			"distinctNames":      distinct(funcNames),
			"byKind":             funcKinds,
			"initCount":          initCount,
			"filesWith2PlusInit": multiInit,
		},
		"GoceneMethod": map[string]any{
			"total":                 len(methods),
			"distinctNames":         distinct(methodNames),
			"concrete":              concrete,
			"interfaceSpec":         iface,
			"recvPtr":               recvPtr,
			"parameterizedReceiver": paramRecv,
			"unresolvedRecvQN":      len(defectRows),
			"defectMethods":         defectRows,
		},
		"GoceneField": map[string]any{
			"total":         len(fields),
			"distinctNames": distinct(fieldNames),
			"embedded":      embedded,
		},
		"GoceneConstant": map[string]any{
			"total":         len(consts),
			"distinctNames": distinct(constNames),
			"typed":         constTyped,
		},
		"GoceneVariable": map[string]any{
			"total":                 len(vars),
			"distinctNames":         distinct(varNames),
			"typed":                 varTyped,
			"blankUnderscore":       blankVars,
			"blankUnderscoreGroups": blankVarGroups,
		},
		"GoceneExternalPackage": map[string]any{
			"total":     len(extPkgs),
			"stdlib":    stdlib,
			"nonStdlib": nonStdlib,
		},
		"GoceneExternalType": map[string]any{
			"total":                  len(extTypes),
			"distinctReferenceTexts": len(extTypeTexts),
			"byOrigin":               extTypeOrigin,
		},
		"GoceneMissingPackage": map[string]any{
			"total": len(missingPkgs),
			"list":  missingList,
		},
		"edges": map[string]any{
			"CONTAINS_PACKAGE":      len(pkgs),
			"CONTAINS_FILE":         len(containsFile),
			"CONTAINS_FILE_package": cfPackage,
			"CONTAINS_FILE_module":  cfModule,
			"DECLARES_TYPE":         len(types),
			"DECLARES_FUNCTION":     len(funcs),
			"DECLARES_CONSTANT":     len(consts),
			"DECLARES_VARIABLE":     len(vars),
			"DECLARES_METHOD_file":  len(methods),
			"DECLARES_METHOD_type":  declaresMethodType,
			"DECLARES_FIELD":        len(fields),
			"EMBEDS":                len(embeds),
			"EMBEDS_type":           embedByKind["type"],
			"EMBEDS_external":       embedByKind["external"],
			"IMPORTS":               len(importEdges),
			"IMPORTS_package":       impByKind["package"],
			"IMPORTS_external":      impByKind["external"],
			"IMPORTS_missing":       impByKind["missing"],
			"IMPORTS_blank":         blankImp,
			"IMPORTS_dot":           dotImp,
			"GENERATED_FROM":        len(genEdges),
		},
		"collisions":         collisions,
		"topCollisionGroups": topGroups,
		"dotImports":         map[string]any{"count": dotImp, "list": dotList},
		"parseErrors":        map[string]any{"goFilesTotal": goFileTotal, "errors": parseErrRows},
		"generatedFiles":     genFileList,
		"generatedFrom":      genEdgeRows,
	}

	b, err := json.MarshalIndent(doc, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(outDir, "populations.json"), append(b, '\n'), 0o644))
}
