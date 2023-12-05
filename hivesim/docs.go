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

// Converts a string to a valid markdown link.
func toMarkdownLink(title string) string {
	removeChars := []string{":", "#", "'", "\"", "`", "*", "+", ",", ";"}
	title = strings.ReplaceAll(strings.ToLower(title), " ", "-")
	for _, invalidChar := range removeChars {
		title = strings.ReplaceAll(title, invalidChar, "")
	}
	return title
}

// Formats the description string to be printed in the markdown file.
func formatDescription(desc string) string {
	desc = dedent.Dedent(desc)
	// Replace single quotes with backticks, since backticks are used for markdown code blocks and
	// they cannot be escaped in golang (when used within backticks string).
	desc = strings.ReplaceAll(desc, "'", "`")
	return desc
}

// Represents a single test case to be printed in the markdown file.
type markdownTestCase simapi.TestRequest

// Returns the command-line to run the test case.
func (tc *markdownTestCase) commandLine(simName string, suiteName string) string {
	return fmt.Sprintf("./hive --client <CLIENTS> --sim %s --sim.limit \"%s/%s\"", simName, suiteName, tc.Name)
}

// Returns true if the test case should be printed in the markdown file.
func (tc *markdownTestCase) toBePrinted() bool {
	return tc.Description != ""
}

// Returns the test case display name.
func (tc *markdownTestCase) displayName() string {
	if tc.DisplayName != "" {
		return tc.DisplayName
	}
	return tc.Name
}

// Returns the test case description.
func (tc *markdownTestCase) description() string {
	return formatDescription(tc.Description)
}

// Returns the test case markdown representation.
// Requires the simulation name and the suite name.
// The depth parameter is used to determine the number of '#' to use for the test case title.
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

// Represents a test suite to be printed in the markdown file.
type markdownSuite struct {
	simapi.TestRequest
	running bool
	tests   map[TestID]*markdownTestCase
}

// Returns true if the test suite should be printed in the markdown file.
func (s *markdownSuite) displayName() string {
	if s.DisplayName != "" {
		return s.DisplayName
	}
	return fmt.Sprintf("`%s`", s.Name)
}

// Returns the markdown file path for the test suite.
// If the location is set, it will be used as the directory path, and the filename will be
// `TESTS.md`.
// Otherwise, the filename will be `TESTS-<SUITE_NAME>.md`.
func (s *markdownSuite) mardownFilePath() string {
	if s.Location != "" {
		return fmt.Sprintf("%s/TESTS.md", s.Location)
	}
	title := strings.ReplaceAll(strings.ToUpper(s.Name), " ", "-")
	return fmt.Sprintf("TESTS-%s.md", title)
}

// Returns the command-line to run the test suite.
func (s *markdownSuite) commandLine(simName string) string {
	return fmt.Sprintf("./hive --client <CLIENTS> --sim %s --sim.limit \"%s/\"", simName, s.Name)
}

// Returns true if the test suite should be printed in the markdown file.
func (s *markdownSuite) description() string {
	return formatDescription(s.Description)
}

// Returns a sorted list of the test IDs
func (s *markdownSuite) testIDs() []TestID {
	testIDs := make([]TestID, 0, len(s.tests))
	for testID := range s.tests {
		testIDs = append(testIDs, testID)
	}
	sort.Slice(testIDs, func(i, j int) bool {
		return testIDs[i] < testIDs[j]
	})
	return testIDs
}

// Returns a list of all the unique categories in all of the test cases.
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

// Returns the markdown representation of the test suite.
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

// Writes the markdown representation of the test suite to a file.
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

// Docs collector object:
// - Collects the test cases and test suites.
// - Generates markdown files.
type docsCollector struct {
	simName   string
	outputDir string
	suites    map[SuiteID]*markdownSuite
}

// Returns the simulator name from the path of the currently running binary.
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
// Tries to parse the simulator name and also the output directory from the environment variables.
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

// Returns a sorted list of the suite IDs
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

// Starts a new test suite, and appends it to the suites map.
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

// Ends a test suite. If the suite does not exist, returns an error.
// If all suites are done, generates the markdown files.
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

// Starts a new test case, and appends it to the tests map in the test suite.
// If the suite does not exist, returns an error.
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

// Ends a test case. If the suite or the test case or suite do not exist, returns an error.
func (docs *docsCollector) EndTest(testSuite SuiteID, test TestID, testResult TestResult) error {
	// Check if suite exists.
	if _, ok := docs.suites[testSuite]; !ok {
		return fmt.Errorf("test suite %d does not exist", testSuite)
	}
	// Check if test exists.
	if _, ok := docs.suites[testSuite].tests[test]; !ok {
		return fmt.Errorf("test %d does not exist", test)
	}
	return nil
}

// Return a generic "Client" client type.
func (docs *docsCollector) ClientTypes() ([]*ClientDefinition, error) {
	return []*ClientDefinition{
		{
			Name:    "Client",
			Version: "1.0.0",
		},
	}, nil
}

// Generates the markdown index file that will point to the test suites' markdown files.
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

// Generates the markdown index file that will point to the test suites' markdown files.
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

// Generates the markdown files for all the test suites, including the index file.
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

// FileWriter interface. Used to create the markdown files.
type FileWriter interface {
	CreateWriter(path string) (io.WriteCloser, error)
}

// FileWriter implementation that writes to the filesystem.
// The basePath is the root directory where the files will be created.
type fileWriter struct {
	basePath string
}

// Creates a new file writer. The path starts from the base directory that is the fileWriter.
// Subdirectories will be created if they do not exist.
func (fw fileWriter) CreateWriter(path string) (io.WriteCloser, error) {
	filePath := filepath.FromSlash(fmt.Sprintf("%s/%s", filepath.ToSlash(fw.basePath), path))
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, err
	}
	return os.Create(filePath)
}

// NewFileWriter creates a new file writer with the given base path.
func NewFileWriter(basePath string) FileWriter {
	return fileWriter{basePath}
}
