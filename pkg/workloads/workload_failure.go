// Copyright 2026 The Kube-burner Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package workloads

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kube-burner/kube-burner/v2/pkg/workloads"
	log "github.com/sirupsen/logrus"
)

type failedJobSummary struct {
	jobName string
	detail  string
}

type jobSummaryRecord struct {
	Passed          bool   `json:"passed"`
	ExecutionErrors string `json:"executionErrors"`
	JobConfig       struct {
		Name string `json:"name"`
	} `json:"jobConfig"`
}

// finalizeWorkloadExit aligns the process exit code with local jobSummary artifacts
// and prints concise failure lines on stderr for harness subprocess capture.
func finalizeWorkloadExit(uuid string, rc int) int {
	failures := collectFailedJobSummaries(uuid)
	for _, failure := range failures {
		fmt.Fprintf(os.Stderr, "kube-burner-ocp: job %q failed: %s\n", failure.jobName, failure.detail)
	}
	if len(failures) > 0 && rc == 0 {
		log.Warn("kube-burner-ocp: jobSummary reports failure but workload exit code was 0; returning 1")
		rc = 1
	}
	if rc != 0 {
		logPath := fmt.Sprintf("kube-burner-%s.log", workloadLogPrefix(uuid))
		if _, err := os.Stat(logPath); err == nil {
			fmt.Fprintf(os.Stderr, "kube-burner-ocp: run log file %s (use --log-level=debug or inspect local jobSummary.json under collected-metrics-<uuid>/)\n", logPath)
		}
	}
	return rc
}

func workloadLogPrefix(uuid string) string {
	if strings.HasPrefix(uuid, "ocp-") {
		return uuid
	}
	return "ocp-" + uuid
}

func collectFailedJobSummaries(uuid string) []failedJobSummary {
	dirs := localMetricsDirectories(uuid)
	var failures []failedJobSummary
	seen := make(map[string]bool)
	for _, dir := range dirs {
		path := filepath.Join(dir, "jobSummary.json")
		for _, summary := range parseJobSummaryFile(path) {
			key := summary.jobName + "|" + summary.detail
			if seen[key] {
				continue
			}
			seen[key] = true
			failures = append(failures, summary)
		}
	}
	return failures
}

func localMetricsDirectories(uuid string) []string {
	candidates := []string{
		fmt.Sprintf("collected-metrics-%s", uuid),
		"collected-metrics",
	}
	var dirs []string
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			dirs = append(dirs, dir)
		}
	}
	if len(workloads.ConfigSpec.MetricsEndpoints) == 0 {
		return uniqueStrings(dirs)
	}
	for _, endpoint := range workloads.ConfigSpec.MetricsEndpoints {
		dir := strings.TrimSpace(endpoint.MetricsDirectory)
		if dir == "" {
			continue
		}
		dir = strings.ReplaceAll(dir, "{{.UUID}}", uuid)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			dirs = append(dirs, dir)
		}
	}
	return uniqueStrings(dirs)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	var out []string
	for _, v := range values {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func parseJobSummaryFile(path string) []failedJobSummary {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := strings.TrimSpace(string(payload))
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if len(lines) > 1 {
		var records []jobSummaryRecord
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			records = append(records, decodeJobSummaryRecord([]byte(line)))
		}
		return summariesFromRecords(records)
	}
	if text[0] == '[' {
		var records []jobSummaryRecord
		if err := json.Unmarshal(payload, &records); err != nil {
			return nil
		}
		return summariesFromRecords(records)
	}
	return summariesFromRecords([]jobSummaryRecord{decodeJobSummaryRecord(payload)})
}

func decodeJobSummaryRecord(payload []byte) jobSummaryRecord {
	var doc jobSummaryRecord
	_ = json.Unmarshal(payload, &doc)
	return doc
}

func summariesFromRecords(records []jobSummaryRecord) []failedJobSummary {
	var out []failedJobSummary
	for _, record := range records {
		if record.Passed || record.JobConfig.Name == "" {
			continue
		}
		detail := strings.TrimSpace(record.ExecutionErrors)
		if detail == "" {
			detail = "passed=false in jobSummary"
		}
		out = append(out, failedJobSummary{jobName: record.JobConfig.Name, detail: detail})
	}
	return out
}
