package hivesim

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ethereum/hive/internal/simapi"
	"github.com/lithammer/dedent"
	"gopkg.in/inconshreveable/log15.v2"
)

func toMarkdownLink(title string) string {
	removeChars := []string{":", "#", "'", "\"", "`", "*", "+", ",", ";"}
	title = strings.ReplaceAll(strings.ToLower(title), " ", "-")
	for _, invalidChar := range removeChars {
		title = strings.ReplaceAll(title, invalidChar, "")
	}
	return title
}

func formatDescription(desc string) string {
	desc = dedent.Dedent(desc)
	// Replace single quotes with backticks, since backticks are used for markdown code blocks and
	// they cannot be escaped in golang (when used within backticks string).
	desc = strings.ReplaceAll(desc, "'", "`")
	return desc
}

type markdownTestCase simapi.TestRequest

func (tc *markdownTestCase) commandLine(simName string, suiteName string) string {
	return fmt.Sprintf("./hive --client <CLIENTS> --sim %s --sim.limit \"%s/%s\"", simName, suiteName, tc.Name)
}

func (tc *markdownTestCase) toBePrinted() bool {
	return tc.Description != ""
}

func (tc *markdownTestCase) displayName() string {
	if tc.DisplayName != "" {
		return tc.DisplayName
	}
	return tc.Name
}

func (tc *markdownTestCase) description() string {
	return formatDescription(tc.Description)
}

func (tc *markdownTestCase) toMarkdown(simName string, suiteName string, depth int) string {
	sb := strings.Builder{}

	// Print test case display name
	sb.WriteString(fmt.Sprintf("%s - %s\n\n", strings.Repeat("#", depth), tc.displayName()))

	// Print command-line to run
	sb.WriteString(fmt.Sprintf("%s Run\n\n", strings.Repeat("#", depth+1)))
	sb.WriteString("<details>\n")
	sb.WriteString("<summary>Command-line</summary>\n\n")
	sb.WriteString(fmt.Sprintf("```bash\n%s\n```\n\n", tc.commandLine(simName, suiteName)))
	sb.WriteString("</details>\n\n")

	// Print description
	sb.WriteString(fmt.Sprintf("%s Description\n\n", strings.Repeat("#", depth+1)))
	sb.WriteString(fmt.Sprintf("%s\n\n", tc.description()))

	return sb.String()
}

type markdownSuite struct {
	simapi.TestRequest
	running bool
	tests   map[TestID]*markdownTestCase
}

func (s *markdownSuite) displayName() string {
	if s.DisplayName != "" {
		return s.DisplayName
	}
	return fmt.Sprintf("`%s`", s.Name)
}

func (s *markdownSuite) mardownFilePath() string {
	if s.Location != "" {
		return fmt.Sprintf("%s/TESTS.md", s.Location)
	}
	title := strings.ReplaceAll(strings.ToUpper(s.Name), " ", "-")
	return fmt.Sprintf("TESTS-%s.md", title)
}

func (s *markdownSuite) commandLine(simName string) string {
	return fmt.Sprintf("./hive --client <CLIENTS> --sim %s --sim.limit \"%s/\"", simName, s.Name)
}

func (s *markdownSuite) description() string {
	return formatDescription(s.Description)
}

func (s *markdownSuite) testIDs() []TestID {
	// Returns a sorted list of the test IDs
	testIDs := make([]TestID, 0, len(s.tests))
	for testID := range s.tests {
		testIDs = append(testIDs, testID)
	}
	sort.Slice(testIDs, func(i, j int) bool {
		return testIDs[i] < testIDs[j]
	})
	return testIDs
}

func (s *markdownSuite) getCategories() []string {
	categoryMap := make(map[string]struct{})
	categories := make([]string, 0)
	for _, tcID := range s.testIDs() {
		tc := s.tests[tcID]
		if tc.toBePrinted() {
			if _, ok := categoryMap[tc.Category]; !ok {
				categories = append(categories, tc.Category)
				categoryMap[tc.Category] = struct{}{}
			}
		}
	}
	return categories
}

func (s *markdownSuite) toMarkdown(simName string) (string, error) {
	headerBuilder := strings.Builder{}
	categoryBuilder := map[string]*strings.Builder{}
	headerBuilder.WriteString(fmt.Sprintf("# %s - Test Cases\n\n", s.displayName()))

	headerBuilder.WriteString(fmt.Sprintf("%s\n\n", s.description()))

	headerBuilder.WriteString("## Run Suite\n\n")

	headerBuilder.WriteString("<details>\n")
	headerBuilder.WriteString("<summary>Command-line</summary>\n\n")
	headerBuilder.WriteString(fmt.Sprintf("```bash\n%s\n```\n\n", s.commandLine(simName)))
	headerBuilder.WriteString("</details>\n\n")

	categories := s.getCategories()

	tcDepth := 3
	if len(categories) > 1 {
		headerBuilder.WriteString("## Test Case Categories\n\n")
		for _, category := range categories {
			if category == "" {
				category = "Other"
			}
			headerBuilder.WriteString(fmt.Sprintf("- [%s](#category-%s)\n\n", category, toMarkdownLink(category)))
		}
	} else {
		headerBuilder.WriteString("## Test Cases\n\n")
	}

	for _, tcID := range s.testIDs() {
		tc := s.tests[tcID]
		if !tc.toBePrinted() {
			continue
		}
		contentBuilder, ok := categoryBuilder[tc.Category]
		if !ok {
			contentBuilder = &strings.Builder{}
			categoryBuilder[tc.Category] = contentBuilder
		}
		contentBuilder.WriteString(tc.toMarkdown(simName, s.Name, tcDepth))
	}

	if len(categoryBuilder) > 1 {
		for _, category := range categories {
			if category == "" {
				category = "Other"
			}
			contentBuilder, ok := categoryBuilder[category]
			if !ok {
				continue
			}
			headerBuilder.WriteString(fmt.Sprintf("## Category: %s\n\n", category))
			headerBuilder.WriteString(contentBuilder.String())
		}
	} else {
		for _, contentBuilder := range categoryBuilder {
			headerBuilder.WriteString(contentBuilder.String())
		}
	}

	return headerBuilder.String(), nil
}

func (s *markdownSuite) toMarkdownFile(fw FileWriter, simName string) error {
	// Create the file.
	file, err := fw.CreateWriter(s.mardownFilePath())
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the markdown.
	markdown, err := s.toMarkdown(simName)
	if err != nil {
		return err
	}
	_, err = file.Write([]byte(markdown))
	return err
}

// Docs collector object
type docsCollector struct {
	simName   string
	outputDir string
	suites    map[SuiteID]*markdownSuite
}

func simulatorNameFromBinaryPath() string {
	execPath, err := os.Executable()
	if err != nil {
		panic(err)
	}
	var (
		path  = filepath.Dir(execPath)
		name  = filepath.Base(path)
		names = make([]string, 0)
	)
	for path != "/" && name != "simulators" {
		names = append([]string{name}, names...)
		path = filepath.Dir(path)
		name = filepath.Base(path)
	}
	return strings.Join(names, "/")
}

// NewDocsCollector creates a new docs collector object.
func NewDocsCollector() *docsCollector {
	docs := &docsCollector{
		suites: make(map[SuiteID]*markdownSuite),
	}
	// Set the simulation name: if `HIVE_SIMULATOR_NAME` is set, use that, otherwise use the
	// folder name and its parent.
	docs.simName = os.Getenv("HIVE_SIMULATOR_NAME")
	if docs.simName == "" {
		docs.simName = simulatorNameFromBinaryPath()
	}

	// Set the output directory: if `HIVE_DOCS_OUTPUT_DIR` is set, use that, otherwise use the
	// current directory.
	docs.outputDir = os.Getenv("HIVE_DOCS_OUTPUT_DIR")
	if docs.outputDir == "" {
		var err error
		docs.outputDir, err = os.Getwd()
		if err != nil {
			panic(err)
		}
	}

	return docs
}

// Returns true if any suite is still running
func (docs *docsCollector) AnyRunning() bool {
	for _, s := range docs.suites {
		if s.running {
			return true
		}
	}
	return false
}

func (docs *docsCollector) suiteIDs() []SuiteID {
	// Returns a sorted list of the suite IDs
	suiteIDs := make([]SuiteID, 0, len(docs.suites))
	for suiteID := range docs.suites {
		suiteIDs = append(suiteIDs, suiteID)
	}
	sort.Slice(suiteIDs, func(i, j int) bool {
		return suiteIDs[i] < suiteIDs[j]
	})
	return suiteIDs
}

func (docs *docsCollector) StartSuite(suite *simapi.TestRequest, simlog string) (SuiteID, error) {
	// Create a new markdown suite.
	markdownSuite := &markdownSuite{
		TestRequest: *suite,
		running:     true,
		tests:       make(map[TestID]*markdownTestCase),
	}
	// Next suite id
	suiteID := SuiteID(len(docs.suites))
	// Add the suite to the map.
	docs.suites[suiteID] = markdownSuite
	// Return the suite ID.
	return suiteID, nil
}

func (docs *docsCollector) EndSuite(testSuite SuiteID) error {
	suite, ok := docs.suites[testSuite]
	if !ok {
		return fmt.Errorf("test suite %d does not exist", testSuite)
	}
	suite.running = false
	if !docs.AnyRunning() {
		// Generate markdown files when all suites are done.
		if err := docs.genSimulatorMarkdownFiles(NewFileWriter(docs.outputDir)); err != nil {
			log15.Error("can't generate markdown files", "err", err)
		}
	}
	return nil
}

func (docs *docsCollector) StartTest(testSuite SuiteID, test *simapi.TestRequest) (TestID, error) {
	// Create a new markdown test case.
	markdownTest := markdownTestCase(*test)
	// Check if suite exists.
	if _, ok := docs.suites[testSuite]; !ok {
		return 0, fmt.Errorf("test suite %d does not exist", testSuite)
	}
	// Next test id
	testID := TestID(len(docs.suites[testSuite].tests))
	// Add the test to the map.
	docs.suites[testSuite].tests[testID] = &markdownTest
	// Return the test ID.
	return testID, nil
}

func (docs *docsCollector) EndTest(testSuite SuiteID, test TestID, testResult TestResult) error {
	// No-op in docs mode.
	return nil
}

func (docs *docsCollector) ClientTypes() ([]*ClientDefinition, error) {
	// Return a dummy "docs" client type.
	return []*ClientDefinition{
		{
			Name:    "Client",
			Version: "1.0.0",
		},
	}, nil
}

func (docs *docsCollector) generateIndex(simName string) (string, error) {
	headerBuilder := strings.Builder{}
	headerBuilder.WriteString(fmt.Sprintf("# Simulator `%s` Test Cases\n\n", simName))
	headerBuilder.WriteString("## Test Suites\n\n")
	for _, sID := range docs.suiteIDs() {
		s := docs.suites[sID]
		headerBuilder.WriteString(fmt.Sprintf("### - [%s](%s)\n", s.displayName(), s.mardownFilePath()))
		headerBuilder.WriteString(s.Description)
		headerBuilder.WriteString("\n\n")
	}
	return headerBuilder.String() + "\n", nil
}

func (docs *docsCollector) generateIndexFile(fw FileWriter, simName string) error {
	markdownIndex, err := docs.generateIndex(simName)
	if err != nil {
		return err
	}
	file, err := fw.CreateWriter("TESTS.md")
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write([]byte(markdownIndex))
	return err
}

type FileWriter interface {
	CreateWriter(path string) (io.WriteCloser, error)
}

type fileWriter struct {
	path string
}

var _ FileWriter = (*fileWriter)(nil)

func (fw *fileWriter) CreateWriter(path string) (io.WriteCloser, error) {
	filePath := filepath.FromSlash(fmt.Sprintf("%s/%s", fw.path, path))
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, err
	}
	return os.Create(filePath)
}

func NewFileWriter(path string) FileWriter {
	return &fileWriter{path}
}

func (docs *docsCollector) genSimulatorMarkdownFiles(fw FileWriter) error {
	err := docs.generateIndexFile(fw, docs.simName)
	if err != nil {
		return err
	}
	for _, sID := range docs.suiteIDs() {
		s := docs.suites[sID]
		if err := s.toMarkdownFile(fw, docs.simName); err != nil {
			return err
		}
	}
	return nil
}
