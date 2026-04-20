package display

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

// Palette colors used for diff output. Writes through these honor the shared
// fatih/color NoColor switch, which is toggled by -no-color at the CLI layer.
var (
	Green  = color.New(color.FgGreen)
	Red    = color.New(color.FgRed)
	Yellow = color.New(color.FgYellow)
	Bold   = color.New(color.Bold)
	Cyan   = color.New(color.FgCyan)
)

// ColorizeDiff takes a raw diff/log string and prints it with color.
// Lines starting with ++ are green, lines starting with -- are red.
func ColorizeDiff(text string, noColor bool) {
	if noColor {
		fmt.Print(text)
		return
	}
	for _, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "++"):
			Green.Println(line)
		case strings.HasPrefix(line, "--"):
			Red.Println(line)
		case strings.HasPrefix(line, "@@"):
			Cyan.Println(line)
		default:
			fmt.Println(line)
		}
	}
}

// CompactDiff prints only lines that contain differences (inline -- ++ markers)
// or structural markers (>>, <<, "response differs"), plus surrounding context lines.
func CompactDiff(text string, context int, noColor bool) {
	lines := strings.Split(text, "\n")

	// Mark which lines have actual diffs or are header lines.
	isDiff := make([]bool, len(lines))
	isHeader := make([]bool, len(lines))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(line, " -- ") && strings.Contains(line, " ++ ") {
			// Inline diff: key: -- "old" ++ "new"
			isDiff[i] = true
		} else if strings.HasPrefix(trimmed, "++ ") || strings.HasPrefix(trimmed, "-- ") {
			// Line-level add/remove
			isDiff[i] = true
		} else if strings.HasPrefix(trimmed, "response differs") {
			isHeader[i] = true
		}
		// Skip >> and << lines (raw request/response): they're just noise.
	}

	// Build set of lines to show (diff lines + context, plus headers).
	show := make([]bool, len(lines))
	for i := range lines {
		if isHeader[i] {
			show[i] = true
		}
		if isDiff[i] {
			for j := max(0, i-context); j <= min(len(lines)-1, i+context); j++ {
				show[j] = true
			}
		}
	}

	// Suppress >> and << lines (raw request/response blobs).
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ">> ") || strings.HasPrefix(trimmed, "<< ") {
			show[i] = false
		}
	}

	lastShown := -1
	for i, line := range lines {
		if !show[i] {
			continue
		}
		if lastShown >= 0 && i > lastShown+1 {
			Cyan.Println("  ...")
		}
		lastShown = i

		trimmed := strings.TrimSpace(line)
		isLineDiff := strings.HasPrefix(trimmed, "++ ") || strings.HasPrefix(trimmed, "-- ")
		isInlineDiff := !isLineDiff && isDiff[i]

		if noColor {
			if isInlineDiff {
				printInlineDiffPlain(line)
			} else {
				fmt.Println(line)
			}
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "++ "):
			Green.Println(line)
		case strings.HasPrefix(trimmed, "-- "):
			Red.Println(line)
		case isInlineDiff:
			colorizeInlineDiff(line)
		default:
			fmt.Println(line)
		}
	}
}

// printInlineDiffPlain prints a line with inline diffs as "old -> new" without color.
func printInlineDiffPlain(line string) {
	for len(line) > 0 {
		dIdx := strings.Index(line, " -- ")
		if dIdx < 0 {
			fmt.Print(line)
			break
		}
		pIdx := strings.Index(line[dIdx:], " ++ ")
		if pIdx < 0 {
			fmt.Print(line)
			break
		}
		pIdx += dIdx
		rest := line[pIdx+4:]
		endIdx := findValueEnd(rest)
		newVal := rest[:endIdx]
		fmt.Print(line[:dIdx+1])
		fmt.Print(line[dIdx+4 : pIdx])
		fmt.Print(" -> ")
		fmt.Print(newVal)
		line = rest[endIdx:]
	}
	fmt.Println()
}

// colorizeInlineDiff prints a line with inline -- old ++ new markers colorized.
func colorizeInlineDiff(line string) {
	for len(line) > 0 {
		dIdx := strings.Index(line, " -- ")
		if dIdx < 0 {
			fmt.Print(line)
			break
		}
		// Find matching ++
		pIdx := strings.Index(line[dIdx:], " ++ ")
		if pIdx < 0 {
			fmt.Print(line)
			break
		}
		pIdx += dIdx

		// Find end of ++ value: look for comma, closing bracket, or end of line.
		rest := line[pIdx+4:]
		endIdx := findValueEnd(rest)
		newVal := rest[:endIdx]

		fmt.Print(line[:dIdx])
		Red.Print(line[dIdx+4 : pIdx])
		fmt.Print(" -> ")
		Green.Print(newVal)
		line = rest[endIdx:]
	}
	fmt.Println()
}

// findValueEnd finds the end of a JSON value in an inline diff.
func findValueEnd(s string) int {
	if len(s) == 0 {
		return 0
	}
	if s[0] == '"' {
		// Find closing quote (handle escaped quotes).
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' {
				i++
				continue
			}
			if s[i] == '"' {
				return i + 1
			}
		}
		return len(s)
	}
	// Non-quoted: ends at comma, space, or end.
	for i := 0; i < len(s); i++ {
		if s[i] == ',' || s[i] == ' ' || s[i] == '}' || s[i] == ']' {
			return i
		}
	}
	return len(s)
}

// PassFail returns a colored pass/fail string.
func PassFail(pass bool) string {
	if pass {
		return Green.Sprint("PASS")
	}
	return Red.Sprint("FAIL")
}

// PassFailCount returns a colored string like "10/12".
func PassFailCount(passes, total int) string {
	if passes == total {
		return Green.Sprintf("%d/%d", passes, total)
	}
	return Yellow.Sprintf("%d/%d", passes, total)
}
