package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func setupStatusTest(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	jobsDir = filepath.Join(base, "jobs")
	resultsDir = filepath.Join(base, "results")
	targetsDir = filepath.Join(base, "targets")
	if err := os.MkdirAll(jobsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(resultsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jobsMu.Lock()
	jobs = map[string]*Job{}
	clientRequestJobs = map[string]string{}
	jobsMu.Unlock()
	return base
}

func statusResponse(t *testing.T, id string) map[string]any {
	t.Helper()
	req := httptest.NewRequest("GET", "/api/status?job="+id, nil)
	rec := httptest.NewRecorder()
	handleStatus(rec, req)
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, rec.Body.String())
	}
	return body
}

func TestMissingRunningJobIsRecoveringNotCompleted(t *testing.T) {
	setupStatusTest(t)
	id := "job-restarting"
	if err := os.MkdirAll(filepath.Join(jobsDir, id), 0o755); err != nil {
		t.Fatal(err)
	}
	writeJobState(&Job{ID: id}, "running")
	body := statusResponse(t, id)
	if body["state"] != "recovering" {
		t.Fatalf("state = %v, want recovering", body["state"])
	}
	if body["definitive"] != false {
		t.Fatalf("definitive = %v, want false", body["definitive"])
	}
}

func TestFinishedJobIsDefinitive(t *testing.T) {
	setupStatusTest(t)
	id := "job-finished"
	dir := filepath.Join(jobsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJobState(&Job{ID: id}, "finished")
	if err := os.WriteFile(filepath.Join(dir, "job.log"), []byte("[go] job finished\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := statusResponse(t, id)
	if body["state"] != "finished" || body["definitive"] != true {
		t.Fatalf("unexpected response: %#v", body)
	}
	if body["returncode"] != float64(0) {
		t.Fatalf("returncode = %v, want 0", body["returncode"])
	}
}

func TestInMemoryRunningJobStaysRunning(t *testing.T) {
	setupStatusTest(t)
	id := "job-running"
	j := &Job{
		ID: id, Running: true,
		Progress: Progress{Stage: "scan", TargetTotal: 100, Scan: phaseProgress("scan", 25, 100)},
		Rows:     map[string]*Row{}, ScanDone: map[string]bool{}, RecheckDone: map[string]bool{}, SpeedDone: map[string]bool{},
	}
	jobsMu.Lock()
	jobs[id] = j
	jobsMu.Unlock()
	body := statusResponse(t, id)
	if body["running"] != true || body["state"] != "running" {
		t.Fatalf("unexpected response: %#v", body)
	}
	if body["definitive"] != false {
		t.Fatalf("definitive = %v, want false", body["definitive"])
	}
}
