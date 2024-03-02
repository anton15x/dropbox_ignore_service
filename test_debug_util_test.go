package main_test

import (
	"io/fs"
	"path/filepath"
	"testing"
)

//lint:ignore U1000 Ignore unused function

func printFileTree(t *testing.T, root string) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		requireNoError(t, err)

		if d.IsDir() {
			t.Logf("d-%s", path)
		} else {
			t.Logf("f-%s", path)
		}

		return nil
	})
	requireNoError(t, err)
}
