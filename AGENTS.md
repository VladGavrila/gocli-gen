# AGENTS.md -- gocli-gen

> Reference for AI agents working on this codebase. Covers architecture, conventions, data flow, templates, and common tasks.

## Project Overview

**Binary:** `gocli-gen`
**Module:** `github.com/VladGavrila/gocli-gen`
**Go version:** 1.22+
**Dependencies:** cobra, yaml.v3
**Purpose:** Generate complete, idiomatic Go CLI+TUI projects from OpenAPI 3.x specs. The generated output is a one-time scaffold meant to be owned and modified by the developer -- there is no re-generation workflow.

## Directory Structure

```
gocli-gen/
├── main.go                          # Entry point: ldflags version injection → cmd.Execute()
├── Makefile                         # build, dev, test, clean targets
├── go.mod
├── .github/workflows/
│   └── release.yml                  # GitHub Actions: cross-platform build on release creation
├── cmd/
│   └── root.go                      # Cobra CLI: generate, --version, --upgrade
├── pkg/
│   ├── spec/
│   │   ├── ir.go                    # Intermediate representation types
│   │   ├── parser.go                # OpenAPI 3.x → IR (YAML/JSON)
│   │   ├── resolver.go              # $ref resolution, allOf/oneOf flattening
│   │   └── grouper.go               # Groups endpoints into resources by path
│   ├── naming/
│   │   ├── naming.go                # ToGoName, ToCLIName, ToSnakeCase, ToPlural
│   │   ├── naming_test.go           # Unit tests for naming functions
│   │   └── typemap.go               # OpenAPI type+format → Go type
│   └── codegen/
│       ├── generator.go             # Orchestrates template execution + file writing
│       └── templates/               # Embedded .go.tmpl files (28 templates)
├── internal/
│   └── upgrade/
│       └── upgrade.go               # GitHub release self-upgrade for gocli-gen itself
│           ├── main.go.tmpl
│           ├── go_mod.tmpl
│           ├── makefile.tmpl
│           ├── gocli_gen_yaml.tmpl
│           ├── cli/
│           │   ├── root.go.tmpl     # Root command, persistent flags, newService()
│           │   ├── config.go.tmpl   # config init|show commands
│           │   ├── version.go.tmpl
│           │   └── resource.go.tmpl # Per-resource CLI (list/get/create/update/delete)
│           ├── internal/
│           │   ├── api/
│           │   │   └── types.go.tmpl        # Go structs from components.schemas
│           │   ├── client/
│           │   │   ├── client.go.tmpl       # HTTP client with pluggable auth
│           │   │   └── errors.go.tmpl       # APIError, IsNotFound, etc.
│           │   ├── config/
│           │   │   └── config.go.tmpl       # YAML config with env var overrides
│           │   ├── output/
│           │   │   ├── output.go.tmpl       # Formatter interface
│           │   │   ├── json.go.tmpl
│           │   │   ├── table.go.tmpl        # Lipgloss table
│           │   │   └── text.go.tmpl
│           │   ├── service/
│           │   │   ├── service.go.tmpl      # Aggregate AppService
│           │   │   └── resource.go.tmpl     # Per-resource service interface + impl
│           │   └── upgrade/
│           │       └── upgrade.go.tmpl      # GitHub release self-upgrade
│           ├── tests/
│           │   ├── helpers.sh.tmpl
│           │   ├── run_all.sh.tmpl
│           │   └── test_resource.sh.tmpl
│           └── tui/                        # Optional (generated with --tui)
│               ├── tui.go.tmpl          # Bubble Tea 3-screen router (Home/List/Detail)
│               ├── styles.go.tmpl       # Lipgloss styles
│               ├── home.go.tmpl         # Resource picker screen
│               ├── list.go.tmpl         # Generic JSON table (bubbles/table)
│               ├── detail.go.tmpl       # Scrollable key-value detail (bubbles/viewport)
│               ├── filter.go.tmpl       # Text input filter component
│               └── resources.go.tmpl    # Per-resource closures (generated from IR)
```

## Architecture & Data Flow

```
OpenAPI 3.x spec (YAML/JSON)
  → spec.Parse()           # parser.go: raw map[string]any parsing
    → resolver              # resolver.go: $ref, allOf, oneOf flattening
    → parseSchemas()        # components.schemas → []*Schema
    → parsePaths()          # paths → []*Endpoint
    → groupEndpoints()      # grouper.go: endpoints → map[string]*Resource
    → detectAuth()          # securitySchemes → AuthScheme
  → spec.ApplyConfig()     # gocli-gen.yaml overrides (aliases, scope, write_flag)
  → codegen.Generator
    → generateFile()        # template execution + go/format
    → generateAPITypes()    # domain-split type files
    → runGoimports()        # optional post-processing
  → output directory        # Complete Go project ready for `go build`
```

### Layer Responsibilities

| Layer | Package | Role |
|-------|---------|------|
| CLI entry | `cmd/` | Parse flags, invoke spec.Parse + codegen.Generate, --version/--upgrade |
| Spec parsing | `pkg/spec/` | OpenAPI → Intermediate Representation |
| Naming | `pkg/naming/` | Go naming conventions, pluralization, type mapping |
| Code generation | `pkg/codegen/` | Template execution, file writing, formatting |
| Templates | `pkg/codegen/templates/` | Embedded `.tmpl` files for all generated code |
| Self-upgrade | `internal/upgrade/` | Check GitHub releases, download + replace binary |

## Intermediate Representation (IR)

All types live in `pkg/spec/ir.go`. The IR is the contract between parser and generator.

```
Project
├── Name, Module, GithubRepo, EnvPrefix, BasePath
├── APITitle, APIVersion
├── Auth: AuthScheme {Type, HeaderName, HeaderPrefix}
├── ScopeParam: string (most common scope, e.g. "project")
├── TUI: bool (whether to generate TUI code)
├── Schemas: []*Schema
│   └── Schema {Name, GoName, Fields, IsArray, Ref, Raw}
│       └── Field {Name, GoName, GoType, JSONTag, Required}
└── Resources: []*Resource
    ├── Name, Plural, GoName, GoPlural, CLIName
    ├── Aliases, Scope, WriteFlag, TableColumns
    ├── Endpoints: []*Endpoint
    │   └── Endpoint {Method, Path, OperationID, Summary, Parameters, RequestBody, Response}
    └── Actions: []*Action
        └── Action {Name, GoName, CLIName, Short, Method, Path, HasBody, IDParam, ScopeParam, QueryParams, PathParams, RequestBody, Response}
```

**Key invariants:**
- `Resource.Actions` contains both CRUD (`list`, `get`, `create`, `update`, `delete`) and custom actions
- Only CRUD actions are currently generated as CLI subcommands and service methods
- `Action.Response.Ref != ""` indicates a typed response (references `components.schemas`)
- `Resource.Scope` is either `"global"` or a scope parameter name (e.g., `"project"`)

## Resource Grouping Algorithm (`grouper.go`)

Two-phase algorithm:

1. **Count phase:** For each endpoint, find its resource segment (first non-`{param}` path segment). Count how many direct endpoints (no sub-path after resource) each segment has, keyed by `{scope, segment}`.

2. **Assign phase:** For each endpoint:
   - If its resource segment has <2 direct endpoints AND has a scope param → collapse as a custom action on the scope resource
   - Otherwise → it's a primary resource with standard CRUD action inference

**Action inference from HTTP method + path shape:**
- `GET` on collection → `list`
- `GET` on item (`{id}` after resource) → `get`
- `POST` on collection → `create`
- `PUT`/`PATCH` on item → `update`
- `DELETE` on item → `delete`
- Sub-path segments → custom action named from the last segment

**Example:** Given paths `/{project}/item`, `/{project}/item/{ref}`, `/{project}/tree`, `/{project}/audit`:
- `item` has 2+ direct endpoints → primary resource with list/get/create/update/delete
- `tree` has 1 direct endpoint → collapses as `project.tree` action
- `audit` has 1 direct endpoint → collapses as `project.audit` action

## Template Functions

All template functions are registered in `generator.go`'s FuncMap. Key categories:

### String Helpers
`toLower`, `toUpper`, `toGoName`, `toCLIName`, `toSnakeCase`, `toPlural`, `join`, `hasPrefix`, `hasSuffix`, `trimPrefix`, `trimSuffix`, `contains`, `replace`, `quote`, `backtick`

### Resource Inspection
| Function | Signature | Purpose |
|----------|-----------|---------|
| `hasAction` | `(r, name) bool` | Check if resource has a named action |
| `getAction` | `(r, name) *Action` | Get action by name |
| `isScoped` | `(r) bool` | True if resource has a scope param |
| `hasWriteOps` | `(r) bool` | True if any POST/PUT/DELETE/PATCH actions |
| `writeActions` | `(r) []*Action` | Filter to write operations |
| `readActions` | `(r) []*Action` | Filter to GET operations |
| `crudActions` | `(r) []*Action` | Filter to list/get/create/update/delete |
| `customActions` | `(r) []*Action` | Filter to non-CRUD actions |

### Response Type Inspection
| Function | Signature | Purpose |
|----------|-----------|---------|
| `hasTypedResponse` | `(a) bool` | True if action response references a named schema |
| `responseGoType` | `(a) string` | Go type name of the response schema |
| `anyActionHasTypedResponse` | `(r) bool` | True if any CRUD action (except delete) has typed response |
| `schemaByGoName` | `(name) *Schema` | Look up schema by Go type name |
| `listArrayField` | `(s) *Field` | Find primary array field in a list response schema |
| `leafFields` | `(s) []*Field` | Scalar fields suitable for table columns (max 6) |
| `fieldAccessExpr` | `(f) string` | `item.Field` or `fmt.Sprint(item.Field)` |
| `fieldAccessExprData` | `(f) string` | Same but with `data.` prefix (for get commands) |
| `arrayElemType` | `(f) string` | Element type of an array field |
| `cliNeedsFmt` | `(r) bool` | Whether CLI file needs `fmt` import for table formatting |

## Template Data Context

Templates receive different data depending on what they generate:

| Template | Data Type | Key Fields |
|----------|-----------|------------|
| Static files (main, makefile, client, config, output, upgrade) | `*spec.Project` | `.Name`, `.Module`, `.EnvPrefix`, `.Auth`, `.TUI`, `.Resources`, `.Schemas` |
| `service.go.tmpl` | `*spec.Project` | `.Module`, `.Resources` |
| `types.go.tmpl` | `struct{APITitle, APIVersion, Schemas}` | `.Schemas` (subset per domain file) |
| `resource.go.tmpl` (service) | `struct{Project, Resource}` | `.Project.Module`, `.Resource.*` |
| `resource.go.tmpl` (CLI) | `struct{Project, Resource}` | `.Project.Module`, `.Resource.*` |
| `test_resource.sh.tmpl` | `struct{Project, Resource}` | `.Project.Name`, `.Resource.CLIName`, `.Resource.Actions` |

## Generated Output Structure

Running `gocli-gen generate --spec api.yaml --name myapp --module github.com/org/myapp` produces:

```
myapp/
├── main.go                        # cli.Execute() entry point
├── Makefile                       # build, dev, test, clean
├── go.mod                         # Module with cobra, lipgloss, yaml deps (+bubbletea if --tui)
├── gocli-gen.yaml                 # Generation config reference
├── cli/
│   ├── root.go                    # Root cmd, persistent flags, newService(), newAuthProvider()
│   ├── config.go                  # config init|show
│   ├── version.go
│   └── <resource>.go              # One per resource: list/get/create/update/delete subcommands
├── internal/
│   ├── api/
│   │   ├── types.go               # Shared/unassociated schemas
│   │   └── <resource>.go          # Domain-specific schemas (response/request types + deps)
│   ├── client/
│   │   ├── client.go              # Get/Post/Put/Delete/PostForm/GetRaw, AuthProvider interface
│   │   └── errors.go              # APIError, IsNotFound, IsUnauthorized, IsForbidden
│   ├── config/
│   │   └── config.go              # YAML config, env override, Load/Save/Validate
│   ├── output/
│   │   ├── output.go              # Formatter interface, Print(), PrintItem()
│   │   ├── json.go                # JSON output (MarshalIndent)
│   │   ├── table.go               # Lipgloss table
│   │   └── text.go                # Plain text
│   ├── service/
│   │   ├── service.go             # AppService aggregate
│   │   └── <resource>.go          # Interface + impl (typed returns when possible)
│   └── upgrade/
│       └── upgrade.go             # GitHub release self-upgrade
├── tui/                           # Only generated with --tui flag
│   ├── tui.go                     # Bubble Tea 3-screen router (Home/List/Detail)
│   ├── resources.go               # Per-resource closures (list/get → JSON)
│   ├── home.go                    # Resource picker with cursor navigation
│   ├── list.go                    # Generic JSON table (bubbles/table)
│   ├── detail.go                  # Scrollable key-value detail (bubbles/viewport)
│   ├── filter.go                  # Text input filter component
│   └── styles.go                  # Lipgloss styles
└── tests/
    ├── helpers.sh                 # assert, assert_fail, assert_output_contains, print_report
    ├── run-all.sh
    └── test-<resource>.sh         # Help + validation tests per resource
```

## TUI Architecture (Optional, `--tui` flag)

The TUI is generated only when `--tui` is passed to `gocli-gen generate`. Without it, no `tui/` directory is created and bubbletea/bubbles are not added to `go.mod`.

### Design: Generic JSON Approach

The TUI displays data from any API without knowing specific types at compile time. A **Resource Registry** (`tui/resources.go`) generates closures that call the typed service methods and marshal results to `[]byte` JSON. TUI screens parse this JSON at runtime.

```go
type TUIResource struct {
    Name   string
    Plural string
    Scoped bool
    ListFn func(scope string) ([]byte, error)    // nil if no list action
    GetFn  func(scope, id string) ([]byte, error) // nil if no get action
}
```

### 3-Screen Architecture

| Screen | File | Purpose |
|--------|------|---------|
| Home | `home.go` | Resource picker — lists all resources, cursor navigation |
| List | `list.go` | Generic table (`bubbles/table`) — columns auto-detected from JSON keys |
| Detail | `detail.go` | Scrollable key-value display (`bubbles/viewport`) |

**Navigation:** Home→List (select resource), List→Detail (enter on row), Detail→List (esc), List→Home (esc)

### Generated App Usage

The generated app gets a `--tui` flag on the root command:
```bash
./myapp --tui                    # Launch TUI (uses config for URL/token)
./myapp --tui --project PROJ     # Launch TUI with scope pre-set
```

### JSON Table Auto-Detection

`list.go` handles arbitrary JSON responses:
1. Tries to parse as `[]map[string]any` (direct array)
2. Falls back to finding the first array field in an object wrapper
3. Extracts column headers from the first item's keys (ID-like fields first)
4. Skips nested objects/arrays for table display

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Config | Direct `yaml.v3`, no Viper | Simpler, no case-sensitivity issues |
| Auth | `AuthProvider` interface with 4 implementations | Detected from spec's `securitySchemes`, selected at runtime via config `auth_type` |
| Base path | Stored in config as-is, no suffix auto-appended | URL is a runtime concern, not codegen-time |
| Service returns | Typed (`*api.XxxType`) when response references a named schema, `json.RawMessage` fallback | Type safety where possible without being fragile |
| API type files | Split by resource association (response/request refs + 1-level field deps) | Keeps domain types together, shared types in `types.go` |
| Table output | Auto-detect array field + leaf fields from response schema | `list` commands iterate array, `get` commands show key-value; falls back to `PrintItem` |
| Templates | `embed.FS` + `text/template` | Readable output, embedded in binary, no external deps |
| Formatting | `go/format` + optional `goimports` post-processing | Always produces valid Go; `goimports` fixes unused imports |
| TUI | Optional via `--tui` flag | No bubbletea deps or `tui/` dir unless opted in |
| TUI data | Generic JSON (marshal typed results → parse at runtime) | Works with any API without compile-time type knowledge |

## Common Tasks

### Building gocli-gen

```bash
make dev              # go build -o gocli-gen .
go test ./...         # Run unit tests (naming package)
```

### Versioning & Release

- `var version string` in `main.go` is injected at build time via `-ldflags "-X main.version=<tag>"`
- `cmd.Version` defaults to `"dev"` for local builds; `main.go` overrides it when `version != ""`
- `--version` uses cobra's built-in version flag
- `--upgrade` calls `internal/upgrade.Run(Version)` to self-update from GitHub releases
- The `.github/workflows/release.yml` workflow triggers on release creation, builds `gocli-gen-macos-arm64` and `gocli-gen-linux-amd64`, and uploads them as release assets

### Release Process

1. Decide on a version tag (e.g., `v0.1.0`)
2. Create a GitHub release with that tag
3. The workflow builds cross-platform binaries with the version injected via ldflags
4. Users can then run `gocli-gen --upgrade` to fetch the latest release

### Generating a Project

```bash
gocli-gen generate \
  --spec path/to/openapi.json \
  --name myapp \
  --module github.com/org/myapp \
  --output /tmp/myapp \
  --config path/to/gocli-gen.yaml \  # optional overrides
  --tui                              # optional: generate Bubble Tea TUI
```

Then in the generated project:
```bash
cd /tmp/myapp
go mod tidy
go build -o myapp .
./tests/run-all.sh ./myapp
```

### Adding a New Template

1. Create `pkg/codegen/templates/<path>/<name>.tmpl`
2. Register it in `generator.go`'s `Generate()` method:
   - Static file → add to `staticFiles` slice
   - Per-resource file → add to the `for _, res := range g.project.Resources` loop
   - TUI file → add to the `tuiFiles` slice (inside the `if g.project.TUI` block)
3. Set `isGo: true` if it generates `.go` files (enables `go/format`)
4. Templates are embedded via `//go:embed templates` — no build step needed

### Adding a New Template Function

1. Add the function to `generator.go`'s `funcMap` in `NewGenerator()`
2. Document it in the Template Functions section above
3. Use it in templates as `{{functionName arg1 arg2}}`

### Modifying the IR

1. Add/change fields in `pkg/spec/ir.go`
2. Populate them in `pkg/spec/parser.go` (during parsing) or `pkg/spec/grouper.go` (during grouping)
3. Update templates that consume the new fields
4. Test with: `go run . generate --spec <spec> --name test --module github.com/test/test --output /tmp/test && cd /tmp/test && go mod tidy && go build ./...`

### Modifying the Resource Grouper

The grouper lives in `pkg/spec/grouper.go`. Key functions:

| Function | Purpose |
|----------|---------|
| `groupEndpoints()` | Entry point: endpoints → resource map |
| `countSegmentEndpoints()` | Phase 1: count direct endpoints per segment |
| `assignEndpoint()` | Phase 2: assign each endpoint to a resource |
| `findResourceSegment()` | Find first non-`{param}` segment in a path |
| `inferAction()` | HTTP method + path shape → CRUD action name |
| `buildAction()` | Create Action struct from endpoint data |

**Testing changes:** Run the generator against a known spec and verify:
1. Resource count is reasonable (not 1:1 with endpoints)
2. CRUD actions are correctly detected
3. Custom actions collapse onto scope resources
4. Generated project compiles with `go build ./...`

### Modifying Naming

The naming package (`pkg/naming/`) is imported by both `spec` and `codegen`. Key functions:

- `ToGoName(s)` — handles acronyms (`id` → `ID`, `url` → `URL`, `httpServer` → `HTTPServer`)
- `ToCLIName(s)` — lowercase hyphenated (`ProjectType` → `project-type`)
- `ToSnakeCase(s)` — lowercase underscored
- `ToPlural(s)` — English pluralization with irregular forms
- `splitWords(s)` — word boundary detection (camelCase, PascalCase, snake_case, kebab-case)

Always run `go test ./pkg/naming/...` after changes.

## Shell Template Gotcha

Go's `text/template` and shell `${VAR}` syntax collide. The pattern `${ENV_PREFIX_LIVE_TESTS}` must be written as:

```
{{printf "${%s_LIVE_TESTS:-0}" .EnvPrefix}}
```

Never write `${{{.EnvPrefix}}_LIVE_TESTS}` — the `${` + `{{` sequence breaks the template parser.

## Known Limitations & Future Work

### Custom Actions Not Generated as CLI Subcommands
The CLI template (`resource.go.tmpl`) only generates `list`, `get`, `create`, `update`, `delete` subcommands. Custom actions (e.g., `tree`, `audit`, `access`, `hide`, `clone`) are parsed into the IR (`Resource.Actions`) but not emitted as CLI commands. Resources with only custom actions (like `project` in the Matrix API) appear as empty parent commands.

**To fix:** Add a `customActions` section to `resource.go.tmpl` that iterates `{{range customActions .Resource}}` and generates a cobra command per action.

### Write Commands Pass `nil` Bodies
Generated `create` and `update` commands call the service with `nil` as the request body. They don't generate CLI flags for the request body fields.

**To fix:** In `resource.go.tmpl`, for create/update actions with `RequestBody != nil`, generate cobra flags from the schema's fields and build the request struct.

### Root-Level Resources Not Detected
APIs where CRUD operations live at the root path (`GET /`, `POST /`, `GET /{id}`) don't produce a resource because there's no non-param path segment. The Matrix API's project endpoints (`GET /`, `POST /`, `GET /{project}`, `DELETE /{project}`) fall into this category.

**To fix:** Add special handling in `grouper.go` for paths with only param segments — treat the first param as the resource name.

### Resource Naming Uses Raw Path Segments
Resources are named from API path segments (`cat`, `needle`, `needleminimal`) rather than human-friendly names (`category`, `search`). The `gocli-gen.yaml` config can override aliases but not the canonical name.

**To fix:** Add a `cli_name` override in `gocli-gen.yaml` resource config, or infer better names from the OpenAPI `tags` section.

### Test Scripts Are Skeletal
Generated test scripts only verify `--help` output and argument validation. They don't test actual API interactions or output formatting.

## Error Handling Conventions

- All errors propagate up via `return fmt.Errorf("context: %w", err)`
- Template execution errors include the template path
- `go/format` failures produce a warning to stderr but still write the unformatted file (for debugging)
- The generator fails fast on the first error — no partial output


## Release

When the user requests release notes after completing an implementation:

1. **Ask the user what version number to release** — do not assume or auto-increment.
2. Once the user provides the version, update `var version` in `cli/root.go`:
   ```go
   var version = "<new-version>"
   ```
3. Confirm the build passes with `make build` and that `mxreq --version` prints the new version.