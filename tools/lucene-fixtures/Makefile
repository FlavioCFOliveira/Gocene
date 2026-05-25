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

.PHONY: harness-build harness-test harness-gen harness-verify harness-clean

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
