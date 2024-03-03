package main_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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

func CheckTestParallel(t *testing.T) {
	t.Parallel()
}

func MkdirTemp(t *testing.T, rootDir string) string {
	path, err := os.MkdirTemp(rootDir, t.Name())
	require.Nil(t, err)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("test failed, printing file tree of %s:", path)
			printFileTree(t, path)
		}
	})
	return path
}
