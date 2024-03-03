package main_test

import (
	"io/fs"
	"path/filepath"
	"testing"

	main "github.com/anton15x/dropbox_ignore_service"
)

//lint:ignore U1000 Ignore unused function

func printFileTree(t *testing.T, root string) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		requireNoError(t, err)

		hasFlag, err := main.HasDropboxIgnoreFlag(path)
		requireNoError(t, err)
		hasFlagSign := "-"
		if hasFlag {
			hasFlagSign = "i"
		}

		if d.IsDir() {
			t.Logf("d%s-%s", hasFlagSign, path)
		} else {
			t.Logf("f%s-%s", hasFlagSign, path)
		}

		return nil
	})
	requireNoError(t, err)
}

func CheckTestParallel(t *testing.T) {
	t.Parallel()
}

func PrintFileTreeIfTestFailed(t *testing.T, path string) {
	if t.Failed() {
		t.Logf("test failed, printing file tree of %s:", path)
		printFileTree(t, path)
	}
}

func PrintDropboxIgnorerStats(t *testing.T, i *main.DropboxIgnorer) {
	t.Logf("DropboxPath: %s", i.DropboxPath())

	ignoreFiles := i.IgnoreFiles().Values
	t.Logf("ignoreFiles: %d", len(ignoreFiles))
	for i, file := range ignoreFiles {
		t.Logf("ignoreFiles[%d]: %s", i, file)
	}

	ignoredPaths := i.IgnoredPathsSet().Values
	t.Logf("ignoredPaths: %d", len(ignoreFiles))
	for i, file := range ignoredPaths {
		t.Logf("ignoredPaths[%d]: %s", i, file)
	}
}

func PrintDropboxIgnorerStatsIfTestFailed(t *testing.T, i *main.DropboxIgnorer) {
	if t.Failed() {
		t.Logf("test failed, printing dropbox ignorer stats")
		PrintDropboxIgnorerStats(t, i)
	}
}
