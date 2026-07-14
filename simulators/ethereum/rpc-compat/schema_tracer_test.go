package main

import (
	"strings"
	"testing"
)

// TestTracerSchemaSelection verifies that speconly validation of debug_trace*
// responses uses the anyOf branch matching the tracer requested by the test,
// instead of the whole result schema, whose unconstrained named-tracer branch
// accepts any value.
func TestTracerSchemaSelection(t *testing.T) {
	schemas, err := parseSpec("testdata/openrpc-tracer.json")
	if err != nil {
		t.Fatalf("parseSpec: %v", err)
	}

	txCallTracer := `{"method":"debug_traceTransaction","params":["0xabc",{"tracer":"callTracer"}]}`
	txOpcode := `{"method":"debug_traceTransaction","params":["0xabc"]}`
	txPrestate := `{"method":"debug_traceTransaction","params":["0xabc",{"tracer":"prestateTracer"}]}`
	blockCallTracer := `{"method":"debug_traceBlockByNumber","params":["0x1",{"tracer":"callTracer"}]}`
	callTracerCall := `{"method":"debug_traceCall","params":[{},"latest",{"tracer":"callTracer"}]}`

	// A frame missing required fields must fail the callTracer branch, at the
	// top level and nested.
	schema, err := schemas.forRequest("debug_traceTransaction", txCallTracer)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateResult(schema, []byte(`{"type":"CALL","from":"0xaa"}`)); err != nil {
		t.Errorf("valid frame rejected: %v", err)
	}
	if err := validateResult(schema, []byte(`{"structLogs":[]}`)); err == nil {
		t.Error("opcode-shaped response passed the callTracer branch")
	}
	if err := validateResult(schema, []byte(`{"type":"CALL","from":"0xaa","calls":[{"from":"0xbb"}]}`)); err == nil {
		t.Error("nested frame missing required 'type' was accepted")
	}

	// No tracer selects the opcode branch.
	schema, err = schemas.forRequest("debug_traceTransaction", txOpcode)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateResult(schema, []byte(`{"type":"CALL","from":"0xaa"}`)); err == nil {
		t.Error("callTracer-shaped response passed the opcode branch")
	}

	// Unknown tracers keep the whole-schema escape hatch.
	schema, err = schemas.forRequest("debug_traceTransaction", txPrestate)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateResult(schema, []byte(`{"anything":true}`)); err != nil {
		t.Errorf("named-tracer escape hatch rejected a response: %v", err)
	}

	// Block methods narrow under items and enforce the entry shape.
	schema, err = schemas.forRequest("debug_traceBlockByNumber", blockCallTracer)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateResult(schema, []byte(`[{"txHash":"0xaa","result":{}}]`)); err != nil {
		t.Errorf("valid block entry rejected: %v", err)
	}
	if err := validateResult(schema, []byte(`[{"txHash":"0xaa"}]`)); err == nil {
		t.Error("entry with neither result nor error was accepted")
	}

	// debug_traceCall reads the tracer from its third parameter; the fixture
	// spec has no such method, which must be an error, not a silent fallback.
	if _, err := schemas.forRequest("debug_traceCall", callTracerCall); err == nil {
		t.Error("expected error for a known tracer with no branch in the spec")
	} else if !strings.Contains(err.Error(), "Call tracer") {
		t.Errorf("unexpected error: %v", err)
	}

	// Non-trace methods use the full schema.
	schema, err = schemas.forRequest("eth_call", `{"method":"eth_call","params":[{},"latest"]}`)
	if err != nil {
		t.Fatal(err)
	}
	if schema == nil {
		t.Error("expected the full schema for a non-trace method")
	}
}
