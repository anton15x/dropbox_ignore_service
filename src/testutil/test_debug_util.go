package testutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anton15x/dropbox_ignore_service/src/attr"
	"github.com/anton15x/dropbox_ignore_service/src/dropboxignorer"
	"github.com/stretchr/testify/require"
)

//lint:ignore U1000 Ignore unused function

func printFileTree(t *testing.T, root string) {
	t.Helper()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		require.NoError(t, err)

		hasFlag, err := attr.HasDropboxIgnoreFlag(path)
		require.NoError(t, err)
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
	require.NoError(t, err)
}

func CheckTestParallel(t *testing.T) {
	t.Helper()

	if os.Getenv("DISABLE_PARALLEL_TEST") != "" {
		return
	}

	t.Parallel()
}

func CheckTestLarge(t *testing.T) {
	t.Helper()

	largeTestEnvVariable := "ENABLE_LARGE_TESTS"
	if os.Getenv(largeTestEnvVariable) != "" {
		return
	}

	t.Skipf("to run large tests, set environment variable %q to 1 (or any other non empty value)", largeTestEnvVariable)
	t.SkipNow()
}

func PrintFileTreeIfTestFailed(t *testing.T, path string) {
	t.Helper()

	if t.Failed() {
		t.Logf("test failed, printing file tree of %s:", path)
		printFileTree(t, path)
	}
}

func PrintDropboxIgnorerStats(t *testing.T, i *dropboxignorer.DropboxIgnorer) {
	t.Helper()

	t.Logf("DropboxPath: %s", i.DropboxPath())

	ignoreFiles := i.IgnoreFiles().Values()
	t.Logf("ignoreFiles: %d", len(ignoreFiles))
	for i, file := range ignoreFiles {
		t.Logf("ignoreFiles[%d]: %s", i, file)
	}

	ignoredPaths := i.IgnoredPathsSet().Values()
	t.Logf("ignoredPaths: %d", len(ignoreFiles))
	for i, file := range ignoredPaths {
		t.Logf("ignoredPaths[%d]: %s", i, file)
	}
}

func PrintDropboxIgnorerStatsIfTestFailed(t *testing.T, i *dropboxignorer.DropboxIgnorer) {
	t.Helper()

	if t.Failed() {
		t.Logf("test failed, printing dropbox ignorer stats")
		PrintDropboxIgnorerStats(t, i)
	}
}

func RetriedExecute(t *testing.T, name string, f func() error) error {
	err := f()
	if err == nil {
		return nil
	}

	// first retry after a short time, than increase
	// to minify test time
	for _, d := range []time.Duration{time.Second / 5, time.Second, 5 * time.Second} {
		t.Logf("%s failed, retry again after %s (err=%s)", name, d.String(), err)
		time.Sleep(d)

		err = f()
		if err == nil {
			return nil
		}
	}

	return err
}
