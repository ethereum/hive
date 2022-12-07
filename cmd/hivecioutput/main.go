package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ethereum/hive/internal/libhive"
	"gopkg.in/inconshreveable/log15.v2"
)

const (
	Header = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
)

type Testsuites struct {
	XMLName   xml.Name `xml:"testsuites"`
	Text      string   `xml:",chardata"`
	ID        string   `xml:"id,attr"`
	Name      string   `xml:"name,attr"`
	Tests     int      `xml:"tests,attr"`
	Failures  int      `xml:"failures,attr"`
	Skips     int      `xml:"skips,attr"`
	Time      string   `xml:"time,attr"`
	Testsuite []Testsuite
}

type Testsuite struct {
	XMLName  xml.Name `xml:"testsuite"`
	Text     string   `xml:",chardata"`
	ID       int      `xml:"id,attr"`
	Name     string   `xml:"name,attr"`
	Tests    int      `xml:"tests,attr"`
	Failures int      `xml:"failures,attr"`
	Skips    int      `xml:"skips,attr"`
	Time     string   `xml:"time,attr"`
	Testcase []Testcase
}

type Testcase struct {
	XMLName xml.Name `xml:"testcase"`
	Text    string   `xml:",chardata"`
	ID      int      `xml:"id,attr"`
	Name    string   `xml:"name,attr"`
	Time    string   `xml:"time,attr"`
	Failure *Failure
	Skipped *Skipped
}

type Failure struct {
	XMLName xml.Name `xml:"failure,omitempty"`
	Text    string   `xml:",chardata"`
	Message string   `xml:"message,attr"`
	Type    string   `xml:"type,attr"`
}

type Skipped struct {
	XMLName xml.Name `xml:"skipped"`
	Text    string   `xml:",chardata"`
	Message string   `xml:"message,attr"`
	Type    string   `xml:"type,attr"`
}

type Exclusions struct {
	TestSuites []struct {
		Name      string   `json:"name"`
		TestCases []string `json:"testcases"`
	} `json:"testSuites"`
}

func main() {
	var exitcode bool
	flag.BoolVar(&exitcode, "exitcode", false, "Return exit code 1 on failed tests")

	var (
		resultsdir     = flag.String("resultsdir", "/tmp/TestResults", "Results dir to scan")
		outdir         = flag.String("outdir", "/tmp/", "Output dir for xunit xml")
		exclusionsFile = flag.String("exclusionsfile", "", "File containing list of test names to exclude")
	)
	flag.Parse()

	log15.Info(fmt.Sprintf("loading results from %s", *resultsdir))
	log15.Info(fmt.Sprintf("outputting xunit xml to %s", *outdir))
	log15.Info(fmt.Sprintf("return exit code 1 on failed tests %v", exitcode))
	log15.Info(fmt.Sprintf("exclusions file %s", *exclusionsFile))

	// load exclusions
	exclusions := &Exclusions{}
	if *exclusionsFile != "" {
		file, err := os.Open(*exclusionsFile)
		if err != nil {
			log15.Info(fmt.Sprintf("error opening exclusions file %s", *exclusionsFile))
		}
		bytes, err := io.ReadAll(file)
		if err != nil {
			log15.Info(fmt.Sprintf("error reading exclusions file %s", *exclusionsFile))
		}
		err = json.Unmarshal(bytes, &exclusions)
		if err != nil {
			log15.Info(fmt.Sprintf("error unmarshalling exclusions file %s", *exclusionsFile))
		}
		file.Close()

		// print exclusions
		if len(exclusions.TestSuites) > 0 {
			log15.Info(fmt.Sprintf("excluded tests: %v", exclusions))
		}
	}

	outputs := []*libhive.TestSuite{}

	rd := os.DirFS(*resultsdir)
	err := walkSummaryFiles(rd, ".", collectOutput, &outputs)
	if err != nil {
		log15.Info(fmt.Errorf("error reading results: %w", err).Error())
		os.Exit(1)
	}

	pass, run := outputXUnitXmlFile(&outputs, *outdir, exclusions)

	// tests run and passed
	if pass && run {
		log15.Info("tests passed!")
		os.Exit(0)
	}

	// no tests run
	if !run {
		log15.Info("no tests run!")
		if exitcode {
			os.Exit(1)
		}
	}

	// tests run but failed
	log15.Info("tests failed!")
	if exitcode {
		os.Exit(1)
	}
}

func outputXUnitXmlFile(outputs *[]*libhive.TestSuite, path string, exclusions *Exclusions) (bool, bool) {
	opTs := Testsuites{}
	totalTests := 0
	totalFailures := 0
	totalSkipped := 0
	suiteNo := 0

	for _, ts := range *outputs {
		log15.Info("test suite", "name", ts.Name)
		tests := 0
		failures := 0
		skipped := 0
		caseNo := 0
		tsTime := time.Second * 0
		suiteNo++

		oTs := Testsuite{
			Text:     ts.Description,
			ID:       suiteNo,
			Name:     ts.Name,
			Time:     "",
			Testcase: []Testcase{},
		}

		for _, tc := range ts.TestCases {
			caseNo++
			testSkipped := false

			tcTime := tc.End.Sub(tc.Start)
			tsTime += tcTime

			oTc := Testcase{
				Text: tc.Description,
				ID:   caseNo,
				Name: tc.Name,
				Time: fmt.Sprintf("%v", tcTime.Seconds()),
			}

			if contains(exclusions, ts.Name, tc.Name) {
				testSkipped = true
			}

			if !tc.SummaryResult.Pass && !testSkipped {
				failures++
				oTc.Failure = &Failure{
					Text:    tc.SummaryResult.Details,
					Message: "Error",
					Type:    "ERROR",
				}
			}

			// only skip on failure, if we pass, might as well include in results
			if !tc.SummaryResult.Pass && testSkipped {
				skipped++
				log15.Info("test skipped", "name", tc.Name)
				oTc.Skipped = &Skipped{
					Text:    tc.SummaryResult.Details,
					Message: "Skipped",
					Type:    "SKIPPED",
				}
			}

			tests++
			oTs.Testcase = append(oTs.Testcase, oTc)
			oTs.Tests = tests
			oTs.Failures = failures
			oTs.Skips = skipped
			oTs.Time = fmt.Sprintf("%v", tsTime.Seconds())
		}

		opTs.Testsuite = append(opTs.Testsuite, oTs)

		totalTests += tests
		totalFailures += failures
		totalSkipped += skipped
	}

	opTs.Tests = totalTests
	opTs.Failures = totalFailures
	opTs.Skips = totalSkipped
	opTs.ID = "0"
	opTs.Name = "Hive Test Run"
	opTs.Time = ""

	xmlContent, _ := xml.MarshalIndent(opTs, "", " ")
	xmlContent = []byte(xml.Header + string(xmlContent))
	err := ioutil.WriteFile(filepath.Join(path, "output.xml"), xmlContent, 0644)
	if err != nil {
		log15.Error(err.Error())
	}

	testsRun := totalTests > 0

	if totalFailures > 0 {
		return false, testsRun
	}
	return true, testsRun
}

func collectOutput(ts *libhive.TestSuite, outputs *[]*libhive.TestSuite) {
	*outputs = append(*outputs, ts)
}

type parseTs func(*libhive.TestSuite, *[]*libhive.TestSuite)

func walkSummaryFiles(fsys fs.FS, dir string, parse parseTs, ts *[]*libhive.TestSuite) error {
	logfiles, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}
	// Sort by name newest-first.
	sort.Slice(logfiles, func(i, j int) bool {
		return logfiles[i].Name() > logfiles[j].Name()
	})

	for _, entry := range logfiles {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || skipFile(name) {
			continue
		}
		suite, _ := parseSuite(fsys, filepath.Join(dir, name))
		if suite != nil {
			parse(suite, ts)
		}
	}
	return nil
}

func parseSuite(fsys fs.FS, path string) (*libhive.TestSuite, fs.FileInfo) {
	file, err := fsys.Open(path)
	if err != nil {
		log.Printf("Can't access summary file: %s", err)
		return nil, nil
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("Can't access summary file: %s", err)
		return nil, nil
	}

	var info libhive.TestSuite
	if err := json.NewDecoder(file).Decode(&info); err != nil {
		log.Printf("Skipping invalid summary file %s: %v", fileInfo.Name(), err)
		return nil, nil
	}
	if !suiteValid(&info) {
		log.Printf("Skipping invalid summary file %s", fileInfo.Name())
		return nil, nil
	}
	return &info, fileInfo
}

func suiteValid(s *libhive.TestSuite) bool {
	return s.SimulatorLog != ""
}

func skipFile(f string) bool {
	return f == "errorReport.json" || f == "containerErrorReport.json" || strings.HasPrefix(f, ".")
}

func contains(ex *Exclusions, tsName, tcName string) bool {
	for _, ts := range ex.TestSuites {
		if tsName == ts.Name {
			for _, tc := range ts.TestCases {
				if removeImageName(tcName) == tc {
					return true
				}
			}
		}
	}
	return false
}

func removeImageName(s string) string {
	// regex to remove the last bracketed string (image name appended to test name)
	reg := regexp.MustCompile(`\([^)]*\)$`)
	return strings.Trim(reg.ReplaceAllString(s, ""), " ")
}
