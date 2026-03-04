package main

import "github.com/VladGavrila/gocli-gen/cmd"

// version is set at build time via ldflags: -X main.version=<tag>
var version string

func main() {
	if version != "" {
		cmd.Version = version
	}
	cmd.Execute()
}
