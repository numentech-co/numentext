package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"numentext/internal/app"
	"numentext/internal/config"
	"numentext/internal/plugin"
)

// version is set at build time via ldflags:
//
//	go build -ldflags "-X main.version=1.0.0" .
var version = "dev"

func main() {
	// Print version and exit
	if len(os.Args) >= 2 && os.Args[1] == "--version" {
		fmt.Println("numentext " + version)
		os.Exit(0)
	}

	// Handle plugin CLI commands before starting the app
	if len(os.Args) >= 2 {
		handled, code := handlePluginCLI(os.Args[1:])
		if handled {
			os.Exit(code)
		}
	}

	// Check for --merge flag: numentext --merge LOCAL BASE REMOTE MERGED
	if len(os.Args) >= 6 && os.Args[1] == "--merge" {
		localPath := os.Args[2]
		basePath := os.Args[3]
		remotePath := os.Args[4]
		mergedPath := os.Args[5]

		application := app.New()
		if err := application.OpenMergeFiles(localPath, basePath, remotePath, mergedPath); err != nil {
			fmt.Fprintf(os.Stderr, "Merge error: %v\n", err)
			os.Exit(1)
		}
		application.StartMergeMode()
		if err := application.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	application := app.New()
	// Open files passed as command line arguments
	for _, arg := range os.Args[1:] {
		application.OpenFileByPath(arg)
	}
	if err := application.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// handlePluginCLI handles --plugin-* CLI flags.
// Returns (handled bool, exitCode int).
func handlePluginCLI(args []string) (bool, int) {
	if len(args) == 0 {
		return false, 0
	}

	pluginDir := filepath.Join(config.ConfigDir(), "plugins")
	cfg := config.Load()

	switch args[0] {
	case "--plugin-install":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: numentext --plugin-install <repo-url>")
			return true, 1
		}
		if err := plugin.InstallPlugin(pluginDir, args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Install error: %v\n", err)
			return true, 1
		}
		return true, 0

	case "--plugin-update":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: numentext --plugin-update <name>")
			return true, 1
		}
		if err := plugin.UpdatePlugin(pluginDir, args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Update error: %v\n", err)
			return true, 1
		}
		return true, 0

	case "--plugin-remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: numentext --plugin-remove <name>")
			return true, 1
		}
		if err := plugin.RemovePlugin(pluginDir, args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Remove error: %v\n", err)
			return true, 1
		}
		return true, 0

	case "--plugin-list":
		if err := plugin.ListPlugins(pluginDir); err != nil {
			fmt.Fprintf(os.Stderr, "List error: %v\n", err)
			return true, 1
		}
		return true, 0

	case "--plugin-search":
		keyword := ""
		if len(args) >= 2 {
			keyword = strings.Join(args[1:], " ")
		}
		registryURL := cfg.PluginRegistryURL
		if err := plugin.SearchPlugins(registryURL, keyword); err != nil {
			fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
			return true, 1
		}
		return true, 0
	}

	return false, 0
}
