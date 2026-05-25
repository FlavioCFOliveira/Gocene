# Sprint 114 binary-compatibility harness (Java/JDK 21 + Lucene 10.4.0).
#
# Targets:
#   harness-build   Build the uber-jar (target/lucene-fixtures.jar) and run unit tests.
#   harness-test    Run unit tests only (no jar packaging).
#   harness-gen     Generate a fixture. Vars: SCENARIO, SEED, TARGET.
#   harness-verify  Verify a fixture. Vars: SCENARIO, SEED, SOURCE.
#   harness-clean   Remove target/.
#
# Environment:
#   JAVA_HOME must point to a JDK 21 install (see tools/lucene-fixtures/README.md).
#   Maven 3.6+ on PATH.

THIS_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
POM := $(THIS_DIR)pom.xml
JAR := $(THIS_DIR)target/lucene-fixtures.jar

SCENARIO ?= smoke
SEED ?= 0
TARGET ?= /tmp/gocene-fixture
SOURCE ?= $(TARGET)

# corpus-baseline target uses the canary seed 0xC0FFEE (decimal 12648430).
CORPUS_SEED ?= 12648430
CORPUS_DIR ?= /tmp/gocene-corpus-baseline

.PHONY: harness-build harness-test harness-gen harness-verify harness-clean corpus-baseline corpus-manifest

harness-build:
	mvn -B -q -f $(POM) verify

harness-test:
	mvn -B -q -f $(POM) test

harness-gen: harness-build
	java -jar $(JAR) gen $(SCENARIO) $(SEED) $(TARGET)

harness-verify: harness-build
	java -jar $(JAR) verify $(SCENARIO) $(SEED) $(SOURCE)

harness-clean:
	mvn -B -q -f $(POM) clean

# Produce every registered scenario under $(CORPUS_DIR) at the canary seed.
# Sprint 114 T3 acceptance criterion #4: must succeed end-to-end; outputs are
# gitignored (CORPUS_DIR defaults to /tmp/, outside the repo).
corpus-baseline: harness-build
	@rm -rf $(CORPUS_DIR)
	@mkdir -p $(CORPUS_DIR)
	@for s in $$(java -jar $(JAR) list); do \
		echo "[corpus] $$s seed=$(CORPUS_SEED)"; \
		java -jar $(JAR) gen $$s $(CORPUS_SEED) $(CORPUS_DIR)/$$s || exit $$?; \
	done
	@echo "[corpus] baseline produced at $(CORPUS_DIR)"

# Regenerate the canonical manifest (committed to the repo) from the harness.
corpus-manifest: harness-build
	java -jar $(JAR) manifest > $(THIS_DIR)manifests/baseline.tsv
	@echo "[corpus] wrote $(THIS_DIR)manifests/baseline.tsv"
