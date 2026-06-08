package main

import (
	"encoding/json"
	"fmt"
	"os"

	openrpc "github.com/open-rpc/meta-schema"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// methodSchemas maps a JSON-RPC method name to its result schema, parsed from
// an OpenRPC document. It is used to validate the responses of "speconly"
// tests against the spec rather than byte-comparing them to a recorded
// example, which is necessary for methods whose response is client- or
// config-specific (e.g. eth_capabilities retention windows).
type methodSchemas map[string]*openrpc.JSONSchemaObject

// parseSpec reads a (dereferenced) OpenRPC document and returns the result
// schema for each method. Methods without a usable result schema are skipped;
// callers that don't find a method fall back to structural matching.
func parseSpec(filename string) (methodSchemas, error) {
	spec, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var doc openrpc.OpenrpcDocument
	if err := json.Unmarshal(spec, &doc); err != nil {
		return nil, fmt.Errorf("unable to parse OpenRPC document: %w", err)
	}
	if doc.Methods == nil {
		return nil, fmt.Errorf("OpenRPC document has no methods")
	}

	schemas := make(methodSchemas)
	for _, m := range *doc.Methods {
		if m.MethodObject == nil || m.MethodObject.Name == nil {
			continue
		}
		method := m.MethodObject
		if method.Result == nil ||
			method.Result.ContentDescriptorObject == nil ||
			method.Result.ContentDescriptorObject.Schema == nil ||
			method.Result.ContentDescriptorObject.Schema.JSONSchemaObject == nil {
			// No result schema (e.g. reference-only or notification); skip it.
			continue
		}
		schemas[string(*method.Name)] = method.Result.ContentDescriptorObject.Schema.JSONSchemaObject
	}
	return schemas, nil
}

// validateResult validates a JSON-RPC result value against the given result
// schema. This mirrors the validation done by execution-apis' speccheck tool,
// so "speconly" tests in hive enforce the same contract: any response that is
// valid per the OpenRPC schema passes, regardless of which optional fields a
// particular client or configuration includes.
func validateResult(schema *openrpc.JSONSchemaObject, result []byte) error {
	// Set $schema explicitly to force jsonschema to use draft 2019-09, the
	// draft the execution-apis schemas are written against.
	draft := openrpc.Schema("https://json-schema.org/draft/2019-09/schema")
	schema.Schema = &draft

	b, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("unable to marshal schema to json: %w", err)
	}
	s, err := jsonschema.CompileString("result", string(b))
	if err != nil {
		return fmt.Errorf("unable to compile schema: %w", err)
	}
	var x interface{}
	if err := json.Unmarshal(result, &x); err != nil {
		return fmt.Errorf("unable to parse result: %w", err)
	}
	return s.Validate(x)
}
