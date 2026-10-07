// Command matrix renders the capability matrix from a hive results
// directory: one row per client, one column per verb, plus the clients that
// have no PBT work at all and therefore never ran.
//
//	go -C simulators/ethereum/pbt-artifacts run ./tools/matrix "$PWD/workspace/logs" > CAPABILITY.md
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// absent are clients with no PBT branch to pin, listed so the gap is stated
// rather than silent.
var absent = map[string]string{
	"reth": "no PBT branch",
}

// columns are split per artifact: no client does all four.
var columns = []struct{ key, header string }{
	{"genesis_root", "anchor state"},
	{"verify_preimages", "consume preimages"},
	{"verify_snapshot", "consume snapshot"},
	{"produce_preimages", "produce preimages"},
	{"produce_snapshot", "produce snapshot"},
	{"produce_negatives", "refuse a bad source"},
}

type suiteFile struct {
	Name           string `json:"name"`
	TestDetailsLog string `json:"testDetailsLog"`
	TestCases      map[string]struct {
		Name          string `json:"name"`
		SummaryResult struct {
			Pass bool `json:"pass"`
			Log  struct {
				Begin int64 `json:"begin"`
				End   int64 `json:"end"`
			} `json:"log"`
		} `json:"summaryResult"`
	} `json:"testCases"`
}

func main() {
	dir := "workspace/logs"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	rows, failures, agreement, err := collect(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	render(os.Stdout, rows, failures, agreement)
}

// collect reads every suite file in dir. The newest run mentioning a client
// replaces whatever an older one said about it.
func collect(dir string) (map[string]map[string]string, map[string][]string, map[string]string, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, nil, nil, err
	}
	slices.SortFunc(entries, func(a, b string) int {
		fa, erra := os.Stat(a)
		fb, errb := os.Stat(b)
		if erra != nil || errb != nil {
			return strings.Compare(a, b)
		}
		return fa.ModTime().Compare(fb.ModTime())
	})
	var (
		rows      = make(map[string]map[string]string)
		failures  = make(map[string][]string)
		agreement = make(map[string]string)
	)
	for _, path := range entries {
		if filepath.Base(path) == "hive.json" {
			continue
		}
		blob, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, nil, err
		}
		var suite suiteFile
		if err := json.Unmarshal(blob, &suite); err != nil || suite.Name != "pbt-artifacts" {
			continue
		}
		details, err := os.ReadFile(filepath.Join(dir, suite.TestDetailsLog))
		if err != nil {
			details = nil
		}
		var (
			runRows     = make(map[string]map[string]string)
			runFailures = make(map[string][]string)
			seen        = make(map[string]bool)
		)
		for _, tc := range suite.TestCases {
			name := tc.Name
			if artifact, ok := strings.CutPrefix(name, "agreement/"); ok {
				verdict := strings.TrimSpace(slice(details, tc.SummaryResult.Log.Begin, tc.SummaryResult.Log.End))
				if verdict == "" {
					verdict = "no detail recorded"
				}
				agreement[artifact] = verdict
				continue
			}
			client, rest, ok := strings.Cut(name, "/")
			if !ok {
				continue
			}
			seen[client] = true
			switch {
			case rest == "capability-matrix":
				text := slice(details, tc.SummaryResult.Log.Begin, tc.SummaryResult.Log.End)
				if fields := parseRow(text); fields != nil {
					runRows[client] = fields
				}
			case !tc.SummaryResult.Pass:
				runFailures[client] = append(runFailures[client], rest)
			}
		}
		for client := range seen {
			delete(rows, client)
			delete(failures, client)
		}
		for client, fields := range runRows {
			rows[client] = fields
		}
		for client, cases := range runFailures {
			failures[client] = cases
		}
	}
	return rows, failures, agreement, nil
}

func slice(blob []byte, begin, end int64) string {
	if blob == nil || begin < 0 || end > int64(len(blob)) || begin >= end {
		return ""
	}
	return string(blob[begin:end])
}

// parseRow reads one "client=... key=value ..." line.
func parseRow(text string) map[string]string {
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.HasPrefix(line, "client=") {
			continue
		}
		fields := make(map[string]string)
		for _, field := range strings.Fields(line) {
			k, v, ok := strings.Cut(field, "=")
			if !ok || k == "client" {
				continue
			}
			fields[k] = v
		}
		return fields
	}
	return nil
}

func render(out io.Writer, rows map[string]map[string]string, failures map[string][]string, agreement map[string]string) {
	fmt.Fprintln(out, "# EIP-8347 offline-artifact capability matrix")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Measured by `hive --sim ethereum/pbt-artifacts`. A cell is the count of")
	fmt.Fprintln(out, "fixtures the client judged as the EIP requires, `unsupported` where the client")
	fmt.Fprintln(out, "has no such feature, and `inconclusive` where an earlier check made the rest")
	fmt.Fprintln(out, "unreadable.")
	fmt.Fprintln(out)

	header := "|client|"
	sep := "|---|"
	for _, c := range columns {
		header += c.header + "|"
		sep += "---|"
	}
	fmt.Fprintln(out, header)
	fmt.Fprintln(out, sep)

	names := slices.Collect(maps.Keys(rows))
	for client := range failures {
		if _, ok := rows[client]; !ok {
			names = append(names, client)
		}
	}
	slices.Sort(names)

	for _, name := range names {
		line := "|" + name + "|"
		for _, c := range columns {
			v := rows[name][c.key]
			if v == "" {
				v = "not run"
			}
			line += v + "|"
		}
		fmt.Fprintln(out, line)
	}
	for client, why := range absent {
		line := "|" + client + "|" + why + "|"
		for range columns[1:] {
			line += "|"
		}
		fmt.Fprintln(out, line)
	}

	if len(agreement) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "## Producer agreement")
		fmt.Fprintln(out)
		for _, artifact := range []string{"preimages", "snapshot"} {
			if verdict, ok := agreement[artifact]; ok {
				fmt.Fprintf(out, "- **%s**: %s\n", artifact, verdict)
			}
		}
	}

	if len(failures) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "## Cases that did not hold")
		for _, name := range names {
			cases := failures[name]
			if len(cases) == 0 {
				continue
			}
			slices.Sort(cases)
			fmt.Fprintf(out, "\n**%s**\n", name)
			for _, c := range cases {
				fmt.Fprintf(out, "- `%s`\n", c)
			}
		}
	}
}
