package libhive

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lithammer/dedent"
)

func toMarkdownLink(title string) string {
	removeChars := []string{":", "#", "'", "\"", "`", "*", "+", ",", ";"}
	title = strings.ReplaceAll(strings.ToLower(title), " ", "-")
	for _, invalidChar := range removeChars {
		title = strings.ReplaceAll(title, invalidChar, "")
	}
	return title
}

func toMarkdownFileName(title string) string {
	title = strings.ReplaceAll(strings.ToUpper(title), " ", "-")
	return fmt.Sprintf("TESTS-%s.md", title)
}

type markdownTestCase TestCase

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
	desc := dedent.Dedent(tc.Description)
	// Replace single quotes with backticks, since backticks are used for markdown code blocks and
	// they cannot be escaped in golang.
	desc = strings.ReplaceAll(desc, "'", "`")
	return desc
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

type markdownSuite TestSuite

func (s *markdownSuite) displayName() string {
	if s.DisplayName != "" {
		return s.DisplayName
	}
	return fmt.Sprintf("`%s`", s.Name)
}

func (s *markdownSuite) mardownFileName() string {
	return toMarkdownFileName(s.Name)
}

func (s *markdownSuite) commandLine(simName string) string {
	return fmt.Sprintf("./hive --client <CLIENTS> --sim %s --sim.limit \"%s/\"", simName, s.Name)
}

func (s *markdownSuite) description() string {
	desc := dedent.Dedent(s.Description)
	// Replace single quotes with backticks, since backticks are used for markdown code blocks and
	// they cannot be escaped in golang.
	desc = strings.ReplaceAll(desc, "'", "`")
	return desc
}

func (s *markdownSuite) testCases() map[TestID]*markdownTestCase {
	testCases := make(map[TestID]*markdownTestCase)
	for testID, tc := range s.TestCases {
		testCases[testID] = (*markdownTestCase)(tc)
	}
	return testCases
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

	categories := getCategories(s.testCases())

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

	for _, tc := range s.testCases() {
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
	file, err := fw.CreateWriter(s.mardownFileName())
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

func toMarkdownSuiteMap(testMap map[TestSuiteID]*TestSuite) map[TestSuiteID]*markdownSuite {
	markdownMap := make(map[TestSuiteID]*markdownSuite)
	for testID, suite := range testMap {
		markdownMap[testID] = (*markdownSuite)(suite)
	}
	return markdownMap
}

func generateIndex(simName string, testMap map[TestSuiteID]*markdownSuite) (string, error) {
	headerBuilder := strings.Builder{}
	headerBuilder.WriteString(fmt.Sprintf("# Simulator `%s` Test Cases\n\n", simName))
	headerBuilder.WriteString("## Test Suites\n\n")
	for _, s := range testMap {
		headerBuilder.WriteString(fmt.Sprintf("### - [%s](%s)\n", s.displayName(), s.mardownFileName()))
		headerBuilder.WriteString(s.Description)
		headerBuilder.WriteString("\n\n")
	}
	return headerBuilder.String() + "\n", nil
}

func generateIndexFile(fw FileWriter, simName string, testMap map[TestSuiteID]*markdownSuite) error {
	markdownIndex, err := generateIndex(simName, testMap)
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

func GenSimulatorMarkdownFiles(fw FileWriter, simName string, testMap map[TestSuiteID]*TestSuite) error {
	markdownSuiteMap := toMarkdownSuiteMap(testMap)
	err := generateIndexFile(fw, simName, markdownSuiteMap)
	if err != nil {
		return err
	}
	for _, s := range markdownSuiteMap {
		if err := s.toMarkdownFile(fw, simName); err != nil {
			return err
		}
	}
	return nil
}
