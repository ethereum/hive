package main

import (
	"testing"
	"testing/fstest"
	"time"
)

func TestBuildRunStatusFromSuitePlanBeforeSuitesFinish(t *testing.T) {
	simLog := "1770000000-simulator-abc.log"
	now := time.Unix(1770000010, 0)
	fsys := fstest.MapFS{
		simLog: {
			Data: []byte(
				`HIVE_SUITE_PLAN {"devnet":"devnet5","suites":[{"name":"rpc-compat","tests":64},{"name":"sync","tests":2},{"name":"client-interop","tests":2}]}` + "\n" +
					`HIVE_RUN_HEARTBEAT {}` + "\n",
			),
			ModTime: time.Unix(1770000005, 0),
		},
	}

	status, err := buildRunStatusAt(fsys, now)
	if err != nil {
		t.Fatal(err)
	}
	if status.SimLog != simLog {
		t.Fatalf("simlog mismatch: got %q, want %q", status.SimLog, simLog)
	}
	if status.State != "running" {
		t.Fatalf("state mismatch: got %q, want running", status.State)
	}
	if status.Devnet != "devnet5" {
		t.Fatalf("devnet mismatch: got %q, want devnet5", status.Devnet)
	}
	wantStates := []suiteRunStatus{
		{Name: "rpc-compat", State: "in-progress", Total: 64},
		{Name: "sync", State: "pending", Total: 2},
		{Name: "client-interop", State: "pending", Total: 2},
	}
	if len(status.Suites) != len(wantStates) {
		t.Fatalf("suite count mismatch: got %d, want %d", len(status.Suites), len(wantStates))
	}
	for i, want := range wantStates {
		if status.Suites[i] != want {
			t.Fatalf("suite %d mismatch: got %+v, want %+v", i, status.Suites[i], want)
		}
	}
}

func TestBuildRunStatusFromSuitePlanAndCompletedSuites(t *testing.T) {
	simLog := "1770000000-simulator-abc.log"
	now := time.Unix(1770000110, 0)
	fsys := fstest.MapFS{
		simLog: {
			Data: []byte(
				`HIVE_SUITE_PLAN {"suites":[{"name":"rpc-compat","tests":3},{"name":"sync","tests":1},{"name":"client-interop","tests":1}]}` + "\n" +
					`HIVE_SUITE_PROGRESS {"suite":"sync"}` + "\n" +
					`Starting lean-spec local helper on devnet4 using /app/devnet4` + "\n" +
					`HIVE_RUN_HEARTBEAT {}` + "\n",
			),
			ModTime: time.Unix(1770000000, 0),
		},
		"1770000100-rpc.json": {
			Data: []byte(`{
				"name":"rpc-compat",
				"testCases":{
					"1":{"name":"rpc-compat: client launch","start":"2026-01-01T00:00:00Z","end":"2026-01-01T00:00:01Z","summaryResult":{"pass":true},"clientInfo":{"1":{"name":"ream_devnet4"}}},
					"2":{"name":"rpc-compat: test one","start":"2026-01-01T00:00:00Z","end":"2026-01-01T00:00:01Z","summaryResult":{"pass":true},"clientInfo":{"1":{"name":"ream_devnet4"}}},
					"3":{"name":"rpc-compat: shared context","start":"2026-01-01T00:00:00Z","end":"2026-01-01T00:00:01Z","summaryResult":{"pass":true},"clientInfo":{},"multiTestContext":true},
					"4":{"name":"rpc-compat: test two","start":"2026-01-01T00:00:00Z","end":"2026-01-01T00:00:01Z","summaryResult":{"pass":true},"clientInfo":{"1":{"name":"ream_devnet4"}}}
				},
				"simLog":"1770000000-simulator-abc.log"
			}`),
			ModTime: time.Unix(1770000100, 0),
		},
	}

	status, err := buildRunStatusAt(fsys, now)
	if err != nil {
		t.Fatal(err)
	}
	if status.SimLog != simLog {
		t.Fatalf("simlog mismatch: got %q, want %q", status.SimLog, simLog)
	}
	if status.State != "running" {
		t.Fatalf("state mismatch: got %q, want running", status.State)
	}
	if status.Devnet != "devnet4" {
		t.Fatalf("devnet mismatch: got %q, want devnet4", status.Devnet)
	}
	completedAt := time.Unix(1770000100, 0)
	wantStates := []suiteRunStatus{
		{Name: "rpc-compat", State: "finished", Finished: 3, Total: 3},
		{Name: "sync", State: "in-progress", Finished: 1, Total: 1},
		{Name: "client-interop", State: "pending", Total: 1},
	}
	if len(status.Suites) != len(wantStates) {
		t.Fatalf("suite count mismatch: got %d, want %d", len(status.Suites), len(wantStates))
	}
	for i, want := range wantStates {
		got := status.Suites[i]
		if want.State == "finished" {
			if got.CompletedAt == nil || !got.CompletedAt.Equal(completedAt) {
				t.Fatalf("suite %d completedAt mismatch: got %v, want %v", i, got.CompletedAt, completedAt)
			}
			got.CompletedAt = nil
		}
		if got != want {
			t.Fatalf("suite %d mismatch: got %+v, want %+v", i, got, want)
		}
	}
}

func TestBuildRunStatusUsesRuntimeCountsWhenPlanIsStale(t *testing.T) {
	simLog := "1770000000-simulator-abc.log"
	fsys := fstest.MapFS{
		simLog: {
			Data: []byte(
				`HIVE_SUITE_PLAN {"suites":[{"name":"sync","tests":1},{"name":"rpc-compat","tests":1},{"name":"client-interop","tests":1}]}` + "\n" +
					`HIVE_SUITE_PROGRESS {"suite":"rpc-compat"}` + "\n" +
					`HIVE_SUITE_PROGRESS {"suite":"rpc-compat"}` + "\n" +
					`HIVE_RUN_HEARTBEAT {}` + "\n",
			),
			ModTime: time.Unix(1770000000, 0),
		},
		"1770000100-sync.json": {
			Data: []byte(`{
				"name":"sync",
				"testCases":{
					"1":{"name":"sync: client launch","start":"2026-01-01T00:00:00Z","end":"2026-01-01T00:00:01Z","summaryResult":{"pass":true},"clientInfo":{}},
					"2":{"name":"sync: checkpoint sync fresh start","start":"2026-01-01T00:00:00Z","end":"2026-01-01T00:00:01Z","summaryResult":{"pass":true},"clientInfo":{}},
					"3":{"name":"sync: optimistic catch up","start":"2026-01-01T00:00:00Z","end":"2026-01-01T00:00:01Z","summaryResult":{"pass":true},"clientInfo":{}},
					"4":{"name":"sync: finalized catch up","start":"2026-01-01T00:00:00Z","end":"2026-01-01T00:00:01Z","summaryResult":{"pass":true},"clientInfo":{}}
				},
				"simLog":"1770000000-simulator-abc.log"
			}`),
			ModTime: time.Unix(1770000100, 0),
		},
	}

	status, err := buildRunStatusAt(fsys, time.Unix(1770000110, 0))
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "running" {
		t.Fatalf("state mismatch: got %q, want running", status.State)
	}
	if got := status.Suites[0]; got.Name != "sync" || got.Finished != 3 || got.Total != 3 {
		t.Fatalf("completed suite mismatch: got %+v, want sync 3/3", got)
	}
	if got := status.Suites[1]; got.Name != "rpc-compat" || got.Finished != 2 || got.Total != 2 {
		t.Fatalf("in-progress suite mismatch: got %+v, want rpc-compat 2/2", got)
	}
}

func TestBuildRunStatusFinished(t *testing.T) {
	simLog := "1770000000-simulator-abc.log"
	fsys := fstest.MapFS{
		simLog: {
			Data: []byte(`HIVE_SUITE_PLAN {"suites":["rpc-compat"]}` + "\n"),
		},
		"1770000100-rpc.json": {
			Data: []byte(`{
				"name":"rpc-compat",
				"testCases":{"1":{"name":"rpc-compat: test","start":"2026-01-01T00:00:00Z","end":"2026-01-01T00:00:01Z","summaryResult":{"pass":true},"clientInfo":{}}},
				"simLog":"1770000000-simulator-abc.log"
			}`),
		},
	}

	status, err := buildRunStatus(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "finished" {
		t.Fatalf("state mismatch: got %q, want finished", status.State)
	}
	if got := status.Suites[0].State; got != "finished" {
		t.Fatalf("suite state mismatch: got %q, want finished", got)
	}
}

func TestBuildRunStatusHidesInterruptedStaleRun(t *testing.T) {
	simLog := "1770000000-simulator-abc.log"
	fsys := fstest.MapFS{
		simLog: {
			Data: []byte(
				`HIVE_SUITE_PLAN {"suites":[{"name":"rpc-compat","tests":64},{"name":"sync","tests":2}]}` + "\n" +
					`HIVE_RUN_HEARTBEAT {}` + "\n",
			),
			ModTime: time.Unix(1770000000, 0),
		},
	}

	status, err := buildRunStatusAt(fsys, time.Unix(1770000000, 0).Add(runStatusStaleAfter+time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "unknown" {
		t.Fatalf("state mismatch: got %q, want unknown", status.State)
	}
	if len(status.Suites) != 0 {
		t.Fatalf("stale run should not expose suites, got %d", len(status.Suites))
	}
}

func TestBuildRunStatusHidesClosedIncompleteRun(t *testing.T) {
	simLog := "1770000000-simulator-abc.log"
	fsys := fstest.MapFS{
		simLog: {
			Data: []byte(
				`HIVE_SUITE_PLAN {"suites":[{"name":"rpc-compat","tests":64},{"name":"sync","tests":2}]}` + "\n" +
					`HIVE_RUN_HEARTBEAT {}` + "\n",
			),
			ModTime: time.Unix(1770000005, 0),
		},
	}

	status, err := buildRunStatusAtWithOptions(fsys, time.Unix(1770000010, 0), runStatusOptions{
		simulatorLogWriteState: func(string) simulatorLogWriteState {
			return simulatorLogWriteClosed
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "unknown" {
		t.Fatalf("state mismatch: got %q, want unknown", status.State)
	}
	if len(status.Suites) != 0 {
		t.Fatalf("closed incomplete run should not expose suites, got %d", len(status.Suites))
	}
}

func TestBuildRunStatusKeepsOpenStaleRun(t *testing.T) {
	simLog := "1770000000-simulator-abc.log"
	fsys := fstest.MapFS{
		simLog: {
			Data: []byte(
				`HIVE_SUITE_PLAN {"suites":[{"name":"rpc-compat","tests":64},{"name":"sync","tests":2}]}` + "\n" +
					`HIVE_RUN_HEARTBEAT {}` + "\n",
			),
			ModTime: time.Unix(1770000000, 0),
		},
	}

	status, err := buildRunStatusAtWithOptions(fsys, time.Unix(1770000000, 0).Add(runStatusStaleAfter+time.Second), runStatusOptions{
		simulatorLogWriteState: func(string) simulatorLogWriteState {
			return simulatorLogWriteOpen
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "running" {
		t.Fatalf("state mismatch: got %q, want running", status.State)
	}
}

func TestBuildRunStatusHidesInterruptedMarkedRun(t *testing.T) {
	simLog := "1770000000-simulator-abc.log"
	fsys := fstest.MapFS{
		simLog: {
			Data: []byte(
				`HIVE_SUITE_PLAN {"suites":[{"name":"rpc-compat","tests":64},{"name":"sync","tests":2}]}` + "\n" +
					`HIVE_RUN_HEARTBEAT {}` + "\n" +
					`HIVE_RUN_INTERRUPTED {"signal":"SIGINT"}` + "\n",
			),
			ModTime: time.Unix(1770000005, 0),
		},
	}

	status, err := buildRunStatusAt(fsys, time.Unix(1770000010, 0))
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "unknown" {
		t.Fatalf("state mismatch: got %q, want unknown", status.State)
	}
	if len(status.Suites) != 0 {
		t.Fatalf("interrupted run should not expose suites, got %d", len(status.Suites))
	}
}

func TestBuildRunStatusHidesCompletedRun(t *testing.T) {
	simLog := "1770000000-simulator-abc.log"
	fsys := fstest.MapFS{
		simLog: {
			Data: []byte(
				`HIVE_SUITE_PLAN {"suites":[{"name":"rpc-compat","tests":1}]}` + "\n" +
					`HIVE_RUN_COMPLETE {}` + "\n",
			),
			ModTime: time.Unix(1770000000, 0),
		},
	}

	status, err := buildRunStatusAt(fsys, time.Unix(1770000010, 0))
	if err != nil {
		t.Fatal(err)
	}
	if status.State != "finished" {
		t.Fatalf("state mismatch: got %q, want finished", status.State)
	}
	if len(status.Suites) != 0 {
		t.Fatalf("completed run should not expose suites, got %d", len(status.Suites))
	}
}
