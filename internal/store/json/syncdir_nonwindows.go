//go:build !windows

package json

import (
	"errors"
	"fmt"
	"os"
)

// syncDir flushes the directory entry for dir to stable storage.
func syncDir(dir string) (err error) {
	d, err := os.Open(dir) //nolint:gosec // dir is derived from the configured store path
	if err != nil {
		return fmt.Errorf("open %q: %w", dir, err)
	}
	defer func() {
		if closeErr := d.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close %q: %w", dir, closeErr))
		}
	}()
	if syncErr := d.Sync(); syncErr != nil {
		return fmt.Errorf("sync %q: %w", dir, syncErr)
	}
	return nil
}
