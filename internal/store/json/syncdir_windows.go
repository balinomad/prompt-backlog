package json

import (
	"fmt"
	"os"
)

// syncDir verifies that dir exists.
//
// Platform workaround: skip the directory flush on Windows.
// Root cause: Windows does not support flushing a directory handle, so the
// rename is the durable operation.
// Exit condition: remove when the store no longer requires directory sync on
// Windows, or when a supported directory-flush primitive exists.
func syncDir(dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("stat %q: %w", dir, err)
	}
	return nil
}
