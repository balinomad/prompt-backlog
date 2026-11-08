package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/balinomad/prompt-backlog/internal/service"
	"github.com/balinomad/prompt-backlog/internal/store"
	"github.com/balinomad/prompt-backlog/internal/store/json"
)

const jsonExt = ".json"

const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

var errUsage = errors.New("usage")

func main() {
	if code := run(os.Args[1:], os.Stderr); code != exitSuccess {
		os.Exit(code)
	}
}

// run executes the application and returns a process exit code.
func run(args []string, stderr io.Writer) int {
	err := runApp(args, stderr)
	if err == nil {
		return exitSuccess
	}
	if errors.Is(err, errUsage) {
		return exitUsage
	}
	//nolint:errcheck // A stderr write failure cannot change the exit code.
	fmt.Fprintf(stderr, "%s: %v\n", AppName, err)
	return exitFailure
}

// runApp parses args and runs the application until shutdown.
func runApp(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet(AppName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var dataPath string
	fs.StringVar(&dataPath, "data", "", "path to the data `file` (.json)")
	fs.StringVar(&dataPath, "d", "", "shorthand for -data")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %w", errUsage, err)
	}
	if dataPath == "" {
		//nolint:errcheck // A stderr write failure cannot change the exit code.
		fmt.Fprintf(stderr, "%s: -data flag is required\n", AppName)
		fs.Usage()
		return errUsage
	}

	s, err := openStore(dataPath)
	if err != nil {
		return err
	}

	svc, err := service.New(s)
	if err != nil {
		return err
	}

	app, err := New(svc)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil {
		return fmt.Errorf("run app: %w", err)
	}
	return nil
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
