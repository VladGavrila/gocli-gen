package spec

import (
	"fmt"
	"strings"

	"github.com/VladGavrila/gocli-gen/pkg/naming"
)

// resolver handles $ref resolution and allOf/oneOf flattening in raw OpenAPI data.
type resolver struct {
	components map[string]any // components.schemas from the spec
}

func newResolver(components map[string]any) *resolver {
	return &resolver{components: components}
}

// resolveRef follows a $ref pointer and returns the resolved schema map.
// For example, "#/components/schemas/ProjectType" → the ProjectType schema map.
func (r *resolver) resolveRef(ref string) (map[string]any, string, error) {
	// Only handle local references
	if !strings.HasPrefix(ref, "#/components/schemas/") {
		return nil, "", fmt.Errorf("unsupported $ref: %s", ref)
	}
	name := strings.TrimPrefix(ref, "#/components/schemas/")
	schema, ok := r.components[name]
	if !ok {
		return nil, name, fmt.Errorf("schema not found: %s", name)
	}
	m, ok := schema.(map[string]any)
	if !ok {
		return nil, name, fmt.Errorf("schema %s is not an object", name)
	}
	return m, name, nil
}

// resolveSchema resolves a schema that may contain $ref, allOf, or oneOf.
// Returns the flattened properties map and the list of required fields.
func (r *resolver) resolveSchema(schema map[string]any) (properties map[string]any, required []string, schemaType string) {
	// Handle $ref at top level
	if ref, ok := schema["$ref"].(string); ok {
		resolved, _, err := r.resolveRef(ref)
		if err != nil {
			return nil, nil, ""
		}
		return r.resolveSchema(resolved)
	}

	// Handle allOf: merge all sub-schemas
	if allOf, ok := schema["allOf"].([]any); ok {
		merged := make(map[string]any)
		var mergedRequired []string
		for _, item := range allOf {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			props, req, _ := r.resolveSchema(m)
			for k, v := range props {
				merged[k] = v
			}
			mergedRequired = append(mergedRequired, req...)
		}
		return merged, mergedRequired, "object"
	}

	// Handle oneOf: take the first variant (best effort)
	if oneOf, ok := schema["oneOf"].([]any); ok && len(oneOf) > 0 {
		if m, ok := oneOf[0].(map[string]any); ok {
			return r.resolveSchema(m)
		}
	}

	// Handle direct properties
	props := make(map[string]any)
	if p, ok := schema["properties"].(map[string]any); ok {
		props = p
	}

	if req, ok := schema["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
	}

	st, _ := schema["type"].(string)
	return props, required, st
}

// resolveFieldType determines the Go type for a schema property.
func (r *resolver) resolveFieldType(prop map[string]any) (goType string, isArray bool) {
	// Handle $ref
	if ref, ok := prop["$ref"].(string); ok {
		_, name, err := r.resolveRef(ref)
		if err != nil {
			return "any", false
		}
		return naming.ToGoName(name), false
	}

	typ, _ := prop["type"].(string)
	format, _ := prop["format"].(string)

	switch typ {
	case "string":
		return "string", false
	case "integer":
		if format == "int64" {
			return "int64", false
		}
		return "int", false
	case "number":
		if format == "float" {
			return "float32", false
		}
		return "float64", false
	case "boolean":
		return "bool", false
	case "array":
		if items, ok := prop["items"].(map[string]any); ok {
			itemType, _ := r.resolveFieldType(items)
			return "[]" + itemType, true
		}
		return "[]any", true
	case "object":
		// Check for additionalProperties (map type)
		if addProps, ok := prop["additionalProperties"].(map[string]any); ok {
			valType, _ := r.resolveFieldType(addProps)
			return "map[string]" + valType, false
		}
		return "map[string]any", false
	default:
		// No type specified — might be a $ref in items, allOf, etc.
		if _, ok := prop["allOf"]; ok {
			return "any", false
		}
		if _, ok := prop["oneOf"]; ok {
			return "any", false
		}
		return "any", false
	}
}
