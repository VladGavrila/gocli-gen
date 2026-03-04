package codegen

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/VladGavrila/gocli-gen/pkg/naming"
	"github.com/VladGavrila/gocli-gen/pkg/spec"
)

//go:embed templates
var templateFS embed.FS

// Generator orchestrates template execution and file writing.
type Generator struct {
	project *spec.Project
	outDir  string
	funcMap template.FuncMap
}

// NewGenerator creates a new Generator.
func NewGenerator(project *spec.Project, outDir string) *Generator {
	g := &Generator{
		project: project,
		outDir:  outDir,
	}
	g.funcMap = template.FuncMap{
		"toLower":     strings.ToLower,
		"toUpper":     strings.ToUpper,
		"toGoName":    naming.ToGoName,
		"toCLIName":   naming.ToCLIName,
		"toSnakeCase": naming.ToSnakeCase,
		"toPlural":    naming.ToPlural,
		"join":        strings.Join,
		"hasPrefix":   strings.HasPrefix,
		"hasSuffix":   strings.HasSuffix,
		"trimPrefix":  strings.TrimPrefix,
		"trimSuffix":  strings.TrimSuffix,
		"contains":    strings.Contains,
		"replace":     strings.ReplaceAll,
		"quote":       func(s string) string { return `"` + s + `"` },
		"backtick":    func(s string) string { return "`" + s + "`" },
		"add":         func(a, b int) int { return a + b },
		"sub":         func(a, b int) int { return a - b },
		"isLast":      func(i, length int) bool { return i == length-1 },

		// Resource helpers
		"hasAction": func(r *spec.Resource, name string) bool {
			for _, a := range r.Actions {
				if a.Name == name {
					return true
				}
			}
			return false
		},
		"getAction": func(r *spec.Resource, name string) *spec.Action {
			for _, a := range r.Actions {
				if a.Name == name {
					return a
				}
			}
			return nil
		},
		"isScoped": func(r *spec.Resource) bool {
			return r.Scope != "" && r.Scope != "global"
		},
		"hasWriteOps": func(r *spec.Resource) bool {
			for _, a := range r.Actions {
				switch a.Method {
				case "POST", "PUT", "DELETE", "PATCH":
					return true
				}
			}
			return false
		},
		"writeActions": func(r *spec.Resource) []*spec.Action {
			var result []*spec.Action
			for _, a := range r.Actions {
				switch a.Method {
				case "POST", "PUT", "DELETE", "PATCH":
					result = append(result, a)
				}
			}
			return result
		},
		"readActions": func(r *spec.Resource) []*spec.Action {
			var result []*spec.Action
			for _, a := range r.Actions {
				if a.Method == "GET" {
					result = append(result, a)
				}
			}
			return result
		},
		"crudActions": func(r *spec.Resource) []*spec.Action {
			var result []*spec.Action
			for _, a := range r.Actions {
				switch a.Name {
				case "list", "get", "create", "update", "delete":
					result = append(result, a)
				}
			}
			return result
		},
		"customActions": func(r *spec.Resource) []*spec.Action {
			var result []*spec.Action
			for _, a := range r.Actions {
				switch a.Name {
				case "list", "get", "create", "update", "delete":
					continue
				default:
					result = append(result, a)
				}
			}
			return result
		},

		// Response type helpers
		"hasTypedResponse": func(a *spec.Action) bool {
			return a.Response != nil && a.Response.Ref != ""
		},
		"responseGoType": func(a *spec.Action) string {
			if a.Response == nil || a.Response.Ref == "" {
				return ""
			}
			return a.Response.GoName
		},
		// Schema lookup helpers
		"schemaByGoName": func(name string) *spec.Schema {
			for _, s := range project.Schemas {
				if s.GoName == name {
					return s
				}
			}
			return nil
		},
		"listArrayField": func(s *spec.Schema) *spec.Field {
			if s == nil {
				return nil
			}
			// Find the first array field whose element is a named type
			for _, f := range s.Fields {
				if strings.HasPrefix(f.GoType, "[]") && !strings.HasPrefix(f.GoType, "[]string") && !strings.HasPrefix(f.GoType, "[]int") && !strings.HasPrefix(f.GoType, "[]bool") {
					return f
				}
			}
			// Fallback: any array field
			for _, f := range s.Fields {
				if strings.HasPrefix(f.GoType, "[]") {
					return f
				}
			}
			return nil
		},
		"leafFields": func(s *spec.Schema) []*spec.Field {
			if s == nil {
				return nil
			}
			var result []*spec.Field
			for _, f := range s.Fields {
				switch f.GoType {
				case "string", "int", "int64", "float64", "float32", "bool":
					result = append(result, f)
				}
			}
			// Limit to 6 columns for readability
			if len(result) > 6 {
				result = result[:6]
			}
			return result
		},
		"fieldAccessExpr": func(f *spec.Field) string {
			switch f.GoType {
			case "string":
				return "item." + f.GoName
			default:
				return "fmt.Sprint(item." + f.GoName + ")"
			}
		},
		"cliNeedsFmt": func(r *spec.Resource) bool {
			crudNames := map[string]bool{"list": true, "get": true}
			for _, a := range r.Actions {
				if !crudNames[a.Name] || a.Response == nil || a.Response.Ref == "" {
					continue
				}
				respGoName := a.Response.GoName
				var respSchema *spec.Schema
				for _, s := range project.Schemas {
					if s.GoName == respGoName {
						respSchema = s
						break
					}
				}
				if respSchema == nil {
					continue
				}
				if a.Name == "list" {
					// Check if list array field has leaf elements with non-string fields
					for _, f := range respSchema.Fields {
						if strings.HasPrefix(f.GoType, "[]") && !strings.HasPrefix(f.GoType, "[]string") && !strings.HasPrefix(f.GoType, "[]int") && !strings.HasPrefix(f.GoType, "[]bool") {
							elemType := strings.TrimPrefix(f.GoType, "[]")
							for _, es := range project.Schemas {
								if es.GoName == elemType {
									for _, ef := range es.Fields {
										if ef.GoType != "string" && (ef.GoType == "int" || ef.GoType == "int64" || ef.GoType == "float64" || ef.GoType == "float32" || ef.GoType == "bool") {
											return true
										}
									}
								}
							}
							break
						}
					}
				}
				if a.Name == "get" {
					for _, f := range respSchema.Fields {
						if f.GoType != "string" && (f.GoType == "int" || f.GoType == "int64" || f.GoType == "float64" || f.GoType == "float32" || f.GoType == "bool") {
							return true
						}
					}
				}
			}
			return false
		},
		"fieldAccessExprData": func(f *spec.Field) string {
			switch f.GoType {
			case "string":
				return "data." + f.GoName
			default:
				return "fmt.Sprint(data." + f.GoName + ")"
			}
		},
		"arrayElemType": func(f *spec.Field) string {
			if f == nil || !strings.HasPrefix(f.GoType, "[]") {
				return ""
			}
			return strings.TrimPrefix(f.GoType, "[]")
		},

		"anyActionHasTypedResponse": func(r *spec.Resource) bool {
			crudNames := map[string]bool{"list": true, "get": true, "create": true, "update": true, "delete": true}
			for _, a := range r.Actions {
				if crudNames[a.Name] && a.Name != "delete" && a.Response != nil && a.Response.Ref != "" {
					return true
				}
			}
			return false
		},
	}
	return g
}

// Generate produces all output files.
func (g *Generator) Generate() error {
	// Create output directory
	if err := os.MkdirAll(g.outDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// Generate static files
	staticFiles := []struct {
		tmpl string
		out  string
		isGo bool
	}{
		{"templates/main.go.tmpl", "main.go", true},
		{"templates/go_mod.tmpl", "go.mod", false},
		{"templates/makefile.tmpl", "Makefile", false},
		{"templates/gocli_gen_yaml.tmpl", "gocli-gen.yaml", false},

		// internal/client
		{"templates/internal/client/client.go.tmpl", "internal/client/client.go", true},
		{"templates/internal/client/errors.go.tmpl", "internal/client/errors.go", true},

		// internal/config
		{"templates/internal/config/config.go.tmpl", "internal/config/config.go", true},

		// internal/output
		{"templates/internal/output/output.go.tmpl", "internal/output/output.go", true},
		{"templates/internal/output/json.go.tmpl", "internal/output/json.go", true},
		{"templates/internal/output/table.go.tmpl", "internal/output/table.go", true},
		{"templates/internal/output/text.go.tmpl", "internal/output/text.go", true},

		// internal/upgrade
		{"templates/internal/upgrade/upgrade.go.tmpl", "internal/upgrade/upgrade.go", true},

		// cli
		{"templates/cli/root.go.tmpl", "cli/root.go", true},
		{"templates/cli/config.go.tmpl", "cli/config.go", true},
		{"templates/cli/version.go.tmpl", "cli/version.go", true},

		// tests
		{"templates/tests/helpers.sh.tmpl", "tests/helpers.sh", false},
		{"templates/tests/run_all.sh.tmpl", "tests/run-all.sh", false},
	}

	for _, sf := range staticFiles {
		if err := g.generateFile(sf.tmpl, sf.out, g.project, sf.isGo); err != nil {
			return fmt.Errorf("generating %s: %w", sf.out, err)
		}
	}

	// Conditionally generate TUI files
	if g.project.TUI {
		tuiFiles := []struct {
			tmpl string
			out  string
		}{
			{"templates/tui/tui.go.tmpl", "tui/tui.go"},
			{"templates/tui/styles.go.tmpl", "tui/styles.go"},
			{"templates/tui/home.go.tmpl", "tui/home.go"},
			{"templates/tui/list.go.tmpl", "tui/list.go"},
			{"templates/tui/detail.go.tmpl", "tui/detail.go"},
			{"templates/tui/filter.go.tmpl", "tui/filter.go"},
			{"templates/tui/resources.go.tmpl", "tui/resources.go"},
		}
		for _, tf := range tuiFiles {
			if err := g.generateFile(tf.tmpl, tf.out, g.project, true); err != nil {
				return fmt.Errorf("generating %s: %w", tf.out, err)
			}
		}
	}

	// Generate API type files split by resource
	if err := g.generateAPITypes(); err != nil {
		return err
	}

	// Generate service aggregate
	if err := g.generateFile("templates/internal/service/service.go.tmpl", "internal/service/service.go", g.project, true); err != nil {
		return fmt.Errorf("generating service/service.go: %w", err)
	}

	// Generate per-resource files
	for _, res := range g.project.Resources {
		data := struct {
			Project  *spec.Project
			Resource *spec.Resource
		}{g.project, res}

		// Service file
		svcOut := fmt.Sprintf("internal/service/%s.go", res.CLIName)
		if err := g.generateFile("templates/internal/service/resource.go.tmpl", svcOut, data, true); err != nil {
			return fmt.Errorf("generating %s: %w", svcOut, err)
		}

		// CLI file
		cliOut := fmt.Sprintf("cli/%s.go", res.CLIName)
		if err := g.generateFile("templates/cli/resource.go.tmpl", cliOut, data, true); err != nil {
			return fmt.Errorf("generating %s: %w", cliOut, err)
		}

		// Test file
		testOut := fmt.Sprintf("tests/test-%s.sh", res.CLIName)
		if err := g.generateFile("templates/tests/test_resource.sh.tmpl", testOut, data, false); err != nil {
			return fmt.Errorf("generating %s: %w", testOut, err)
		}
	}

	// Make shell scripts executable
	g.makeExecutable("tests/helpers.sh")
	g.makeExecutable("tests/run-all.sh")
	for _, res := range g.project.Resources {
		g.makeExecutable(fmt.Sprintf("tests/test-%s.sh", res.CLIName))
	}

	// Run goimports if available
	g.runGoimports()

	return nil
}

// generateAPITypes splits schemas into per-resource files + a shared types.go.
func (g *Generator) generateAPITypes() error {
	// Build mapping: schema GoName → resource name
	schemaToResource := make(map[string]string)

	// Phase 1: direct references from actions (response/request)
	for _, res := range g.project.Resources {
		for _, a := range res.Actions {
			if a.Response != nil && a.Response.GoName != "" && a.Response.GoName != "Response" {
				schemaToResource[a.Response.GoName] = res.CLIName
			}
			if a.RequestBody != nil && a.RequestBody.GoName != "" && a.RequestBody.GoName != "RequestBody" {
				schemaToResource[a.RequestBody.GoName] = res.CLIName
			}
		}
	}

	// Phase 2: follow field type references (one level deep)
	for _, s := range g.project.Schemas {
		resName, ok := schemaToResource[s.GoName]
		if !ok {
			continue
		}
		for _, f := range s.Fields {
			ft := strings.TrimPrefix(f.GoType, "[]")
			ft = strings.TrimPrefix(ft, "*")
			if ft == "" || ft == "string" || ft == "int" || ft == "int64" || ft == "float64" || ft == "float32" || ft == "bool" || ft == "any" || strings.HasPrefix(ft, "map[") {
				continue
			}
			// This field references another schema — associate it too if not yet assigned
			if _, exists := schemaToResource[ft]; !exists {
				schemaToResource[ft] = resName
			}
		}
	}

	// Group schemas by resource
	resourceSchemas := make(map[string][]*spec.Schema)
	var sharedSchemas []*spec.Schema

	for _, s := range g.project.Schemas {
		if resName, ok := schemaToResource[s.GoName]; ok {
			resourceSchemas[resName] = append(resourceSchemas[resName], s)
		} else {
			sharedSchemas = append(sharedSchemas, s)
		}
	}

	// Generate shared types.go
	if len(sharedSchemas) > 0 {
		data := struct {
			APITitle   string
			APIVersion string
			Schemas    []*spec.Schema
		}{g.project.APITitle, g.project.APIVersion, sharedSchemas}
		if err := g.generateFile("templates/internal/api/types.go.tmpl", "internal/api/types.go", data, true); err != nil {
			return fmt.Errorf("generating api/types.go: %w", err)
		}
	}

	// Generate per-resource type files
	for resName, schemas := range resourceSchemas {
		data := struct {
			APITitle   string
			APIVersion string
			Schemas    []*spec.Schema
		}{g.project.APITitle, g.project.APIVersion, schemas}
		outPath := fmt.Sprintf("internal/api/%s.go", resName)
		if err := g.generateFile("templates/internal/api/types.go.tmpl", outPath, data, true); err != nil {
			return fmt.Errorf("generating %s: %w", outPath, err)
		}
	}

	return nil
}

// generateFile executes a template and writes the output to a file.
func (g *Generator) generateFile(tmplPath, outPath string, data any, isGo bool) error {
	tmplData, err := templateFS.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("reading template %s: %w", tmplPath, err)
	}

	tmpl, err := template.New(filepath.Base(tmplPath)).Funcs(g.funcMap).Parse(string(tmplData))
	if err != nil {
		return fmt.Errorf("parsing template %s: %w", tmplPath, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("executing template %s: %w", tmplPath, err)
	}

	output := buf.Bytes()

	// Format Go files
	if isGo {
		formatted, err := format.Source(output)
		if err != nil {
			// Write unformatted so we can debug
			fmt.Fprintf(os.Stderr, "warning: could not format %s: %v\n", outPath, err)
		} else {
			output = formatted
		}
	}

	// Ensure parent directory exists
	fullPath := filepath.Join(g.outDir, outPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", outPath, err)
	}

	return os.WriteFile(fullPath, output, 0o644)
}

// makeExecutable sets the executable bit on a file.
func (g *Generator) makeExecutable(path string) {
	fullPath := filepath.Join(g.outDir, path)
	_ = os.Chmod(fullPath, 0o755)
}

// runGoimports runs goimports on all generated Go files if available.
func (g *Generator) runGoimports() {
	goimports, err := exec.LookPath("goimports")
	if err != nil {
		return // goimports not available, skip
	}
	cmd := exec.Command(goimports, "-w", g.outDir)
	_ = cmd.Run()
}
