package json

import (
	"path/filepath"
	"testing"
)

func TestSyncDir(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		if err := syncDir(t.TempDir()); err != nil {
			t.Errorf("syncDir() = %v, want nil", err)
		}
	})

	t.Run("missing directory", func(t *testing.T) {
		t.Parallel()
		if err := syncDir(filepath.Join(t.TempDir(), "missing")); err == nil {
			t.Error("syncDir() = nil, want an error")
		}
	})
}
