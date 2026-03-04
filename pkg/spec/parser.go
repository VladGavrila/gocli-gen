package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/VladGavrila/gocli-gen/pkg/naming"
	"gopkg.in/yaml.v3"
)

// Parse reads an OpenAPI 3.x spec file and returns a Project IR.
func Parse(specPath, name, module string) (*Project, error) {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}

	// Parse as generic map (supports both YAML and JSON)
	var raw map[string]any
	ext := strings.ToLower(filepath.Ext(specPath))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parsing JSON spec: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parsing YAML spec: %w", err)
		}
	default:
		// Try JSON first, then YAML
		if err := json.Unmarshal(data, &raw); err != nil {
			if err := yaml.Unmarshal(data, &raw); err != nil {
				return nil, fmt.Errorf("could not parse spec as JSON or YAML")
			}
		}
	}

	p := &Project{
		Name:   name,
		Module: module,
		Auth: AuthScheme{
			Type:         "token",
			HeaderPrefix: "Token",
		},
	}

	// Extract info
	if info, ok := raw["info"].(map[string]any); ok {
		p.APITitle, _ = info["title"].(string)
		p.APIVersion, _ = info["version"].(string)
	}

	// Detect base path from servers
	if servers, ok := raw["servers"].([]any); ok && len(servers) > 0 {
		if srv, ok := servers[0].(map[string]any); ok {
			if url, ok := srv["url"].(string); ok {
				p.BasePath = extractBasePath(url)
			}
		}
	}

	// Derive env prefix from name
	p.EnvPrefix = strings.ToUpper(name)

	// Extract schemas from components
	components := extractMap(raw, "components")
	schemas := extractMap(components, "schemas")
	res := newResolver(schemas)
	p.Schemas = parseSchemas(schemas, res)

	// Detect auth from security schemes
	secSchemes := extractMap(components, "securitySchemes")
	p.Auth = detectAuth(secSchemes)

	// Parse paths into endpoints
	paths := extractMap(raw, "paths")
	endpoints := parsePaths(paths, res)

	// Group endpoints into resources
	resourceMap := groupEndpoints(endpoints)
	p.Resources = sortedResources(resourceMap)

	// Detect common scope parameter
	p.ScopeParam = detectScopeParam(p.Resources)

	return p, nil
}

// ApplyConfig applies overrides from a gocli-gen.yaml config file.
func ApplyConfig(p *Project, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	if v, ok := cfg["github_repo"].(string); ok {
		p.GithubRepo = v
	}
	if v, ok := cfg["env_prefix"].(string); ok {
		p.EnvPrefix = v
	}
	if v, ok := cfg["base_path_suffix"].(string); ok {
		p.BasePath = v
	}
	if v, ok := cfg["auth_type"].(string); ok {
		p.Auth.Type = v
	}
	if v, ok := cfg["auth_header"].(string); ok {
		p.Auth.HeaderPrefix = v
	}

	// Apply resource overrides
	if resources, ok := cfg["resources"].(map[string]any); ok {
		for resName, resCfg := range resources {
			rc, ok := resCfg.(map[string]any)
			if !ok {
				continue
			}
			res := findResource(p.Resources, resName)
			if res == nil {
				continue
			}
			if aliases, ok := rc["aliases"].([]any); ok {
				res.Aliases = nil
				for _, a := range aliases {
					if s, ok := a.(string); ok {
						res.Aliases = append(res.Aliases, s)
					}
				}
			}
			if scope, ok := rc["scope"].(string); ok {
				res.Scope = scope
			}
			if wf, ok := rc["write_flag"].(string); ok {
				res.WriteFlag = wf
			}
			if cols, ok := rc["table_columns"].([]any); ok {
				res.TableColumns = nil
				for _, c := range cols {
					if s, ok := c.(string); ok {
						res.TableColumns = append(res.TableColumns, s)
					}
				}
			}
		}
	}

	return nil
}

// parseSchemas converts raw components.schemas into IR Schema types.
func parseSchemas(schemas map[string]any, res *resolver) []*Schema {
	var result []*Schema
	for name, raw := range schemas {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		s := parseSchema(name, m, res)
		if s != nil {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// parseSchema converts a single raw schema into an IR Schema.
func parseSchema(name string, raw map[string]any, res *resolver) *Schema {
	props, required, _ := res.resolveSchema(raw)

	requiredSet := make(map[string]bool)
	for _, r := range required {
		requiredSet[r] = true
	}

	s := &Schema{
		Name:   name,
		GoName: naming.ToGoName(name),
		Raw:    raw,
	}

	// Check if it's an array type
	if typ, _ := raw["type"].(string); typ == "array" {
		s.IsArray = true
	}

	for fieldName, fieldRaw := range props {
		fm, ok := fieldRaw.(map[string]any)
		if !ok {
			continue
		}
		goType, _ := res.resolveFieldType(fm)

		f := &Field{
			Name:     fieldName,
			GoName:   naming.ToGoName(fieldName),
			GoType:   goType,
			JSONTag:  fieldName,
			Required: requiredSet[fieldName],
		}
		if desc, ok := fm["description"].(string); ok {
			f.Description = desc
		}
		s.Fields = append(s.Fields, f)
	}

	// Sort fields for deterministic output
	sort.Slice(s.Fields, func(i, j int) bool {
		return s.Fields[i].Name < s.Fields[j].Name
	})

	return s
}

// parsePaths extracts endpoints from the paths section of the spec.
func parsePaths(paths map[string]any, res *resolver) []*Endpoint {
	var endpoints []*Endpoint

	for path, methods := range paths {
		methodMap, ok := methods.(map[string]any)
		if !ok {
			continue
		}
		for method, opRaw := range methodMap {
			// Skip non-method keys like "parameters", "summary"
			switch strings.ToLower(method) {
			case "get", "post", "put", "delete", "patch":
			default:
				continue
			}

			op, ok := opRaw.(map[string]any)
			if !ok {
				continue
			}

			ep := &Endpoint{
				Method: strings.ToUpper(method),
				Path:   path,
			}
			ep.OperationID, _ = op["operationId"].(string)
			ep.Summary, _ = op["summary"].(string)
			ep.Description, _ = op["description"].(string)

			// Parse tags
			if tags, ok := op["tags"].([]any); ok {
				for _, t := range tags {
					if s, ok := t.(string); ok {
						ep.Tags = append(ep.Tags, s)
					}
				}
			}

			// Parse parameters
			ep.Parameters = parseParameters(op, res)

			// Parse request body
			if reqBody, ok := op["requestBody"].(map[string]any); ok {
				ep.RequestBody = parseRequestBody(reqBody, res)
			}

			// Parse response
			if responses, ok := op["responses"].(map[string]any); ok {
				ep.Response = parseResponse(responses, res)
			}

			endpoints = append(endpoints, ep)
		}
	}

	return endpoints
}

// parseParameters extracts parameters from an operation.
func parseParameters(op map[string]any, res *resolver) []*Param {
	params, ok := op["parameters"].([]any)
	if !ok {
		return nil
	}
	var result []*Param
	for _, raw := range params {
		pm, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		p := &Param{}
		p.Name, _ = pm["name"].(string)
		p.In, _ = pm["in"].(string)
		p.Description, _ = pm["description"].(string)
		if req, ok := pm["required"].(bool); ok {
			p.Required = req
		}

		// Determine type
		if schema, ok := pm["schema"].(map[string]any); ok {
			p.Type, _ = res.resolveFieldType(schema)
		} else {
			p.Type = "string"
		}

		p.GoName = naming.ToGoName(p.Name)
		p.CLIName = naming.ToCLIName(p.Name)

		result = append(result, p)
	}
	return result
}

// parseRequestBody extracts the request body schema.
func parseRequestBody(reqBody map[string]any, res *resolver) *Schema {
	content, ok := reqBody["content"].(map[string]any)
	if !ok {
		return nil
	}
	// Try application/json first
	for _, ct := range []string{"application/json", "application/xml", "*/*"} {
		if media, ok := content[ct].(map[string]any); ok {
			if schema, ok := media["schema"].(map[string]any); ok {
				if ref, ok := schema["$ref"].(string); ok {
					_, name, err := res.resolveRef(ref)
					if err == nil {
						return &Schema{
							Name:   name,
							GoName: naming.ToGoName(name),
							Ref:    ref,
						}
					}
				}
				return parseSchema("RequestBody", schema, res)
			}
		}
	}
	return nil
}

// parseResponse extracts the primary success response schema.
func parseResponse(responses map[string]any, res *resolver) *Schema {
	// Try 200, 201, 2xx in order
	for _, code := range []string{"200", "201", "202", "204"} {
		if resp, ok := responses[code].(map[string]any); ok {
			content, ok := resp["content"].(map[string]any)
			if !ok {
				continue
			}
			if media, ok := content["application/json"].(map[string]any); ok {
				if schema, ok := media["schema"].(map[string]any); ok {
					if ref, ok := schema["$ref"].(string); ok {
						_, name, err := res.resolveRef(ref)
						if err == nil {
							return &Schema{
								Name:   name,
								GoName: naming.ToGoName(name),
								Ref:    ref,
							}
						}
					}
					return parseSchema("Response", schema, res)
				}
			}
		}
	}
	return nil
}

// extractBasePath extracts the path suffix from a server URL.
// e.g., "http://localhost:8080/1" → "/1"
func extractBasePath(serverURL string) string {
	// Strip protocol
	url := serverURL
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(url, prefix) {
			url = strings.TrimPrefix(url, prefix)
			break
		}
	}
	// Find path after host
	idx := strings.Index(url, "/")
	if idx < 0 {
		return ""
	}
	return url[idx:]
}

// detectAuth determines auth scheme from security schemes.
func detectAuth(secSchemes map[string]any) AuthScheme {
	for _, raw := range secSchemes {
		scheme, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := scheme["type"].(string)
		switch typ {
		case "http":
			httpScheme, _ := scheme["scheme"].(string)
			switch strings.ToLower(httpScheme) {
			case "bearer":
				return AuthScheme{Type: "bearer", HeaderPrefix: "Bearer"}
			case "basic":
				return AuthScheme{Type: "basic", HeaderPrefix: "Basic"}
			}
		case "apiKey":
			in, _ := scheme["in"].(string)
			name, _ := scheme["name"].(string)
			if in == "header" {
				return AuthScheme{Type: "apikey", HeaderName: name}
			}
		}
	}
	// Default to token auth
	return AuthScheme{Type: "token", HeaderPrefix: "Token"}
}

// detectScopeParam finds the most common scope parameter across resources.
func detectScopeParam(resources []*Resource) string {
	counts := make(map[string]int)
	for _, r := range resources {
		if r.Scope != "" && r.Scope != "global" {
			counts[r.Scope]++
		}
	}
	var best string
	var bestCount int
	for param, count := range counts {
		if count > bestCount {
			best = param
			bestCount = count
		}
	}
	return best
}

// extractMap safely extracts a nested map from a parent map.
func extractMap(parent map[string]any, key string) map[string]any {
	if parent == nil {
		return nil
	}
	if m, ok := parent[key].(map[string]any); ok {
		return m
	}
	return nil
}

// findResource finds a resource by name in the slice.
func findResource(resources []*Resource, name string) *Resource {
	for _, r := range resources {
		if r.Name == name {
			return r
		}
	}
	return nil
}
