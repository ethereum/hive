// Functions for parsing the io format used in execution-apis.
package main

import (
	"encoding/json"
	"strings"
	"unicode"

	"github.com/ethereum/hive/hivesim"
)

func ParseIoContent(t *hivesim.T, lines []string) (RPC, RPCResp) {
	var req RPC
	var resp RPCResp

	for _, l := range lines {
		// remove leading/trailing spaces and tabs
		l = strings.TrimFunc(l, unicode.IsSpace)
		if len(l) <= 2 {
			continue
		}

		// >> is request, << is response
		switch l[:2] {
		case ">>":
			req = ParseIoRequest(l[2:])
			//t.Log(reqRPC)
		case "<<":
			resp = ParseIoResponse(l[2:])
			//t.Log(reqRPC)
		default:
			//t.Logf("Skipped reading line: %v\n", l)
		}

	}

	return req, resp
}

func ParseIoRequest(line string) RPC {
	var r RPC
	err := json.Unmarshal([]byte(line), &r)
	if err != nil {
		panic(err)
	}

	return r
}

func ParseIoResponse(line string) RPCResp {
	var r RPCResp
	err := json.Unmarshal([]byte(line), &r)
	if err != nil {
		panic(err)
	}

	return r
}
