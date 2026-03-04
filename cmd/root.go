package cmd

import (
	"fmt"
	"os"

	"github.com/VladGavrila/gocli-gen/internal/upgrade"
	"github.com/VladGavrila/gocli-gen/pkg/codegen"
	"github.com/VladGavrila/gocli-gen/pkg/spec"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "1.0.0"

var (
	flagSpec    string
	flagName    string
	flagModule  string
	flagOutput  string
	flagConfig  string
	flagTUI     bool
	flagUpgrade bool
)

var rootCmd = &cobra.Command{
	Use:     "gocli-gen",
	Short:   "Generate a Go CLI+TUI project from an OpenAPI spec",
	Long:    "gocli-gen reads an OpenAPI 3.x specification and generates a complete, idiomatic Go CLI project with Cobra commands, service layer, HTTP client, config management, output formatters, and optional Bubble Tea TUI.",
	Version: Version,
	Run: func(cmd *cobra.Command, args []string) {
		if flagUpgrade {
			if err := upgrade.Run(Version); err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				os.Exit(1)
			}
			return
		}
		cmd.Help() //nolint:errcheck
	},
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a Go CLI project from an OpenAPI spec",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse the OpenAPI spec into IR
		project, err := spec.Parse(flagSpec, flagName, flagModule)
		if err != nil {
			return fmt.Errorf("parsing spec: %w", err)
		}

		// Load optional config overrides
		if flagConfig != "" {
			if err := spec.ApplyConfig(project, flagConfig); err != nil {
				return fmt.Errorf("applying config: %w", err)
			}
		}

		// Set optional features
		project.TUI = flagTUI

		// Determine output directory
		outDir := flagOutput
		if outDir == "" {
			outDir = project.Name
		}

		// Generate the project
		gen := codegen.NewGenerator(project, outDir)
		if err := gen.Generate(); err != nil {
			return fmt.Errorf("generating project: %w", err)
		}

		fmt.Printf("Generated project in %s/\n", outDir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)

	rootCmd.Flags().BoolVar(&flagUpgrade, "upgrade", false, "upgrade gocli-gen to the latest release")

	generateCmd.Flags().StringVar(&flagSpec, "spec", "", "Path to OpenAPI 3.x spec (YAML or JSON)")
	_ = generateCmd.MarkFlagRequired("spec")
	generateCmd.Flags().StringVar(&flagName, "name", "", "Binary name (e.g., mxreq)")
	_ = generateCmd.MarkFlagRequired("name")
	generateCmd.Flags().StringVar(&flagModule, "module", "", "Go module path (e.g., github.com/Foo/mxreq)")
	_ = generateCmd.MarkFlagRequired("module")
	generateCmd.Flags().StringVar(&flagOutput, "output", "", "Output directory (defaults to --name)")
	generateCmd.Flags().StringVar(&flagConfig, "config", "", "Path to gocli-gen.yaml config file")
	generateCmd.Flags().BoolVar(&flagTUI, "tui", false, "Generate Bubble Tea TUI code")
}

// Execute runs the root command.
func Execute() {
	rootCmd.Version = Version
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
