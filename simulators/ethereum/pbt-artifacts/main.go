// Simulator pbt-artifacts measures EIP-8347 artifact conformance per client
// through one shim script per client; see README.md.
package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/ethereum/hive/hivesim"
)

const (
	fixtureDir = "fixtures"
	fixtureTar = "fixtures.tar"
	anchor     = "0"

	// Shim exit codes.
	exitAccept      = 0
	exitReject      = 1
	exitUnsupported = 3
)

// matrixKeys are the capability-matrix columns tools/matrix renders, in the
// order a row lists them.
var matrixKeys = []string{
	"genesis_root", "verify_preimages", "verify_snapshot",
	"produce_preimages", "produce_snapshot", "produce_negatives",
}

var (
	mptRootRE   = regexp.MustCompile(`(?m)^mpt_root=(0x[0-9a-fA-F]{64})`)
	snapshotRE  = regexp.MustCompile(`(?m)^snapshot=([A-Za-z0-9+/=]+)`)
	preimagesRE = regexp.MustCompile(`(?m)^preimages=([A-Za-z0-9+/=]+)`)
)

func main() {
	fixtures, err := loadManifest(fixtureDir)
	if err != nil {
		panic(fmt.Sprintf("fixture set is unusable: %v", err))
	}
	suite := hivesim.Suite{
		Name:        "pbt-artifacts",
		Description: "Conformance of the EIP-8347 offline migration artifacts: the preimage file and the PBT snapshot.",
	}
	suite.Add(hivesim.TestSpec{
		Name:        "artifact conformance",
		Description: "Runs every fixture against every client.",
		AlwaysRun:   true,
		Run:         func(t *hivesim.T) { runAllClients(t, fixtures) },
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

func runAllClients(t *hivesim.T, fixtures *manifest) {
	clients, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatalf("cannot list client types: %v", err)
	}
	producers := &producerSet{by: map[string][]producerResult{}}
	var wg sync.WaitGroup
	for _, ct := range clients {
		wg.Add(1)
		go func(ct *hivesim.ClientDefinition) {
			defer wg.Done()
			t.Run(hivesim.TestSpec{
				Name:      fmt.Sprintf("%s: artifacts", ct.Name),
				AlwaysRun: true,
				Run:       func(t *hivesim.T) { runClient(t, fixtures, ct, producers) },
			})
		}(ct)
	}
	wg.Wait()
	runAgreement(t, fixtures, producers)
}

// client runs shim verbs against one datadir.
type client struct {
	*hivesim.Client
}

func (c *client) run(verb string, args ...string) *hivesim.ExecInfo {
	info, err := c.Exec(append([]string{"pbt-artifacts.sh", verb}, args...)...)
	if err != nil {
		return &hivesim.ExecInfo{ExitCode: -1, Stderr: err.Error()}
	}
	return info
}

func runClient(t *hivesim.T, fixtures *manifest, ct *hivesim.ClientDefinition, producers *producerSet) {
	shim := shimFor(ct.Name)
	files := map[string]string{
		"/genesis.json":              filepath.Join(fixtureDir, fixtures.Genesis.File),
		"/pbt-fixtures.tar":          fixtureTar,
		"/hive-bin/pbt-artifacts.sh": shim,
		"/hive-bin/pbt-common.sh":    filepath.Join("shims", "common.sh"),
	}
	report := &report{clientType: ct.Name, fields: map[string]string{}}
	defer report.publish(t)
	c := &client{Client: t.StartClient(ct.Name, hivesim.Params{"HIVE_LOGLEVEL": "3"}, hivesim.WithStaticFiles(files))}

	anchored := runGenesisRoot(t, c, fixtures, report)
	for _, suite := range []string{"preimages", "snapshot"} {
		runVerifySuite(t, c, fixtures, report, suite, anchored)
	}
	unconverted := runConvert(t, c, fixtures, report, producers, anchored)
	runProduceNegatives(t, c, fixtures, report, unconverted)
}

// runGenesisRoot checks the client built the anchor state the fixtures were
// derived from; nothing else is measurable otherwise.
func runGenesisRoot(t *hivesim.T, c *client, fixtures *manifest, report *report) bool {
	var ok bool
	t.Run(hivesim.TestSpec{
		Name:      fmt.Sprintf("%s/genesis/state-root", c.Type),
		AlwaysRun: true,
		Run: func(t *hivesim.T) {
			info := c.run("genesis-root")
			t.Logf("%s%s", info.Stdout, info.Stderr)
			switch info.ExitCode {
			case exitAccept:
			case exitUnsupported:
				report.set("genesis_root", "unsupported")
				return
			default:
				report.set("genesis_root", "crash")
				t.Fatalf("crash: genesis-root exited %d\n%s", info.ExitCode, info.Stderr)
			}
			got := match(mptRootRE, info.Stdout)
			if !strings.EqualFold(got, fixtures.Genesis.StateRoot) {
				report.set("genesis_root", "mismatch")
				t.Fatalf("anchor state root %s, the fixtures are anchored at %s", got, fixtures.Genesis.StateRoot)
			}
			report.set("genesis_root", "ok")
			ok = true
		},
	})
	return ok
}

// runVerifySuite runs one suite's fixtures. The sound pair goes first: a
// client that rejects it cannot be judged on what else it rejects.
func runVerifySuite(t *hivesim.T, c *client, fixtures *manifest, report *report, suite string, anchored bool) {
	cases := fixtures.cases(suite)
	key := "verify_" + suite
	if !anchored {
		report.set(key, "inconclusive")
		skipAll(t, c.Type, suite, len(cases), "inconclusive: anchor state root mismatch")
		return
	}

	var baseline int
	var reason string
	t.Run(hivesim.TestSpec{
		Name:      fmt.Sprintf("%s/%s/valid", c.Type, suite),
		AlwaysRun: true,
		Run: func(t *hivesim.T) {
			info := c.run("verify", fixtures.Valid.Snapshot, fixtures.Valid.Preimages, anchor)
			t.Logf("%s%s", info.Stdout, info.Stderr)
			baseline = info.ExitCode
			switch info.ExitCode {
			case exitAccept:
			case exitUnsupported:
				reason = "unsupported: this client cannot verify artifacts"
			default:
				reason = fmt.Sprintf("inconclusive: the sound pair did not verify (exit %d)", info.ExitCode)
				if info.ExitCode == exitReject {
					t.Fatalf("the sound artifact pair was rejected\n%s", info.Stderr)
				}
				t.Fatalf("crash: verify exited %d\n%s", info.ExitCode, info.Stderr)
			}
		},
	})
	switch baseline {
	case exitAccept:
	case exitUnsupported:
		report.set(key, "unsupported")
		skipAll(t, c.Type, suite, len(cases), reason)
		return
	default:
		report.set(key, "inconclusive")
		skipAll(t, c.Type, suite, len(cases), reason)
		return
	}

	var scored, passed int
	for _, tc := range cases {
		t.Run(hivesim.TestSpec{
			Name:        fmt.Sprintf("%s/%s", c.Type, tc.ID),
			Description: tc.Description,
			Run: func(t *hivesim.T) {
				info := c.run("verify", tc.Snapshot, tc.Preimages, anchor)
				t.Logf("%s%s", info.Stdout, info.Stderr)
				scored++
				switch info.ExitCode {
				case exitReject:
					if strings.TrimSpace(info.Stderr) == "" {
						t.Fatalf("rejected without saying why")
					}
					passed++
				case exitAccept:
					t.Fatalf("accepted an artifact that breaks %s", tc.ID)
				case exitUnsupported:
					t.Fatalf("unsupported: verify stopped working mid-suite")
				default:
					t.Fatalf("crash: verify exited %d\n%s", info.ExitCode, info.Stderr)
				}
			},
		})
	}
	report.set(key, fmt.Sprintf("%d/%d", passed, scored))
}

// runConvert measures the production leg per artifact: a client may produce
// one and not the other. It returns why nothing was produced, or "" when
// something was.
func runConvert(t *hivesim.T, c *client, fixtures *manifest, report *report, producers *producerSet, anchored bool) string {
	if !anchored {
		report.set("produce_preimages", "inconclusive")
		report.set("produce_snapshot", "inconclusive")
		return "inconclusive"
	}
	produced := map[string][]byte{}
	var stderr string
	unconverted := "crash"
	t.Run(hivesim.TestSpec{
		Name:      fmt.Sprintf("%s/convert/run", c.Type),
		AlwaysRun: true,
		Run: func(t *hivesim.T) {
			info := c.run("convert", anchor)
			t.Logf("%s%s", info.Stdout, info.Stderr)
			stderr = info.Stderr
			switch info.ExitCode {
			case exitAccept:
			case exitUnsupported:
				report.set("produce_preimages", "unsupported")
				report.set("produce_snapshot", "unsupported")
				unconverted = "unsupported"
				return
			default:
				report.set("produce_preimages", "crash")
				report.set("produce_snapshot", "crash")
				t.Fatalf("crash: convert exited %d\n%s", info.ExitCode, info.Stderr)
			}
			res := map[string]*regexp.Regexp{"preimages": preimagesRE, "snapshot": snapshotRE}
			for _, name := range []string{"preimages", "snapshot"} {
				raw := match(res[name], info.Stdout)
				if raw == "" {
					continue
				}
				blob, err := base64.StdEncoding.DecodeString(raw)
				if err != nil {
					report.set("produce_preimages", "crash")
					report.set("produce_snapshot", "crash")
					t.Fatalf("the %s produced is not valid base64: %v", name, err)
				}
				produced[name] = blob
			}
			if len(produced) == 0 {
				report.set("produce_preimages", "crash")
				report.set("produce_snapshot", "crash")
				t.Fatalf("convert exited 0 but produced nothing\n%s", info.Stdout)
			}
		},
	})
	if len(produced) == 0 {
		return unconverted
	}
	for _, name := range []string{"preimages", "snapshot"} {
		blob, ok := produced[name]
		if !ok {
			report.set("produce_"+name, "unsupported")
			t.Run(hivesim.TestSpec{
				Name: fmt.Sprintf("%s/convert/%s", c.Type, name),
				Run:  func(t *hivesim.T) { t.Logf("unsupported: no %s produced\n%s", name, stderr) },
			})
			continue
		}
		producers.add(name, c.Type, blob)
		verdict := "differs"
		t.Run(hivesim.TestSpec{
			Name: fmt.Sprintf("%s/convert/%s", c.Type, name),
			Run: func(t *hivesim.T) {
				want, err := fixtures.read(fixtures.validFile(name))
				if err != nil {
					t.Fatalf("crash: %v", err)
				}
				if i := firstDiff(blob, want); i >= 0 {
					t.Fatalf("differs from the canonical %s at byte %d (%d produced, %d expected)", name, i, len(blob), len(want))
				}
				verdict = "byte-identical"
			},
		})
		report.set("produce_"+name, verdict)
	}
	return ""
}

// runProduceNegatives asks the converter to refuse a defective source; it
// runs only where the sound source converted. Unsupported is a capability,
// not a failure.
func runProduceNegatives(t *hivesim.T, c *client, fixtures *manifest, report *report, unconverted string) {
	cases := fixtures.cases("produce")
	if unconverted != "" {
		report.set("produce_negatives", unconverted)
		skipAll(t, c.Type, "produce", len(cases), unconverted+": the sound source did not convert")
		return
	}
	var scored, passed int
	for _, tc := range cases {
		t.Run(hivesim.TestSpec{
			Name:        fmt.Sprintf("%s/%s", c.Type, tc.ID),
			Description: tc.Description,
			Run: func(t *hivesim.T) {
				info := c.run("convert", append([]string{anchor}, tc.Defect...)...)
				t.Logf("%s%s", info.Stdout, info.Stderr)
				if info.ExitCode == exitUnsupported {
					return
				}
				scored++
				switch {
				case info.ExitCode == exitAccept:
					t.Fatalf("converted a source that breaks %s", tc.ID)
				case info.ExitCode != exitReject:
					t.Fatalf("crash: convert exited %d\n%s", info.ExitCode, info.Stderr)
				case strings.TrimSpace(info.Stderr) == "":
					t.Fatalf("rejected without saying why")
				case match(snapshotRE, info.Stdout) != "" || match(preimagesRE, info.Stdout) != "":
					t.Fatalf("rejected, yet printed an artifact")
				}
				passed++
			},
		})
	}
	if scored == 0 {
		report.set("produce_negatives", "unsupported")
		return
	}
	report.set("produce_negatives", fmt.Sprintf("%d/%d", passed, scored))
}

// producerSet collects what each client produced, so the run can judge
// agreement between them once every client has had its turn.
type producerSet struct {
	mu sync.Mutex
	by map[string][]producerResult
}

type producerResult struct {
	client string
	blob   []byte
}

func (p *producerSet) add(artifact, client string, blob []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.by[artifact] = append(p.by[artifact], producerResult{client, blob})
}

// runAgreement fails an artifact when any producer differs from the
// canonical bytes; one producer is inconclusive.
func runAgreement(t *hivesim.T, fixtures *manifest, producers *producerSet) {
	for _, artifact := range []string{"preimages", "snapshot"} {
		results := producers.by[artifact] // every writer has returned
		t.Run(hivesim.TestSpec{
			Name: "agreement/" + artifact,
			Run: func(t *hivesim.T) {
				want, err := fixtures.read(fixtures.validFile(artifact))
				if err != nil {
					t.Fatalf("crash: %v", err)
				}
				var agree, differ []string
				for _, r := range results {
					if bytes.Equal(r.blob, want) {
						agree = append(agree, r.client)
					} else {
						differ = append(differ, r.client)
					}
				}
				switch {
				case len(results) == 0:
					t.Logf("inconclusive: no client produced a %s", artifact)
				case len(agree) == 0:
					t.Fatalf("no producer reproduces the canonical %s: %s all differ", artifact, strings.Join(differ, ", "))
				case len(differ) > 0:
					t.Fatalf("%s differ from the canonical %s; %s match", strings.Join(differ, ", "), artifact, strings.Join(agree, ", "))
				case len(agree) == 1:
					t.Logf("inconclusive: only %s produced a %s", agree[0], artifact)
				default:
					t.Logf("%d producers agree byte for byte: %s", len(agree), strings.Join(agree, ", "))
				}
			},
		})
	}
}

// skipAll records that a suite did not run and why; the failure that caused
// it already has its own row.
func skipAll(t *hivesim.T, clientName, suite string, n int, reason string) {
	t.Run(hivesim.TestSpec{
		Name: fmt.Sprintf("%s/%s/NOT-RUN", clientName, suite),
		Run:  func(t *hivesim.T) { t.Logf("%s (%d fixtures)", reason, n) },
	})
}

// report is one client's row of the capability matrix.
type report struct {
	clientType string
	fields     map[string]string
}

func (r *report) set(key, value string) { r.fields[key] = value }

// publish writes the row as an always-passing test, so it survives in the
// results whatever the cases did.
func (r *report) publish(t *hivesim.T) {
	var b strings.Builder
	fmt.Fprintf(&b, "client=%s", r.clientType)
	for _, k := range matrixKeys {
		if v, ok := r.fields[k]; ok {
			fmt.Fprintf(&b, " %s=%s", k, v)
		}
	}
	summary := b.String()
	t.Run(hivesim.TestSpec{
		Name: fmt.Sprintf("%s/capability-matrix", r.clientType),
		Run:  func(t *hivesim.T) { t.Log(summary) },
	})
}

// shimFor tries <client>_<nametag> then the base name; a client with no shim
// gets unsupported.sh.
func shimFor(clientName string) string {
	for _, name := range []string{clientName, strings.SplitN(clientName, "_", 2)[0]} {
		path := filepath.Join("shims", name+".sh")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		return path
	}
	return filepath.Join("shims", "unsupported.sh")
}

func match(re *regexp.Regexp, out string) string {
	if m := re.FindStringSubmatch(out); len(m) == 2 {
		return m[1]
	}
	return ""
}

// firstDiff returns the offset of the first differing byte, or -1.
func firstDiff(got, want []byte) int {
	for i := range min(len(got), len(want)) {
		if got[i] != want[i] {
			return i
		}
	}
	if len(got) != len(want) {
		return min(len(got), len(want))
	}
	return -1
}
