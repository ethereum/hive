// Test description struct and methods to add items to the description
package utils

import (
	"fmt"
	"strings"

	"github.com/lithammer/dedent"
)

// Description struct to hold the description of a test structured in different categories
type Description struct {
	Main        string
	Subsections []*subsection
}

type subsection struct {
	category string
	items    []string
}

const (
	CategoryTestnetConfiguration         = "Testnet Configuration"
	CategoryVerificationsExecutionClient = "Verifications (Execution Client)"
	CategoryVerificationsConsensusClient = "Verifications (Consensus Client)"
)

// NewDescription creates a new instance of Description
func NewDescription(main string) *Description {
	return &Description{
		Main:        main,
		Subsections: make([]*subsection, 0),
	}
}

func (d *Description) getSubsection(category string) *subsection {
	for _, s := range d.Subsections {
		if s.category == category {
			return s
		}
	}
	s := &subsection{
		category: category,
		items:    make([]string, 0),
	}
	d.Subsections = append(d.Subsections, s)
	return s
}

// Add method to add an item to a category
func (d *Description) Add(category, item string) {
	// Check if the category already exists
	s := d.getSubsection(category)

	// Append the item to the category
	s.items = append(s.items, item)
}

func (d *Description) Format() string {
	// Create a string builder
	sb := strings.Builder{}
	// Add the main description
	sb.WriteString(dedent.Dedent(d.Main))
	// Iterate over the categories
	for _, s := range d.Subsections {
		// Add the category to the string builder
		sb.WriteString(fmt.Sprintf("\n\n#### %s\n\n", s.category))
		// Iterate over the Subsections
		for _, item := range s.items {
			// Add the item to the string builder
			sb.WriteString(dedent.Dedent(item))
		}
	}
	return sb.String()
}
