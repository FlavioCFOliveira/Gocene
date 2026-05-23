// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package smartcn

import (
	"bufio"
	"os"
	"strings"
)

// AnalysisDataDir holds the configured analysis data directory.
//
// Go port of org.apache.lucene.analysis.cn.smart.AnalyzerProfile.ANALYSIS_DATA_DIR.
//
// Deviation: in Go we do not panic at package-init time when the dictionary
// directory is absent; AnalysisDataDir is set to "" and callers (e.g.
// WordDictionary.Load) will report the error lazily. This matches idiomatic
// Go error handling.
var AnalysisDataDir = resolveDataDir()

func resolveDataDir() string {
	dirName := "analysis-data"
	propName := "analysis.properties"

	// Check environment variable / OS equivalent of -Danalysis.data.dir.
	if v := os.Getenv("ANALYSIS_DATA_DIR"); v != "" {
		return v
	}

	candidates := []string{
		dirName,
		"lib/" + dirName,
		propName,
		"lib/" + propName,
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.IsDir() {
			abs, err := os.Getwd()
			if err == nil {
				return abs + "/" + candidate
			}
			return candidate
		}
		// Regular file — try to read as properties file.
		if dir := getAnalysisDataDirFromFile(candidate); dir != "" {
			return dir
		}
		break
	}
	return ""
}

// getAnalysisDataDirFromFile reads analysis.data.dir from a Java-style
// properties file.
func getAnalysisDataDirFromFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "analysis.data.dir=") {
			return strings.TrimPrefix(line, "analysis.data.dir=")
		}
	}
	return ""
}
