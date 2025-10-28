// Contains logic that is test-type specific and logic used for comparing RPC responses.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/ethereum/hive/hivesim"
)

func determineTestResultExactMatch(t *hivesim.T, actualResp RPCResp, testObject RPCTestcase) hivesim.TestResult {
	testResult := hivesim.TestResult{}

	equal, diffs, err := CompareRPCResp(testObject.ExpectedOutput, actualResp)
	if err != nil {
		panic(err)
	}
	diffsAggregatedString := "\n" + "- " + strings.Join(diffs, "\n- ")

	// set pass/fail bool
	testResult.Pass = equal
	testResult.Details = diffsAggregatedString

	// print result info
	t.Log("Passed test?", equal)
	if !equal {
		t.Log(diffsAggregatedString)
	}

	return testResult
}

func determineTestResultRangedMatch(t *hivesim.T, actualResp RPCResp, testObject RPCTestcase) hivesim.TestResult {
	testResult := hivesim.TestResult{}

	// TODO: implement

	return testResult
}

func determineTestResultTypeMatch(t *hivesim.T, actualResp RPCResp, testObject RPCTestcase) hivesim.TestResult {
	testResult := hivesim.TestResult{}

	// TODO: implement

	return testResult
}

// -------------------- Helper functions for comparisons ----------------------

func CompareRPCResp(a, b RPCResp) (bool, []string, error) {
	var diffs []string

	// jsonrpc
	if a.JSONRPC != b.JSONRPC {
		diffs = append(diffs, fmt.Sprintf("$.jsonrpc: %q != %q", a.JSONRPC, b.JSONRPC))
	}

	// id
	if a.ID != b.ID {
		diffs = append(diffs, fmt.Sprintf("$.id: %d != %d", a.ID, b.ID))
	}

	// error
	errEqual, errDiffs, err := compareError("$.error", a.Error, b.Error)
	if err != nil {
		return false, nil, err
	}
	diffs = append(diffs, errDiffs...)

	// result
	resEqual, resDiffs, err := compareRaw("$.result", a.Result, b.Result)
	if err != nil {
		return false, nil, err
	}
	diffs = append(diffs, resDiffs...)

	return len(diffs) == 0 && errEqual && resEqual && a.JSONRPC == b.JSONRPC && a.ID == b.ID, diffs, nil
}

func compareError(path string, ea, eb *RPCError) (bool, []string, error) {
	var diffs []string
	if ea == nil && eb == nil {
		return true, nil, nil
	}
	if (ea == nil) != (eb == nil) {
		diffs = append(diffs, fmt.Sprintf("%s: one is null, the other is not", path))
		return false, diffs, nil
	}

	if ea.Code != eb.Code {
		diffs = append(diffs, fmt.Sprintf("%s.code: %d != %d", path, ea.Code, eb.Code))
	}
	if ea.Message != eb.Message {
		diffs = append(diffs, fmt.Sprintf("%s.message: %q != %q", path, ea.Message, eb.Message))
	}

	eq, ds, err := compareRaw(path+".data", ea.Data, eb.Data)
	if err != nil {
		return false, nil, err
	}
	diffs = append(diffs, ds...)
	return len(diffs) == 0 && eq && ea.Code == eb.Code && ea.Message == eb.Message, diffs, nil
}

func compareRaw(path string, ra, rb *json.RawMessage) (bool, []string, error) {
	var diffs []string
	switch {
	case ra == nil && rb == nil:
		return true, nil, nil
	case (ra == nil) != (rb == nil):
		diffs = append(diffs, fmt.Sprintf("%s: one is null, the other is not", path))
		return false, diffs, nil
	}

	var va, vb any
	if err := unmarshalUseNumber(*ra, &va); err != nil {
		return false, nil, fmt.Errorf("%s: invalid JSON in A: %w", path, err)
	}
	if err := unmarshalUseNumber(*rb, &vb); err != nil {
		return false, nil, fmt.Errorf("%s: invalid JSON in B: %w", path, err)
	}

	diffs = append(diffs, jsonDiff(path, va, vb)...)
	return len(diffs) == 0, diffs, nil
}

func unmarshalUseNumber(raw json.RawMessage, out *any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(out)
}

func escapeKey(k string) string { return k }

func jsonDiff(path string, va, vb any) []string {
	if va == nil && vb == nil {
		return nil
	}
	if (va == nil) != (vb == nil) {
		return []string{fmt.Sprintf("%s: null != %T", path, vb)}
	}

	ta := reflect.TypeOf(va)
	tb := reflect.TypeOf(vb)

	if ta != tb {
		return []string{fmt.Sprintf("%s: type %T != %T", path, va, vb)}
	}

	switch a := va.(type) {
	case map[string]any:
		b := vb.(map[string]any)
		var diffs []string
		for k := range a {
			if _, ok := b[k]; !ok {
				diffs = append(diffs, fmt.Sprintf("%s.%s: present in A, missing in B", path, k))
			}
		}
		for k := range b {
			if _, ok := a[k]; !ok {
				diffs = append(diffs, fmt.Sprintf("%s.%s: present in B, missing in A", path, k))
			}
		}
		for k := range a {
			if _, ok := b[k]; ok {
				diffs = append(diffs, jsonDiff(path+"."+escapeKey(k), a[k], b[k])...)
			}
		}
		return diffs

	case []any:
		b := vb.([]any)
		if len(a) != len(b) {
			return []string{fmt.Sprintf("%s: array length %d != %d", path, len(a), len(b))}
		}
		var diffs []string
		for i := range a {
			diffs = append(diffs, jsonDiff(fmt.Sprintf("%s[%d]", path, i), a[i], b[i])...)
		}
		return diffs

	case json.Number:
		if a != vb.(json.Number) {
			return []string{fmt.Sprintf("%s: number %s != %s", path, a.String(), vb.(json.Number).String())}
		}
		return nil

	case string:
		if a != vb.(string) {
			return []string{fmt.Sprintf("%s: %q != %q", path, a, vb.(string))}
		}
		return nil

	case bool:
		if a != vb.(bool) {
			return []string{fmt.Sprintf("%s: %v != %v", path, a, vb.(bool))}
		}
		return nil

	case nil:
		return nil

	default:
		if !reflect.DeepEqual(va, vb) {
			return []string{fmt.Sprintf("%s: %v != %v", path, va, vb)}
		}
		return nil
	}
}
