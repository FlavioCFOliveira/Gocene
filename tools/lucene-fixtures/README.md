# `lucene-fixtures` — Gocene binary-compatibility harness

This directory hosts a self-contained Java/Maven project (JDK 21 + Apache
Lucene 10.4.0) that produces and verifies the binary artefacts used by
Gocene's compatibility test suite. It was introduced by Sprint 114 (T2) and
is the foundation on top of which T3 (golden corpus), T5 (combined scenarios)
and T6 (CI gating) are built.

## Layout

```
tools/lucene-fixtures/
├── pom.xml                      Maven project, depends on lucene-core 10.4.0
│                                plus every sub-module Gocene mirrors.
├── Makefile                     Convenience targets (build, test, gen, verify).
├── src/main/java/.../Main.java  Entry point dispatching gen / verify / list.
├── src/main/java/.../Scenarios.java        Scenario registry.
├── src/main/java/.../CorpusScenario.java   Scenario interface.
├── src/main/java/.../scenarios/SmokeScenario.java   Sprint 114 T2 smoke fixture.
└── src/test/java/.../SmokeScenarioTest.java         JUnit 5 round-trip tests.
```

## Requirements

| Tool   | Minimum version | Notes                                               |
| ------ | --------------- | --------------------------------------------------- |
| JDK    | 21              | Eclipse Temurin recommended (LTS).                  |
| Maven  | 3.6+            | Maven 3.8.7 ships with Debian 12.                   |
| Disk   | ~400 MB free    | Maven cache + Lucene 10.4.0 artefacts.              |

### Local JDK 21 (no system install)

`tools/lucene-fixtures` does not assume root access. The recommended local
setup is to drop Eclipse Temurin 21 into `~/.local` and export `JAVA_HOME`:

```bash
mkdir -p ~/.local && cd ~/.local
curl -fsSL -o jdk21.tar.gz \
    "https://api.adoptium.net/v3/binary/latest/21/ga/linux/$(uname -m | sed s/aarch64/aarch64/;s/x86_64/x64/)/jdk/hotspot/normal/eclipse"
tar -xzf jdk21.tar.gz && rm jdk21.tar.gz
ln -sfn jdk-21.* jdk-21
export JAVA_HOME="$HOME/.local/jdk-21"
export PATH="$JAVA_HOME/bin:$PATH"
java -version  # must report 21.x
```

## Offline / cached build

The first `mvn verify` run downloads Lucene 10.4.0 and its transitive
dependencies into `~/.m2/repository`. Once cached, subsequent runs work
offline:

```bash
mvn -B -q -o -f tools/lucene-fixtures/pom.xml verify
```

CI workers re-use the Maven cache via `actions/cache` keyed on
`pom.xml`. See `.github/workflows/ci.yml`.

## Usage

```bash
# Build the uber-jar and run unit tests.
make -f tools/lucene-fixtures/Makefile harness-build

# Generate the smoke fixture.
make -f tools/lucene-fixtures/Makefile harness-gen \
    SCENARIO=smoke SEED=0 TARGET=/tmp/gocene-smoke

# Verify the same fixture.
make -f tools/lucene-fixtures/Makefile harness-verify \
    SCENARIO=smoke SEED=0 SOURCE=/tmp/gocene-smoke

# List registered scenarios.
java -jar tools/lucene-fixtures/target/lucene-fixtures.jar list
```

## Exit codes (`gen` / `verify`)

| Code | Meaning                                                  |
| ---- | -------------------------------------------------------- |
| `0`  | success                                                  |
| `1`  | argument / usage error                                   |
| `2`  | unknown scenario name                                    |
| `3`  | IO error                                                 |
| `4`  | verification failure (artefact does not match scenario)  |

## Scenarios

The Sprint 114 T2 deliverable is the registry plus the **smoke** scenario:

* **`smoke`** — a `smoke.dat` file containing a four-`int64` payload wrapped
  in a `CodecUtil` index header / footer. Deterministic for a given seed,
  byte-identical between Lucene and Gocene. Used as a connectivity check
  for the harness and as the basis for the Lucene→Gocene→Gocene→Lucene
  round-trip integration test in `internal/compat/smoke/`.

T3 adds foundational format scenarios (postings, doc values, points,
HNSW, …). T5 adds combined end-to-end scenarios.

## Coexistence with `tools/fixture-gen/`

The older `tools/fixture-gen/` project (committed before Sprint 114)
produces a single multi-format Lucene index used by the pre-existing
`testdata/lucene-10.4.0-fixtures/` corpus. It is **not** replaced by
`tools/lucene-fixtures/`; both coexist. Future sprints may migrate
`fixture-gen` into the registry, but Sprint 114 keeps them separate so
that existing tests are not perturbed.
