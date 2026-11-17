package workloads

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFinalizeWorkloadExitPromotesZeroRC(t *testing.T) {
	dir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	metricsDir := filepath.Join(dir, "collected-metrics-test-uuid")
	if err := os.Mkdir(metricsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jobSummary := `[{"passed":false,"executionErrors":"hook failed","jobConfig":{"name":"virt-parallel-migration"}}]`
	if err := os.WriteFile(filepath.Join(metricsDir, "jobSummary.json"), []byte(jobSummary), 0o644); err != nil {
		t.Fatal(err)
	}

	rc := finalizeWorkloadExit("test-uuid", 0)
	if rc != 1 {
		t.Fatalf("expected rc=1, got %d", rc)
	}
}

func TestFinalizeWorkloadExitKeepsNonZeroRC(t *testing.T) {
	rc := finalizeWorkloadExit("absent-uuid", 2)
	if rc != 2 {
		t.Fatalf("expected rc=2, got %d", rc)
	}
}

func TestParseJobSummaryFileNDJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "jobSummary.json")
	payload := `{"passed":false,"executionErrors":"alert fired","jobConfig":{"name":"density"}}
{"passed":true,"jobConfig":{"name":"gc"}}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	got := parseJobSummaryFile(path)
	if len(got) != 1 || got[0].jobName != "density" {
		t.Fatalf("unexpected summaries: %+v", got)
	}
}
