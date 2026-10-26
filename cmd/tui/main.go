package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/balinomad/llm-desk/internal/service"
	"github.com/balinomad/llm-desk/internal/store"
	"github.com/balinomad/llm-desk/internal/store/json"
)

const jsonExt = ".json"

func main() {
	var dataPath string
	flag.StringVar(&dataPath, "data", "", "path to the data `file` (.json)")
	flag.StringVar(&dataPath, "d", "", "shorthand for -data")
	flag.Parse()

	if dataPath == "" {
		fmt.Fprintf(os.Stderr, "%s: -data flag is required\n", AppName)
		flag.Usage()
		os.Exit(1)
	}

	s, err := openStore(dataPath)
	if err != nil {
		fatal(err)
	}

	svc, err := service.New(s)
	if err != nil {
		fatal(err)
	}

	app, err := New(svc)
	if err != nil {
		fatal(err)
	}

	if err := app.Run(); err != nil {
		fatal(err)
	}
}

// fatal reports err and terminates the process.
func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", AppName, err)
	os.Exit(1)
}

// openStore returns a Store implementation for the given data file path.
// The storage format is determined from the file extension.
// Supported extensions: .json.
func openStore(path string) (store.Store, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case jsonExt:
		return json.New(path), nil
	default:
		return nil, fmt.Errorf("unsupported data file format %q", ext)
	}
}
