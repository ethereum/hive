package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	openrpc "github.com/open-rpc/meta-schema"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/tidwall/gjson"
)

// tracerConfigParamIndex maps debug tracing methods to the position of their
// TraceConfig parameter.
var tracerConfigParamIndex = map[string]int{
	"debug_traceTransaction":   1,
	"debug_traceBlockByNumber": 1,
	"debug_traceBlockByHash":   1,
	"debug_traceCall":          2,
}

// tracerBranchTitles maps a requested tracer name to the title prefix of the
// result-schema anyOf branch that defines its output. The empty name is the
// opcode (struct) logger.
var tracerBranchTitles = map[string]string{
	"":           "Opcode tracer",
	"callTracer": "Call tracer",
}

// methodSchemas holds the compiled result schema for each method, plus
// tracer-narrowed variants for the debug tracing methods.
type methodSchemas struct {
	full   map[string]*jsonschema.Schema
	tracer map[string]map[string]*jsonschema.Schema
}

// forRequest returns the result schema for a speconly response. For debug
// tracing methods, the anyOf branch matching the requested tracer is
// selected — validating against the whole anyOf is vacuous because the
// unconstrained named-tracer branch accepts anything. A known tracer with no
// matching branch is an error, not a fallback, so a spec title reword cannot
// silently disable validation.
func (s methodSchemas) forRequest(method, request string) (*jsonschema.Schema, error) {
	if idx, ok := tracerConfigParamIndex[method]; ok {
		tracer := gjson.Get(request, fmt.Sprintf("params.%d.tracer", idx)).String()
		if title, known := tracerBranchTitles[tracer]; known {
			if schema := s.tracer[method][tracer]; schema != nil {
				return schema, nil
			}
			return nil, fmt.Errorf("no %q branch in %s result schema for tracer %q", title, method, tracer)
		}
	}
	return s.full[method], nil
}

// parseSpec reads a (dereferenced) OpenRPC document and returns the compiled
// result schema for each method. Each schema is compiled once here and reused
// for every test.
func parseSpec(filename string) (methodSchemas, error) {
	schemas := methodSchemas{
		full:   make(map[string]*jsonschema.Schema),
		tracer: make(map[string]map[string]*jsonschema.Schema),
	}
	spec, err := os.ReadFile(filename)
	if err != nil {
		return schemas, err
	}
	var doc openrpc.OpenrpcDocument
	if err := json.Unmarshal(spec, &doc); err != nil {
		return schemas, fmt.Errorf("unable to parse OpenRPC document: %w", err)
	}
	if doc.Methods == nil {
		return schemas, fmt.Errorf("OpenRPC document has no methods")
	}

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
			return schemas, fmt.Errorf("unable to compile result schema for %s: %w", name, err)
		}
		schemas.full[name] = schema
	}
	if err := parseTracerSchemas(spec, schemas.tracer); err != nil {
		return schemas, err
	}
	return schemas, nil
}

// parseTracerSchemas extracts and compiles the tracer-specific anyOf branches
// of the debug tracing methods' result schemas. It works on the raw document,
// not the typed one: the generated union types do not round-trip
// (re-marshaling a normalized "type" array double-wraps it into invalid
// schema JSON). Absent branches are not stored; forRequest errors when a
// test needs one.
func parseTracerSchemas(spec []byte, out map[string]map[string]*jsonschema.Schema) error {
	var rawDoc struct {
		Methods []struct {
			Name   string `json:"name"`
			Result struct {
				Schema map[string]interface{} `json:"schema"`
			} `json:"result"`
		} `json:"methods"`
	}
	if err := json.Unmarshal(spec, &rawDoc); err != nil {
		return fmt.Errorf("unable to parse OpenRPC document: %w", err)
	}
	for _, m := range rawDoc.Methods {
		if _, ok := tracerConfigParamIndex[m.Name]; !ok || m.Result.Schema == nil {
			continue
		}
		for tracer, title := range tracerBranchTitles {
			narrowed := narrowResultSchema(m.Result.Schema, title)
			if narrowed == nil {
				continue
			}
			schema, err := compileRawSchema(narrowed)
			if err != nil {
				return fmt.Errorf("unable to compile %q branch of %s result schema: %w", title, m.Name, err)
			}
			if out[m.Name] == nil {
				out[m.Name] = make(map[string]*jsonschema.Schema)
			}
			out[m.Name][tracer] = schema
		}
	}
	return nil
}

// narrowResultSchema returns the anyOf branch of a result schema whose title
// starts with wantTitle, either at the top level or (for the block trace
// methods) under items, re-wrapped as an array schema.
func narrowResultSchema(schema map[string]interface{}, wantTitle string) map[string]interface{} {
	if branch := pickBranch(schema, wantTitle); branch != nil {
		return branch
	}
	if items, ok := schema["items"].(map[string]interface{}); ok {
		if branch := pickBranch(items, wantTitle); branch != nil {
			narrowed := map[string]interface{}{"type": "array", "items": branch}
			if title, ok := schema["title"]; ok {
				narrowed["title"] = title
			}
			return narrowed
		}
	}
	return nil
}

// pickBranch returns the anyOf branch of node whose title starts with wantTitle.
func pickBranch(node map[string]interface{}, wantTitle string) map[string]interface{} {
	branches, ok := node["anyOf"].([]interface{})
	if !ok {
		return nil
	}
	for _, b := range branches {
		branch, ok := b.(map[string]interface{})
		if !ok {
			continue
		}
		title, _ := branch["title"].(string)
		if strings.HasPrefix(title, wantTitle) {
			return branch
		}
	}
	return nil
}

// compileRawSchema compiles a schema given as a raw map, forcing draft
// 2019-09 like compileSchema.
func compileRawSchema(schema map[string]interface{}) (*jsonschema.Schema, error) {
	withDraft := make(map[string]interface{}, len(schema)+1)
	for k, v := range schema {
		withDraft[k] = v
	}
	withDraft["$schema"] = "https://json-schema.org/draft/2019-09/schema"
	b, err := json.Marshal(withDraft)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal schema to json: %w", err)
	}
	return jsonschema.CompileString("result", string(b))
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
