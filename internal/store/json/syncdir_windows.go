package json

import (
	"fmt"
	"os"
)

// syncDir verifies that dir exists. Windows does not support flushing a
// directory handle, so the rename is the durable operation.
func syncDir(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("stat %q: %w", dir, err)
	}
	return nil
}
