package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/hive/internal/libhive"
)

const suitePlanMarker = "HIVE_SUITE_PLAN "
const suiteProgressMarker = "HIVE_SUITE_PROGRESS "
const runHeartbeatMarker = "HIVE_RUN_HEARTBEAT "
const runCompleteMarker = "HIVE_RUN_COMPLETE "
const runInterruptedMarker = "HIVE_RUN_INTERRUPTED "
const runStatusStaleAfter = 30 * time.Second

var simulatorLogPattern = regexp.MustCompile(`^[0-9]+-simulator-.+\.log$`)

type serveRunStatus struct {
	fsys   fs.FS
	logDir string
}

type runStatus struct {
	SimLog       string           `json:"simLog,omitempty"`
	State        string           `json:"state"`
	Start        time.Time        `json:"start,omitempty"`
	LastActivity time.Time        `json:"lastActivity,omitempty"`
	Suites       []suiteRunStatus `json:"suites,omitempty"`
}

type suiteRunStatus struct {
	Name        string     `json:"name"`
	State       string     `json:"state"`
	Finished    int        `json:"finished,omitempty"`
	Total       int        `json:"total,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

type suitePlan struct {
	Suites []suitePlanEntry `json:"suites"`
}

type suitePlanEntry struct {
	Name  string `json:"name"`
	Total int    `json:"tests,omitempty"`
}

func (entry *suitePlanEntry) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		entry.Name = name
		return nil
	}
	var object struct {
		Name  string `json:"name"`
		Total int    `json:"tests"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	entry.Name = object.Name
	entry.Total = object.Total
	return nil
}

type suiteProgress struct {
	Suite string `json:"suite"`
}

type runDigest struct {
	Plan        []suitePlanEntry
	Progress    map[string]int
	Heartbeat   bool
	Complete    bool
	Interrupted bool
}

type completedSuiteStatus struct {
	Finished    int
	CompletedAt time.Time
}

type simulatorLogWriteState int

const (
	simulatorLogWriteUnknown simulatorLogWriteState = iota
	simulatorLogWriteOpen
	simulatorLogWriteClosed
)

type runStatusOptions struct {
	simulatorLogWriteState func(string) simulatorLogWriteState
}

func (h serveRunStatus) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status, err := buildRunStatusWithOptions(h.fsys, runStatusOptions{
		simulatorLogWriteState: h.simulatorLogWriteState,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("cache-control", "no-cache")
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}

func buildRunStatus(fsys fs.FS) (runStatus, error) {
	return buildRunStatusWithOptions(fsys, runStatusOptions{})
}

func buildRunStatusWithOptions(fsys fs.FS, options runStatusOptions) (runStatus, error) {
	return buildRunStatusAtWithOptions(fsys, time.Now(), options)
}

func buildRunStatusAt(fsys fs.FS, now time.Time) (runStatus, error) {
	return buildRunStatusAtWithOptions(fsys, now, runStatusOptions{})
}

func buildRunStatusAtWithOptions(fsys fs.FS, now time.Time, options runStatusOptions) (runStatus, error) {
	simLog, simLogInfo, err := latestSimulatorLog(fsys)
	if errors.Is(err, fs.ErrNotExist) {
		return runStatus{State: "unknown"}, nil
	}
	if err != nil {
		return runStatus{}, err
	}

	status := runStatus{
		SimLog:       simLog,
		State:        "unknown",
		Start:        simLogStartTime(simLog),
		LastActivity: simLogInfo.ModTime(),
	}
	digest, err := readRunDigest(fsys, simLog)
	if err != nil {
		return status, nil
	}
	if len(digest.Plan) == 0 {
		return status, nil
	}
	if digest.Interrupted {
		return runStatus{State: "unknown"}, nil
	}
	if digest.Complete {
		status.State = "finished"
		return status, nil
	}

	completedSuites := make(map[string]completedSuiteStatus)
	_ = walkSummaryFiles(fsys, ".", func(suite *libhive.TestSuite, file fs.FileInfo) error {
		if suite.SimulatorLog != simLog {
			return nil
		}
		completedSuites[suite.Name] = completedSuiteStatus{
			Finished:    countRunStatusTests(suite),
			CompletedAt: file.ModTime(),
		}
		if file.ModTime().After(status.LastActivity) {
			status.LastActivity = file.ModTime()
		}
		return nil
	})

	inProgressAssigned := false
	finishedCount := 0
	status.Suites = make([]suiteRunStatus, 0, len(digest.Plan))
	for _, suite := range digest.Plan {
		state := "pending"
		total := suite.Total
		finished := digest.Progress[suite.Name]
		if finished > total {
			total = finished
		}
		var completedAt *time.Time
		if completed, ok := completedSuites[suite.Name]; ok {
			state = "finished"
			finishedCount++
			finished = completed.Finished
			completedAt = &completed.CompletedAt
			if finished > total {
				total = finished
			}
		} else if !inProgressAssigned {
			state = "in-progress"
			inProgressAssigned = true
		}
		status.Suites = append(status.Suites, suiteRunStatus{
			Name:        suite.Name,
			State:       state,
			Finished:    finished,
			Total:       total,
			CompletedAt: completedAt,
		})
	}

	if finishedCount == len(status.Suites) {
		status.State = "finished"
	} else {
		logWriteState := simulatorLogWriteUnknown
		if options.simulatorLogWriteState != nil {
			logWriteState = options.simulatorLogWriteState(simLog)
		}
		if !runStatusActive(digest, status.LastActivity, now, logWriteState) {
			return runStatus{State: "unknown"}, nil
		}
		status.State = "running"
	}
	return status, nil
}

func latestSimulatorLog(fsys fs.FS) (string, fs.FileInfo, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return "", nil, err
	}
	names := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !simulatorLogPattern.MatchString(entry.Name()) {
			continue
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		return "", nil, fs.ErrNotExist
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	info, err := fs.Stat(fsys, names[0])
	return names[0], info, err
}

func readRunDigest(fsys fs.FS, simLog string) (runDigest, error) {
	file, err := fsys.Open(simLog)
	if err != nil {
		return runDigest{}, err
	}
	defer file.Close()

	digest := runDigest{Progress: make(map[string]int)}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if rawPlan, ok := strings.CutPrefix(line, suitePlanMarker); ok {
			var plan suitePlan
			if err := json.Unmarshal([]byte(rawPlan), &plan); err != nil {
				return runDigest{}, err
			}
			digest.Plan = plan.Suites
			continue
		}
		if rawProgress, ok := strings.CutPrefix(line, suiteProgressMarker); ok {
			var progress suiteProgress
			if err := json.Unmarshal([]byte(rawProgress), &progress); err == nil && progress.Suite != "" {
				digest.Progress[progress.Suite]++
			}
			continue
		}
		if strings.HasPrefix(line, runHeartbeatMarker) {
			digest.Heartbeat = true
			continue
		}
		if strings.HasPrefix(line, runCompleteMarker) {
			digest.Complete = true
			continue
		}
		if strings.HasPrefix(line, runInterruptedMarker) {
			digest.Interrupted = true
		}
	}
	if err := scanner.Err(); err != nil {
		return runDigest{}, err
	}
	return digest, nil
}

func runStatusActive(digest runDigest, lastActivity time.Time, now time.Time, logWriteState simulatorLogWriteState) bool {
	if digest.Complete || digest.Interrupted || lastActivity.IsZero() {
		return false
	}
	switch logWriteState {
	case simulatorLogWriteOpen:
		return true
	case simulatorLogWriteClosed:
		return false
	case simulatorLogWriteUnknown:
	}
	return now.Sub(lastActivity) <= runStatusStaleAfter
}

func (h serveRunStatus) simulatorLogWriteState(simLog string) simulatorLogWriteState {
	if h.logDir == "" || simLog == "" {
		return simulatorLogWriteUnknown
	}
	target := filepath.Join(h.logDir, filepath.FromSlash(simLog))
	return openForWriteState(target)
}

func openForWriteState(target string) simulatorLogWriteState {
	targetInfo, err := os.Stat(target)
	if err != nil {
		return simulatorLogWriteUnknown
	}
	procs, err := os.ReadDir("/proc")
	if err != nil {
		return simulatorLogWriteUnknown
	}

	for _, proc := range procs {
		if !proc.IsDir() || !isNumeric(proc.Name()) {
			continue
		}
		fdDir := filepath.Join("/proc", proc.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			fdPath := filepath.Join(fdDir, fd.Name())
			fdInfo, err := os.Stat(fdPath)
			if err != nil || !os.SameFile(targetInfo, fdInfo) {
				continue
			}
			if fdOpenForWrite(filepath.Join("/proc", proc.Name(), "fdinfo", fd.Name())) {
				return simulatorLogWriteOpen
			}
		}
	}
	return simulatorLogWriteClosed
}

func fdOpenForWrite(fdinfoPath string) bool {
	data, err := os.ReadFile(fdinfoPath)
	if err != nil {
		return true
	}
	for _, line := range strings.Split(string(data), "\n") {
		rawFlags, ok := strings.CutPrefix(line, "flags:")
		if !ok {
			continue
		}
		flags, err := strconv.ParseInt(strings.TrimSpace(rawFlags), 8, 64)
		if err != nil {
			return true
		}
		accessMode := flags & 3
		return accessMode == int64(os.O_WRONLY) || accessMode == int64(os.O_RDWR)
	}
	return true
}

func isNumeric(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return value != ""
}

func countRunStatusTests(suite *libhive.TestSuite) int {
	total := 0
	for _, test := range suite.TestCases {
		if isRunStatusSetupTest(test.Name, suite.Name) {
			continue
		}
		total++
	}
	return total
}

func isRunStatusSetupTest(testName string, suiteName string) bool {
	name := strings.ToLower(strings.TrimSpace(testName))
	suite := strings.ToLower(strings.TrimSpace(suiteName))
	return suite != "" && (name == suite+": client launch" || name == suite+": matrix")
}

func simLogStartTime(simLog string) time.Time {
	timestamp, _, ok := strings.Cut(simLog, "-")
	if !ok {
		return time.Time{}
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}
