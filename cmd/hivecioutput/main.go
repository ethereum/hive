package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/inconshreveable/log15.v2"

	"github.com/ethereum/hive/internal/libhive"
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
}

type Failure struct {
	XMLName xml.Name `xml:"failure,omitempty"`
	Text    string   `xml:",chardata"`
	Message string   `xml:"message,attr"`
	Type    string   `xml:"type,attr"`
}

func main() {
	var exitcode bool
	flag.BoolVar(&exitcode, "exitcode", true, "Return exit code 1 on failed tests")

	var (
		resultsdir = flag.String("resultsdir", "/tmp/TestResults", "Results dir to scan")
		outdir     = flag.String("outdir", "/tmp/", "Output dir for xunit xml")
	)
	flag.Parse()

	log15.Info(fmt.Sprintf("loading results from %s", *resultsdir))
	log15.Info(fmt.Sprintf("outputting xunit xml to %s", *outdir))
	log15.Info(fmt.Sprintf("return exit code 1 on failed tests %v", exitcode))

	outputs := []*libhive.TestSuite{}

	rd := os.DirFS(*resultsdir)
	op, err := walkSummaryFiles(rd, ".", collectOutput, &outputs)
	if err != nil {
		log15.Info(fmt.Errorf("error reading results: %w", err).Error())
		os.Exit(1)
	}

	outputXUnitXmlFile(&outputs, *outdir)

	if op {
		log15.Info("tests passed!")
		os.Exit(0)
	}

	log15.Info("tests failed!")
	if exitcode {
		os.Exit(1)
	}
}

func outputXUnitXmlFile(outputs *[]*libhive.TestSuite, path string) {
	opTs := Testsuites{}
	totalTests := 0
	totalFailures := 0
	suiteNo := 0

	for _, ts := range *outputs {
		log15.Info("test suite", "name", ts.Name)
		tests := 0
		failures := 0
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

			tcTime := tc.End.Sub(tc.Start)
			tsTime += tcTime

			oTc := Testcase{
				Text: tc.Description,
				ID:   caseNo,
				Name: tc.Name,
				Time: fmt.Sprintf("%v", tcTime.Seconds()),
			}

			tests++
			if !tc.SummaryResult.Pass {
				failures++
				oTc.Failure = &Failure{
					Text:    tc.SummaryResult.Details,
					Message: "Error",
					Type:    "ERROR",
				}
			}

			oTs.Testcase = append(oTs.Testcase, oTc)
			oTs.Tests = tests
			oTs.Failures = failures
			oTs.Time = fmt.Sprintf("%v", tsTime.Seconds())
		}

		opTs.Testsuite = append(opTs.Testsuite, oTs)

		totalTests += tests
		totalFailures += failures
	}

	opTs.Tests = totalTests
	opTs.Failures = totalFailures
	opTs.ID = "0"
	opTs.Name = "Hive Test Run"
	opTs.Time = ""

	xmlContent, _ := xml.MarshalIndent(opTs, "", " ")
	xmlContent = []byte(xml.Header + string(xmlContent))
	err := ioutil.WriteFile(filepath.Join(path, "output.xml"), xmlContent, 0644)
	if err != nil {
		log15.Error(err.Error())
	}
}

func collectOutput(ts *libhive.TestSuite, outputs *[]*libhive.TestSuite) (bool, error) {
	// collect output
	*outputs = append(*outputs, ts)

	// determine whether any cases failed and break to return early if so
	pass := true
	for _, tc := range ts.TestCases {
		if !tc.SummaryResult.Pass {
			pass = false
			break
		}
	}

	return pass, nil
}

type parseTs func(*libhive.TestSuite, *[]*libhive.TestSuite) (bool, error)

func walkSummaryFiles(fsys fs.FS, dir string, parse parseTs, ts *[]*libhive.TestSuite) (bool, error) {
	logfiles, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return false, err
	}
	// Sort by name newest-first.
	sort.Slice(logfiles, func(i, j int) bool {
		return logfiles[i].Name() > logfiles[j].Name()
	})

	overallPass := true

	for _, entry := range logfiles {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || skipFile(name) {
			continue
		}
		suite, _ := parseSuite(fsys, filepath.Join(dir, name))
		if suite != nil {
			pass, err := parse(suite, ts)
			if err != nil {
				return false, err
			}

			// set overall pass false on failure, allow for
			// loop to continue to collect all results
			if !pass {
				overallPass = false
			}
		}
	}
	return overallPass, nil
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
