package tools

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
)

// inputSchema infers the JSON Schema for the input struct T (exactly as
// mcp.AddTool would) and then lets customize add rules that struct tags
// can't express, such as allowed values or number ranges.
//
// The SDK validates every tool call against this schema, and AI clients
// read it to learn which values are allowed.
func inputSchema[T any](customize func(props map[string]*jsonschema.Schema)) *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		// Only happens if T has a field type JSON Schema can't describe:
		// a programming mistake we want to see immediately at startup.
		panic(fmt.Sprintf("inferring input schema: %v", err))
	}
	customize(schema.Properties)
	return schema
}

// enum converts allowed string values into a JSON Schema "enum" list.
func enum(values []string) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

// addPagingRules documents defaults and limits for the per_page and page arguments.
func addPagingRules(props map[string]*jsonschema.Schema, defaultPerPage int) {
	props["per_page"].Minimum = jsonschema.Ptr(1.0)
	props["per_page"].Maximum = jsonschema.Ptr(float64(maxPerPage))
	props["per_page"].Default = []byte(fmt.Sprint(defaultPerPage))
	props["page"].Minimum = jsonschema.Ptr(1.0)
	props["page"].Default = []byte("1")
}
