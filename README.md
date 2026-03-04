# gocli-gen

Generate a complete Go CLI project from an OpenAPI 3.x spec.

Given an API spec, `gocli-gen` produces a ready-to-build Go project with:
- **Cobra CLI** with per-resource commands (list, get, create, update, delete)
- **Service layer** with typed interfaces and response unmarshaling
- **HTTP client** with pluggable auth (Token, Bearer, Basic, API Key)
- **Output formatters** (JSON, table via lipgloss, plain text)
- **YAML config** with env var overrides (`~/.config/<name>/config.yaml`)
- **Bubble Tea TUI** (optional, `--tui` flag) with generic resource browsing
- **Shell tests** (help + validation per resource)
- **GitHub self-upgrade** (`--upgrade` flag)

The output is a **one-time scaffold** -- you own the generated code and modify it freely.

## Installation

Download a pre-built binary from the [Releases](https://github.com/VladGavrila/gocli-gen/releases) page

### macOS: removing the quarantine flag

macOS Gatekeeper will block unsigned binaries downloaded from the internet. After downloading, remove the quarantine attribute:

```bash
xattr -c path/to/mxreq
```

You only need to do this once after downloading.

### Upgrade

```bash
gocli-gen --upgrade
```

This checks GitHub releases for a newer version and replaces the binary in place.

## Usage

```bash
gocli-gen generate \
  --spec path/to/openapi.yaml \
  --name myapp \
  --module github.com/org/myapp \
  --output ./myapp \
  --tui                              # optional: include Bubble Tea TUI
```

Then build and run:

```bash
cd myapp
go mod tidy
go build -o myapp .
./myapp --help
./tests/run-all.sh ./myapp
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--version` | Print version |
| `--upgrade` | Upgrade to the latest GitHub release |

### Generate Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--spec` | yes | Path to OpenAPI 3.x spec (YAML or JSON) |
| `--name` | yes | Binary name (e.g., `myapp`) |
| `--module` | yes | Go module path (e.g., `github.com/org/myapp`) |
| `--output` | no | Output directory (defaults to `--name`) |
| `--config` | no | Path to `gocli-gen.yaml` for customization |
| `--tui` | no | Generate Bubble Tea TUI code |

## Customization

Create a `gocli-gen.yaml` file to override defaults:

```yaml
github_repo: org/myapp          # For --upgrade self-update
env_prefix: MYAPP               # Env var prefix (default: uppercase of --name)
auth_type: bearer               # token | bearer | basic | apikey

resources:
  user:
    aliases: ["u"]              # CLI aliases
    scope: global               # global or a scope param name
  item:
    aliases: ["it"]
    scope: project              # Requires --project flag
    write_flag: reason          # Write ops require --reason
    table_columns: [ID, Title]  # Table columns (not yet implemented)
```

Pass it with `--config gocli-gen.yaml`.

## What Gets Generated

```
myapp/
├── main.go
├── Makefile
├── go.mod
├── cli/                    # Cobra commands
│   ├── root.go             # Root cmd, persistent flags (--url, --token, --project, --output, --debug)
│   ├── config.go           # config init|show
│   ├── version.go
│   └── <resource>.go       # list/get/create/update/delete per resource
├── internal/
│   ├── api/                # Go structs from components.schemas (split by resource)
│   ├── client/             # HTTP client with auth, debug, error types
│   ├── config/             # YAML config with env var overrides
│   ├── output/             # JSON, table (lipgloss), text formatters
│   ├── service/            # Interface-based services per resource
│   └── upgrade/            # GitHub release self-upgrade
├── tui/                    # Bubble Tea TUI (only with --tui)
│   ├── tui.go              # Screen router (Home/List/Detail)
│   ├── resources.go        # Per-resource list/get closures
│   ├── home.go, list.go, detail.go, filter.go, styles.go
└── tests/                  # Shell test scripts
```

## How It Works

1. **Parse** the OpenAPI spec into an intermediate representation (schemas, endpoints, parameters, auth)
2. **Group** endpoints into resources by analyzing path segments
3. **Infer** CRUD actions from HTTP methods and path shapes
4. **Detect** auth scheme from `securitySchemes`
5. **Execute** Go templates with the IR data
6. **Format** output with `go/format` and optionally `goimports`

### Resource Grouping

Endpoints are grouped by their first non-parameter path segment:

| Path | Resource | Action |
|------|----------|--------|
| `GET /users` | user | list |
| `GET /users/{id}` | user | get |
| `POST /users` | user | create |
| `GET /{project}/item` | item (scoped) | list |
| `GET /{project}/tree` | project | tree (custom) |

Segments with fewer than 2 direct endpoints collapse as custom actions on their scope resource, keeping the command count manageable.

### Auth Detection

From `components.securitySchemes`:

| Spec | Generated Auth |
|------|---------------|
| `type: http, scheme: bearer` | BearerAuth |
| `type: http, scheme: basic` | BasicAuth |
| `type: apiKey, in: header` | APIKeyAuth |
| No security schemes | TokenAuth (default) |

Auth type is selected at runtime from the config file's `auth_type` field.

## TUI Mode

When generated with `--tui`, the output binary gets a `--tui` flag that launches an interactive terminal UI:

```bash
./myapp --tui                    # Launch TUI
./myapp --tui --project PROJ     # Launch with scope pre-set
```

The TUI provides three screens:

- **Home** -- pick a resource to browse
- **List** -- generic table view (columns auto-detected from JSON response keys, filterable with `/`)
- **Detail** -- scrollable key-value view of a single item

The TUI works with any API by marshaling typed service responses to JSON and parsing them at runtime. No compile-time type knowledge is needed per-resource.

## After Generation

The generated project is a starting point. Common next steps:

- **Add CLI flags to write commands** -- `create` and `update` commands currently pass `nil` bodies
- **Add custom action subcommands** -- non-CRUD actions are parsed but not yet generated as CLI commands
- **Tune table columns** -- auto-detected columns use the first 6 scalar fields alphabetically
- **Add live integration tests** -- generated tests only check `--help` and argument validation
- **Customize resource names** via `gocli-gen.yaml` aliases

## License

MIT
