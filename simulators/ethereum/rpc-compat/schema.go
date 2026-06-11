package main

import (
	"encoding/json"
	"fmt"
	"os"

	openrpc "github.com/open-rpc/meta-schema"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// methodSchemas maps a JSON-RPC method name to its compiled result schema,
// parsed from an OpenRPC document.
type methodSchemas map[string]*jsonschema.Schema

// parseSpec reads a (dereferenced) OpenRPC document and returns the compiled
// result schema for each method. Each schema is compiled once here and reused
// for every test.
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
		name := string(*method.Name)
		schema, err := compileSchema(method.Result.ContentDescriptorObject.Schema.JSONSchemaObject)
		if err != nil {
			return nil, fmt.Errorf("unable to compile result schema for %s: %w", name, err)
		}
		schemas[name] = schema
	}
	return schemas, nil
}

// compileSchema compiles an OpenRPC result schema into a reusable validator.
func compileSchema(schema *openrpc.JSONSchemaObject) (*jsonschema.Schema, error) {
	// Set $schema explicitly to force jsonschema to use draft 2019-09, the
	// draft the execution-apis schemas are written against.
	draft := openrpc.Schema("https://json-schema.org/draft/2019-09/schema")
	schema.Schema = &draft

	b, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal schema to json: %w", err)
	}
	return jsonschema.CompileString("result", string(b))
}

// validateResult validates a JSON-RPC result value against a compiled result
// schema.
func validateResult(schema *jsonschema.Schema, result []byte) error {
	var x interface{}
	if err := json.Unmarshal(result, &x); err != nil {
		return fmt.Errorf("unable to parse result: %w", err)
	}
	return schema.Validate(x)
}
