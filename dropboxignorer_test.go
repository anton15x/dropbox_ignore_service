package main_test

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	main "github.com/anton15x/dropbox_ignore_service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEnvironment(t *testing.T) (string, *log.Logger, context.Context) {
	tmpDir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)

	// TODO: test dependent instead of global log?
	l := log.Default()

	return tmpDir, l, ctx
}

/**
 * checks is tow filepaths are equal
 *
 * github acitons fails on macOS because of this error:
 * --- FAIL: TestDropboxIgnorerListenEvents/base_name_and_subfolder_variant_watch_only (0.17s)
 *
 * 	dropboxignorer_test.go:227:
 * 	    	Error Trace:	/Users/runner/work/dropbox_ignore_service/dropbox_ignore_service/dropboxignorer_test.go:227
 * 	    	Error:      	Not equal:
 * 	    	            	expected: "/var/folders/qv/pdh5wsgn0lq3dp77zj602b5c0000gn/T/TestDropboxIgnorerListenEventsbase_name_and_subfolder_variant_watch_only2776636403/001/my_project/node_modules"
 * 	    	            	actual  : "/private/var/folders/qv/pdh5wsgn0lq3dp77zj602b5c0000gn/T/TestDropboxIgnorerListenEventsbase_name_and_subfolder_variant_watch_only2776636403/001/my_project/node_modules"
 */
func equalFilePaths(t *testing.T, dropboxDir, expected, got string) {
	if expected != got {
		expectedRel, err := filepath.Rel(dropboxDir, expected)
		require.Nil(t, err)
		gotRel, err := filepath.Rel(dropboxDir, got)
		require.Nil(t, err)
		if expectedRel == gotRel {
			expected = expectedRel
			got = gotRel
			t.Logf("equalFilePaths filepath.Rel to dropboxDir equal for expected: %s and got: %s", expected, got)
		} else {
			expectedStat, err := os.Stat(expected)
			require.Nil(t, err)
			gotStat, err := os.Stat(got)
			require.Nil(t, err)
			if os.SameFile(expectedStat, gotStat) {
				got = expected
				t.Logf("equalFilePaths os.SameFile equal for expected: %s and got: %s", expected, got)
			}
		}
	}
	require.Equal(t, expected, got)
}

func createDropboxignore(t *testing.T, filename string, patterns ...string) {
	data := []byte(strings.Join(patterns, "\n"))
	err := os.WriteFile(filename, data, os.ModePerm)
	require.Nil(t, err)
}

type fileTester struct {
	t *testing.T
	m map[string]bool
}

func NewFileTester(t *testing.T) *fileTester {
	return &fileTester{
		t: t,
		m: make(map[string]bool),
	}
}

func (f *fileTester) Mkdir(path string, isIgnored bool) {
	require.Nil(f.t, os.Mkdir(path, os.ModePerm))
	f.m[path] = isIgnored
}

func (f *fileTester) Check() {
	for path, expectedIsIgnored := range f.m {
		isIgnored, err := main.HasDropboxIgnoreFlag(path)
		require.Nil(f.t, err)
		require.Equal(f.t, expectedIsIgnored, isIgnored, path)
	}
}

func readChanTimeout[T any](t *testing.T, c chan T, duration time.Duration) (T, bool) {
	select {
	case val, ok := <-c:
		return val, ok
	case <-time.After(duration):
		t.Errorf("read chan timeout")
		panic("linter fix")
	}
}

func TestDropboxIgnorerListenEvents(t *testing.T) {
	type iTestFolder struct {
		path    string
		ignored bool
	}
	tests := []struct {
		name    string
		tryRun  bool
		prepare func(t *testing.T, root string)
		folders []*iTestFolder
	}{
		{
			name: "base_name",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "node_modules")
			},
			folders: []*iTestFolder{
				{filepath.Join("node_modules"), true},
				{filepath.Join("keep"), false},
				{filepath.Join("keep", "keep"), false},
				{filepath.Join("keep", "node_modules"), true},
				{filepath.Join("keep", "node_modules", "node_modules"), false},
			},
		},
		{
			name: "base_name_and_subfolder",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "my_project/node_modules")
			},
			folders: []*iTestFolder{
				{filepath.Join("node_modules"), false},
				{filepath.Join("my_project"), false},
				{filepath.Join("my_project", "src"), false},
				{filepath.Join("my_project", "node_modules"), true},
				{filepath.Join("keep"), false},
				{filepath.Join("keep", "node_modules"), false},
				{filepath.Join("keep", "my_project"), false},
				{filepath.Join("keep", "my_project", "node_modules"), true},
				{filepath.Join("keep", "my_project", "node_modules", "my_project"), false},
				{filepath.Join("keep", "my_project", "node_modules", "my_project", "node_modules"), false},
			},
		},
		{
			name: "pattern_root_folder",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "/node_modules")
			},
			folders: []*iTestFolder{
				{filepath.Join("node_modules"), true},
				{filepath.Join("keep"), false},
				{filepath.Join("keep", "keep"), false},
				{filepath.Join("keep", "node_modules"), false},
			},
		},
		{
			name: "pattern_root_with_subfolder",
			prepare: func(t *testing.T, root string) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "/my_project/node_modules")
			},
			folders: []*iTestFolder{
				{filepath.Join("my_project"), false},
				{filepath.Join("my_project", "src"), false},
				{filepath.Join("my_project", "node_modules"), true},
				{filepath.Join("subfolder"), false},
				{filepath.Join("subfolder", "my_project"), false},
				{filepath.Join("subfolder", "my_project", "node_modules"), false},
				{filepath.Join("subfolder", "node_modules"), false},
			},
		},
	}

	// TODO: test matrix?
	testVariants := []struct {
		name          string
		initialCreate bool
		tryRun        bool
	}{
		{
			name:          "initial_create",
			initialCreate: true,
			tryRun:        false,
		},
		{
			name:          "watch_only",
			initialCreate: false,
			tryRun:        false,
		},
		{
			name:          "initial_create_try_run",
			initialCreate: true,
			tryRun:        true,
		},
		{
			name:          "watch_only_try_run",
			initialCreate: false,
			tryRun:        true,
		},
	}

	for _, test := range tests {
		for _, testVariant := range testVariants {
			test := test
			testVariant := testVariant
			t.Run(test.name+"_variant_"+testVariant.name, func(t *testing.T) {
				t.Parallel()

				dropboxDir, logger, ctx := setupTestEnvironment(t)
				ctxCancelAble, ctxCancel := context.WithTimeout(ctx, 60*time.Second)
				defer ctxCancel()

				test.prepare(t, dropboxDir)

				folders := make([]*iTestFolder, len(test.folders))
				for i, folder := range test.folders {
					f := *folder
					f.path = filepath.Join(dropboxDir, folder.path)
					folders[i] = &f
				}
				test.folders = folders

				if testVariant.initialCreate {
					for _, folder := range test.folders {
						require.Nil(t, os.Mkdir(folder.path, os.ModePerm))
					}
				}

				if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
					time.Sleep(time.Second)
				}

				var wg sync.WaitGroup
				i, err := main.NewDropboxIgnorer(dropboxDir, test.tryRun, logger, ctxCancelAble, &wg, main.NewSortedStringSet(), main.NewSortedStringSet())
				require.Nil(t, err)
				wg.Wait()

				if testVariant.initialCreate {
					for _, folder := range test.folders {
						isIgnored, err := main.HasDropboxIgnoreFlag(folder.path)
						require.Nil(t, err)
						require.Equal(t, folder.ignored && !test.tryRun, isIgnored, folder.path)
					}
				}

				ignoredFilesChan := make(chan string, 5)
				i.ListenForEvents(ignoredFilesChan)

				if runtime.GOOS == "linux" {
					time.Sleep(time.Second)
				}

				if testVariant.initialCreate {
					for i := len(test.folders) - 1; i >= 0; i-- {
						folder := test.folders[i]
						require.Nil(t, os.Remove(folder.path))
					}
				}

				if runtime.GOOS == "linux" {
					time.Sleep(time.Second)
				}

				for _, folder := range test.folders {
					require.Nil(t, os.Mkdir(folder.path, os.ModePerm))
					// TODO: fast creating folders lead to missing folder change events on linux
					if runtime.GOOS == "linux" {
						time.Sleep(time.Second)
					}

					if folder.ignored {
						log.Printf("waiting for folder create event of %s", folder.path)
						equalFilePaths(t, dropboxDir, folder.path, <-ignoredFilesChan)
						isIgnored, err := main.HasDropboxIgnoreFlag(folder.path)
						require.Nil(t, err)
						require.Equal(t, folder.ignored && !test.tryRun, isIgnored, folder.path)
					}
				}

				for _, folder := range test.folders {
					isIgnored, err := main.HasDropboxIgnoreFlag(folder.path)
					require.Nil(t, err)
					require.Equal(t, folder.ignored && !test.tryRun, isIgnored, folder.path)
				}

				// c should be empty now:
				for ok := true; ok; {
					select {
					case p := <-ignoredFilesChan:
						assert.Fail(t, "expected no additional events but got:", p)
					default:
						ok = false
					}
				}

				ctxCancel()
				wg.Wait()

				// c should still be empty:
				for ok := true; ok; {
					select {
					case p := <-ignoredFilesChan:
						// on macOS the root folder gets notified as an event after closing
						equalFilePaths(t, dropboxDir, dropboxDir, p)
					default:
						ok = false
					}
				}
			})
		}
	}
}
