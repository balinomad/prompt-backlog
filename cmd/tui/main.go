package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/balinomad/llm-desk/internal/store"
	"github.com/balinomad/llm-desk/internal/store/json"
)

func main() {
	var dataPath string
	flag.StringVar(&dataPath, "data", "", "path to the data `file` (.json)")
	flag.StringVar(&dataPath, "d", "", "shorthand for -data")
	flag.Parse()

	if dataPath == "" {
		fmt.Fprintf(os.Stderr, "llm-desk: -data flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	s, err := openStore(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "llm-desk: %v\n", err)
		os.Exit(1)
	}

	app, err := New(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "llm-desk: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "llm-desk: %v\n", err)
		os.Exit(1)
	}
}

// openStore returns a Store implementation for the given data file path.
// The storage format is determined from the file extension.
// Supported extensions: .json
func openStore(path string) (store.Store, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return json.New(path), nil
	default:
		return nil, fmt.Errorf("unsupported data file format %q", ext)
	}
}
