package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const host = "127.0.0.1"
const port = "8080"

const (
	defaultScanWorkers          = 32
	maxScanWorkers              = 300
	defaultTimeoutSeconds       = 8
	maxTimeoutSeconds           = 25
	defaultRecheckSamples       = 3
	maxRecheckSamples           = 20
	defaultRecheckWorkers       = 8
	maxRecheckWorkers           = 64
	defaultSpeedWorkers         = 4
	maxSpeedWorkers             = 100
	defaultSpeedDurationSeconds = 5
	maxSpeedDurationSeconds     = 20
	defaultSpeedMB              = 5
	maxSpeedMB                  = 50
	defaultSpeedTimeoutSeconds  = 20
	maxSpeedTimeoutSeconds      = 90
)

type FlexInt int

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `" `)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, _ := strconv.Atoi(s)
	*f = FlexInt(n)
	return nil
}
func (f FlexInt) Int() int { return int(f) }

type StartRequest struct {
	Config           string   `json:"config"`
	Mode             string   `json:"mode"`
	ManualTargets    string   `json:"manual_targets"`
	ManualTargetsAlt string   `json:"manualTargets"`
	ISPCategory      string   `json:"isp_category"`
	ISPSelected      []string `json:"isp_selected"`
	ISPAll           bool     `json:"isp_all"`
	ISP              string   `json:"isp"`

	MaxHosts            FlexInt `json:"max_hosts"`
	Concurrency         FlexInt `json:"concurrency"`
	Timeout             FlexInt `json:"timeout"`
	URL                 string  `json:"url"`
	LogLevel            string  `json:"loglevel"`
	Recheck             bool    `json:"recheck"`
	RecheckSamples      FlexInt `json:"recheck_samples"`
	RecheckWorkers      FlexInt `json:"recheck_workers"`
	SpeedTest           bool    `json:"speed_test"`
	SpeedSource         string  `json:"speed_source"`
	SpeedWorkers        FlexInt `json:"speed_workers"`
	SpeedDuration       FlexInt `json:"speed_duration"`
	SpeedMB             FlexInt `json:"speed_mb"`
	SpeedTimeout        FlexInt `json:"speed_timeout"`
	GenerateBestConfigs bool    `json:"generate_best_configs"`
	KeepConfigs         bool    `json:"keep_configs"`
}

type Event struct {
	Rank            int     `json:"rank"`
	IP              string  `json:"ip"`
	Phase           string  `json:"phase"`
	Latency         string  `json:"latency"`
	LatencyValue    float64 `json:"latency_value"`
	AvgLatency      string  `json:"avg_latency"`
	AvgLatencyValue float64 `json:"avg_latency_value"`
	Pass            string  `json:"pass"`
	PassValue       int     `json:"pass_value"`
	PassTotal       int     `json:"pass_total"`
	Speed           string  `json:"speed"`
	SpeedValue      float64 `json:"speed_value"`
}

type PhaseProgress struct {
	Done      int    `json:"done"`
	Total     int    `json:"total"`
	Remaining int    `json:"remaining"`
	Percent   int    `json:"percent"`
	Stage     string `json:"stage"`
}

type Progress struct {
	Done         int           `json:"done"`
	Total        int           `json:"total"`
	Remaining    int           `json:"remaining"`
	Percent      int           `json:"percent"`
	Stage        string        `json:"stage"`
	TargetTotal  int           `json:"target_total"`
	CleanCount   int           `json:"clean_count"`
	RecheckCount int           `json:"recheck_count"`
	SpeedCount   int           `json:"speed_count"`
	FailCount    int           `json:"fail_count"`
	Scan         PhaseProgress `json:"scan"`
	Recheck      PhaseProgress `json:"recheck"`
	Speed        PhaseProgress `json:"speed"`
}

type FileInfo struct {
	Name string `json:"name"`
	Size string `json:"size"`
}
type Row struct {
	IP            string
	LatencyMS     float64
	AvgLatencyMS  float64
	RecheckPassed int
	RecheckTotal  int
	SpeedMbps     float64
	Phase         string
}
type Job struct {
	ID          string
	Req         StartRequest
	Dir         string
	ResultDir   string
	LogPath     string
	Ctx         context.Context
	Cancel      context.CancelFunc
	Running     bool
	ReturnCode  *int
	Started     time.Time
	Progress    Progress
	Rows        map[string]*Row
	Targets     []string
	ScanDone    map[string]bool
	RecheckDone map[string]bool
	SpeedDone   map[string]bool
	Paused      bool
	Cancelled   bool
	mu          sync.Mutex
}

var baseDir, xrayPath, jobsDir, resultsDir, targetsDir, ispDataDir string
var jobsMu sync.Mutex
var jobs = map[string]*Job{}

func main() {
	baseDir = getenv("RKH_BASE_DIR", ".")
	xrayPath = getenv("RKH_XRAY_EXEC", filepath.Join(baseDir, "xray"))
	jobsDir = filepath.Join(baseDir, "web_runtime", "jobs")
	resultsDir = filepath.Join(baseDir, "web_runtime", "results")
	targetsDir = filepath.Join(baseDir, "web_runtime", "targets")
	ispDataDir = filepath.Join(baseDir, "r")
	_ = os.MkdirAll(jobsDir, 0755)
	_ = os.MkdirAll(resultsDir, 0755)
	_ = os.MkdirAll(targetsDir, 0755)
	http.HandleFunc("/fonts/", handleFontAsset)
	http.HandleFunc("/simorgh_icon.png", handleBrandIcon)
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/start", handleStart)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/stop", handleStop)
	http.HandleFunc("/api/cancel", handleCancel)
	http.HandleFunc("/api/continue", handleContinue)
	http.HandleFunc("/api/isps", handleISPs)
	http.HandleFunc("/api/results", handleResults)
	http.HandleFunc("/api/logs", handleLogs)
	http.HandleFunc("/download", handleDownload)
	_ = http.ListenAndServe(host+":"+port, nil)
}

func getenv(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}

func handleFontAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/fonts/")
	if name == "" || filepath.Base(name) != name {
		http.NotFound(w, r)
		return
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".ttf":
		w.Header().Set("Content-Type", "font/ttf")
	case ".otf":
		w.Header().Set("Content-Type", "font/otf")
	default:
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	http.ServeFile(w, r, filepath.Join(baseDir, "fonts", name))
}

func handleBrandIcon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store, max-age=0")
	http.ServeFile(w, r, filepath.Join(baseDir, "simorgh_icon.png"))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, err := os.ReadFile(filepath.Join(baseDir, "index.html"))
	if err != nil {
		http.Error(w, "index.html missing", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func handleStart(w http.ResponseWriter, r *http.Request) {
	var req StartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	normalize(&req)
	id := time.Now().Format("20060102-150405") + "-" + randHex(3)
	dir := filepath.Join(jobsDir, id)
	res := filepath.Join(resultsDir, id)
	_ = os.MkdirAll(dir, 0755)
	_ = os.MkdirAll(res, 0755)
	ctx, cancel := context.WithCancel(context.Background())
	j := &Job{ID: id, Req: req, Dir: dir, ResultDir: res, LogPath: filepath.Join(dir, "job.log"), Ctx: ctx, Cancel: cancel, Running: true, Started: time.Now(), Rows: map[string]*Row{}, ScanDone: map[string]bool{}, RecheckDone: map[string]bool{}, SpeedDone: map[string]bool{}}
	jobsMu.Lock()
	jobs[id] = j
	jobsMu.Unlock()
	saveJSON(filepath.Join(dir, "payload.json"), req)
	writeText(filepath.Join(dir, "vless.txt"), req.Config)
	go runJob(j)
	writeJSON(w, map[string]any{"ok": true, "job_id": id})
}

func normalize(r *StartRequest) {
	if r.Mode == "" {
		r.Mode = "manual"
	}
	if r.ManualTargets == "" {
		r.ManualTargets = r.ManualTargetsAlt
	}

	if r.Concurrency.Int() <= 0 {
		r.Concurrency = defaultScanWorkers
	}
	if r.Concurrency.Int() > maxScanWorkers {
		r.Concurrency = maxScanWorkers
	}

	if r.Timeout.Int() <= 0 {
		r.Timeout = defaultTimeoutSeconds
	}
	if r.Timeout.Int() > maxTimeoutSeconds {
		r.Timeout = maxTimeoutSeconds
	}

	if r.URL == "" {
		r.URL = "https://www.gstatic.com/generate_204"
	}
	if r.LogLevel == "" {
		r.LogLevel = "warning"
	}

	if r.RecheckSamples.Int() <= 0 {
		r.RecheckSamples = defaultRecheckSamples
	}
	if r.RecheckSamples.Int() > maxRecheckSamples {
		r.RecheckSamples = maxRecheckSamples
	}

	if r.RecheckWorkers.Int() <= 0 {
		r.RecheckWorkers = defaultRecheckWorkers
	}
	if r.RecheckWorkers.Int() > maxRecheckWorkers {
		r.RecheckWorkers = maxRecheckWorkers
	}

	if r.SpeedSource == "" {
		r.SpeedSource = "gstatic"
	}
	switch r.SpeedSource {
	case "gstatic", "speed.cloudflare.com", "cdnjs.cloudflare.com":
	default:
		r.SpeedSource = "gstatic"
	}

	if r.SpeedWorkers.Int() <= 0 {
		r.SpeedWorkers = defaultSpeedWorkers
	}
	if r.SpeedWorkers.Int() > maxSpeedWorkers {
		r.SpeedWorkers = maxSpeedWorkers
	}

	if r.SpeedDuration.Int() <= 0 {
		r.SpeedDuration = defaultSpeedDurationSeconds
	}
	if r.SpeedDuration.Int() > maxSpeedDurationSeconds {
		r.SpeedDuration = maxSpeedDurationSeconds
	}

	if r.SpeedMB.Int() <= 0 {
		r.SpeedMB = defaultSpeedMB
	}
	if r.SpeedMB.Int() > maxSpeedMB {
		r.SpeedMB = maxSpeedMB
	}

	if r.SpeedTimeout.Int() <= 0 {
		r.SpeedTimeout = defaultSpeedTimeoutSeconds
	}
	if r.SpeedTimeout.Int() > maxSpeedTimeoutSeconds {
		r.SpeedTimeout = maxSpeedTimeoutSeconds
	}

	if len(r.ISPSelected) == 0 && r.ISP != "" && r.ISP != "all" && r.ISP != "selected" {
		for _, p := range strings.Split(r.ISP, ",") {
			if strings.TrimSpace(p) != "" {
				r.ISPSelected = append(r.ISPSelected, strings.TrimSpace(p))
			}
		}
	}
	if r.ISP == "all" {
		r.ISPAll = true
	}
}

func runJob(j *Job) {
	rc := 0
	defer func() {
		j.mu.Lock()
		defer j.mu.Unlock()
		if j.Paused {
			return
		}
		j.Running = false
		if j.ReturnCode == nil {
			val := rc
			j.ReturnCode = &val
		}
	}()
	logf := openAppend(j.LogPath)
	defer logf.Close()
	defer func() {
		if v := recover(); v != nil {
			rc = 1
			logLine(logf, fmt.Sprintf("[go] recovered panic: %v", v))
		}
	}()
	logLine(logf, "[go] job started "+j.ID)
	logLine(logf, "[go] backend native mode")
	logLine(logf, "[go] xray="+xrayPath)

	if len(j.Targets) == 0 {
		targets, err := collectTargets(j.Req)
		if err != nil {
			logLine(logf, "[go] target error: "+err.Error())
			rc = 1
			return
		}
		if j.Req.MaxHosts.Int() > 0 && len(targets) > j.Req.MaxHosts.Int() {
			targets = targets[:j.Req.MaxHosts.Int()]
		}
		j.mu.Lock()
		j.Targets = append([]string(nil), targets...)
		j.mu.Unlock()
		writeLines(filepath.Join(targetsDir, "targets_"+j.ID+".txt"), targets)
		logLine(logf, fmt.Sprintf("Loaded %d targets", len(targets)))
		jobInitProgress(j, len(targets))
	}

	remaining := jobRemainingTargets(j)
	if len(remaining) > 0 {
		_ = scanTargets(j, logf, remaining)
		writeRows(j.ResultDir, "clean_ips", jobCurrentRows(j))
		if j.Ctx.Err() != nil {
			if j.Paused {
				logLine(logf, "[go] job paused during scan")
				return
			}
			rc = 130
			return
		}
	}
	jobFinishPhase(j, "scan")

	clean := jobCurrentRows(j)
	if j.Req.Recheck && len(clean) > 0 {
		pending := jobPendingRecheckRows(j)
		if len(pending) > 0 {
			_ = recheckTargets(j, logf, pending)
			writeRows(j.ResultDir, "clean_ips_rechecked", jobCurrentRows(j))
			if j.Ctx.Err() != nil {
				if j.Paused {
					logLine(logf, "[go] job paused during recheck")
					return
				}
				rc = 130
				return
			}
		}
		jobFinishPhase(j, "recheck")
	}

	clean = jobCurrentRows(j)
	if j.Req.SpeedTest && len(clean) > 0 {
		pending := jobPendingSpeedRows(j)
		if len(pending) > 0 {
			_ = speedTestTargets(j, logf, pending)
			writeRows(j.ResultDir, "clean_ips_speed_tested", jobCurrentRows(j))
			if j.Ctx.Err() != nil {
				if j.Paused {
					logLine(logf, "[go] job paused during speed")
					return
				}
				rc = 130
				return
			}
		}
		jobFinishPhase(j, "speed")
	}

	jobFinished(j)
	logLine(logf, "[go] job finished")
}

func jobRemainingTargets(j *Job) []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := []string{}
	for _, ip := range j.Targets {
		if !j.ScanDone[ip] {
			out = append(out, ip)
		}
	}
	return out
}

func jobCurrentRows(j *Job) []Row {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]Row, 0, len(j.Rows))
	for _, r := range j.Rows {
		if r == nil {
			continue
		}
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		li, lj := out[i].LatencyMS, out[j].LatencyMS
		if li == 0 {
			li = out[i].AvgLatencyMS
		}
		if lj == 0 {
			lj = out[j].AvgLatencyMS
		}
		if li == 0 && lj == 0 {
			return out[i].IP < out[j].IP
		}
		if li == 0 {
			return false
		}
		if lj == 0 {
			return true
		}
		return li < lj
	})
	return out
}

func jobPendingRecheckRows(j *Job) []Row {
	all := jobCurrentRows(j)
	j.mu.Lock()
	defer j.mu.Unlock()
	out := []Row{}
	for _, r := range all {
		if !j.RecheckDone[r.IP] {
			out = append(out, r)
		}
	}
	return out
}

func jobPendingSpeedRows(j *Job) []Row {
	all := jobCurrentRows(j)
	j.mu.Lock()
	defer j.mu.Unlock()
	out := []Row{}
	for _, r := range all {
		if !j.SpeedDone[r.IP] {
			out = append(out, r)
		}
	}
	return out
}

type ISPManifestEntry struct {
	ID                  string `json:"id"`
	Category            string `json:"category"`
	Name                string `json:"name"`
	File                string `json:"file"`
	IPCount             uint64 `json:"ip_count"`
	RangeCount          int    `json:"range_count"`
	CollapsedRangeCount int    `json:"collapsed_range_count"`
}

type ISPManifest struct {
	Version int                `json:"version"`
	Files   []ISPManifestEntry `json:"files"`
}

func unpackResource(blob []byte) ([]byte, error) {
	if len(blob) < 5 || string(blob[:4]) != "OB39" {
		return nil, fmt.Errorf("invalid resource package")
	}
	body := append([]byte(nil), blob[4:]...)
	for i, j := 0, len(body)-1; i < j; i, j = i+1, j-1 {
		body[i], body[j] = body[j], body[i]
	}
	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("resource unpack failed: %w", err)
	}
	defer zr.Close()
	plain, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("resource read failed: %w", err)
	}
	return plain, nil
}

func readPackedResource(rel string) ([]byte, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return nil, fmt.Errorf("invalid resource path")
	}
	blob, err := os.ReadFile(filepath.Join(ispDataDir, clean))
	if err != nil {
		return nil, err
	}
	return unpackResource(blob)
}

func loadISPManifest() (ISPManifest, error) {
	var m ISPManifest
	b, err := readPackedResource("m.bin")
	if err != nil {
		return m, fmt.Errorf("ISP index unavailable: %w", err)
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("invalid ISP index: %w", err)
	}
	if m.Version != 2 {
		return m, fmt.Errorf("unsupported ISP index version: %d", m.Version)
	}
	return m, nil
}

func loadISPEntry(e ISPManifestEntry) ([]byte, error) {
	plain, err := readPackedResource(e.File)
	if err != nil {
		return nil, fmt.Errorf("ISP resource unavailable for %s: %w", e.Name, err)
	}
	return plain, nil
}

func collectTargets(req StartRequest) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	limit := req.MaxHosts.Int()
	// MaxHosts <= 0 means unlimited. This is intentional for bulk scans such as /16 (~65k IPs).

	add := func(ip string) bool {
		ip = strings.TrimSpace(ip)
		if ip == "" || seen[ip] || net.ParseIP(ip) == nil {
			return false
		}
		if limit > 0 && len(out) >= limit {
			return true
		}
		seen[ip] = true
		out = append(out, ip)
		return limit > 0 && len(out) >= limit
	}

	remaining := func() int {
		if limit <= 0 {
			return 0
		}
		r := limit - len(out)
		if r < 0 {
			return 0
		}
		return r
	}

	if strings.ToLower(req.Mode) == "isp" {
		cat := req.ISPCategory
		if cat != "international" {
			cat = "iran"
		}

		manifest, err := loadISPManifest()
		if err != nil {
			return nil, err
		}
		allow := map[string]bool{}
		for _, s := range req.ISPSelected {
			s = strings.TrimSpace(s)
			if s != "" {
				allow[s] = true
			}
		}
		if !req.ISPAll && len(allow) == 0 {
			return nil, fmt.Errorf("no ISP selected; select at least one ISP")
		}

		matchedFiles := 0
		for _, entry := range manifest.Files {
			if entry.Category != cat {
				continue
			}
			if !req.ISPAll && !allow[entry.ID] {
				continue
			}
			matchedFiles++
			if limit > 0 && len(out) >= limit {
				break
			}
			plain, err := loadISPEntry(entry)
			if err != nil {
				return nil, err
			}
			ips, err := parseTargetData(plain, remaining())
			if err != nil {
				return nil, fmt.Errorf("invalid ISP data for %s: %w", entry.Name, err)
			}
			for _, ip := range ips {
				if add(ip) {
					break
				}
			}
		}
		if matchedFiles == 0 {
			return nil, fmt.Errorf("selected ISP was not found in category %s", cat)
		}
	}
	tmp := filepath.Join(targetsDir, "manual_tmp.txt")
	writeText(tmp, req.ManualTargets)
	ips, err := parseTargetFile(tmp, limit)
	for _, ip := range ips {
		if add(ip) {
			break
		}
	}
	if err != nil {
		return out, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid manual targets loaded")
	}
	return out, nil
}

func parseTargetFile(path string, capN int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseTargetReader(f, capN)
}

func parseTargetData(data []byte, capN int) ([]string, error) {
	return parseTargetReader(strings.NewReader(string(data)), capN)
}

func parseTargetReader(r io.Reader, capN int) ([]string, error) {
	out := []string{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(strings.Split(sc.Text(), "#")[0])
		if line == "" {
			continue
		}
		tok := strings.Fields(line)[0]
		remaining := 0
		if capN > 0 {
			remaining = capN - len(out)
			if remaining <= 0 {
				return out[:capN], nil
			}
		}
		ips := expandTarget(tok, remaining)
		out = append(out, ips...)
		if capN > 0 && len(out) >= capN {
			return out[:capN], nil
		}
	}
	return out, sc.Err()
}
func expandTarget(t string, maxN int) []string {
	if maxN == 0 {
		maxN = 1 << 30
	}
	out := []string{}
	if strings.Contains(t, "/") {
		ip, netw, err := net.ParseCIDR(t)
		if err != nil {
			return nil
		}
		ip = ip.Mask(netw.Mask)
		for ; netw.Contains(ip); incIP(ip) {
			out = append(out, ip.String())
			if len(out) >= maxN {
				break
			}
		}
		return out
	}
	if strings.Contains(t, "-") {
		p := strings.SplitN(t, "-", 2)
		a := net.ParseIP(strings.TrimSpace(p[0])).To4()
		b := net.ParseIP(strings.TrimSpace(p[1])).To4()
		if a == nil || b == nil {
			return nil
		}
		for ip := append(net.IP(nil), a...); cmpIP(ip, b) <= 0; incIP(ip) {
			out = append(out, ip.String())
			if len(out) >= maxN {
				break
			}
		}
		return out
	}
	if net.ParseIP(t) != nil {
		return []string{t}
	}
	return nil
}
func incIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] > 0 {
			break
		}
	}
}
func cmpIP(a, b net.IP) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

type VLESSConfig struct {
	ID          string
	Port        int
	Network     string
	Security    string
	SNI         string
	HostHeader  string
	Path        string
	Flow        string
	Fingerprint string
	PublicKey   string
	ShortID     string
	SpiderX     string
	ServiceName string
	Authority   string
	Mode        string
	RawQuery    url.Values
}

func scanTargets(j *Job, logf *os.File, targets []string) []Row {
	workers := clampInt(j.Req.Concurrency.Int(), 1, maxScanWorkers)
	logLine(logf, fmt.Sprintf("[go] scan workers=%d timeout=%ds target_cap=%d", workers, j.Req.Timeout.Int(), j.Req.MaxHosts.Int()))

	in := make(chan string, workers*2)
	out := make(chan Row, workers*2)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ip := range in {
				if j.Ctx.Err() != nil {
					return
				}
				lat, status, err := probeXray(j.Ctx, j, ip, time.Duration(j.Req.Timeout.Int())*time.Second)
				if err == nil {
					_ = status
					row := Row{IP: ip, LatencyMS: lat, Phase: "SCAN"}
					jobScanResult(j, ip, &row, true)
					logLine(logf, fmt.Sprintf("OK %s %.1f ms", ip, lat))
					out <- row
				} else {
					done := jobScanResult(j, ip, nil, false)
					if done%250 == 0 {
						logLine(logf, fmt.Sprintf("SCAN progress: %d/%d", done, len(targets)))
					}
				}
			}
		}()
	}

	go func() {
		defer func() {
			close(in)
			wg.Wait()
			close(out)
		}()
		for _, ip := range targets {
			select {
			case <-j.Ctx.Done():
				return
			case in <- ip:
			}
		}
	}()

	rows := []Row{}
	for r := range out {
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].LatencyMS < rows[j].LatencyMS })
	return rows
}

func recheckTargets(j *Job, logf *os.File, rows []Row) []Row {
	samples := clampInt(j.Req.RecheckSamples.Int(), 1, maxRecheckSamples)
	workers := clampInt(j.Req.RecheckWorkers.Int(), 1, maxRecheckWorkers)
	if workers > len(rows) && len(rows) > 0 {
		workers = len(rows)
	}
	jobSetRecheckTotal(j, len(rows))
	logLine(logf, fmt.Sprintf("[go] re-check workers=%d samples=%d pending=%d", workers, samples, len(rows)))

	type outcome struct {
		row       Row
		completed bool
		panicText string
	}
	jobs := make(chan Row, max(1, workers*2))
	out := make(chan outcome, max(1, workers*2))
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for row := range jobs {
				if j.Ctx.Err() != nil {
					return
				}
				res := outcome{row: row, completed: true}
				func() {
					defer func() {
						if v := recover(); v != nil {
							res.completed = true
							res.panicText = fmt.Sprintf("worker=%d ip=%s panic=%v", workerID, row.IP, v)
						}
					}()
					sum := 0.0
					pass := 0
					for sample := 0; sample < samples; sample++ {
						if j.Ctx.Err() != nil {
							res.completed = false
							return
						}
						lat, _, err := probeXray(j.Ctx, j, row.IP, time.Duration(j.Req.Timeout.Int())*time.Second)
						if err == nil {
							sum += lat
							pass++
						}
					}
					if pass > 0 {
						res.row.AvgLatencyMS = sum / float64(pass)
					}
					res.row.RecheckPassed = pass
					res.row.RecheckTotal = samples
					res.row.Phase = "RE-CHECK"
				}()
				select {
				case <-j.Ctx.Done():
					return
				case out <- res:
				}
			}
		}(w + 1)
	}

	go func() {
		defer close(jobs)
		for _, row := range rows {
			select {
			case <-j.Ctx.Done():
				return
			case jobs <- row:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(out)
	}()

	completedRows := make([]Row, 0, len(rows))
	for res := range out {
		if res.panicText != "" {
			logLine(logf, "[go] re-check recovered "+res.panicText)
		}
		if !res.completed {
			continue
		}
		done := jobRecheckResult(j, res.row)
		completedRows = append(completedRows, res.row)
		_, total := jobRecheckProgress(j)
		if done%5 == 0 || done == total {
			logLine(logf, fmt.Sprintf("RE progress: %d/%d", done, total))
		}
		if res.row.RecheckPassed > 0 {
			logLine(logf, fmt.Sprintf("RE-OK %s latency_avg=%.1f ms pass=%d/%d", res.row.IP, res.row.AvgLatencyMS, res.row.RecheckPassed, samples))
		}
	}

	sort.Slice(completedRows, func(i, k int) bool {
		if completedRows[i].AvgLatencyMS == 0 {
			return false
		}
		if completedRows[k].AvgLatencyMS == 0 {
			return true
		}
		return completedRows[i].AvgLatencyMS < completedRows[k].AvgLatencyMS
	})
	return completedRows
}

func speedTestTargets(j *Job, logf *os.File, rows []Row) []Row {
	workers := clampInt(j.Req.SpeedWorkers.Int(), 1, maxSpeedWorkers)
	if workers > len(rows) && len(rows) > 0 {
		workers = len(rows)
	}
	jobSetSpeedTotal(j, len(rows))
	logLine(logf, fmt.Sprintf("[go] speed source=%s workers=%d duration=%ds size=%dMB timeout=%ds", j.Req.SpeedSource, workers, j.Req.SpeedDuration.Int(), j.Req.SpeedMB.Int(), j.Req.SpeedTimeout.Int()))

	jobs := make(chan int, workers*2)
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if j.Ctx.Err() != nil {
					return
				}
				speed, detail, err := speedTestXray(j.Ctx, j, rows[i].IP)
				if err == nil && speed > 0 {
					rows[i].SpeedMbps = speed
					logLine(logf, fmt.Sprintf("SPD %s %.2f Mbps %s", rows[i].IP, speed, detail))
				} else if err != nil {
					logLine(logf, fmt.Sprintf("SPD-FAIL %s %s", rows[i].IP, err.Error()))
				}
				rows[i].Phase = "SPEED"
				done := jobSpeedResult(j, rows[i])
				if done%20 == 0 || done == len(rows) {
					logLine(logf, fmt.Sprintf("Speed progress: %d/%d", done, len(rows)))
				}
			}
		}()
	}

	for i := range rows {
		select {
		case <-j.Ctx.Done():
			close(jobs)
			wg.Wait()
			sort.Slice(rows, func(i, j int) bool { return rows[i].SpeedMbps > rows[j].SpeedMbps })
			return rows
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()

	sort.Slice(rows, func(i, j int) bool { return rows[i].SpeedMbps > rows[j].SpeedMbps })
	return rows
}

func firstVLESS(raw string) (string, error) {
	for _, part := range strings.Fields(raw) {
		if strings.HasPrefix(strings.ToLower(part), "vless://") {
			return part, nil
		}
	}
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(raw), "vless://") {
		return raw, nil
	}
	return "", fmt.Errorf("no vless:// config found")
}

func parseVLESS(raw string) (*VLESSConfig, error) {
	line, err := firstVLESS(raw)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(line)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(u.Scheme) != "vless" {
		return nil, fmt.Errorf("unsupported config scheme: %s", u.Scheme)
	}

	q := u.Query()
	port := 443
	if u.Port() != "" {
		if p, err := strconv.Atoi(u.Port()); err == nil && p > 0 {
			port = p
		}
	}

	network := q.Get("type")
	if network == "" {
		network = "tcp"
	}

	security := q.Get("security")
	if security == "" {
		security = "none"
	}

	sni := q.Get("sni")
	if sni == "" {
		sni = q.Get("serverName")
	}
	if sni == "" {
		sni = u.Hostname()
	}

	return &VLESSConfig{
		ID:          u.User.Username(),
		Port:        port,
		Network:     network,
		Security:    security,
		SNI:         sni,
		HostHeader:  q.Get("host"),
		Path:        q.Get("path"),
		Flow:        q.Get("flow"),
		Fingerprint: q.Get("fp"),
		PublicKey:   q.Get("pbk"),
		ShortID:     q.Get("sid"),
		SpiderX:     q.Get("spx"),
		ServiceName: q.Get("serviceName"),
		Authority:   q.Get("authority"),
		Mode:        q.Get("mode"),
		RawQuery:    q,
	}, nil
}

func buildXrayConfig(req StartRequest, ip string, localPort int) ([]byte, error) {
	vc, err := parseVLESS(req.Config)
	if err != nil {
		return nil, err
	}
	if vc.ID == "" {
		return nil, fmt.Errorf("missing VLESS UUID")
	}

	user := map[string]any{
		"id":         vc.ID,
		"encryption": "none",
	}
	if vc.Flow != "" {
		user["flow"] = vc.Flow
	}

	outbound := map[string]any{
		"protocol": "vless",
		"settings": map[string]any{
			"vnext": []any{
				map[string]any{
					"address": ip,
					"port":    vc.Port,
					"users":   []any{user},
				},
			},
		},
	}

	stream := map[string]any{
		"network": vc.Network,
	}
	if vc.Security != "" && vc.Security != "none" {
		stream["security"] = vc.Security
	}

	switch vc.Security {
	case "tls":
		tlsSettings := map[string]any{"serverName": vc.SNI, "allowInsecure": false}
		if vc.Fingerprint != "" {
			tlsSettings["fingerprint"] = vc.Fingerprint
		}
		stream["tlsSettings"] = tlsSettings
	case "reality":
		realitySettings := map[string]any{"serverName": vc.SNI}
		if vc.Fingerprint != "" {
			realitySettings["fingerprint"] = vc.Fingerprint
		}
		if vc.PublicKey != "" {
			realitySettings["publicKey"] = vc.PublicKey
		}
		if vc.ShortID != "" {
			realitySettings["shortId"] = vc.ShortID
		}
		if vc.SpiderX != "" {
			realitySettings["spiderX"] = vc.SpiderX
		}
		stream["realitySettings"] = realitySettings
	}

	switch vc.Network {
	case "ws":
		ws := map[string]any{}
		if vc.Path != "" {
			ws["path"] = vc.Path
		}
		if vc.HostHeader != "" {
			ws["headers"] = map[string]any{"Host": vc.HostHeader}
		}
		stream["wsSettings"] = ws
	case "grpc":
		grpc := map[string]any{}
		if vc.ServiceName != "" {
			grpc["serviceName"] = vc.ServiceName
		}
		if vc.Mode == "multi" {
			grpc["multiMode"] = true
		}
		if vc.Authority != "" {
			grpc["authority"] = vc.Authority
		}
		stream["grpcSettings"] = grpc
	case "httpupgrade":
		hu := map[string]any{}
		if vc.Path != "" {
			hu["path"] = vc.Path
		}
		if vc.HostHeader != "" {
			hu["host"] = vc.HostHeader
		}
		stream["httpupgradeSettings"] = hu
	case "splithttp", "xhttp":
		xh := map[string]any{}
		if vc.Path != "" {
			xh["path"] = vc.Path
		}
		if vc.HostHeader != "" {
			xh["host"] = vc.HostHeader
		}
		if vc.Mode != "" {
			xh["mode"] = vc.Mode
		}
		stream["xhttpSettings"] = xh
	}

	outbound["streamSettings"] = stream

	cfg := map[string]any{
		"log": map[string]any{"loglevel": safeLogLevel(req.LogLevel)},
		"inbounds": []any{
			map[string]any{
				"listen":   "127.0.0.1",
				"port":     localPort,
				"protocol": "http",
				"settings": map[string]any{"timeout": 30},
			},
		},
		"outbounds": []any{outbound},
	}

	return json.Marshal(cfg)
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func waitForPort(ctx context.Context, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c, err := net.DialTimeout("tcp", addr, 180*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}

	return fmt.Errorf("xray local proxy did not start")
}

func startXray(ctx context.Context, j *Job, ip string, timeout time.Duration) (*exec.Cmd, int, string, error) {
	port, err := freePort()
	if err != nil {
		return nil, 0, "", err
	}

	cfg, err := buildXrayConfig(j.Req, ip, port)
	if err != nil {
		return nil, 0, "", err
	}

	cfgPath := filepath.Join(j.Dir, "xray_"+safeFileName(ip)+"_"+strconv.Itoa(port)+".json")
	if err := os.WriteFile(cfgPath, cfg, 0600); err != nil {
		return nil, 0, "", err
	}

	xctx, _ := context.WithCancel(ctx)
	cmd := exec.CommandContext(xctx, xrayPath, "run", "-config", cfgPath)
	cmd.Dir = baseDir

	logPath := "discarded"
	var logFile *os.File
	level := safeLogLevel(j.Req.LogLevel)
	if j.Req.KeepConfigs || level == "debug" || level == "info" {
		logPath = filepath.Join(j.Dir, "xray_"+safeFileName(ip)+"_"+strconv.Itoa(port)+".log")
		logFile, _ = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	}
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}

	if err := cmd.Start(); err != nil {
		if logFile != nil {
			_ = logFile.Close()
		}
		if !j.Req.KeepConfigs {
			_ = os.Remove(cfgPath)
		}
		return nil, 0, "", err
	}

	startupTimeout := clampDuration(timeout/2, 1800*time.Millisecond, 5*time.Second)
	if err := waitForPort(ctx, port, startupTimeout); err != nil {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		if logFile != nil {
			_ = logFile.Close()
		}
		if !j.Req.KeepConfigs {
			_ = os.Remove(cfgPath)
		}
		return nil, 0, logPath, err
	}

	if !j.Req.KeepConfigs {
		_ = os.Remove(cfgPath)
	}

	go func() {
		_ = cmd.Wait()
		if logFile != nil {
			_ = logFile.Close()
		}
	}()

	return cmd, port, logPath, nil
}

func stopXray(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}

func probeXray(ctx context.Context, j *Job, ip string, timeout time.Duration) (float64, int, error) {
	if timeout <= 0 {
		timeout = defaultTimeoutSeconds * time.Second
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout+3*time.Second)
	defer cancel()
	cmd, port, logPath, err := startXray(probeCtx, j, ip, timeout)
	if err != nil {
		return 0, 0, err
	}
	defer stopXray(cmd)

	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyURL(proxyURL),
			ForceAttemptHTTP2:     true,
			ResponseHeaderTimeout: timeout,
			IdleConnTimeout:       2 * time.Second,
			TLSHandshakeTimeout:   timeout,
		},
		Timeout: timeout,
	}

	var lastErr error
	for _, target := range latencyProbeURLs(j.Req.URL) {
		if probeCtx.Err() != nil {
			return 0, 0, probeCtx.Err()
		}
		req, err := http.NewRequestWithContext(probeCtx, "GET", target, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "RKh-CFS-Android-Xray-Probe/1.0")
		req.Header.Set("Cache-Control", "no-cache")

		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", target, err)
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()

		lat := float64(time.Since(start).Microseconds()) / 1000.0
		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			return lat, resp.StatusCode, nil
		}
		lastErr = fmt.Errorf("%s: bad HTTP status through xray: %d", target, resp.StatusCode)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no latency probe URL available")
	}
	return 0, 0, fmt.Errorf("xray request failed: %w; log=%s", lastErr, logPath)
}

func speedTestXray(ctx context.Context, j *Job, ip string) (float64, string, error) {
	durationSeconds := clampInt(j.Req.SpeedDuration.Int(), 1, maxSpeedDurationSeconds)
	measurementWindow := time.Duration(durationSeconds) * time.Second

	timeoutSeconds := clampInt(j.Req.SpeedTimeout.Int(), durationSeconds+8, maxSpeedTimeoutSeconds)
	hardTimeout := time.Duration(timeoutSeconds) * time.Second

	cmd, localPort, logPath, err := startXray(ctx, j, ip, hardTimeout)
	if err != nil {
		return 0, "", err
	}
	defer stopXray(cmd)

	targetBytes := int64(clampInt(j.Req.SpeedMB.Int(), 1, maxSpeedMB)) * 1024 * 1024
	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", localPort))

	newTransport := func(closeConnections bool) *http.Transport {
		return &http.Transport{
			Proxy:                 http.ProxyURL(proxyURL),
			ForceAttemptHTTP2:     false,
			DisableCompression:    true,
			DisableKeepAlives:     closeConnections,
			MaxIdleConns:          2,
			MaxIdleConnsPerHost:   1,
			MaxConnsPerHost:       1,
			ResponseHeaderTimeout: clampDuration(hardTimeout/2, 4*time.Second, 12*time.Second),
			TLSHandshakeTimeout:   clampDuration(hardTimeout/2, 4*time.Second, 12*time.Second),
			IdleConnTimeout:       12 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}

	transport := newTransport(false)
	defer func() { transport.CloseIdleConnections() }()
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	type endpoint struct {
		name string
		url  func(int64, int64) string
	}
	var selected endpoint
	switch j.Req.SpeedSource {
	case "speed.cloudflare.com":
		selected = endpoint{
			name: "speed.cloudflare.com",
			url: func(n, nonce int64) string {
				return fmt.Sprintf("https://speed.cloudflare.com/__down?bytes=%d&cb=%d", n, nonce)
			},
		}
	case "cdnjs.cloudflare.com":
		selected = endpoint{
			name: "cdnjs.cloudflare.com",
			url: func(_ int64, nonce int64) string {
				return fmt.Sprintf("https://cdnjs.cloudflare.com/ajax/libs/three.js/r128/three.min.js?cb=%d", nonce)
			},
		}
	default:
		selected = endpoint{
			name: "gstatic",
			url: func(_ int64, nonce int64) string {
				return fmt.Sprintf("https://www.gstatic.com/charts/loader.js?cb=%d", nonce)
			},
		}
	}
	endpoints := []endpoint{selected}

	// Use only the source selected by the user. Repeated small requests keep
	// the test usable on fragile VLESS routes without silently changing hosts.
	chunkSizes := []int64{256 * 1024, 128 * 1024, 64 * 1024, 32 * 1024}
	chunkIndex := 0
	endpointIndex := 0
	consecutiveFailures := 0
	var totalBytes int64
	var measureStarted time.Time
	var lastDataAt time.Time
	var lastSource string
	var lastErr error
	failureNotes := make([]string, 0, 6)
	hardDeadline := time.Now().Add(hardTimeout)

	for totalBytes < targetBytes && time.Now().Before(hardDeadline) {
		if ctx.Err() != nil {
			return 0, "", ctx.Err()
		}
		if !measureStarted.IsZero() && time.Since(measureStarted) >= measurementWindow {
			break
		}

		chunkBytes := chunkSizes[chunkIndex]
		if remaining := targetBytes - totalBytes; remaining < chunkBytes {
			chunkBytes = remaining
		}
		if chunkBytes < 32*1024 {
			chunkBytes = 32 * 1024
		}

		ep := endpoints[endpointIndex%len(endpoints)]
		speedURL := ep.url(chunkBytes, time.Now().UnixNano())

		requestBudget := clampDuration(hardTimeout/5, 3*time.Second, 6*time.Second)
		if remaining := time.Until(hardDeadline); remaining < requestBudget {
			requestBudget = remaining
		}
		if !measureStarted.IsZero() {
			if remaining := measurementWindow - time.Since(measureStarted); remaining > 0 && remaining < requestBudget {
				requestBudget = remaining + 1200*time.Millisecond
			}
		}
		if requestBudget <= 0 {
			break
		}

		reqCtx, cancel := context.WithTimeout(ctx, requestBudget)
		req, reqErr := http.NewRequestWithContext(reqCtx, http.MethodGet, speedURL, nil)
		if reqErr != nil {
			cancel()
			lastErr = reqErr
			break
		}
		req.Header.Set("User-Agent", "SIMORGH-Scanner/0.3.0")
		req.Header.Set("Accept", "application/octet-stream,*/*")
		req.Header.Set("Accept-Encoding", "identity")
		req.Header.Set("Cache-Control", "no-cache, no-store, max-age=0")
		req.Header.Set("Pragma", "no-cache")

		requestStarted := time.Now()
		resp, requestErr := client.Do(req)
		if requestErr != nil {
			cancel()
			lastErr = fmt.Errorf("%s: %w", ep.name, requestErr)
			if len(failureNotes) < cap(failureNotes) {
				failureNotes = append(failureNotes, lastErr.Error())
			}
			consecutiveFailures++
			if chunkIndex < len(chunkSizes)-1 {
				chunkIndex++
			}
			endpointIndex++
			if consecutiveFailures >= 2 {
				transport.CloseIdleConnections()
			}
			if consecutiveFailures >= 4 && !transport.DisableKeepAlives {
				transport.CloseIdleConnections()
				transport = newTransport(true)
				client.Transport = transport
			}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			_ = resp.Body.Close()
			cancel()
			lastErr = fmt.Errorf("%s: HTTP %d", ep.name, resp.StatusCode)
			if len(failureNotes) < cap(failureNotes) {
				failureNotes = append(failureNotes, lastErr.Error())
			}
			consecutiveFailures++
			endpointIndex++
			continue
		}

		buf := make([]byte, 32*1024)
		var requestBytes int64
		for requestBytes < chunkBytes {
			readBuf := buf
			if remaining := chunkBytes - requestBytes; remaining < int64(len(readBuf)) {
				readBuf = readBuf[:remaining]
			}
			n, readErr := resp.Body.Read(readBuf)
			if n > 0 {
				now := time.Now()
				if measureStarted.IsZero() {
					// Include request setup for the first successful chunk, but exclude
					// failed endpoint attempts that transferred no data.
					measureStarted = requestStarted
				}
				requestBytes += int64(n)
				totalBytes += int64(n)
				lastDataAt = now
				lastSource = ep.name
			}
			if readErr != nil {
				if readErr != io.EOF && requestBytes == 0 {
					lastErr = fmt.Errorf("%s read: %w", ep.name, readErr)
				}
				break
			}
			if !measureStarted.IsZero() && time.Since(measureStarted) >= measurementWindow {
				break
			}
			if totalBytes >= targetBytes {
				break
			}
		}
		_ = resp.Body.Close()
		cancel()

		if requestBytes >= 16*1024 {
			consecutiveFailures = 0
			// Grow chunks again after a healthy transfer, but never jump directly
			// back to the largest request size.
			if requestBytes >= chunkBytes*3/4 && chunkIndex > 1 {
				chunkIndex--
			}
		} else {
			consecutiveFailures++
			if chunkIndex < len(chunkSizes)-1 {
				chunkIndex++
			}
			endpointIndex++
		}
	}

	if totalBytes >= 16*1024 && !measureStarted.IsZero() {
		if lastDataAt.IsZero() {
			lastDataAt = time.Now()
		}
		elapsed := lastDataAt.Sub(measureStarted).Seconds()
		if elapsed < 0.20 {
			elapsed = time.Since(measureStarted).Seconds()
		}
		if elapsed > 0 {
			mbps := float64(totalBytes) * 8 / elapsed / 1_000_000
			detail := fmt.Sprintf("[%d bytes / %.2fs / %s]", totalBytes, elapsed, lastSource)
			return mbps, detail, nil
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no speed data received")
	}
	note := strings.Join(failureNotes, " | ")
	if note != "" {
		return 0, "", fmt.Errorf("speed test through xray failed: %w; attempts=%s; log=%s", lastErr, note, logPath)
	}
	return 0, "", fmt.Errorf("speed test through xray failed: %w; log=%s", lastErr, logPath)
}

func safeFileName(s string) string {
	return regexp.MustCompile(`[^A-Za-z0-9_.-]`).ReplaceAllString(s, "_")
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func clampDuration(v, minV, maxV time.Duration) time.Duration {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func safeLogLevel(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "debug", "info", "warning", "error", "none":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "warning"
	}
}

func latencyProbeURLs(primary string) []string {
	seen := map[string]bool{}
	urls := []string{}
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		urls = append(urls, v)
	}
	add(primary)
	add("https://www.gstatic.com/generate_204")
	add("https://cp.cloudflare.com/generate_204")
	add("https://edge.microsoft.com/captiveportal/generate_204")
	add("https://connectivitycheck.gstatic.com/generate_204")
	return urls
}

var logWriteMu sync.Mutex

func phaseProgress(stage string, done, total int) PhaseProgress {
	if done < 0 {
		done = 0
	}
	if total < 0 {
		total = 0
	}
	if total > 0 && done > total {
		done = total
	}
	pct := 0
	if total > 0 {
		pct = int(float64(done) / float64(total) * 100)
		if pct > 100 {
			pct = 100
		}
	}
	return PhaseProgress{Done: done, Total: total, Remaining: max(0, total-done), Percent: pct, Stage: stage}
}

func jobInitProgress(j *Job, total int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.Progress.TargetTotal > 0 {
		return
	}
	j.Progress = Progress{
		Done:        0,
		Total:       total,
		Remaining:   total,
		Percent:     0,
		Stage:       "scan",
		TargetTotal: total,
		Scan:        phaseProgress("scan", 0, total),
		Recheck:     phaseProgress("waiting", 0, 0),
		Speed:       phaseProgress("waiting", 0, 0),
	}
	if j.Rows == nil {
		j.Rows = map[string]*Row{}
	}
	if j.ScanDone == nil {
		j.ScanDone = map[string]bool{}
	}
	if j.RecheckDone == nil {
		j.RecheckDone = map[string]bool{}
	}
	if j.SpeedDone == nil {
		j.SpeedDone = map[string]bool{}
	}
}

func upsertJobRowLocked(j *Job, row Row) {
	if j.Rows == nil {
		j.Rows = map[string]*Row{}
	}
	r := j.Rows[row.IP]
	if r == nil {
		cp := row
		j.Rows[row.IP] = &cp
		return
	}
	if row.LatencyMS > 0 {
		r.LatencyMS = row.LatencyMS
	}
	if row.AvgLatencyMS > 0 {
		r.AvgLatencyMS = row.AvgLatencyMS
	}
	if row.RecheckTotal > 0 {
		r.RecheckPassed = row.RecheckPassed
		r.RecheckTotal = row.RecheckTotal
	}
	if row.SpeedMbps > 0 {
		r.SpeedMbps = row.SpeedMbps
	}
	if row.Phase != "" {
		r.Phase = row.Phase
	}
}

func jobScanResult(j *Job, ip string, row *Row, ok bool) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.ScanDone == nil {
		j.ScanDone = map[string]bool{}
	}
	if j.ScanDone[ip] {
		if ok && row != nil {
			upsertJobRowLocked(j, *row)
		}
		return j.Progress.Scan.Done
	}
	j.ScanDone[ip] = true
	j.Progress.Stage = "scan"
	j.Progress.Scan.Stage = "scan"
	j.Progress.Scan.Done++
	j.Progress.Scan = phaseProgress("scan", j.Progress.Scan.Done, j.Progress.Scan.Total)
	j.Progress.Done = j.Progress.Scan.Done
	j.Progress.Total = j.Progress.TargetTotal
	j.Progress.Remaining = j.Progress.Scan.Remaining
	j.Progress.Percent = j.Progress.Scan.Percent
	if ok && row != nil {
		j.Progress.CleanCount++
		upsertJobRowLocked(j, *row)
	} else {
		j.Progress.FailCount++
	}
	return j.Progress.Scan.Done
}

func jobSetRecheckTotal(j *Job, total int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.RecheckDone == nil {
		j.RecheckDone = map[string]bool{}
	}
	j.Progress.Stage = "recheck"
	expectedTotal := len(j.RecheckDone) + total
	j.Progress.Recheck = phaseProgress("recheck", len(j.RecheckDone), expectedTotal)
	j.Progress.Percent = j.Progress.Recheck.Percent
}

func jobRecheckResult(j *Job, row Row) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.RecheckDone == nil {
		j.RecheckDone = map[string]bool{}
	}
	if !j.RecheckDone[row.IP] {
		j.RecheckDone[row.IP] = true
		j.Progress.Recheck.Done++
	}
	j.Progress.Stage = "recheck"
	j.Progress.Recheck = phaseProgress("recheck", j.Progress.Recheck.Done, j.Progress.Recheck.Total)
	j.Progress.RecheckCount = j.Progress.Recheck.Done
	j.Progress.Done = j.Progress.Recheck.Done
	j.Progress.Total = j.Progress.Recheck.Total
	j.Progress.Remaining = j.Progress.Recheck.Remaining
	j.Progress.Percent = j.Progress.Recheck.Percent
	upsertJobRowLocked(j, row)
	return j.Progress.Recheck.Done
}

func jobRecheckProgress(j *Job) (int, int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.Progress.Recheck.Done, j.Progress.Recheck.Total
}

func jobSetSpeedTotal(j *Job, total int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.SpeedDone == nil {
		j.SpeedDone = map[string]bool{}
	}
	j.Progress.Stage = "speed"
	expectedTotal := len(j.SpeedDone) + total
	j.Progress.Speed = phaseProgress("speed", len(j.SpeedDone), expectedTotal)
	j.Progress.Percent = j.Progress.Speed.Percent
}

func jobSpeedResult(j *Job, row Row) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.SpeedDone == nil {
		j.SpeedDone = map[string]bool{}
	}
	if !j.SpeedDone[row.IP] {
		j.SpeedDone[row.IP] = true
		j.Progress.Speed.Done++
	}
	j.Progress.Stage = "speed"
	j.Progress.Speed = phaseProgress("speed", j.Progress.Speed.Done, j.Progress.Speed.Total)
	j.Progress.SpeedCount = j.Progress.Speed.Done
	j.Progress.Done = j.Progress.Speed.Done
	j.Progress.Total = j.Progress.Speed.Total
	j.Progress.Remaining = j.Progress.Speed.Remaining
	j.Progress.Percent = j.Progress.Speed.Percent
	upsertJobRowLocked(j, row)
	return j.Progress.Speed.Done
}

func jobFinishPhase(j *Job, name string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	switch name {
	case "scan":
		j.Progress.Scan = phaseProgress("scan", j.Progress.Scan.Total, j.Progress.Scan.Total)
	case "recheck":
		j.Progress.Recheck = phaseProgress("recheck", j.Progress.Recheck.Total, j.Progress.Recheck.Total)
	case "speed":
		j.Progress.Speed = phaseProgress("speed", j.Progress.Speed.Total, j.Progress.Speed.Total)
	}
}

func jobFinished(j *Job) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Progress.Stage = "finished"
	j.Progress.Percent = 100
	if j.Progress.Scan.Total > 0 {
		j.Progress.Scan = phaseProgress("scan", j.Progress.Scan.Total, j.Progress.Scan.Total)
	}
	if j.Progress.Recheck.Total > 0 {
		j.Progress.Recheck = phaseProgress("recheck", j.Progress.Recheck.Total, j.Progress.Recheck.Total)
	}
	if j.Progress.Speed.Total > 0 {
		j.Progress.Speed = phaseProgress("speed", j.Progress.Speed.Total, j.Progress.Speed.Total)
	}
	j.Progress.Done = j.Progress.TargetTotal
	j.Progress.Total = j.Progress.TargetTotal
	j.Progress.Remaining = 0
}

func jobSnapshot(j *Job) (bool, any, Progress, []Event) {
	j.mu.Lock()
	defer j.mu.Unlock()
	running := j.Running
	var rc any = nil
	if j.ReturnCode != nil {
		rc = *j.ReturnCode
	}
	p := j.Progress
	rows := make([]*Row, 0, len(j.Rows))
	for _, r := range j.Rows {
		cp := *r
		rows = append(rows, &cp)
	}
	return running, rc, p, rowsToEvents(rows)
}

func rowsToEvents(list []*Row) []Event {
	sort.Slice(list, func(i, j int) bool {
		li, lj := list[i].LatencyMS, list[j].LatencyMS
		if li == 0 {
			li = list[i].AvgLatencyMS
		}
		if lj == 0 {
			lj = list[j].AvgLatencyMS
		}
		if li == 0 && lj == 0 {
			return list[i].IP < list[j].IP
		}
		if li == 0 {
			return false
		}
		if lj == 0 {
			return true
		}
		return li < lj
	})
	out := []Event{}
	for i, r := range list {
		ev := Event{Rank: i + 1, IP: r.IP, Phase: r.Phase, Latency: "-", AvgLatency: "-", Pass: "-", Speed: "-"}
		if ev.Phase == "" {
			ev.Phase = "SCAN"
		}
		if r.LatencyMS > 0 {
			ev.LatencyValue = r.LatencyMS
			ev.Latency = fmt.Sprintf("%.1f ms", r.LatencyMS)
		}
		if r.AvgLatencyMS > 0 {
			ev.AvgLatencyValue = r.AvgLatencyMS
			ev.AvgLatency = fmt.Sprintf("%.1f ms", r.AvgLatencyMS)
		}
		if r.RecheckTotal > 0 {
			ev.PassValue = r.RecheckPassed
			ev.PassTotal = r.RecheckTotal
			ev.Pass = fmt.Sprintf("%d/%d", r.RecheckPassed, r.RecheckTotal)
		}
		if r.SpeedMbps > 0 {
			ev.SpeedValue = r.SpeedMbps
			ev.Speed = fmt.Sprintf("%.2f Mbps", r.SpeedMbps)
		}
		out = append(out, ev)
	}
	return out
}

func jobAbort(j *Job, stage string, rc int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Running = false
	val := rc
	j.ReturnCode = &val
	j.Progress.Stage = stage
	j.Paused = stage == "stopped"
	j.Cancelled = stage == "cancelled"
	if stage == "stopped" || stage == "cancelled" {
		if j.Progress.Scan.Total > 0 && j.Progress.Scan.Done >= j.Progress.Scan.Total {
			j.Progress.Scan = phaseProgress("scan", j.Progress.Scan.Total, j.Progress.Scan.Total)
		}
	}
}
func handleStatus(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("job")
	if id == "" {
		id = latestJobID()
	}
	jobsMu.Lock()
	j := jobs[id]
	jobsMu.Unlock()
	if j != nil {
		running, rc, progress, events := jobSnapshot(j)
		writeJSON(w, map[string]any{"job_id": id, "running": running, "returncode": rc, "events": events, "progress": progress, "files": resultFiles(id)})
		return
	}
	logText := readTail(filepath.Join(jobsDir, id, "job.log"), 1200000)
	events := parseEvents(logText, filepath.Join(resultsDir, id))
	progress := progressFromEvents(logText, events)
	writeJSON(w, map[string]any{"job_id": id, "running": false, "returncode": nil, "events": events, "progress": progress, "files": resultFiles(id)})
}
func parseEvents(logText, resultDir string) []Event {
	rows := map[string]*Row{}
	get := func(ip string) *Row {
		if rows[ip] == nil {
			rows[ip] = &Row{IP: ip, Phase: "SCAN"}
		}
		return rows[ip]
	}
	ipRe := `((?:\d{1,3}\.){3}\d{1,3})`

	// Only successful IPs become Live Ranking events.
	// FAIL lines are counted in progressFromEvents, but never returned as events.
	for _, m := range regexp.MustCompile(`\bOK\s+`+ipRe+`\s+([0-9.]+)\s*ms`).FindAllStringSubmatch(logText, -1) {
		r := get(m[1])
		r.LatencyMS = atof(m[2])
		r.Phase = "SCAN"
	}
	for _, m := range regexp.MustCompile(ipRe+`.{0,100}?latency_avg=([0-9.]+)\s*ms.{0,80}?pass=(\d+)\s*/\s*(\d+)`).FindAllStringSubmatch(logText, -1) {
		r := get(m[1])
		r.AvgLatencyMS = atof(m[2])
		r.RecheckPassed = atoi(m[3])
		r.RecheckTotal = atoi(m[4])
		r.Phase = "RE-CHECK"
	}
	for _, m := range regexp.MustCompile(`\bSPD\s+`+ipRe+`\s+([0-9.]+)\s*Mbps`).FindAllStringSubmatch(logText, -1) {
		r := get(m[1])
		r.SpeedMbps = atof(m[2])
		r.Phase = "SPEED"
	}

	mergeCSV(rows, resultDir)

	list := make([]*Row, 0, len(rows))
	for _, r := range rows {
		if r.LatencyMS > 0 || r.AvgLatencyMS > 0 || r.SpeedMbps > 0 || r.RecheckTotal > 0 {
			list = append(list, r)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		li, lj := list[i].LatencyMS, list[j].LatencyMS
		if li == 0 {
			li = list[i].AvgLatencyMS
		}
		if lj == 0 {
			lj = list[j].AvgLatencyMS
		}
		if li == 0 && lj == 0 {
			return list[i].IP < list[j].IP
		}
		if li == 0 {
			return false
		}
		if lj == 0 {
			return true
		}
		return li < lj
	})
	out := []Event{}
	for i, r := range list {
		ev := Event{Rank: i + 1, IP: r.IP, Phase: r.Phase, Latency: "-", AvgLatency: "-", Pass: "-", Speed: "-"}
		if r.LatencyMS > 0 {
			ev.LatencyValue = r.LatencyMS
			ev.Latency = fmt.Sprintf("%.1f ms", r.LatencyMS)
		}
		if r.AvgLatencyMS > 0 {
			ev.AvgLatencyValue = r.AvgLatencyMS
			ev.AvgLatency = fmt.Sprintf("%.1f ms", r.AvgLatencyMS)
		}
		if r.RecheckTotal > 0 {
			ev.PassValue = r.RecheckPassed
			ev.PassTotal = r.RecheckTotal
			ev.Pass = fmt.Sprintf("%d/%d", r.RecheckPassed, r.RecheckTotal)
		}
		if r.SpeedMbps > 0 {
			ev.SpeedValue = r.SpeedMbps
			ev.Speed = fmt.Sprintf("%.2f Mbps", r.SpeedMbps)
		}
		out = append(out, ev)
	}
	return out
}

func mergeCSV(rows map[string]*Row, dir string) {
	for _, name := range []string{"clean_ips.csv", "clean_ips_rechecked.csv", "clean_ips_speed_tested.csv"} {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		recs, _ := csv.NewReader(f).ReadAll()
		_ = f.Close()
		if len(recs) < 2 {
			continue
		}
		head := map[string]int{}
		for i, h := range recs[0] {
			head[h] = i
		}
		for _, rec := range recs[1:] {
			ip := csvVal(rec, head, "ip")
			if ip == "" {
				continue
			}
			r := rows[ip]
			if r == nil {
				r = &Row{IP: ip}
				rows[ip] = r
			}
			if v := atof(csvVal(rec, head, "latency_ms")); v > 0 {
				r.LatencyMS = v
			}
			if v := atof(csvVal(rec, head, "avg_latency_ms")); v > 0 {
				r.AvgLatencyMS = v
			}
			if v := atoi(csvVal(rec, head, "recheck_passed")); v > 0 {
				r.RecheckPassed = v
			}
			if v := atoi(csvVal(rec, head, "recheck_total")); v > 0 {
				r.RecheckTotal = v
			}
			if v := atof(csvVal(rec, head, "speed_mbps")); v > 0 {
				r.SpeedMbps = v
			}
		}
	}
}
func csvVal(rec []string, head map[string]int, k string) string {
	i, ok := head[k]
	if !ok || i >= len(rec) {
		return ""
	}
	return rec[i]
}
func progressFromEvents(logText string, events []Event) Progress {
	stage := "scan"
	if strings.Contains(logText, "Speed progress") || strings.Contains(logText, "SPD ") {
		stage = "speed"
	} else if strings.Contains(logText, "RE-OK") || strings.Contains(logText, "latency_avg=") || strings.Contains(logText, "RE progress") {
		stage = "recheck"
	}

	clean, recheck, speed := 0, 0, 0
	for _, e := range events {
		if e.LatencyValue > 0 {
			clean++
		}
		if e.PassTotal > 0 {
			recheck++
		}
		if e.SpeedValue > 0 {
			speed++
		}
	}
	fail := len(regexp.MustCompile(`\bFAIL\s+((?:\d{1,3}\.){3}\d{1,3})\s+`).FindAllStringSubmatch(logText, -1))
	attempted := clean + fail

	total := 0
	if m := regexp.MustCompile(`Loaded\s+(\d+)\s+targets?`).FindStringSubmatch(logText); len(m) > 1 {
		total = atoi(m[1])
	}

	scanP := phaseProgress("scan", attempted, total)
	recheckP := phaseProgress("waiting", 0, 0)
	speedP := phaseProgress("waiting", 0, 0)
	if recheck > 0 || stage == "recheck" || stage == "speed" {
		recheckP = phaseProgress("recheck", recheck, clean)
	}
	if speed > 0 || stage == "speed" {
		speedP = phaseProgress("speed", speed, clean)
	}

	pct := scanP.Percent
	done := scanP.Done
	shownTotal := scanP.Total
	if stage == "recheck" {
		pct = recheckP.Percent
		done = recheckP.Done
		shownTotal = recheckP.Total
	}
	if stage == "speed" {
		pct = speedP.Percent
		done = speedP.Done
		shownTotal = speedP.Total
	}
	if strings.Contains(logText, "job finished") {
		pct = 100
		stage = "finished"
		if total > 0 {
			done = total
			shownTotal = total
		}
		scanP = phaseProgress("scan", total, total)
		if recheckP.Total > 0 {
			recheckP = phaseProgress("recheck", recheckP.Total, recheckP.Total)
		}
		if speedP.Total > 0 {
			speedP = phaseProgress("speed", speedP.Total, speedP.Total)
		}
	}

	return Progress{Done: done, Total: shownTotal, Remaining: max(0, shownTotal-done), Percent: pct, Stage: stage, TargetTotal: total, CleanCount: clean, RecheckCount: recheck, SpeedCount: speed, FailCount: fail, Scan: scanP, Recheck: recheckP, Speed: speedP}
}

func readJobID(r *http.Request) string {
	id := r.URL.Query().Get("job")
	if id == "" {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		id = body["job_id"]
	}
	return id
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	id := readJobID(r)
	jobsMu.Lock()
	j := jobs[id]
	jobsMu.Unlock()
	if j != nil {
		jobAbort(j, "stopped", 130)
		j.Cancel()
	}
	writeJSON(w, map[string]any{"ok": true, "stopped": id})
}

func handleCancel(w http.ResponseWriter, r *http.Request) {
	id := readJobID(r)
	if id == "" {
		writeJSON(w, map[string]any{"ok": true})
		return
	}

	jobsMu.Lock()
	j := jobs[id]
	if j != nil {
		jobAbort(j, "cancelled", 130)
		j.Cancel()
	}
	delete(jobs, id)
	jobsMu.Unlock()

	// Give xray child processes a moment to receive context cancellation,
	// then remove job/results/targets so UI starts clean.
	go func() {
		time.Sleep(700 * time.Millisecond)
		if j != nil {
			_ = os.RemoveAll(j.Dir)
			_ = os.RemoveAll(j.ResultDir)
		} else {
			_ = os.RemoveAll(filepath.Join(jobsDir, id))
			_ = os.RemoveAll(filepath.Join(resultsDir, id))
		}
		_ = os.Remove(filepath.Join(targetsDir, "targets_"+id+".txt"))
	}()
	writeJSON(w, map[string]any{"ok": true, "cancelled": id})
}

func handleContinue(w http.ResponseWriter, r *http.Request) {
	id := readJobID(r)
	jobsMu.Lock()
	j := jobs[id]
	jobsMu.Unlock()
	if j == nil {
		writeJSON(w, map[string]any{"ok": false, "error": "job not found"})
		return
	}
	j.mu.Lock()
	if j.Running {
		j.mu.Unlock()
		writeJSON(w, map[string]any{"ok": false, "error": "job already running"})
		return
	}
	if !j.Paused {
		j.mu.Unlock()
		writeJSON(w, map[string]any{"ok": false, "error": "job is not paused"})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	j.Ctx = ctx
	j.Cancel = cancel
	j.Running = true
	j.Paused = false
	j.Cancelled = false
	j.ReturnCode = nil
	j.mu.Unlock()
	go runJob(j)
	writeJSON(w, map[string]any{"ok": true, "job_id": id})
}
func handleISPs(w http.ResponseWriter, r *http.Request) {
	cat := r.URL.Query().Get("category")
	if cat != "international" {
		cat = "iran"
	}
	manifest, err := loadISPManifest()
	if err != nil {
		writeJSON(w, map[string]any{"files": []any{}, "error": err.Error()})
		return
	}
	files := []map[string]any{}
	for _, e := range manifest.Files {
		if e.Category != cat {
			continue
		}
		files = append(files, map[string]any{
			"id":          e.ID,
			"stem":        e.ID,
			"name":        e.Name,
			"ip_count":    e.IPCount,
			"range_count": e.RangeCount,
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(fmt.Sprint(files[i]["name"])) < strings.ToLower(fmt.Sprint(files[j]["name"]))
	})
	writeJSON(w, map[string]any{"files": files})
}

func handleLogs(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("job"))
	if id == "" {
		id = latestJobID()
	}
	if id == "" {
		writeJSON(w, map[string]any{"ok": true, "job_id": "", "running": false, "log": "No scan log is available."})
		return
	}
	jobsMu.Lock()
	j := jobs[id]
	jobsMu.Unlock()
	running := false
	stage := ""
	var rc any = nil
	if j != nil {
		j.mu.Lock()
		running = j.Running
		stage = j.Progress.Stage
		if j.ReturnCode != nil {
			rc = *j.ReturnCode
		}
		j.mu.Unlock()
	}
	logText := readTail(filepath.Join(jobsDir, id, "job.log"), 600000)
	writeJSON(w, map[string]any{"ok": true, "job_id": id, "running": running, "stage": stage, "returncode": rc, "log": logText})
}

func handleResults(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("job")
	rows := resultFiles(id)
	body := "<h2>RKh-CFS result files</h2>"
	for _, x := range rows {
		body += fmt.Sprintf("<p><a href='/download?job=%s&file=%s'>%s</a> - %s</p>", id, x.Name, x.Name, x.Size)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte("<!doctype html><body style='background:#100804;color:#fff7ed;font-family:sans-serif'>" + body + "</body>"))
}
func handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("job")
	name := filepath.Base(r.URL.Query().Get("file"))
	http.ServeFile(w, r, filepath.Join(resultsDir, id, name))
}
func resultFiles(id string) []FileInfo {
	dir := filepath.Join(resultsDir, id)
	ents, _ := os.ReadDir(dir)
	out := []FileInfo{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		info, _ := e.Info()
		out = append(out, FileInfo{Name: e.Name(), Size: humanSize(info.Size())})
	}
	return out
}
func writeRows(dir, base string, rows []Row) {
	_ = os.MkdirAll(dir, 0755)

	txt, _ := os.Create(filepath.Join(dir, base+".txt"))
	defer txt.Close()

	cf, _ := os.Create(filepath.Join(dir, base+".csv"))
	defer cf.Close()

	cw := csv.NewWriter(cf)
	_ = cw.Write([]string{"rank", "ip", "latency_ms", "avg_latency_ms", "recheck_passed", "recheck_total", "speed_mbps", "phase"})

	_, _ = fmt.Fprintln(txt, "# RKh-CFS result")
	_, _ = fmt.Fprintln(txt, "# rank | ip | latency_ms | avg_latency_ms | pass | speed_mbps | phase")

	for i, r := range rows {
		pass := "-"
		if r.RecheckTotal > 0 {
			pass = fmt.Sprintf("%d/%d", r.RecheckPassed, r.RecheckTotal)
		}

		lat := "-"
		if r.LatencyMS > 0 {
			lat = fmt.Sprintf("%.1f", r.LatencyMS)
		}

		avg := "-"
		if r.AvgLatencyMS > 0 {
			avg = fmt.Sprintf("%.1f", r.AvgLatencyMS)
		}

		speed := "-"
		if r.SpeedMbps > 0 {
			speed = fmt.Sprintf("%.2f", r.SpeedMbps)
		}

		_, _ = fmt.Fprintf(txt, "%d | %s | %s | %s | %s | %s | %s\n", i+1, r.IP, lat, avg, pass, speed, r.Phase)

		_ = cw.Write([]string{
			strconv.Itoa(i + 1),
			r.IP,
			fmt.Sprintf("%.1f", r.LatencyMS),
			fmt.Sprintf("%.1f", r.AvgLatencyMS),
			strconv.Itoa(r.RecheckPassed),
			strconv.Itoa(r.RecheckTotal),
			fmt.Sprintf("%.2f", r.SpeedMbps),
			r.Phase,
		})
	}

	cw.Flush()
}

func latestJobID() string {
	ents, _ := os.ReadDir(jobsDir)
	best := ""
	var bt time.Time
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		info, _ := e.Info()
		if info.ModTime().After(bt) {
			best = e.Name()
			bt = info.ModTime()
		}
	}
	return best
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
func saveJSON(path string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	_ = os.WriteFile(path, b, 0644)
}
func writeText(path, v string) {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, []byte(v), 0644)
}
func writeLines(path string, lines []string) { writeText(path, strings.Join(lines, "\n")) }
func openAppend(path string) *os.File {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	f, _ := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	return f
}
func logLine(f *os.File, s string) {
	if f == nil {
		return
	}
	logWriteMu.Lock()
	defer logWriteMu.Unlock()
	_, _ = fmt.Fprintln(f, s)
}
func readTail(path string, maxN int) string {
	b, _ := os.ReadFile(path)
	if len(b) > maxN {
		b = b[len(b)-maxN:]
	}
	return string(b)
}
func randHex(n int) string { b := make([]byte, n); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func humanSize(n int64) string {
	if n > 1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(n)/1024/1024)
	}
	if n > 1024 {
		return fmt.Sprintf("%.2f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%d B", n)
}
func atof(s string) float64 { v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64); return v }
func atoi(s string) int     { v, _ := strconv.Atoi(strings.TrimSpace(s)); return v }
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
