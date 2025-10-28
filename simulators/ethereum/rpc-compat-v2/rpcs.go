// Defines the different test modes and RPC related data structures.
package main

import (
	"encoding/json"
	"fmt"
	"log"
)

// ------------------------ 1. ExactMatch -------------------------------------

// TestExactMatch requires the client to have a response that exactly
// matches expectations.
type TestExactMatch struct {
	// performance measuring for getting RPC response (this might exist in hive already, just an idea)
	StartTime int
	EndTime   int
}

func (t TestExactMatch) String() string { // value receiver is better, then it works with both actual object and pointer to object
	return "ExactMatch"
}

func (t TestExactMatch) IsATestModeForRPCV2() {}

// ------------------------ 2. RangedMatch ------------------------------------

// TestRangedMatch requires the client to have a response that is within a
// certain range (will be specced for each RPC call).
type TestRangedMatch struct {
	// performance measuring
	StartTime int
	EndTime   int

	// range specification
	MinValue int
	MaxValue int
}

func (t TestRangedMatch) String() string {
	return "RangedMatch"
}

func (t TestRangedMatch) IsATestModeForRPCV2() {}

// ------------------------ 3. TypeMatch (former specsonly) -------------------

// TestTypeMatch requires the client to respond with almost anything as long
// as it has the correct format and type.
type TestTypeMatch struct {
	// performance measuring
	StartTime int
	EndTime   int
}

func (t TestTypeMatch) String() string {
	return "TypeMatch"
}

func (t TestTypeMatch) IsATestModeForRPCV2() {}

// ------------------------- RPC ----------------------------------------------

type TestMode interface {
	IsATestModeForRPCV2()
}

type RPC struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"` // e.g. eth_config
	Params  []any  `json:"params"` // e.g. {"from": "0xdeadbeef", "to": "0xcafe"}
}

func (r *RPC) Dump() {
	// pretty-print dump the json object
	fmt.Println("Request Dump:")
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}

// ------------------------- RPC Error ----------------------------------------

type RPCError struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    *json.RawMessage `json:"data,omitempty"`
}

func (r *RPCError) GetDump() string {
	if r == nil {
		return "<nil>"
	}

	return fmt.Sprintf("Error Dump:\n\tCode: %v\n\tMessage: %v\n\tData: %v\n", r.Code, r.Message, r.Data)
}

// ------------------------- RPC Response -------------------------------------

type RPCResp struct {
	JSONRPC string           `json:"jsonrpc"`
	Result  *json.RawMessage `json:"result"`
	Error   *RPCError        `json:"error"`
	ID      int              `json:"id"`
}

func (r *RPCResp) Dump() {
	// pretty-print dump the json object
	fmt.Println("Response Dump:")
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}

// GetResultHexString is supposed to only be called when collecting the response was successful, so nil results are unexpected and won't be recovered from
func (r *RPCResp) GetResultHexString() string {
	if r == nil {
		log.Fatal("RPCResp is nil")
	}
	if r.Result == nil {
		log.Fatal("RPCResp.Result is nil")
	}

	// unmarshal response into hex string
	var hex string
	if err := json.Unmarshal(*r.Result, &hex); err != nil {
		log.Fatal(err)
	}

	return hex
}

// ------------------------- RPC Testcase -------------------------------------

// RPCTestcase bundles RPC (request) and RPCResp (response) into a testcase object.
// This testcase object also contains the information of the test type, which defines the conditions under which test passed/failed is made.
// The actual result (pass/fail+error string) is not here but in hivesim.T.result
type RPCTestcase struct {
	Filepath       string   `json:"filepath"` // path to io file that contains input/expectedOutput (serves an unique ID for a testcase, its 'name')
	Input          RPC      `json:"input"`
	ExpectedOutput RPCResp  `json:"expectedOutput"`
	TestMode       TestMode `json:"testMode"` // e.g. TestExactMatch
}
