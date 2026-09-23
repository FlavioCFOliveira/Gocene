# Knowledge Model — Gocene

Canonical description of the **shape** of the Label-Property Graph held in the
`gocene` roadmap and reached exclusively through `rmp graph`. This file carries the
label dictionary, the predicate dictionary, the constraints and the index
recommendations — and nothing else. **The nodes, the edges and the hierarchy itself
live in the graph**, not here; this document exists so a reader can compose a
correct statement without exploring the graph first.

- **Roadmap:** `gocene` (resolved from `./.rmp`, key `roadmap`).
- **Engine:** `rmp` 1.17.0 — the graph is a **server/client pair**. `rmp graph serve`
  is the only process that opens the store; `rmp graph client` is the only way to run
  a statement, and it needs a live server for **every** one.
- **Socket:** `~/.roadmaps/gocene/graph.sock` (the derived default — `.rmp` declares
  no `socket` key, so both ends must omit `--socket`).
- **Lucene reference:** Apache Lucene 10.5.0 at `/tmp/lucene`, tag
  `releases/lucene/10.5.0`, commit `f6eaee8148b7569e83c433feacc4f624608188fd`
  (2026-06-19).
- **Live graph, verified 2026-09-23 (module tier synchronised to commit
  `d9992805`):** **210 924 nodes, 334 895 edges, 29 labels defined (28 populated —
  `GoceneMissingPackage` is empty), 36 predicates, 67 property keys (59 node, 11
  edge), 29 constraints, 31 indexes.**

## Scope

The graph holds the **Lucene tier**: the organisational structure of Apache Lucene
10.5.0 and its hierarchy, covering all code in the reference repository —
production, tests, the test framework, demos, benchmarks, the Luke application,
build tooling and code generators. All 112 068 of its nodes are loaded; 39 of its
47 edge pairs are live in the graph (the 8 absent pairs, § 6).

The **Gocene tier** — the organisational structure of the Gocene Go module and its
symbols — is **defined and materialised**: 12 labels, 11 predicates over 16
endpoint pairs (15 populated), first loaded from the working tree at commit
`dd61538c` (2026-09-10) and **resynchronised on 2026-09-23 to a `git archive`
snapshot of commit `f008c200`**, then **synchronised incrementally the same day to
`git archive` snapshots of commits `0d52b534`, `a4769f4d`, `4d6bf725` and `d9992805`** — the post-sync census and the
two-direction diff find 0 differences (§ 6). `PORTED_TO` is **populated** over 10
endpoint pairs: 23 995 edges, derived from measured evidence (§ 7, The PORTED_TO derivation) and verified by
a write-time counter audit and the anchor audit. The
Lucene tier is shaped so the Gocene tier attaches without reshaping it: every
element on both sides carries a stable single-STRING identity that `PORTED_TO` can
target at any granularity — release, module, package, file, class, method, field,
constant.

The **Population** column in each dictionary is the verified count. For the Gocene
tier it is the live graph count, confirmed by the post-resync census and the
two-direction identity and property diff against the inventory of the `d9992805`
snapshot (§ 6, § 7). For the Lucene
tier it is the reference-tree count, verified label by label and predicate by
predicate against `/tmp/lucene` (§ 7); the live edge state — 39 of 47 pairs — is
recorded in § 6. For `PORTED_TO` it is the live edge count of a pair, equal to the
derivation's row count for that pair — verified by the per-pair audit (§ 7).

---

## 1. Label dictionary

Every identity property is a **STRING**. Type is part of the contract:
`{binaryName: 'x'}` and `{binaryName: 42}` bind different nodes.

### `LuceneRelease`
The reference release as a whole; the anchor of the Lucene tier.
- **Identity:** `version` — `"10.5.0"`. **Population: 1.**
- `tag`, `commit`, `commitDate` STRING — the git tag, the hash it resolves to, its date.

### `LuceneModule`
A build unit of the Lucene Gradle build: the root project, every subproject declared
in `settings.gradle`, and every included build. The build's own project list is the
authority — a directory that merely holds code is not a module.
- **Identity:** `path` — repo-relative; `.` is the root project. **Population: 43**
  (1 root + 40 subprojects + 2 included builds).
- `name` STRING — last path segment.
- `gradlePath` STRING — e.g. `lucene:analysis:common`; `:` for the root.
- `isGradleSubproject` BOOLEAN.
- `role` STRING — `root` | `library` | `test-support` | `application` | `benchmark` |
  `example` | `build-tooling`.

### `LuceneSourceSet`
One source root of a module — the compilation and packaging scope a file belongs to.
A separate node, not a property: the same package name exists in more than one source
set, so the source set is part of a package's identity.
- **Identity:** `key` — `<module.path>|<name>`. **Population: 112** (79 hold Java).
- `name` STRING — `java` | `test` | `java21` | `main` | `tools` | `resources` |
  `generated` | `test-files` | `data` | `markdown` | `assets` | `distribution` |
  `binary-release`.
- `kind` STRING — `production` | `test` | `tool` | `resource` | `generated`.
- `jdkFloor` INTEGER — 21 for `src/java21`, null otherwise.

### `LuceneJpmsModule`
A Java Platform Module System declaration (`module-info.java`). It carries Lucene's
declared API surface and service wiring, which the port must reproduce.
- **Identity:** `name` — e.g. `org.apache.lucene.core`. **Population: 37.**
- `open` BOOLEAN — declared `open module`.
- `file` STRING — repo-relative path of its `module-info.java`.

### `LucenePackage`
A Java package **within one module and one source set**.
- **Identity:** `key` — `<module.path>|<sourceSet.name>|<name>`. **Population: 598**
  (347 distinct package names).
- `name` STRING, `depth` INTEGER — dotted name and its segment count.
- `hasPackageInfo` BOOLEAN — a `package-info.java` is present.

### `LuceneFile`
A code file of any language. Non-Java code is in scope: build logic, generators,
grammars and benchmark scripts are code.
- **Identity:** `path` — repo-relative. **Population: 6183.**
- `language` STRING — `java` (5976) | `gradle` (118) | `alg` (34) | `python` (30) |
  `jflex` (7) | `perl` (7) | `shell` (5) | `javacc` (3) | `groovy` (2) | `antlr` (1).
- `role` STRING — `production` | `test` | `test-support` | `example` | `benchmark` |
  `tool` | `build` | `generator` | `package-info` | `module-info`.
- `generated` BOOLEAN — carries a generated-source header (66 files).
- `lines` INTEGER.

### `LuceneClass`
Any declared reference type, **including anonymous and local classes**. One label
rather than five: all kinds share every containment and inheritance relation, so the
kind is a property. Anonymous classes are in scope because each becomes a named Go
type when ported.
- **Identity:** `binaryName` — `pkg.Outer$Inner` for named types,
  `pkg.TopLevel$<n>` for anonymous, `pkg.TopLevel$<n>Name` for local.
  **Population: 11 601.**
- `simpleName` STRING — null for anonymous types (8736 named types, 88.2 % selective).
- `kind` STRING — `class` (7981) | `anonymous` (2865) | `interface` (359) |
  `record` (190) | `enum` (186) | `annotation` (20).
- `isAnonymous`, `isLocal`, `isTopLevel` BOOLEAN — 2865 / 32 / 5622.
- `ordinal` INTEGER — source-encounter order within the top-level type; set only for
  anonymous and local types, where it is part of `binaryName`.
- `modifiers` STRING — comma-joined, e.g. `public,final`.
- `nestingDepth` INTEGER — `$` count in `binaryName`, 0…3.
- `line` INTEGER.

### `LuceneMethod`
A method **or** a constructor. They are the same kind of thing to the port —
callable members with a parameter list — and share every relation.
- **Identity:** `signature` — `binaryName#name(T1,T2,…)`, erased source-level
  parameter types; the name is the literal `<init>` for a constructor.
  **Population: 63 624** (57 003 methods, 6621 constructors).
- `name` STRING — **not an identity**: 35.4 % selective.
- `kind` STRING — `method` | `constructor`.
- `returnType` STRING — erased; null for constructors.
- `paramTypes` STRING — comma-joined; `[]` suffix marks a varargs parameter.
- `arity` INTEGER — 0…18.
- `modifiers` STRING.
- `hasBody` BOOLEAN — false for the 2181 abstract or bodiless interface methods.
- `line` INTEGER.

### `LuceneField`
A field or a constant.
- **Identity:** `key` — `binaryName#name`. **Population: 26 887.**
- `name` STRING — **not an identity**: 38.3 % selective.
- `type` STRING — declared type, source-level.
- `modifiers` STRING.
- `isConstant` BOOLEAN — `static final`; true for 5811.
- `line` INTEGER.

### `LuceneEnumConstant`
- **Identity:** `key` — `binaryName#NAME`. **Population: 698.**
- `name` STRING.
- `ordinal` INTEGER — declaration order. This is the order Lucene serializes, so the
  port must preserve it; it is data, not presentation.

### `LuceneRecordComponent`
- **Identity:** `key` — `binaryName#name`. **Population: 607.**
- `name`, `type` STRING; `position` INTEGER — declaration order.

### `LuceneTypeParameter`
A generic parameter of a class or of a method.
- **Identity:** `key` — `<owner><T>`, the owner being a `LuceneClass.binaryName` or a
  `LuceneMethod.signature`. **Population: 503** (263 class-level, 240 method-level).
- `name` STRING, `position` INTEGER.

### `LuceneInitializer`
A static or instance initializer block. It carries executable semantics the port must
reproduce, and it has no other node to hang from.
- **Identity:** `key` — `binaryName#<clinit>/n` (static) or `binaryName#<init>/n`.
  **Population: 312.**
- `static` BOOLEAN, `ordinal` INTEGER, `line` INTEGER.

### `LuceneSpiService`
A service interface resolved at runtime through `META-INF/services`. SPI names are a
binary-contract surface, not a convenience: they are the strings Lucene resolves.
- **Identity:** `fqn` — e.g. `org.apache.lucene.codecs.PostingsFormat`.
  **Population: 8** (245 provider registrations across 41 service files).

### `ExternalType`
A type referenced by Lucene but declared outside it (JDK, ICU4J, OpenNLP, Morfologik,
spatial4j, ANTLR runtime, JMH, JUnit…). It exists so that `EXTENDS`, `IMPLEMENTS`,
`EXTENDS_ANONYMOUSLY`, `ANNOTATED_WITH`, `THROWS`, `IMPORTS` and `PROVIDES_IMPL`
never dangle.
- **Identity:** `name` — fully qualified wherever the source allows it to be
  determined (from the reference itself, an explicit import, `java.lang`, or a lone
  on-demand import). **Population: 811.**
- `origin` STRING — `jdk` (530) | `third-party` (269) | `unresolved` (12). The 12
  are genuinely undeterminable from source alone: a file with four wildcard imports
  of ANTLR packages, spatial4j `ShapeFactory` builders, and one static-imported
  constant.

### `ExternalJpmsModule`
A JPMS module required by Lucene but not declared in it.
- **Identity:** `name` — e.g. `java.desktop`, `com.ibm.icu`. **Population: 22.**

### `ExternalPackage`
A package named by an on-demand (`import x.y.*`) import that is not a Lucene package.
- **Identity:** `name` — e.g. `org.openjdk.jmh.annotations`. **Population: 21.**

### Gocene tier

The Gocene tier mirrors the organisational structure of the Go module
`github.com/FlavioCFOliveira/Gocene` — the port itself — measured on a `git archive`
snapshot of commit `d9992805` (2026-09-23). It is **loaded and verified**: every
population below is a live graph count, confirmed by the post-resync census and the
two-direction identity and property diff against the inventory (§ 6, § 7).

**Identity rule (all Gocene tier labels).** The natural identity is
`<importPath>.Name` — or `path`, `importPath`, `key`, `sig` where the label says so.
**Where the natural key has more than one record in the whole tree, every record of
the group receives the disambiguator `#<file>#<n>`** (`n` = ordinal of the
declaration within the file): this makes the identity unique — which is what the § 3
`UNIQUE` constraints enforce — while the redeclared name stays visible through the
`name` property and the containment edge to the file. Measured collision groups that
require the disambiguator (groups / records, redeclarations + build-tag groups):
`GoceneType` 3 / 6 (0 + 3), `GoceneFunction` 4 / 8 (1 + 3, `init` excluded),
`GoceneMethod` 6 / 12 (0 + 6), `GoceneField` 14 / 24 (2 + 12), `GoceneConstant` 1 / 2
(1 + 0), `GoceneVariable` 121 / 1785 (120 blank-`_` groups + 1 build-tag pair);
`GocenePackage` and `GoceneFile`: 0. The build-tag groups are the platform pairs
(`*_unix.go` / `*_windows.go` / `*_posix.go`); the redeclaration groups are genuine
repository defects, represented faithfully. `init` functions **always** carry the
`#<file>#<n>` form — Go permits several `func init()` per file (measured: 84 inits
in 82 files, 2 files hold 2) — so their identity is `<importPath>.init#<file>#<n>`.

### `GoceneModule`
The Go module as a whole; the anchor of the Gocene tier.
- **Identity:** `modulePath` — `github.com/FlavioCFOliveira/Gocene`. **Population: 1.**
- `goVersion` STRING — `1.25.0`, from `go.mod`.

### `GocenePackage`
A Go package — a directory declaring one package — plus one node per external test
package (`_test` suffix on the importPath, Go toolchain convention).
- **Identity:** `importPath`. **Population: 336** (267 production + 69 test).
- `name` STRING — the declared package name; 245 distinct.
- `dir` STRING — repo-relative directory; `.` is the root.
- `depth` INTEGER — directory nesting, 0…4.
- `test` BOOLEAN — importPath ends in `_test`; true for the 69.
- `testOnly` BOOLEAN — true when **no non-test Go file declares the package**; true
  for 75 = the 69 test packages plus 6 internal test-only packages (`benchmark`,
  `search/uhighlight/testdata`, `tests/analysis`, `tests/index`, `tests/search`,
  `tests/util`) whose directories hold only test-role Go files — including files
  without the `_test.go` suffix (e.g. `tests/index/random_index_writer.go`), which is
  why the rule is stated on file role, not on the filename.

### `GoceneFile`
A source file of any language — every file in the tree, Go and non-Go.
- **Identity:** `path` — repo-relative. **Population: 4934** (4835 Go + 99 non-Go).
- `language` STRING — `go` (4835) | `java` (80) | `python` (9) | `shell` (7) |
  `makefile` (2) | `gomod` (1).
- `role` STRING — Go files: `production` (2950) | `test` (1878) | `example` (5) |
  `tool` (2); non-Go files: `tool` (96) | `build` (3). A Go file is `test` when its
  package is test-only, its path ends in `_test.go`, or it sits under a
  `testdata/` directory (the Go toolchain's testdata convention).
- `importPath` STRING — for Go files, the declared package's importPath (all 4835
  carry it); for non-Go files, the production package of the directory if one
  exists (measured: the 10 non-Go root files resolve to the root package, the other
  89 to none).
- `generated` BOOLEAN, `generator` STRING — a generated-source header is present
  (measured 1: `analysis/unicode_wordbreak.gen.go`, generated by
  `cmd/gen-unicode-wb/main.go`; the early over-match on the generator itself was
  fixed in the scanner before the final scan).
- `lines` INTEGER.

### `GoceneType`
Any declared type: struct, interface, defined type or alias.
- **Identity:** `qn` — `<importPath>.Name`, with the tier disambiguator where the
  natural key collides. **Population: 7747.**
- `name` STRING — 7058 distinct (91.1 % selective).
- `kind` STRING — `struct` (6169) | `interface` (956) | `defined` (344) |
  `alias` (278).
- `definedFrom` STRING — the right-hand type expression; set for the 344 defined
  types.
- `typeParams` INTEGER — the number of type parameters; set for the 146 generic
  types (values 1…2).
- `line` INTEGER.

### `GoceneFunction`
A package-level function of any kind — including `init`, tests, benchmarks, fuzz
targets and examples. They share every containment relation, so the kind is a
property.
- **Identity:** `qn` — `<importPath>.Name`, with the tier disambiguator; `init`
  functions always carry `#<file>#<n>` (identity rule above).
  **Population: 24249.**
- `name` STRING — 23158 distinct (95.5 % selective).
- `kind` STRING — `function` (11961) | `test` (12052) | `init` (84) |
  `benchmark` (143) | `fuzz` (7) | `example` (2).
- `line` INTEGER.

### `GoceneMethod`
A method — a concrete one or an interface method spec. They are the same kind of
thing to the port (a receiver-scoped callable) and share every relation.
- **Identity:** `sig` — `<importPath>.<receiver>.<name>` (type arguments erased in
  the receiver; Go has no overloading, so receiver + name is the full identity),
  with the tier disambiguator where the natural key collides.
  **Population: 33146** (30346 concrete + 2800 interface specs).
- `name` STRING — **not an identity**: 7833 distinct (23.6 % selective; `String`
  alone appears on 704 receivers).
- `receiver` STRING — the exact source text, e.g. `*GenericAnalysisSPILoader[S]`;
  empty for interface specs.
- `recvQN` STRING — the **base** type's QN: for parameterized receivers the type
  arguments are stripped, since they are fixed by the type's declaration; empty for
  interface specs. 584 concrete methods carry a parameterized receiver (legal Go —
  the canonical receiver of a generic type). Every concrete method's `recvQN`
  resolves to a declared `GoceneType`; a method whose receiver did not would carry
  only the file edge (§ 7).
- `recvPtr` BOOLEAN — pointer receiver; true for 29076 of the 30346 concrete.
- `interface` BOOLEAN — true for the 2800 interface method specs.
- `ifaceQN` STRING — interface specs only: the interface's QN (all 2800 resolve to
  a declared type).
- `line` INTEGER.

### `GoceneField`
A struct field or an embedded field.
- **Identity:** `key` — `<typeQN>.<name-part>`, with the tier disambiguator; the
  name-part of an embedded field is the embedded type's reference — the simple name
  within the same package, the package-qualified (full QN) form across packages.
  **Population: 21399** (19397 + 2002 embedded).
- `name` STRING — **not an identity**: 7056 distinct (33.0 % selective).
- `type` STRING — the declared type expression; a leading `*` marks a pointer
  embed.
- `embedded` BOOLEAN — true for the 2002.
- `line` INTEGER.

### `GoceneConstant`
- **Identity:** `qn` — `<importPath>.Name`, with the tier disambiguator.
  **Population: 3325.**
- `name` STRING — 3024 distinct (90.9 % selective).
- `type` STRING — the declared type; empty for the 2926 untyped constants.
- `line` INTEGER.

### `GoceneVariable`
- **Identity:** `qn` — `<importPath>.Name`, with the tier disambiguator.
  **Population: 3630.**
- `name` STRING — **not an identity**: 1720 distinct (47.4 % selective); the 1819
  blank `_` variables across 156 packages (120 collision groups) are the largest
  source of identity collisions in the tier, hence the `#<file>#<n>` disambiguator.
- `type` STRING — the declared type; empty for the 1553 variables whose type is
  inferred from the initializer.
- `line` INTEGER.

### `GoceneExternalPackage`
A package imported by the module but declared outside it. It exists so that
`IMPORTS` never dangles.
- **Identity:** `importPath`. **Population: 76** (66 standard-library, 8
  `golang.org/x` — `sys/unix`, `sys/windows`, `text/cases`, `text/encoding`,
  `text/encoding/simplifiedchinese`, `text/encoding/unicode`, `text/transform`,
  `text/unicode/norm` — 2 third-party: `github.com/golang/geo/s2`,
  `github.com/stretchr/testify/assert`).

### `GoceneExternalType`
A type referenced by Gocene code that does not resolve to a declared
`GoceneType` — whether genuinely external (standard library) or a dangling
reference (a repository defect: the named QN is not declared, or the type is
declared in a different package). The reference text is preserved as written,
exactly as the Lucene tier's `ExternalType` exists "so that … never dangle".
- **Identity:** `qn` — the reference text: a full QN for a dangling
  fully-qualified reference, the package-qualified or bare name as written
  otherwise. **Population: 13** (13 distinct reference texts — every reference
  text is unique, so the `qn` identity is the reference text itself).
- `origin` STRING — `qualified` (13: the reference text carries a package
  qualifier) | `unresolved` (0 at `d9992805`: a bare simple name whose package
  cannot be determined from the reference text alone — the Lucene tier's
  `origin: 'unresolved'` precedent; the value stays defined so a future bare
  reference is recorded rather than dropped).

### `GoceneMissingPackage`
A package imported by the module but declared nowhere in the tree — a
visible-by-query repository defect.
- **Identity:** `importPath`. **Population: 0** at `d9992805` — the label, its
  constraint and its `IMPORTS` pair stay defined so a future dangling import is
  recorded rather than dropped. The one node the label held until 2026-09-23
  (`…/schema`, imported only by `_test.go` files) was removed with its last import;
  `go list -e -test ./...` over the `d9992805` snapshot reports no missing package.

---

## 2. Predicate dictionary

One row per endpoint pair: the same predicate name between different labels is a
different assertion. **36 distinct predicate names over 73 defined endpoint pairs;
64 of them are live in the graph** — Lucene tier: 31 names over 47 pairs (39
live, § 6); Gocene tier: 11 names over 16 pairs (15 live — `IMPORTS` →
`GoceneMissingPackage` is empty); `PORTED_TO`: 10 pairs (all live); 7 names are
shared by both tiers.

### Containment — the organisational hierarchy

| Predicate | From → To | Asserts | Population |
|---|---|---|---:|
| `HAS_MODULE` | `LuceneRelease` → `LuceneModule` | the release ships this build unit | 43 |
| `HAS_SOURCE_SET` | `LuceneModule` → `LuceneSourceSet` | the module declares this source root | 112 |
| `DECLARES_JPMS_MODULE` | `LuceneModule` → `LuceneJpmsModule` | this module's `module-info.java` | 37 |
| `CONTAINS_PACKAGE` | `LuceneSourceSet` → `LucenePackage` | the source root holds this package | 598 |
| `CONTAINS_FILE` | `LucenePackage` → `LuceneFile` | a Java file declaring this package | 5937 |
| `CONTAINS_FILE` | `LuceneModule` → `LuceneFile` | a code file outside every source root (build logic, scripts) | 181 |
| `CONTAINS_FILE` | `LuceneSourceSet` → `LuceneFile` | a file under a source root with no package (`module-info`, non-Java code) | 65 |
| `DECLARES_TYPE` | `LuceneFile` → `LuceneClass` | top-level type declared by this file | 5622 |
| `DECLARES_NESTED` | `LuceneClass` → `LuceneClass` | nested, inner, local or anonymous type declared inside this one | 5979 |

### Members

| Predicate | From → To | Asserts | Population |
|---|---|---|---:|
| `DECLARES_METHOD` | `LuceneClass` → `LuceneMethod` | member method or constructor | 63624 |
| `DECLARES_FIELD` | `LuceneClass` → `LuceneField` | member field or constant | 26887 |
| `DECLARES_ENUM_CONSTANT` | `LuceneClass` → `LuceneEnumConstant` | enum constant, in declaration order | 698 |
| `DECLARES_RECORD_COMPONENT` | `LuceneClass` → `LuceneRecordComponent` | record component, in declaration order | 607 |
| `DECLARES_INITIALIZER` | `LuceneClass` → `LuceneInitializer` | static or instance initializer block | 312 |
| `DECLARES_TYPE_PARAM` | `LuceneClass` → `LuceneTypeParameter` | class-level generic parameter | 263 |
| `DECLARES_TYPE_PARAM` | `LuceneMethod` → `LuceneTypeParameter` | method-level generic parameter | 240 |
| `HAS_CONSTANT_BODY` | `LuceneEnumConstant` → `LuceneClass` | the constant carries its own class body | 60 |

### Type relationships

| Predicate | From → To | Asserts | Population |
|---|---|---|---:|
| `EXTENDS` | `LuceneClass` → `LuceneClass` | declared superclass or super-interface inside Lucene | 5667 |
| `EXTENDS` | `LuceneClass` → `ExternalType` | declared superclass outside Lucene | 195 |
| `EXTENDS_ANONYMOUSLY` | `LuceneClass` → `LuceneClass` | an anonymous class's supertype, inside Lucene | 2478 |
| `EXTENDS_ANONYMOUSLY` | `LuceneClass` → `ExternalType` | an anonymous class's supertype, outside Lucene | 387 |
| `IMPLEMENTS` | `LuceneClass` → `LuceneClass` | implemented interface inside Lucene | 829 |
| `IMPLEMENTS` | `LuceneClass` → `ExternalType` | implemented interface outside Lucene | 383 |
| `PERMITS` | `LuceneClass` → `LuceneClass` | `sealed` permitted subtype | 10 |

`EXTENDS_ANONYMOUSLY` is deliberately distinct from `EXTENDS`: an anonymous class's
supertype may be an interface, the relation has no `implements` counterpart, and the
port treats the two cases differently.

### References

| Predicate | From → To | Asserts | Population |
|---|---|---|---:|
| `IMPORTS` | `LuceneFile` → `LuceneClass` | a single-type import resolving inside Lucene; edge property `static` BOOLEAN | 36783 |
| `IMPORTS` | `LuceneFile` → `ExternalType` | a single-type import resolving outside Lucene | 17486 |
| `IMPORTS_ON_DEMAND` | `LuceneFile` → `ExternalPackage` | `import x.y.*` of a non-Lucene package | 66 |
| `IMPORTS_ON_DEMAND` | `LuceneFile` → `LucenePackage` | `import x.y.*` of a Lucene package; edge property `candidates` counts the source sets that share the name | 40 |
| `THROWS` | `LuceneMethod` → `ExternalType` | a declared checked exception outside Lucene | 25276 |
| `THROWS` | `LuceneMethod` → `LuceneClass` | a declared checked exception defined in Lucene | 422 |
| `THROWS` | `LuceneMethod` → `LuceneTypeParameter` | a generic exception parameter (`throws E`) | 4 |
| `ANNOTATED_WITH` | `LuceneMethod` → `ExternalType` | member-level annotation usage, annotation outside Lucene | 26210 |
| `ANNOTATED_WITH` | `LuceneMethod` → `LuceneClass` | member-level annotation usage, annotation defined in Lucene | 323 |
| `ANNOTATED_WITH` | `LuceneField` → `ExternalType` | field annotation usage | 214 |
| `ANNOTATED_WITH` | `LuceneClass` → `ExternalType` | type-level annotation usage, annotation outside Lucene | 138 |
| `ANNOTATED_WITH` | `LuceneClass` → `LuceneClass` | type-level annotation usage, annotation defined in Lucene | 7 |

An `IMPORTS` edge is one per (file, type): a file that static-imports several members
of the same type yields one edge, not several.

### Module system and service loading

| Predicate | From → To | Asserts | Population |
|---|---|---|---:|
| `EXPORTS` | `LuceneJpmsModule` → `LucenePackage` | JPMS `exports` — the declared public API surface; edge property `qualified` marks `… to <module>` | 298 |
| `OPENS` | `LuceneJpmsModule` → `LucenePackage` | JPMS `opens` — reflective access | 39 |
| `REQUIRES` | `LuceneJpmsModule` → `LuceneJpmsModule` | JPMS `requires` inside Lucene; edge properties `transitive`, `static` | 91 |
| `REQUIRES` | `LuceneJpmsModule` → `ExternalJpmsModule` | JPMS `requires` of a module outside Lucene | 29 |
| `PROVIDES` | `LuceneJpmsModule` → `LuceneClass` | JPMS `provides <service>`; edge property `impls` lists the `with` classes | 37 |
| `PROVIDES_IMPL` | `LuceneJpmsModule` → `LuceneClass` | one implementation named in a `provides … with …`; edge property `service` | 234 |
| `PROVIDES_IMPL` | `LuceneJpmsModule` → `ExternalType` | an implementation outside Lucene | 5 |
| `USES` | `LuceneJpmsModule` → `LuceneClass` | JPMS `uses` — a consumed service interface | 8 |
| `IMPLEMENTS_SPI` | `LuceneClass` → `LuceneSpiService` | registered in a `META-INF/services` file | 245 |
| `SERVICE_TYPE` | `LuceneSpiService` → `LuceneClass` | the service's own interface declaration | 8 |

### Code generation

| Predicate | From → To | Asserts | Population |
|---|---|---|---:|
| `GENERATED_FROM` | `LuceneFile` → `LuceneFile` | a generated source and the grammar or generator that produces it | 66 |

All 66 generated files are linked to their generator: 7 JFlex grammars, 3 JavaCC
grammars, 1 ANTLR grammar, and the Python and Groovy generators.

### Gocene tier

**11 predicates over 16 endpoint pairs (15 live).** Populations are live graph
counts, confirmed by the post-resync census and the two-direction diff (§ 6).
`PORTED_TO` populations are the derived live counts, verified per pair (§ 7).

| Predicate | From → To | Asserts | Population |
|---|---|---|---:|
| `CONTAINS_PACKAGE` | `GoceneModule` → `GocenePackage` | the module declares this package | 336 |
| `CONTAINS_FILE` | `GocenePackage` → `GoceneFile` | the file's production package (Go files: the declared package; non-Go files: the production package of the directory) | 4845 |
| `CONTAINS_FILE` | `GoceneModule` → `GoceneFile` | a file in a directory with no production package (`docs/`, `scripts/`, `tools/`) | 89 |
| `DECLARES_TYPE` | `GoceneFile` → `GoceneType` | a type declared by this file | 7747 |
| `DECLARES_FUNCTION` | `GoceneFile` → `GoceneFunction` | a package-level function of any kind | 24249 |
| `DECLARES_CONSTANT` | `GoceneFile` → `GoceneConstant` | a package-level constant | 3325 |
| `DECLARES_VARIABLE` | `GoceneFile` → `GoceneVariable` | a package-level variable | 3630 |
| `DECLARES_METHOD` | `GoceneFile` → `GoceneMethod` | a method declared by this file — concrete or interface spec | 33146 |
| `DECLARES_METHOD` | `GoceneType` → `GoceneMethod` | a member method of this type — concrete with a resolvable receiver, or an interface spec of this interface | 33158 |
| `DECLARES_FIELD` | `GoceneType` → `GoceneField` | a member field or embedded field of this type | 21399 |
| `EMBEDS` | `GoceneType` → `GoceneType` | an embedded field whose type resolves inside the module | 1979 |
| `EMBEDS` | `GoceneType` → `GoceneExternalType` | an embedded field whose reference does not resolve to a declared `GoceneType` | 23 |
| `IMPORTS` | `GoceneFile` → `GocenePackage` | an import of a package inside the module; edge property `blank` BOOLEAN, always present — true for the 89 `_` imports | 6600 |
| `IMPORTS` | `GoceneFile` → `GoceneExternalPackage` | an import of a package outside the module; edge property `blank` BOOLEAN, present only on the 8 `_` imports (`true`) | 7868 |
| `IMPORTS` | `GoceneFile` → `GoceneMissingPackage` | an import of a package declared nowhere in the tree | 0 |
| `GENERATED_FROM` | `GoceneFile` → `GoceneFile` | a generated source and the generator that produces it | 1 |

Every `GoceneFile` carries exactly one `CONTAINS_FILE` edge (4845 + 89 = 4934), and
every `IMPORTS` edge from a Go file targets exactly one of the three package labels
(6600 + 7868 + 0 = 14 468; the in-module targets were 6 602 raw inventory rows,
2 of which were exact duplicates). Go also allows dot imports (`import . x`); the
model would carry them as edge property `dot` BOOLEAN — none occurs in the measured
tree, so the property is not declared. A `GoceneMethod` may carry several
`DECLARES_METHOD` type edges when its receiver type is redeclared in the same
package (the repository defects measured in the § 1 identity rule): the method
belongs to every QN that shares the natural identity — measured 33 134 methods
with exactly one type owner, 12 with two (33 158 type→method edges), and 0 with
none (§ 1).

### Port relation — `PORTED_TO`

**10 endpoint pairs.** Populations are the live counts measured 2026-09-23 after the
sync to `d9992805`, each equal to the sync plan's final row count for that pair (§ 7, The
PORTED_TO derivation).

| Predicate | From → To | Asserts | Population |
|---|---|---|---:|
| `PORTED_TO` | `LuceneRelease` → `GoceneModule` | the release is ported to the module — the root of the port | 1 |
| `PORTED_TO` | `LuceneFile` → `GoceneFile` | this Lucene file is ported in this Go file (a Go source comment references the Lucene path) | 789 |
| `PORTED_TO` | `LucenePackage` → `GocenePackage` | this Lucene package is ported to this Go package | 400 |
| `PORTED_TO` | `LuceneClass` → `GoceneType` | this Lucene class is ported to this Go type | 2 392 |
| `PORTED_TO` | `LuceneMethod` → `GoceneMethod` | this method or constructor is ported to this method — Java overloads map many-to-one onto the single Go method | 9 364 |
| `PORTED_TO` | `LuceneField` → `GoceneField` | this Lucene field is ported to this Go field (non-constant members only) | 5 303 |
| `PORTED_TO` | `LuceneField` (isConstant) → `GoceneConstant` | this Lucene constant field is ported to this Go constant | 367 |
| `PORTED_TO` | `LuceneEnumConstant` → `GoceneConstant` | this enum constant is ported to this Go constant | 94 |
| `PORTED_TO` | `LuceneMethod` → `GoceneFunction` | this constructor, static method, test method or test helper is ported to this package-level Go function (`<init>` → `NewX`; test method `testFoo()` → `TestFoo`, `Test<Class>_Foo` or `Test<Class>_testFoo`; static method `foo(…)` → `Foo`) | 5 152 |
| `PORTED_TO` | `LuceneField` → `GoceneVariable` | this Lucene field (a constant held in a Go `var`, or a static test fixture) is ported to this package-level Go variable | 133 |

**Edge properties of `PORTED_TO`.** Besides `gitCommit` and `gitDate`:
- `duplicate` BOOLEAN — `true` when the Lucene artefact is ported **more than
  once**: at the class, member, constant, function and variable levels, the Lucene
  endpoint has two or more `PORTED_TO` edges to the same Go label; at the package
  level, the Go package holds one of the copies of a duplicated class. Absent
  otherwise. Every copy keeps its edge — the flag records the duplication, it does
  not choose a winner, and no code is deleted. Measured 2026-09-23: 1 000 edges
  flagged (192 class, 444 method, 146 field, 20 enum constant, 137 function, 61
  package).
- `copies` INTEGER — set with `duplicate`: the number of targets of the group.

**Placeholder stubs carry no `PORTED_TO`.** A Go type whose whole body is
`struct{ Name, Version string }` (measured: 62 types, all under `backward_codecs/`,
including `backward_codecs.Placeholder`) is not a port of the Lucene class its doc
comment names; such types, their fields and methods, and their `New<Name>` constructors are excluded,
so the absence of an edge reports them as not ported.

**Test-level correspondence.** A Lucene test class has no Go type: its port is a
`_test.go` file of package-level functions. It is expressed with the pairs above,
never with a new label: the test file by `LuceneFile` → `GoceneFile`, each test
method and helper by `LuceneMethod` → `GoceneFunction`, fixtures by
`LuceneField` → `GoceneConstant` / `GoceneVariable`, and nested test classes and
their members by the class, method and field pairs. A partial test port is
visible by query: a test class member without an outgoing `PORTED_TO` is not
ported.

`PORTED_TO` is the **sole authority on port status** (CLAUDE.md § 5.1.3): the
absence of an edge means **"not ported"**, never "unknown" — an unported artefact is
visible by query alone. Derived attributes (for example `LuceneClass.isPorted`) are
computed from the edges and never written independently, so the graph can never
assert a port status that contradicts its own edges.

An edge is written **only on measured, unambiguous evidence** — a file-header path reference, a doc-comment claim naming the Lucene class or member, or a name transliteration within a ported class pair or a ported test-file pair (§ 7). Every member, constant, function and variable edge is **anchored**: it is backed by the `PORTED_TO` edge of its owner's class pair (or, for a test class, of its file pair); an edge whose anchor is gone is removed. Where the evidence is ambiguous (two or more Go names matching one Java name), no edge is written: the relation never asserts what the evidence cannot determine. Some Go comments cite Apache Lucene **10.4.0** — the release the code was actually ported from — while the reference tree is 10.5.0; the 29 path references that resolve only against 10.4.0 (for example `ChecksumIndexOutput`, removed in 10.5.0) are documented as dangling and are never written (§ 7).


### Predicates deliberately not defined yet

| Predicate | Why |
|---|---|
| `OVERRIDES` (`LuceneMethod` → `LuceneMethod`) | needs full hierarchy resolution; `@Override` puts the upper bound at 23878 |
| `CALLS` (`LuceneMethod` → `LuceneMethod`) | needs a call graph, not declaration scanning |
| `IMPLEMENTS` (`GoceneType` → `GoceneType`) | Go interface satisfaction is implicit — there is no declaration to scan. Deriving it needs a full type checker, and it is an inferred non-local fact: same category as `OVERRIDES` and `CALLS` |

---

## 3. Constraints

Each identity property carries a `UNIQUE` constraint. **All 17 Lucene constraints
are created and enforced**, and they were re-created against the loaded data — a
violating dataset would have been rejected, so their acceptance is itself the proof
that every identity key is duplicate-free across the whole reference tree. With the
12 Gocene tier constraints (§ 3, Gocene section), the live graph carries **29 enforced
`UNIQUE` constraints**.

| Label | Property | DDL |
|---|---|---|
| `LuceneRelease` | `version` | `CREATE CONSTRAINT lucene_release_version FOR (x:LuceneRelease) REQUIRE x.version IS UNIQUE` |
| `LuceneModule` | `path` | `CREATE CONSTRAINT lucene_module_path FOR (x:LuceneModule) REQUIRE x.path IS UNIQUE` |
| `LuceneSourceSet` | `key` | `CREATE CONSTRAINT lucene_sourceset_key FOR (x:LuceneSourceSet) REQUIRE x.key IS UNIQUE` |
| `LuceneJpmsModule` | `name` | `CREATE CONSTRAINT lucene_jpms_name FOR (x:LuceneJpmsModule) REQUIRE x.name IS UNIQUE` |
| `LucenePackage` | `key` | `CREATE CONSTRAINT lucene_package_key FOR (x:LucenePackage) REQUIRE x.key IS UNIQUE` |
| `LuceneFile` | `path` | `CREATE CONSTRAINT lucene_file_path FOR (x:LuceneFile) REQUIRE x.path IS UNIQUE` |
| `LuceneClass` | `binaryName` | `CREATE CONSTRAINT lucene_class_binaryname FOR (x:LuceneClass) REQUIRE x.binaryName IS UNIQUE` |
| `LuceneMethod` | `signature` | `CREATE CONSTRAINT lucene_method_signature FOR (x:LuceneMethod) REQUIRE x.signature IS UNIQUE` |
| `LuceneField` | `key` | `CREATE CONSTRAINT lucene_field_key FOR (x:LuceneField) REQUIRE x.key IS UNIQUE` |
| `LuceneEnumConstant` | `key` | `CREATE CONSTRAINT lucene_enumconst_key FOR (x:LuceneEnumConstant) REQUIRE x.key IS UNIQUE` |
| `LuceneRecordComponent` | `key` | `CREATE CONSTRAINT lucene_reccomp_key FOR (x:LuceneRecordComponent) REQUIRE x.key IS UNIQUE` |
| `LuceneTypeParameter` | `key` | `CREATE CONSTRAINT lucene_typeparam_key FOR (x:LuceneTypeParameter) REQUIRE x.key IS UNIQUE` |
| `LuceneInitializer` | `key` | `CREATE CONSTRAINT lucene_initializer_key FOR (x:LuceneInitializer) REQUIRE x.key IS UNIQUE` |
| `LuceneSpiService` | `fqn` | `CREATE CONSTRAINT lucene_spi_fqn FOR (x:LuceneSpiService) REQUIRE x.fqn IS UNIQUE` |
| `ExternalType` | `name` | `CREATE CONSTRAINT external_type_name FOR (x:ExternalType) REQUIRE x.name IS UNIQUE` |
| `ExternalJpmsModule` | `name` | `CREATE CONSTRAINT external_jpms_name FOR (x:ExternalJpmsModule) REQUIRE x.name IS UNIQUE` |
| `ExternalPackage` | `name` | `CREATE CONSTRAINT external_package_name FOR (x:ExternalPackage) REQUIRE x.name IS UNIQUE` |

The engine supports only single-property `IS UNIQUE` and `IS NOT NULL`. Composite
`NODE KEY`, `ASSERT exists(...)` and type constraints are unsupported and fail.

### Why three weaker keys were rejected

| Rejected candidate | Measured failure |
|---|---|
| `LucenePackage` keyed on `name` | one package spans two modules; 247 span two source sets |
| `LuceneClass` keyed on `package + simpleName` | 174 collisions |
| `LuceneMethod` keyed on `name` | 2611 collisions |
| `LuceneMethod` keyed on `name + arity` | 897 collisions — overloads differing only in parameter type |

The erased parameter-type list is therefore mandatory in a method's identity.

### Gocene tier

The 12 constraints are **created and enforced** in the live graph. They were
declared before the load, against the empty Gocene labels — `CREATE CONSTRAINT`
fails (exit 1) against violating data, and an empty label is the only state in
which the declaration is guaranteed to succeed — and the load (`UNWIND … CREATE`,
§ 4) ran with enforcement live, so a duplicate identity would have been rejected at
the write, the strongest defence the engine offers against the pattern-`MERGE`
trap. Their acceptance against the loaded data is itself the proof that every Gocene
identity key is duplicate-free. The 2026-09-23 resync wrote under the same live
enforcement (`SHOW CONSTRAINTS` lists all 29).

| Label | Property | DDL |
|---|---|---|
| `GoceneModule` | `modulePath` | `CREATE CONSTRAINT gocene_module_modulepath FOR (x:GoceneModule) REQUIRE x.modulePath IS UNIQUE` |
| `GocenePackage` | `importPath` | `CREATE CONSTRAINT gocene_package_importpath FOR (x:GocenePackage) REQUIRE x.importPath IS UNIQUE` |
| `GoceneFile` | `path` | `CREATE CONSTRAINT gocene_file_path FOR (x:GoceneFile) REQUIRE x.path IS UNIQUE` |
| `GoceneType` | `qn` | `CREATE CONSTRAINT gocene_type_qn FOR (x:GoceneType) REQUIRE x.qn IS UNIQUE` |
| `GoceneFunction` | `qn` | `CREATE CONSTRAINT gocene_function_qn FOR (x:GoceneFunction) REQUIRE x.qn IS UNIQUE` |
| `GoceneMethod` | `sig` | `CREATE CONSTRAINT gocene_method_sig FOR (x:GoceneMethod) REQUIRE x.sig IS UNIQUE` |
| `GoceneField` | `key` | `CREATE CONSTRAINT gocene_field_key FOR (x:GoceneField) REQUIRE x.key IS UNIQUE` |
| `GoceneConstant` | `qn` | `CREATE CONSTRAINT gocene_constant_qn FOR (x:GoceneConstant) REQUIRE x.qn IS UNIQUE` |
| `GoceneVariable` | `qn` | `CREATE CONSTRAINT gocene_variable_qn FOR (x:GoceneVariable) REQUIRE x.qn IS UNIQUE` |
| `GoceneExternalPackage` | `importPath` | `CREATE CONSTRAINT gocene_externalpackage_importpath FOR (x:GoceneExternalPackage) REQUIRE x.importPath IS UNIQUE` |
| `GoceneExternalType` | `qn` | `CREATE CONSTRAINT gocene_externaltype_qn FOR (x:GoceneExternalType) REQUIRE x.qn IS UNIQUE` |
| `GoceneMissingPackage` | `importPath` | `CREATE CONSTRAINT gocene_missingpackage_importpath FOR (x:GoceneMissingPackage) REQUIRE x.importPath IS UNIQUE` |

### Why the weaker keys fail (Gocene tier)

Same method as above: each candidate is measured against the inventory, not argued.

| Rejected candidate | Measured failure |
|---|---|
| `GocenePackage` keyed on `name` | 147 of 339 packages share their name (43.4 %), in 55 collision groups — `util` declared in 8 packages, `document`, `index` and `search` in 6 each |
| `GoceneType`, `GoceneFunction`, `GoceneConstant`, `GoceneVariable` keyed on `name` | names are package-relative: 7053/7777 types (90.7 %), 23527/24524 functions (95.9 %), 2950/3245 constants (90.9 %) and 1682/3585 variables (46.9 %) are distinct names, the rest recur across the tree — the importPath qualifier is mandatory |
| `GoceneMethod` keyed on `name` | 7753 of 33052 distinct (23.5 %); `String` alone appears on 705 receivers |
| `GoceneMethod` keyed on `name + receiver` | 7 collision groups / 14 records — the same receiver + name declared twice in one package (1 redeclaration, 6 build-tag pairs); the `#<file>#<n>` disambiguator in `sig` (§ 1) is this weaker key failing |
| `GoceneField` keyed on `name` | 6975 of 21497 distinct (32.4 %); the owner type's QN is mandatory in `key` — field and embedded-type names recur across types |
| `GoceneVariable` keyed on `importPath + name` | the 1814 blank `_` variables form 120 same-package collision groups across 155 packages — the file qualifier is mandatory |

---

## 4. Indexes

The engine's index is **single-property, node-only and hash — therefore
equality-only**; a range predicate ignores it and falls back to a label scan.
**31 indexes exist and all are ONLINE** — the 29 constraint-backing `__uniq__`
indexes (17 Lucene, 12 Gocene) plus the two declared by hand.

**Every `UNIQUE` constraint automatically creates its own backing hash index**, named
`__uniq__<Label>.<property>`. That is a measured property of this engine, and it
means the 29 identity lookups need no separate `CREATE INDEX`. Two indexes were
declared by hand:

| Index | Nodes | Selectivity | Origin |
|---|---:|---:|---|
| `__uniq__LuceneMethod.signature` | 63624 | 100 % | constraint |
| `__uniq__GoceneMethod.sig` | 33052 | 100 % | constraint |
| `__uniq__LuceneField.key` | 26887 | 100 % | constraint |
| `__uniq__GoceneFunction.qn` | 24524 | 100 % | constraint |
| `__uniq__GoceneField.key` | 21497 | 100 % | constraint |
| `lucene_class_simplename` on `LuceneClass(simpleName)` | 8736 named | 88.2 % | **declared** — the human-facing lookup ("find `IndexWriter`"), ~1.13 rows per seek |
| `gocene_type_name` on `GoceneType(name)` | 7777 | 90.7 % | **declared** — the human-facing lookup ("find `PostingsWriterBase`") on the largest Gocene label; `EXPLAIN`/`PROFILE` below |
| the remaining 24 `__uniq__` indexes | ≤ 11 601 | 100 % | constraint |

### Proven by `EXPLAIN`, not asserted

| Query | Plan | Latency |
|---|---|---:|
| `MATCH (n:LuceneClass {binaryName: …})` | `NodeByIndexSeek` | 0.00 s |
| `MATCH (n:LuceneClass {simpleName: 'IndexWriter'})` | `NodeByIndexSeek` | 0.00 s |
| `MATCH (n:LuceneFile {path: …})` | `NodeByIndexSeek` | 0.00 s |
| `MATCH (n:LuceneClass {modifiers: 'public'})` — no index | `NodeByLabelScan` + `Filter` | — |
| `MATCH (n:LuceneMethod {signature: …})` | `ParallelScanProject` — the planner prefers a parallel scan over the seek on this label | 0.01 s |
| `MATCH (n:GoceneType {name: …})` | `NodeByIndexSeek` — the `gocene_type_name` index | 0.00 s (PROFILE 2026-09-23: 1 row, 1 dbHit, 3.4 µs) |

The last row is recorded because it contradicts the expectation: the index exists and
is ONLINE, but the planner does not choose it there. Measured latency is 0.01 s
either way, so nothing needs changing — but a reader must not assume a seek.

Explicitly **not** indexed: `LuceneMethod(name)` (35.4 % selective) and
`LuceneField(name)` (38.3 % selective) — neither is an identity, and a seek would
return roughly three rows. Composite indexes are unsupported, so where a lookup is
logically composite, index the more selective single property and let the engine
filter.

### Writing to this graph: `MERGE` does not use the index

**Measured, and it decides how every future load must be written.** On this engine a
`MERGE` on an indexed identity property performs a **label scan**, while `CREATE`
with the same constraints and indexes in place does not:

| Operation, 200 rows against `LuceneMethod` (63 624 nodes) | Time |
|---|---:|
| `MERGE (n:LuceneMethod {signature: …})` | **4.61 s** |
| `CREATE (n:LuceneMethod {signature: …})` | **0.02 s** |
| `MATCH (n:LuceneMethod {signature: …})` | 0.00 s |

Consequences for any bulk write:

- Use `UNWIND … CREATE` for nodes whose keys are known duplicate-free; the `UNIQUE`
  constraints stay enforced and reject any mistake.
- Use `MATCH … MATCH … CREATE` for edges, deduplicated in advance — never
  `MERGE (a:L {k})-[:R]->(b:L2 {k})`, which is also the pattern-`MERGE` trap.
- Reserve `MERGE` for small labels and for genuine upserts.
- Batch about 1000 rows per statement (the engine enforces a 5 s statement budget and
  a 1 048 576-byte statement limit) and run several clients concurrently. The Lucene
  tier — 112 068 nodes and 229 243 edges — loaded in **56.4 s** with ten concurrent
  clients this way (original load; the live graph now holds 39 of its 47 edge
  pairs, § 6), and the Gocene tier — 100 626 nodes and 150 110 edges at
  `dd61538c` — loaded the same way. The 2026-09-23 resync used the same forms
  (`UNWIND … CREATE`, `UNWIND … MATCH … SET`, `UNWIND … MATCH … MATCH … CREATE`,
  ≤ 300 rows per statement), every write checked against its `counters` block.

### Gocene tier

The 12 Gocene `UNIQUE` constraints (§ 3) created their 12 backing `__uniq__Gocene*`
hash indexes at declaration, and every Gocene identity lookup rides on them.
**One further index is created and proven by `EXPLAIN` and `PROFILE`:**

| Index | DDL | Measured plan |
|---|---|---|
| `gocene_type_name` on `GoceneType(name)` | `CREATE INDEX gocene_type_name IF NOT EXISTS FOR (x:GoceneType) ON (x.name)` | `EXPLAIN`: `NodeByIndexSeek` — the mirror of `lucene_class_simplename`, the human-facing lookup ("find `PostingsWriterBase`") on the largest Gocene label: 7 747 nodes, 91.1 % selective, ≈ 1.10 rows per seek; `PROFILE` (2026-09-23): 1 row, 1 dbHit, 3.4 µs |

Explicitly **not** indexed: `GoceneMethod(name)` (23.6 % selective) and
`GoceneField(name)` (33.0 % selective) — neither is an identity, and a seek returns
several hundred rows for the hot names. `GoceneFunction(name)` (95.5 % selective
over 24 249 nodes) is a candidate by the label-size × selectivity rule but is not
declared: the dominant lookup shape for a function is by `qn`, which the
constraint already indexes — revisit it with the same `EXPLAIN` method if the query
shapes demand it.

---

## 5. Provenance convention

Every node and edge carries `gitCommit` (the full Gocene commit hash when the element
was last confirmed) and `gitDate` (that commit's ISO date). Verified 2026-09-23:
**0 of the 210 924 nodes and 0 of the 334 895 edges lack `gitCommit`/`gitDate`**.

- **Lucene tier** — every node and every edge (112 068 / 162 505, including the
  90 511 `DECLARES_METHOD` and `DECLARES_FIELD` edges restored 2026-09-11 by
  derivation) carries the uniform value `gitCommit` =
  `dd61538c8b7949f00cca0f54aeb63d5093334035`, `gitDate` = `2026-09-08`.
- **Gocene tier** — an element carries the commit that **last changed its source
  file** when the element was last created or updated (`git log -1 -- <file>` at the
  synced commit): a symbol takes its declaring file, a field its owner type's file, a
  type→method edge the method's file; a package takes the newest commit among its
  files, an external package or type the newest among the files that reference it,
  the module `go.mod`. Elements unchanged since the first load keep `dd61538c` /
  `2026-09-08` (50 283 of 98 856 nodes, 106 429 of 148 395 edges); the tier carries
  88 distinct `gitCommit` values on nodes and 95 on edges.
- **`PORTED_TO`** — the 6 449 edges of the original 2026-09-11 derivation left
  untouched keep `gitCommit` = `9cbcdc1d31b7fef43ae855f1efe9be1b5b105c2e`, `gitDate`
  = `2026-09-11`; every edge created or updated by a later sync carries the source-file
  commit of its Go endpoint, by the rule above.

The Lucene side's reference
provenance is carried separately: every Lucene node additionally holds
`luceneCommit` = `f6eaee8148b7569e83c433feacc4f624608188fd` (0 nulls), so a future
Lucene bump is detectable by query rather than by memory, and side attribution in a
query is by label, not by `gitCommit` value.

---

## 6. Materialization status

| Tier | Defined | Materialised |
|---|---|---|
| Lucene structural tier (§ 1, § 2) | 17 labels, 31 predicates over 47 endpoint pairs | **112 068 nodes — complete; 162 505 of 229 243 edges — 39 of the 47 endpoint pairs live**. The 8 absent pairs (66 738 edges: `CONTAINS_PACKAGE`, the three `CONTAINS_FILE` variants, `DECLARES_TYPE`, the two `IMPORTS` variants, `GENERATED_FROM`) are tracked as rmp task 363. The two pairs the original load dropped that are derivable from live nodes — `DECLARES_METHOD` (63 624) and `DECLARES_FIELD` (26 887), 90 511 edges — were restored 2026-09-11 by derivation |
| Constraints (§ 3) | 29 (17 Lucene, 12 Gocene) | **29 created and enforced** |
| Indexes (§ 4) | 31 (29 constraint-backing, 2 declared) | **31 ONLINE** |
| Gocene tier (§ 1, § 2) | 12 labels, 11 predicates over 16 endpoint pairs | **98 856 nodes, 148 395 edges over 15 populated pairs** — first loaded at `dd61538c`, resynchronised 2026-09-23 to the `git archive` snapshot of `f008c200`, synchronised incrementally the same day to the snapshots of `0d52b534`, `a4769f4d`, `4d6bf725` and `d9992805`; post-sync census and two-direction identity, property and edge diff over all 12 labels: 0 differences |
| `PORTED_TO` (§ 2) | 10 endpoint pairs | **23 995 edges over the 10 pairs** — materialised 2026-09-11 by measured derivation, re-derived 2026-09-23 over the whole `f008c200` snapshot, for the files changed up to `0d52b534`, over the whole graph with the extended rules, for the files changed up to `a4769f4d`, for the files changed up to `4d6bf725` with the claim verbs of level 9 extended over the whole graph, and for the files changed up to `d9992805` (§ 7): write-time counter audit exact, live set equal to the plan, anchor audit 0 violations, `duplicate` flags consistent with the edges, provenance 100 % |

---

## 7. How the shape was measured and the load verified

No figure in this document was recalled or estimated. Every `.java` file in the
reference tree was parsed by a purpose-built Java 21 declaration scanner: comments and
string, char and text-block literals are blanked, then a brace-tracking recursive walk
extracts `package`, `import`, JPMS directives, type declarations, members, and — by
walking method bodies and field initialisers — anonymous and local classes. Non-Java
code was inventoried by extension and location.

**Scanner validation.** Cross-checked against the `javalang` parser over the 5419
files `javalang` can parse (it rejects 9.3 % of the tree: `module-info.java`,
`record`, `sealed` and other Java 21 syntax). Per-file agreement on named declarations
is **99.10 %**, and every one of the 49 disagreeing files was explained: they contain
exactly the 51 `record` declarations that `javalang` misparses as methods. On the
constructs `javalang` supports, agreement is exact — constructors, fields and enum
constants match to the unit. Two internal consistency checks also hold exactly: the
365 files declaring no top-level type are precisely the 328 `package-info.java` plus
the 37 `module-info.java`, and the 5622 top-level types are precisely
5602×1 + 7×2 + 2×3.

**Load verification.** After loading, every label and every predicate was counted in
the graph and compared with the extraction: **17 of 17 labels and 31 of 31 predicates
match exactly, with zero mismatches.** No node has a null identity property. No node
of any label is edgeless (`LuceneMethod` and `LuceneField` are covered by count
equality with `DECLARES_METHOD` and `DECLARES_FIELD`). Provenance is complete. A
2026-09-11 audit found 8 of the 47 Lucene endpoint pairs absent from the live graph
(66 738 edges, § 6); the two pairs derivable from live nodes — 90 511
`DECLARES_METHOD` and `DECLARES_FIELD` edges — were restored the same day (17
statements, 100 % owner resolution, uniform provenance stamp), and the remainder is
tracked as rmp task 363.

**Name resolution.** Every JPMS directive, every SPI registration and every generated
file resolved: 298 `exports`, 39 `opens`, 120 `requires`, 37 `provides`, 8 `uses`,
245 SPI providers and 66/66 generator links. Of 54 269 imports, 36 783 resolve to a
Lucene type and 17 486 to an external one. Only 12 of 811 `ExternalType` nodes could
not be fully qualified from source alone (§ 1, `ExternalType`).

**The Gocene tier measurement.** The Gocene tier was loaded from the exhaustive
working-tree inventory at `dd61538c`, produced by the Go declaration scanner
(`tools/kg-inventory`) over all 5 239 files (first measured 2026-09-10, re-measured
2026-09-11 after the scanner gained the Go `testdata` role rule — which moved one
file, `highlight/uhighlight/testdata/golden_snippets.go`, from `production` to
`test`, and made `highlight/uhighlight/testdata` a testOnly package). The post-load
census confirms every label population and every endpoint-pair edge count against
the final inventory, and the two-direction identity set diff over all 12 labels is
**empty in both directions**: 0 tree elements absent from the graph, 0 graph
elements absent from the tree (§ 6). Every identity collision in the tree was
measured, not sampled, which is why the disambiguator groups in § 1 carry exact
figures.

**The 2026-09-23 resync.** The tier was resynchronised to commit `f008c200`: the
same scanner ran over a `git archive f008c200` snapshot (tracked files only — the
working tree held uncommitted edits), 5 043 files, 0 parse errors. The whole tier
was diffed against the graph — every node by identity and by every property, every
edge by predicate, endpoint labels, endpoint identities and edge properties — and
the difference applied: 75 files, 606 types, 2 240 functions, 4 157 methods, 2 681
fields, 277 constants, 471 variables, 5 packages, 2 external packages and 1
external type created; 115 files, 548 types, 2 217 functions, 2 372 methods, 3 338
fields, 165 constants, 72 variables, 27 packages, 52 external types and the one
missing package deleted; 25 441 nodes updated; 18 819 edges created, 11 285
deleted plus the surplus copy of 2 duplicated `IMPORTS` edges, 6 edges updated
(`blank`) and 476 previously unstamped edges stamped. The re-run diff is **empty**: 0 node, 0
property and 0 edge differences.

**Package reconciliation against the Go toolchain.** The graph's 336 packages were
reconciled against `go list -e -test ./...` over the `d9992805` snapshot (338 import
paths, no error). The four-way diff is fully accounted for: the three go-list-only
paths — `index/nrt_test`, `queryparser/util`, `search/constant_score_bulk_scorer_test`
— are empty phantom packages (0 Go files of their own; per Go's own `go list` their
single `.go` file belongs to the external test packages the graph does hold), and
the one graph-only package — `search/uhighlight/testdata` — is a real importable
package that the `./...` expansion skips because the go tool ignores `testdata`
directories. No import is left unresolved, which is why `GoceneMissingPackage` is
empty.

### Known limits of the model, stated not hidden

| Limit | Consequence |
|---|---|
| `LuceneClass.ordinal` for anonymous and local types is **source-encounter order**, not a claim of byte-identity with `javac`'s `$N` assignment | identity is proven unique, but a `binaryName` must not be assumed equal to the compiled class-file name |
| Lambdas and method references are not modelled | they are method-body detail, not organisational structure |
| 12 `ExternalType` nodes carry `origin: 'unresolved'` | a bare simple name whose package cannot be determined without a classpath |
| Non-Java code is modelled as files only | `.gradle`, `.groovy`, `.py`, `.jflex`, `.jj`, `.g4` have internal structure that is not decomposed |
| `OVERRIDES` and `CALLS` are absent | both need resolution beyond declaration scanning |
| A file that fails to parse in the scanner keeps a `GoceneFile` node with zero declaration edges | its declarations cannot be extracted — visible by query, not hidden; measured at `d9992805`: 0 such files (10 at `dd61538c`) |
| A concrete `GoceneMethod` whose receiver resolves to no declared type in its package has no `DECLARES_METHOD` type edge | it keeps the file edge — visible by query, not hidden (§ 1 `GoceneMethod`); measured at `d9992805`: 0 such methods (1 at `4d6bf725`) |
| The 13 `GoceneExternalType` reference texts behind the 23 `EMBEDS` → `GoceneExternalType` edges | 7 name standard-library or `golang.org/x` types (`io.Reader`, `io.Closer`, `*bufio.Reader`, `*strings.Reader`, `bytes.Buffer`, `*testing.T`, `transform.NopResetter`); 6 carry a module-package qualifier the scanner does not resolve to a declared `GoceneType` (import aliases `gstore`, `utilhnsw`, `lucene90compressing`, plus `store.Directory`). The reference text is preserved as written — the never-dangle mirror of the Lucene tier's `ExternalType` |
| `GocenePackage.testOnly` is defined on file role, not on the `_test.go` suffix | six packages (`benchmark`, `search/uhighlight/testdata`, `tests/analysis`, `tests/index`, `tests/search`, `tests/util`) hold test-role Go files without the suffix; a suffix rule would misclassify them |
| `GoceneExternalType` mixes genuinely external references with dangling internal ones | 0 `origin: 'unresolved'` names at `d9992805` (the bare name `Sorter` left with its last reference); the reference text is preserved as written — resolving it is a repair, not a re-identification |

### The PORTED_TO derivation

The 10 187 `PORTED_TO` edges were materialised on 2026-09-11 (roadmap task 364) by a derivation script that reads only measured evidence from the two reference trees and the module working tree — no port status was ever assumed, and an artefact with no evidence gets no edge.

**Evidence sources.** (1) **File-header path references** — Go comments carrying a `<module>/src/(java|java21|test)/org/apache/lucene/….java` path into the Lucene reference tree (568 Go files carry such references). (2) **Doc-comment claims** — a Go type's doc comment naming a Lucene FQN *and* making a porting claim on the same line (claim verbs: port/ported/porting, mirror, equivalent, correspond, transl*); an FQN without a claim verb (a bare "see also") never anchors an edge. (3) **Name transliteration within a ported class pair** — for methods, fields and constants, the Go name matched against the Java name by exact match, capitalize-first, and the two measured systematic aliases (`toString` → `String`, `…Att` → `…Attr`).

**The seven levels, with the measured row counts.**

| Level | Pair | Evidence | Edges |
|---|---|---|---:|
| 0 — release | `LuceneRelease` → `GoceneModule` | the reference release is 10.5.0; the module is its port | 1 |
| 1 — file | `LuceneFile` → `GoceneFile` | a Go file's comment references the Lucene path (522 direct + 40 `lucene/`-prefixed) | 562 |
| 2 — class | `LuceneClass` → `GoceneType` | 148 file-pair name matches + 1 534 FQN doc-comment anchors, claim-verb gated | 1 579 |
| 3 — method | `LuceneMethod` → `GoceneMethod` | name transliteration within the 1 579 ported class pairs; Java overloads map many-to-one | 4 716 |
| 4 — field | `LuceneField` → `GoceneField` | name transliteration within the ported class pairs, non-constant members | 2 892 |
| 5 — constant | `LuceneField` (isConstant), `LuceneEnumConstant` → `GoceneConstant` | 7 measured Go naming conventions for a Java constant of a ported class, uniqueness-guarded (120 field + 36 enum) | 156 |
| 6 — package | `LucenePackage` → `GocenePackage` | the ported type's Go package, through the top-level class's file → package key | 281 |
| | | | **10 187** |

**Honesty rules.** Ambiguity is a reason for *no* edge, never a guess: a Java method matched by two or more Go names yields no edge (18 Java methods excluded — `topList` ×5, `readBytes` ×3, `rollup` ×2, `isLeaf` ×2, `findIntersections` ×2, and 8 singles); a Go constant claimed by two or more Java constants yields no edge (29 constants excluded, the most common `BYTES` ×6, `VERSION_CURRENT` ×5, `VERSION_START` ×5, `BLOCK_SIZE` ×4). The 29 file-header path references that resolve only against Apache Lucene **10.4.0** — the release the code was actually ported from — are recorded as dangling in the derivation's stats (for example `ChecksumIndexOutput`, removed in 10.5.0) and never written. The 967 FQNs whose comment line carries no claim verb, and the 103 FQNs whose named class exists in the reference tree but does not match the Go type's name, are counted in the stats and never written.

**Inventory identity note.** Go types that share their name with another type in the same package (build-tag pairs and redeclaration defects; 235 at `dd61538c`, 8 at `f008c200` and at `0d52b534`, 6 at `a4769f4d`, `4d6bf725` and `d9992805`) carry the anchored identity `importPath.TypeName#file.go#N`; the derivation strips the `#…` anchor before deriving package or type names from a qn, and the graph's class/member edges remain self-consistent on the anchored qns.

**Verification.** (1) *Write-time counter audit* — every edge statement carries two `MATCH` clauses, so a missing endpoint binds nothing and creates no edge; the sum of `relationshipsCreated` across all statements equals the derivation's row count exactly (10 187). (2) *Anchor audit* — every method, field and constant edge is backed by the `PORTED_TO` edge of its owning class pair through the `DECLARES` chains (4 716/4 716 methods, 2 892/2 892 fields, 120/120 + 36/36 constants, 0 violations). (3) *Per-pair equality* — the live count of each of the 8 pairs equals the derivation's row count for that pair, with zero stray edges (directed total 322 802). (4) *Provenance* — every one of the 10 187 edges carries `gitCommit` `9cbcdc1d31b7fef43ae855f1efe9be1b5b105c2e` and `gitDate` `2026-09-11` (the HEAD at derivation time).

#### The 2026-09-23 re-derivation (resync to `f008c200`)

**Scope.** Edges whose Go endpoint lies in a file changed between `282c5fe4` (the last full sync, 2026-09-14) and `f008c200` — 958 files — were re-derived; edges into unchanged files were kept; package edges were recomputed from the final class and file pairs.

**Rules** (a superset of the 2026-09-11 rules, applied over the snapshot):
- *Class claims* are read per doc-comment **paragraph** (wrapped lines joined), not per physical line: a claim verb and the Lucene FQN in the same paragraph, gated on the Go type name matching the Lucene simple name (first letter case-insensitive).
- *Field alias* adds `posIncAtt` → `posIncrAttr` to `…Att` → `…Attr`.
- *Constants* (and constant fields held in a Go `var`) match, in the Go type's package, the Java name as is, its CamelCase and lowerCamelCase forms, and those forms prefixed by the Go type name (`<Type><Camel>`, `<type><Camel>`, `<Type>_<NAME>`); an edge is written only when the Java constant has one candidate within its class pair and the Go constant is claimed by one Java constant.
- *Constructors* (`LuceneMethod` → `GoceneFunction`): every `<init>` of a ported class maps to `New<Type>` (`new<Type>` for an unexported type) in the type's package, when that function exists once and either the class has one constructor or the package has no other `New<Type>…` function.
- *Test level*: within a `LuceneFile` → `GoceneFile` pair of a test file, `testX()` maps to the one Go function named `TestX`, `<TestClass>_X`, `<TestClass>X` or `Test<TestClass>_X` in the paired file; any other method, and a static field, maps to the one function or package variable of the same (or capitalised) name in that file. The forms were extended the same day (below: *Derivation rules extended*).
- *Curated edges* written by hand at the `5b606987` and `a40ff560` syncs are kept while their Go file has not changed since a curated sync.
- *Stubs* are excluded, *duplicates* flagged, and every member edge must pass the anchor rule (§ 2, Port relation).
- *Transfers*: an edge lost because its Go node changed identity only (the `#<file>#<n>` disambiguator appeared or disappeared) was re-attached to the surviving declaration when that declaration lies in an unchanged file.

**Counts.** Before the resync 9 948 edges; 575 were removed with the 548 types and other Go nodes deleted by the structural sync (9 373). The plan then created 4 261 (4 199 derived, 62 transfers), deleted 98 (66 stub, 23 unsupported package, 8 not re-derived, 1 unanchored) and updated 1 897 (re-confirmation stamps and `duplicate` flags): **13 536 edges**. Calibration on unchanged files: the rules reproduce 6 955 of the 7 023 original-derivation edges there (99.0 %).

**Verification.** Every write statement's `relationshipsCreated` / `relationshipsDeleted` equals its row count; the live edge set equals the planned final set exactly (13 536, 0 duplicate pairs); anchor audit 0 violations; `duplicate`/`copies` consistent with the edges (0 inconsistencies); 0 edges into stub types or their members; 0 edges without provenance.

**Completion pass (same day, authorised complete resync).** The same rules were then applied to the files unchanged since `282c5fe4`: 8 218 edges created (831 class, 3 306 method, 1 664 field, 1 969 function, 159 + 33 constant, 93 variable, 67 file, 96 package), 0 deleted, 168 updated (`duplicate` flags); pre-existing edges there were kept. Result: **21 754 edges**; the live set equals the plan (0 duplicate pairs), anchor audit 0 violations, 911 `duplicate` flags consistent with the edges, 0 edges into stubs, 0 edges without provenance.

#### The 2026-09-23 incremental sync to `0d52b534`

**Module tier.** The scanner ran over a `git archive 0d52b534` snapshot (5 010 files, 0 parse errors) and the whole tier was diffed against the graph; the differences lie in the 165 paths changed since `f008c200` plus one identity whose disambiguator disappeared (`util.TestIntroSort`, its duplicate declaration deleted). Applied: 19 files, 38 types, 443 functions, 155 methods, 126 fields, 26 constants, 13 variables and 1 package created; 52 files, 62 types, 710 functions, 273 methods, 181 fields, 13 constants, 3 variables and 2 external types deleted; 1 085 nodes updated; 1 137 edges created and 1 843 deleted. Every element of a changed file carries `gitCommit` `0d52b534…` (1 005 kept nodes and 3 170 kept edges re-stamped). The re-run diff is **empty**: 0 node, 0 property and 0 edge differences, 0 unstamped elements.

**`PORTED_TO`.** The deletions removed 24 edges (21 754 → 21 730). The 2026-09-23 rules were applied to the 112 surviving changed files: 130 edges created (7 class, 29 method, 24 field, 14 function, 2 constant, 1 variable, 36 file, 17 package), 3 package edges deleted (no longer supported by a file or class pair), 461 re-derived edges re-stamped; 0 transfers. Result: **21 857 edges**. Verification: every write statement's counter equals its row count; a re-run of the plan against the live set yields 0 creates, 0 deletes and 0 updates; anchor audit 0 violations; 911 `duplicate` flags consistent with the edges.

#### Derivation rules extended (2026-09-23, complete resync)

Three rules were added to the derivation and applied to the whole graph; each keeps the honesty rules (ambiguity writes no edge) and the anchor rule.

- **Test-function forms (level 7).** The naming forms actually used in the module were measured over the 2 834 Lucene test methods of the paired test files: besides `TestX`, `<TestClass>_X`, `<TestClass>X` and `Test<TestClass>_X`, the repository uses `<TestClass>_testX` (203 methods), `<TestClass>_TestX` (2) and, for a `Base…TestCase` class, `Test<Base…>_X` (the `TestCase` suffix dropped). All seven forms are candidates; the paired file must hold exactly one of them.
- **Static methods (level 8).** Within a class pair (the Go type's file) or a file pair (the paired Go file), a Java `static` method maps to the one non-test package-level Go function named as the method (exact or capitalised) whose arity is compatible — equal, or a variadic Go function covering it. The Java (name, arity) must be unique among the class's static methods, and the Go function must be claimed by one Java method only. Arity comes from the Go declaration's parameter list, extracted from the snapshot with the doc-comment evidence.
- **Member-level doc claims (level 9).** A Go function's or method's doc-comment sentence that carries a claim verb names a Lucene member after the verb: `Class.member(params)`, `Class#member(params)`, a fully qualified `pkg.Class.member(…)`, a bare `member(params)`, or a bare constructor `Class(params)`. Between the verb (or the previous claimed reference) and the reference only articles, visibility and kind words may stand (`the`, `Lucene's`, `private`, `static`, `method`, …); any other word — "the tail of", "unlike" — ends the claim. The class resolves only among the classes that anchor the Go element: for a method, the class pairs of its owner type; for a function, the classes paired in its package or declared in its file's paired Lucene files; a bare member resolves in the element's own context (the owner type's classes; the classes paired to the types of the same Go file or declared in its paired Lucene files). The member resolves by name and, when a parameter list is written, by its exact simple-type list, else by the one overload of that arity; without a list only a non-overloaded name resolves. Constructors map to functions only.

**Counts.** The whole-graph plan (existing edges kept, every derivable edge added, package support, anchors and `duplicate` flags recomputed) created **1 429 edges** and deleted none: 234 by the test forms, 451 by the static-method rule (164 of them also evidenced by a doc claim), 743 by member-level doc claims only (550 to functions, 193 to methods), and 1 constant edge of the existing rules in a file the incremental sync did not cover. 113 edges were updated (`duplicate` flags). Result: **23 286 edges**; 1 170 `duplicate` flags.

**Verification.** Every write statement's counter equals its row count; re-running the plan against the live set yields 0 creates, 0 deletes and 0 updates, and so does the incremental plan over the files changed since `f008c200` (a derived edge survives re-derivation); anchor audit 0 violations; 0 duplicate pairs; 0 edges into stubs; 0 edges without provenance. A random sample of 30 new edges (8 test, 8 static, 14 doc claim), each checked against the Java declaration and the Go declaration and comment, found 0 false positives. An earlier sample without the claim-gap rule found 1 (`writeSuffix`, "the tail of `CodecUtil.writeIndexHeader`"), which the gap rule now excludes.

#### The 2026-09-23 incremental sync to `a4769f4d`

**Module tier.** The scanner ran over a `git archive a4769f4d` snapshot (4 959 files, 0 parse errors) and the whole tier was diffed against the graph; the differences lie in the 273 paths of `a4769f4d` plus one identity whose disambiguator disappeared (`search.NewPhraseQueryWithTerms`, its duplicate declaration in `search/boolean_rewrites_test.go` deleted). Applied: 8 files, 53 types, 509 functions, 458 methods, 150 fields, 29 constants and 24 variables created; 59 files, 79 types, 942 functions, 338 methods, 187 fields, 1 constant, 8 variables and 1 package (`search/join_test`) deleted; 2 169 nodes updated; 1 960 edges created and 2 255 deleted. Every element of a changed file carries `gitCommit` `a4769f4d…` (1 690 kept nodes and 5 978 kept edges re-stamped). The re-run diff is **empty**: 0 node, 0 property and 0 edge differences, 0 unstamped elements. `go list -e -test ./...` over the snapshot reports 341 import paths, no error; the four-way reconciliation with the graph's 339 packages is unchanged (the three phantom go-list-only paths and `search/uhighlight/testdata`).

**`PORTED_TO`.** The deletions removed 94 edges (23 286 → 23 192: 77 function, 8 method, 3 class, 3 field, 3 file). The rules of § 7 were applied to the 214 surviving changed files: 343 edges created (3 class, 38 method, 21 field, 259 function, 20 file, 2 package), 0 deleted, 713 re-derived edges re-stamped; 0 transfers. `facets.DrillDownQuery` is now a real port (7 fields, 15 methods), so the explicit stub exclusion was dropped and the type carries its class, file, 12 method and 6 field edges; the stub set is the 62 `Name`/`Version` placeholders. Result: **23 535 edges**. Verification: every write statement's counter equals its row count; a re-run of the incremental plan and of the whole-graph plan against the live set yields 0 creates, 0 deletes and 0 updates; anchor audit 0 violations; 0 duplicate pairs; 1 170 `duplicate` flags consistent with the edges; 0 edges into stubs; 0 edges without provenance.

#### The 2026-09-23 incremental sync to `4d6bf725`

**Module tier.** The scanner ran over a `git archive 4d6bf725` snapshot (4 961 files, 0 parse errors) and the whole tier was diffed against the graph; every difference lies in the 235 paths of `4d6bf725` (220 surviving, 15 deleted). Applied: 17 files, 73 types, 718 functions, 364 methods, 206 fields, 40 constants and 20 variables created; 15 files, 43 types, 437 functions, 244 methods, 147 fields, 22 constants and 8 variables deleted; 2 071 nodes updated; 2 085 edges created and 1 294 deleted. Every element of a changed file carries `gitCommit` `4d6bf725…` (1 796 kept nodes and 5 869 kept edges re-stamped). The re-run diff is **empty**: 0 node, 0 property and 0 edge differences, 0 unstamped elements. `go list -e -test ./...` over the snapshot reports 341 import paths, no error; the four-way reconciliation with the graph's 339 packages is unchanged.

**Level 9 claim verbs extended (whole graph).** The repository writes member-level claims as "`X` renders `Class.member(params)`" (1 923 function and method doc comments use the verb) and type-level claims as "the Go rendering of `pkg.Class`". The claim verbs therefore gain `render`, `renders`, `rendered` and the noun form `Go rendering`; the bare noun `rendering` is not a claim ("the english rendering of num" precedes a real `mirroring` claim). Two narrowings came with the extension, both measured on the candidates it produced: a colon in the gap ends the claim — it opens an explanation of the claimed reference, not another claimed one ("renders the final `Weight.bulkScorer(LeafReaderContext)`: `scorerSupplier(context)` …"); and a reference followed by "of the / a / an" is qualified and names another member ("renders the `randomVector(int)` of the byte subclasses"). Only the first claim verb of a sentence opens a claim: a later verb in the same sentence describes another element ("the returned function renders `tearDown()`"). The extended rules add 201 edges over the snapshot; every candidate was reviewed by its Java and Go names, and each doubtful one against the Go comment, which is how the two narrowings were found; the colon rule also retires 1 pre-existing false positive (`IntervalScorer.ensureFreq()` → `IntervalScorer.Score`, "mirrors `IntervalScorer.score()`: `ensureFreq()`; …"). The five `JoinUtil.createJoinQuery` overloads, previously ported but reachable only through the file edge (`JoinUtil` has no Go type, its overloads carry distinct Go names, and their comments say "Renders"), now carry their method edges.

**`PORTED_TO`.** The deletions removed 204 edges (23 535 → 23 331: 189 function, 5 method, 6 field, 3 file, 1 class). The rules were applied to the 220 surviving changed files: 409 edges created (11 class, 55 method, 20 field, 314 function, 2 constant, 6 file, 1 package), 4 deleted (`toString(String)` of four queries, whose port moved from `String` to `ToString`, re-derived to `ToString`), 858 re-derived edges re-stamped; 0 transfers (23 736). The whole-graph plan with the extended claim verbs then created 96 edges in unchanged files (81 function, 13 method, 1 class, 1 constant), retired 1 and updated 5 `duplicate` flags. Result: **23 831 edges**. Verification: every write statement's counter equals its row count; a re-run of the incremental plan and of the whole-graph plan against the live set yields 0 creates, 0 deletes and 0 updates; anchor audit 0 violations; 0 duplicate pairs; 1 178 `duplicate` flags consistent with the edges; 0 edges into stubs; 0 edges without provenance.

#### The 2026-09-23 incremental sync to `d9992805`

**Module tier.** The scanner ran over a `git archive d9992805` snapshot (4 934 files, 0 parse errors) and the whole tier was diffed against the graph; every difference lies in the 138 paths changed since `4d6bf725` (84 surviving, 54 deleted). The package `grouping` and its external test package were removed with the consolidation into `search/grouping`, and `facets/sortedset_test` with its last file. Applied: 3 packages deleted; 27 files, 92 types, 490 functions, 283 methods, 242 fields, 32 constants and 19 variables created; 54 files, 102 types, 346 functions, 311 methods, 307 fields, 11 constants and 12 variables deleted; 1 126 nodes updated; 1 773 edges created and 1 656 deleted. Every element of a changed file carries `gitCommit` `d9992805…` (91 kept nodes and 1 885 kept edges re-stamped). The re-run diff is **empty**: 0 node, 0 property and 0 edge differences, 0 unstamped elements. `go list -e -test ./...` over the snapshot reports 338 import paths, no error; the four-way reconciliation with the graph's 336 packages is unchanged. The unresolvable-receiver method `(*GroupingSearch).AssembleTopGroupsForTest` left with `grouping/export_test.go`: every concrete method now has a type owner.

**`PORTED_TO`.** The deletions removed 256 edges (23 831 → 23 575: 102 field, 92 method, 33 function, 27 class, 2 package — the `grouping` package pairs). The rules were applied to the 84 surviving changed files: 446 edges created (198 function, 108 field, 92 method, 28 file, 18 class, 1 constant, 1 package), 26 deleted (25 no longer derived from the replaced `search/grouping` files and `ToChildBlockJoinQuery.String`, 1 package pair left without support), 164 updated (87 `duplicate` flags changed — 85 cleared, 2 `copies` 4 → 2 — and 77 re-derived edges re-stamped); 0 transfers. Result: **23 995 edges**. Only `search/grouping` now maps `org.apache.lucene.search.grouping`, so the two package-level `duplicate` flags of that package are gone (2 → 0), together with the class and member flags of the removed copies: 1 178 → 1 000 flags. Verification: every write statement's counter equals its row count; a re-run of the incremental plan and of the whole-graph plan against the live set yields 0 creates, 0 deletes and 0 updates; anchor audit 0 violations; 0 duplicate pairs; 0 edges into stubs; 0 edges without provenance.
