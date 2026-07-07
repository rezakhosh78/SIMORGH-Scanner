package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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

func TestBuildXrayBatchConfigRoutesEachInbound(t *testing.T) {
	req := StartRequest{Config: "vless://11111111-1111-1111-1111-111111111111@example.com:443?security=tls&type=ws&sni=example.com&path=%2Fws"}
	targets := []xrayBatchTarget{
		{IP: "192.0.2.1", Port: 21001},
		{IP: "192.0.2.2", Port: 21002},
		{IP: "192.0.2.3", Port: 21003},
	}
	b, err := buildXrayBatchConfig(req, targets)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	inbounds, _ := cfg["inbounds"].([]any)
	outbounds, _ := cfg["outbounds"].([]any)
	if len(inbounds) != len(targets) || len(outbounds) != len(targets) {
		t.Fatalf("inbounds=%d outbounds=%d targets=%d", len(inbounds), len(outbounds), len(targets))
	}
	routing, _ := cfg["routing"].(map[string]any)
	rules, _ := routing["rules"].([]any)
	if len(rules) != len(targets) {
		t.Fatalf("rules=%d targets=%d", len(rules), len(targets))
	}
	for i, raw := range rules {
		rule := raw.(map[string]any)
		wantOut := "scan-out-" + strconv.Itoa(i)
		if rule["outboundTag"] != wantOut {
			t.Fatalf("rule %d outboundTag=%v want=%s", i, rule["outboundTag"], wantOut)
		}
	}
}

func TestRestoreScanCheckpointIncludesFailedTargets(t *testing.T) {
	base := setupStatusTest(t)
	j := &Job{
		ID:       "checkpoint-job",
		Dir:      filepath.Join(base, "jobs", "checkpoint-job"),
		Rows:     map[string]*Row{},
		ScanDone: map[string]bool{},
	}
	if err := os.MkdirAll(j.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	checkpoint := "v1\t192.0.2.10\tOK\t123.500000\n" +
		"v1\t192.0.2.11\tFAIL\t0\n"
	if err := os.WriteFile(scanCheckpointPath(j), []byte(checkpoint), 0o644); err != nil {
		t.Fatal(err)
	}
	restoreScanCheckpoint(j)
	if len(j.ScanDone) != 2 {
		t.Fatalf("ScanDone=%d want=2", len(j.ScanDone))
	}
	if len(j.Rows) != 1 {
		t.Fatalf("Rows=%d want=1", len(j.Rows))
	}
	if got := j.Rows["192.0.2.10"].LatencyMS; got != 123.5 {
		t.Fatalf("latency=%v want=123.5", got)
	}
	if _, exists := j.Rows["192.0.2.11"]; exists {
		t.Fatal("failed target must not be restored as a clean row")
	}
}
