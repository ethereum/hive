package common

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// IndexFileName is the main index file of the test suite execution database
const IndexFileName string = "index.txt"

// TestSuiteID identifies a test suite context
type TestSuiteID uint32

func (tsID TestSuiteID) String() string {
	return strconv.Itoa(int(tsID))
}

// TestID identifies a test case context
type TestID uint32

func (tsID TestID) String() string {
	return strconv.Itoa(int(tsID))
}

// TestSummary is an entry in a main index of test suite executions
// NB! This record must be small (less than the typically smallest file sector size)
// to ensure cross-platform atomic writes to the index file from multiple processes.
type TestSummary struct {
	FileName      string    `json:"fileName"`
	Name          string    `json:"name"`
	Start         time.Time `json:"start"`
	PrimaryClient string    `json:"primaryClient"`
	Pass          bool      `json:"pass"`
}

// TestSuite is a single run of a simulator, a collection of testcases
type TestSuite struct {
	ID          TestSuiteID          `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	TestCases   map[TestID]*TestCase `json:"testCases"`
	indexMu     sync.Mutex
}

func (testSuite *TestSuite) summarise(suiteFileName string) *TestSummary {

	summary := &TestSummary{
		FileName: suiteFileName,
		Name:     testSuite.Name,
	}

	pass := true
	primaryName := ""
	earliest := time.Now()
	clients := make(map[string]bool, 0)
	for _, testCase := range testSuite.TestCases {
		pass = pass && testCase.SummaryResult.Pass
		if testCase.Start.Before(earliest) {
			earliest = testCase.Start
		}
		for _, clientInfo := range testCase.ClientInfo {
			clients[clientInfo.Name] = true
			primaryName = clientInfo.Name
		}
	}
	summary.Pass = pass
	summary.Start = earliest
	//if the test was for a single client, indicate it
	if len(clients) > 1 {
		summary.PrimaryClient = "Multiple"
	} else {
		if len(clients) == 0 {
			summary.PrimaryClient = "None"
		} else {

			summary.PrimaryClient = primaryName
		}
	}
	return summary
}

// UpdateDB should be called on TestSuite completion to
// write out the test results and update indexes.
// NB Concurrent Hive processes should be able to safely update the index file
// as long as the file system supports POSIX-like atomic writes.
func (testSuite *TestSuite) UpdateDB(outputPath string) error {

	// write out the test suite as a json file of its own
	suiteData, err := json.Marshal(*testSuite)
	if err != nil {
		return err
	}
	fileID, err := uuid.NewUUID()
	if err != nil {
		return err
	}
	suiteFileName := fmt.Sprintf("%s.json", fileID.String())
	suiteFilePath := filepath.Join(outputPath, suiteFileName)
	ioutil.WriteFile(suiteFilePath, suiteData, 0644)

	// Before writing the summary, perhaps crop the index file
	// We cap it at 400KB in size down to 200KB, roughly 1000 lines
	testSuite.indexMu.Lock()
	defer testSuite.indexMu.Unlock()
	indexPathName := filepath.Join(outputPath, IndexFileName)
	stat, err := os.Stat(indexPathName)
	if err == nil && stat.Size() > 400000 {
		truncateHead(indexPathName, 200000)
	}
	// write out the summary
	testSummary := testSuite.summarise(suiteFileName)
	summaryData, err := json.Marshal(*testSummary)
	if err != nil {
		return err
	}
	//now append the index file
	i, err := os.OpenFile(indexPathName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer i.Close()
	if _, err = i.WriteString(string(summaryData) + "\n"); err != nil {
		return err
	}
	return nil
}

// truncateHead truncates the head lines, leaving somewhere slightly below 'size' bytes,
// and ensures that lines are kept intact.
func truncateHead(path string, size int64) error {
	var tmpFileName = fmt.Sprintf("%s.tmp", path)
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	// seek N bytes from the end (whence=2)
	if _, err := file.Seek(-size, 2); err != nil {
		file.Close()
		return err
	}
	reader := bufio.NewReader(file)
	// read until a line-break
	if _, err = reader.ReadString('\n'); err != nil {
		file.Close()
		return fmt.Errorf("seek failed: %v", err)
	}
	// reader is now positioned correctly, we can shove all remaining data into the next index-file
	newFile, err := os.OpenFile(tmpFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		file.Close()
		return err
	}
	io.Copy(newFile, reader)
	newFile.Close()
	file.Close()
	// Now, delete the old one, and swap in the new one
	if err = os.Remove(path); err != nil {
		return err
	}
	return os.Rename(tmpFileName, path)
}

// TestClientResults is the set of per-client results for a test-case
type TestClientResults map[string]*TestResult

// AddResult adds a test result for a client
func (t TestClientResults) AddResult(clientID string, pass bool, detail string) {
	tcr, in := t[clientID]
	if !in {
		tcr = &TestResult{}
		t[clientID] = tcr
	}
	tcr.Pass = pass
	tcr.AddDetail(detail)
}

// TestCase represents a single test case in a test suite
type TestCase struct {
	ID            TestID                     `json:"id"`          // Test case reference number.
	Name          string                     `json:"name"`        // Test case short name.
	Description   string                     `json:"description"` // Test case long description in MD.
	Start         time.Time                  `json:"start"`
	End           time.Time                  `json:"end"`
	SummaryResult TestResult                 `json:"summaryResult"` // The result of the whole test case.
	ClientResults TestClientResults          `json:"clientResults"` // Client specific results, if this test case supports this concept. Not all test cases will identify a specific client as a test failure reason.
	ClientInfo    map[string]*TestClientInfo `json:"clientInfo"`    // Info about each client.
	pseudoInfo    map[string]*TestClientInfo // registry of participating pseudos maintained for client maintenance and not part of the result database
}

// TestResult describes the results of a test at the level of the overall test case and for each client involved in a test case
type TestResult struct {
	Pass    bool   `json:"pass"`
	Details string `json:"details"`
}

// AddDetail adds test result info using a standard text formatting
func (t *TestResult) AddDetail(detail string) {
	t.Details = fmt.Sprintf("%s %s\n", t.Details, detail)
}

// TestClientInfo describes a client that participated in a test case
type TestClientInfo struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	VersionInfo     string    `json:"versionInfo"` //URL to github repo + branch.
	InstantiatedAt  time.Time `json:"instantiatedAt"`
	LogFile         string    `json:"logFile"` //Absolute path to the logfile.
	WasInstantiated bool
}
