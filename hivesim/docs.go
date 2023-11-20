package hivesim

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
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
	sb.WriteString(fmt.Sprintf("%s %s\n\n", strings.Repeat("#", depth), tc.displayName()))

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

func getCategories(testCases map[TestID]*markdownTestCase) []string {
	categoryMap := make(map[string]struct{})
	categories := make([]string, 0)
	for _, tc := range testCases {
		if tc.toBePrinted() {
			if _, ok := categoryMap[tc.Category]; !ok {
				categories = append(categories, tc.Category)
				categoryMap[tc.Category] = struct{}{}
			}
		}
	}
	return categories
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

	categories := getCategories(s.tests)

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

	for _, tc := range s.tests {
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

// Docs Simulation object
type docsSimulation struct {
	testMatcher
	simName   string
	outputDir string
	suites    map[SuiteID]*markdownSuite
}

// NewDocsSimulation creates a new docs simulation object.
func NewDocsSimulation() Simulation {
	var err error
	sim := &docsSimulation{
		suites: make(map[SuiteID]*markdownSuite),
	}
	if p := os.Getenv("HIVE_TEST_PATTERN"); p != "" {
		m, err := parseTestPattern(p)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Warning: ignoring invalid test pattern regexp: "+err.Error())
		}
		sim.testMatcher = m
	}
	// Set the simulation name: if `HIVE_SIMULATOR_NAME` is set, use that, otherwise use the
	// folder name and its parent.
	sim.simName = os.Getenv("HIVE_SIMULATOR_NAME")
	if sim.simName == "" {
		binPath, err := os.Executable()
		if err != nil {
			panic(err)
		}
		binPath = filepath.Dir(binPath)
		simName1, simName2 := filepath.Base(binPath), filepath.Base(filepath.Dir(binPath))
		sim.simName = fmt.Sprintf("%s/%s", simName2, simName1)
	}

	// Set the output directory: if `HIVE_DOCS_OUTPUT_DIR` is set, use that, otherwise use the
	// current directory.
	sim.outputDir = os.Getenv("HIVE_DOCS_OUTPUT_DIR")
	if sim.outputDir == "" {
		sim.outputDir, err = os.Getwd()
		if err != nil {
			panic(err)
		}
	}

	return sim
}

// Returns true if any suite is still running
func (sim *docsSimulation) AnyRunning() bool {
	for _, s := range sim.suites {
		if s.running {
			return true
		}
	}
	return false
}

var _ Simulation = (*docsSimulation)(nil)

func (sim *docsSimulation) SetTestPattern(p string) {
	m, err := parseTestPattern(p)
	if err != nil {
		panic("invalid test pattern regexp: " + err.Error())
	}
	sim.testMatcher = m
}

func (sim *docsSimulation) TestPattern() (suiteExpr string, testNameExpr string) {
	if sim.testMatcher.suite != nil {
		suiteExpr = sim.testMatcher.suite.String()
	}
	if sim.testMatcher.test != nil {
		testNameExpr = sim.testMatcher.test.String()
	}
	return
}

// Return the simulation log level.
func (sim *docsSimulation) LogLevel() int {
	return 1
}

// True for docs simulator since we are only collecting test info for the documentation.
func (sim *docsSimulation) CollectTestsOnly() bool {
	return true
}

func (sim *docsSimulation) EndTest(testSuite SuiteID, test TestID, testResult TestResult) error {
	// No-op in docs mode.
	return nil
}

func (sim *docsSimulation) StartSuite(suite *simapi.TestRequest, simlog string) (SuiteID, error) {
	// Create a new markdown suite.
	markdownSuite := &markdownSuite{
		TestRequest: *suite,
		running:     true,
		tests:       make(map[TestID]*markdownTestCase),
	}
	// Next suite id
	suiteID := SuiteID(len(sim.suites))
	// Add the suite to the map.
	sim.suites[suiteID] = markdownSuite
	// Return the suite ID.
	return suiteID, nil
}

func (sim *docsSimulation) EndSuite(testSuite SuiteID) error {
	suite, ok := sim.suites[testSuite]
	if !ok {
		return fmt.Errorf("test suite %d does not exist", testSuite)
	}
	suite.running = false
	if !sim.AnyRunning() {
		// Generate markdown files when all suites are done.
		if err := sim.genSimulatorMarkdownFiles(NewFileWriter(sim.outputDir)); err != nil {
			log15.Error("can't generate markdown files", "err", err)
		}
	}
	return nil
}

func (sim *docsSimulation) StartTest(testSuite SuiteID, test *simapi.TestRequest) (TestID, error) {
	// Create a new markdown test case.
	markdownTest := markdownTestCase(*test)
	// Check if suite exists.
	if _, ok := sim.suites[testSuite]; !ok {
		return 0, fmt.Errorf("test suite %d does not exist", testSuite)
	}
	// Next test id
	testID := TestID(len(sim.suites[testSuite].tests))
	// Add the test to the map.
	sim.suites[testSuite].tests[testID] = &markdownTest
	// Return the test ID.
	return testID, nil
}

func (sim *docsSimulation) ClientTypes() ([]*ClientDefinition, error) {
	// Return a dummy "docs" client type.
	return []*ClientDefinition{
		{
			Name:    "Client",
			Version: "1.0.0",
		},
	}, nil
}

func (sim *docsSimulation) StartClient(testSuite SuiteID, test TestID, parameters map[string]string, initFiles map[string]string) (string, net.IP, error) {
	// Attempting to start a client in docs mode is an error.
	return "", nil, errors.New("can't start client in docs mode")
}

func (sim *docsSimulation) StartClientWithOptions(testSuite SuiteID, test TestID, clientType string, options ...StartOption) (string, net.IP, error) {
	// Attempting to start a client in docs mode is an error.
	return "", nil, errors.New("can't start client in docs mode")
}

func (sim *docsSimulation) StopClient(testSuite SuiteID, test TestID, nodeid string) error {
	// Attempting to stop a client in docs mode is an error.
	return errors.New("can't stop client in docs mode")
}

func (sim *docsSimulation) PauseClient(testSuite SuiteID, test TestID, nodeid string) error {
	// Attempting to pause a client in docs mode is an error.
	return errors.New("can't pause client in docs mode")
}

func (sim *docsSimulation) UnpauseClient(testSuite SuiteID, test TestID, nodeid string) error {
	// Attempting to unpause a client in docs mode is an error.
	return errors.New("can't unpause client in docs mode")
}

func (sim *docsSimulation) ClientEnodeURL(testSuite SuiteID, test TestID, node string) (string, error) {
	// Attempting to get enode URL in docs mode is an error.
	return "", errors.New("can't get enode URL in docs mode")
}

func (sim *docsSimulation) ClientEnodeURLNetwork(testSuite SuiteID, test TestID, node string, network string) (string, error) {
	// Attempting to get enode URL in docs mode is an error.
	return "", errors.New("can't get enode URL in docs mode")
}

func (sim *docsSimulation) ClientExec(testSuite SuiteID, test TestID, nodeid string, cmd []string) (*ExecInfo, error) {
	// Attempting to exec in docs mode is an error.
	return nil, errors.New("can't exec in docs mode")
}

func (sim *docsSimulation) CreateNetwork(testSuite SuiteID, networkName string) error {
	// Attempting to create a network in docs mode is an error.
	return errors.New("can't create network in docs mode")
}

func (sim *docsSimulation) RemoveNetwork(testSuite SuiteID, network string) error {
	// Attempting to remove a network in docs mode is an error.
	return errors.New("can't remove network in docs mode")
}

func (sim *docsSimulation) ConnectContainer(testSuite SuiteID, network, containerID string) error {
	// Attempting to connect a container in docs mode is an error.
	return errors.New("can't connect container in docs mode")
}

func (sim *docsSimulation) DisconnectContainer(testSuite SuiteID, network, containerID string) error {
	// Attempting to disconnect a container in docs mode is an error.
	return errors.New("can't disconnect container in docs mode")
}

func (sim *docsSimulation) ContainerNetworkIP(testSuite SuiteID, network, containerID string) (string, error) {
	// Attempting to get container IP in docs mode is an error.
	return "", errors.New("can't get container IP in docs mode")
}

func (sim *docsSimulation) generateIndex(simName string) (string, error) {
	headerBuilder := strings.Builder{}
	headerBuilder.WriteString(fmt.Sprintf("# Simulator `%s` Test Cases\n\n", simName))
	headerBuilder.WriteString("## Test Suites\n\n")
	for _, s := range sim.suites {
		headerBuilder.WriteString(fmt.Sprintf("### - [%s](%s)\n", s.displayName(), s.mardownFilePath()))
		headerBuilder.WriteString(s.Description)
		headerBuilder.WriteString("\n\n")
	}
	return headerBuilder.String() + "\n", nil
}

func (sim *docsSimulation) generateIndexFile(fw FileWriter, simName string) error {
	markdownIndex, err := sim.generateIndex(simName)
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

func (sim *docsSimulation) genSimulatorMarkdownFiles(fw FileWriter) error {
	err := sim.generateIndexFile(fw, sim.simName)
	if err != nil {
		return err
	}
	for _, s := range sim.suites {
		if err := s.toMarkdownFile(fw, sim.simName); err != nil {
			return err
		}
	}
	return nil
}
