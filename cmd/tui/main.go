package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/balinomad/llm-desk/internal/store/json"
)

func main() {
	dataPath := dataFilePath()

	app, err := New(json.New(dataPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "llm-desk: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "llm-desk: %v\n", err)
		os.Exit(1)
	}
}

// dataFilePath returns the path to the registry JSON file.
// It follows the XDG Base Directory Specification:
//   - $XDG_DATA_HOME/llm-desk/registry.json  (when XDG_DATA_HOME is set)
//   - $HOME/.local/share/llm-desk/registry.json  (default fallback)
func dataFilePath() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Extremely unlikely; fall back to current directory.
			return filepath.Join(".", "registry.json")
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "llm-desk", "registry.json")
}
