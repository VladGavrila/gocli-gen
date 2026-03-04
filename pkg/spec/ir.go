package spec

// Project is the top-level intermediate representation for a generated CLI project.
type Project struct {
	Name       string // Binary name (e.g., "mxreq")
	Module     string // Go module path
	GithubRepo string // GitHub repo for --upgrade (e.g., "VladGavrila/matrixreq-cli")
	EnvPrefix  string // Environment variable prefix (e.g., "MATRIX")
	BasePath   string // API base path suffix (e.g., "/rest/1")
	APITitle   string // API title from the spec
	APIVersion string // API version from the spec

	Auth       AuthScheme
	Resources  []*Resource
	Schemas    []*Schema
	ScopeParam string // Global scope parameter name (e.g., "project")
	TUI        bool   // Whether to generate TUI code
}

// AuthScheme describes how the generated client should authenticate.
type AuthScheme struct {
	Type         string // "token", "bearer", "basic", "apikey"
	HeaderName   string // Header name for apikey (e.g., "X-API-Key")
	HeaderPrefix string // Value prefix (e.g., "Token", "Bearer")
}

// Resource is a group of related API endpoints (e.g., "project", "item", "user").
type Resource struct {
	Name        string     // Singular name (e.g., "project")
	Plural      string     // Plural name (e.g., "projects")
	GoName      string     // Go-exported name (e.g., "Project")
	GoPlural    string     // Go-exported plural (e.g., "Projects")
	CLIName     string     // CLI command name (e.g., "project")
	Aliases     []string   // CLI aliases (e.g., ["proj"])
	Scope       string     // "global" or scope param name (e.g., "project")
	WriteFlag   string     // Required flag for write ops (e.g., "reason")
	Endpoints   []*Endpoint
	Actions     []*Action // Derived CRUD + custom actions
	TableColumns []string // Columns to show in table output
}

// Endpoint represents a single API operation.
type Endpoint struct {
	Method      string   // HTTP method
	Path        string   // API path (relative to base)
	OperationID string
	Summary     string
	Description string
	Parameters  []*Param
	RequestBody *Schema  // Request body schema (nil if none)
	Response    *Schema  // Primary success response schema
	Tags        []string
}

// Action is a derived CLI action from one or more endpoints.
type Action struct {
	Name        string // "list", "get", "create", "update", "delete", or custom
	GoName      string // "List", "Get", "Create", etc.
	CLIName     string // "list", "get", "create", etc.
	Short       string // Short description for cobra
	Method      string // HTTP method
	Path        string // API path template
	HasBody     bool   // Whether the action sends a request body
	IDParam     string // Name of the ID path parameter (e.g., "id", "ref")
	ScopeParam  string // Name of scope path parameter (e.g., "project")
	QueryParams []*Param
	PathParams  []*Param
	RequestBody *Schema
	Response    *Schema
}

// Param represents an API parameter.
type Param struct {
	Name     string // Original API name
	GoName   string // Go-exported name
	CLIName  string // CLI flag name
	In       string // "path", "query", "header"
	Type     string // Go type
	Required bool
	Description string
}

// Schema represents an API data type.
type Schema struct {
	Name       string   // Schema name from components.schemas
	GoName     string   // Go type name
	Fields     []*Field
	IsRequest  bool     // Whether this is a request body type
	IsResponse bool     // Whether this is a response type
	IsArray    bool     // Whether this wraps an array
	ArrayItem  *Schema  // Item schema for arrays
	Ref        string   // Original $ref if this was a reference
	Raw        map[string]any // Raw OpenAPI schema for complex cases
}

// Field represents a field in a schema.
type Field struct {
	Name     string // Original JSON name
	GoName   string // Go-exported field name
	GoType   string // Go type string
	JSONTag  string // JSON struct tag value
	Required bool
	Description string
}
