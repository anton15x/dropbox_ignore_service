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

/**
 * checks is tow filepaths are equal
 * cspell:disable
 *
 * github acitons fails on macOS because of this error:
 * --- FAIL: TestDropboxIgnorerListenEvents/base_name_and_subfolder_variant_watch_only (0.17s)
 *
 * 	dropboxignorer_test.go:227:
 * 	    	Error Trace:	/Users/runner/work/dropbox_ignore_service/dropbox_ignore_service/dropboxignorer_test.go:227
 * 	    	Error:      	Not equal:
 * 	    	            	expected: "/var/folders/qv/pdh5wsgn0lq3dp77zj602b5c0000gn/T/TestDropboxIgnorerListenEventsbase_name_and_subfolder_variant_watch_only2776636403/001/my_project/node_modules"
 * 	    	            	actual  : "/private/var/folders/qv/pdh5wsgn0lq3dp77zj602b5c0000gn/T/TestDropboxIgnorerListenEventsbase_name_and_subfolder_variant_watch_only2776636403/001/my_project/node_modules"
 * cspell: enable
 */
func equalFilePaths(t *testing.T, dropboxDir, expected, got string) {
	if expected != got {
		expectedRel, err := filepath.Rel(dropboxDir, expected)
		requireNoError(t, err)
		gotRel, err := filepath.Rel(dropboxDir, got)
		requireNoError(t, err)
		if expectedRel == gotRel {
			expected = expectedRel
			got = gotRel
			t.Logf("equalFilePaths filepath.Rel to dropboxDir equal for expected: %s and got: %s", expected, got)
		} else {
			expectedStat, err := os.Stat(expected)
			requireNoError(t, err)
			gotStat, err := os.Stat(got)
			requireNoError(t, err)
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
	if err != nil && os.IsNotExist(err) {
		err = os.Mkdir(filepath.Dir(filename), os.ModePerm)
		requireNoError(t, err)
		err = os.WriteFile(filename, data, os.ModePerm)
	}
	requireNoError(t, err)
}

type fileTester struct {
	t                *testing.T
	m                map[string]bool
	i                *main.DropboxIgnorer
	ignoredFilesChan <-chan string
}

func NewFileTester(t *testing.T, i *main.DropboxIgnorer) *fileTester {
	ignoredFilesChan := make(chan string, 1000)
	i.IgnoredPathsSet().AddAddEventListener(func(s string) {
		ignoredFilesChan <- s
	})
	i.ListenForEvents()

	time.Sleep(time.Second)

	return &fileTester{
		t:                t,
		m:                make(map[string]bool),
		i:                i,
		ignoredFilesChan: ignoredFilesChan,
	}
}

func (f *fileTester) Mkdir(path string, isIgnored bool) {
	requireMkdir(f.t, path)

	f.EditFileStatus(path, isIgnored)
}

func (f *fileTester) EditFileStatus(path string, isIgnored bool) {
	f.m[path] = isIgnored

	if isIgnored {
		val := readChanTimeout(f.t, f.ignoredFilesChan, 10*time.Second, path)
		require.Equal(f.t, path, val)
	}
}

func (f *fileTester) Check() {
	for path, expectedIsIgnored := range f.m {
		isIgnored, err := main.HasDropboxIgnoreFlag(path)
		requireNoError(f.t, err)
		require.Equal(f.t, expectedIsIgnored, isIgnored, path)
	}
}

func readChanTimeout[T any](t *testing.T, c <-chan T, duration time.Duration, errMsg string) T {
	select {
	case val, ok := <-c:
		require.True(t, ok)
		return val
	case <-time.After(duration):
		t.Errorf("read chan timeout of %s reached: %s", duration.String(), errMsg)
		t.FailNow()
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
				// if a slash is st the path, the path is ignore file relative
				{filepath.Join("keep", "my_project", "node_modules"), false},
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

	tmpTestDir := t.TempDir()
	for _, test := range tests {
		for _, testVariant := range testVariants {
			test := test
			testVariant := testVariant
			t.Run(test.name+"_variant_"+testVariant.name, func(t *testing.T) {
				t.Parallel()

				dropboxDir, err := os.MkdirTemp(tmpTestDir, test.name)
				require.Nil(t, err)
				ctx, ctxCancel := context.WithTimeout(context.Background(), time.Minute)
				defer ctxCancel()

				// TODO: test dependent instead of global log?
				logger := log.Default()

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
						requireNoError(t, os.Mkdir(folder.path, os.ModePerm))
					}
				}

				if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
					time.Sleep(3 * time.Second)
				}

				var wg sync.WaitGroup
				ignoredPathsSet := main.NewSortedStringSet()
				ignoreFiles := main.NewSortedStringSet()
				i, err := main.NewDropboxIgnorer(dropboxDir, test.tryRun, logger, ctx, &wg, ignoredPathsSet, ignoreFiles)
				requireNoError(t, err)
				wg.Wait()

				if testVariant.initialCreate {
					for _, folder := range test.folders {
						isIgnored, err := main.HasDropboxIgnoreFlag(folder.path)
						requireNoError(t, err)
						require.Equal(t, folder.ignored && !test.tryRun, isIgnored, folder.path)
					}
				}

				ignoredFilesChan := make(chan string, 2*len(test.folders))
				ignoredPathsSet.AddAddEventListener(func(s string) {
					ignoredFilesChan <- s
				})
				i.ListenForEvents()

				if testVariant.initialCreate {
					for i := len(test.folders) - 1; i >= 0; i-- {
						folder := test.folders[i]
						requireNoError(t, os.Remove(folder.path))
					}
				}

				if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
					time.Sleep(time.Second)
				}

				for _, folder := range test.folders {
					requireNoError(t, os.Mkdir(folder.path, os.ModePerm))
					// TODO: fast creating folders lead to missing folder change events
					if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
						time.Sleep(time.Second)
					}

					if folder.ignored {
						log.Printf("waiting for folder create event of %s", folder.path)

						equalFilePaths(t, dropboxDir, folder.path, readChanTimeout(t, ignoredFilesChan, 30*time.Second, folder.path))
						isIgnored, err := main.HasDropboxIgnoreFlag(folder.path)
						requireNoError(t, err)
						require.Equal(t, folder.ignored && !test.tryRun, isIgnored, folder.path)
					}
				}

				for _, folder := range test.folders {
					isIgnored, err := main.HasDropboxIgnoreFlag(folder.path)
					requireNoError(t, err)
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

func TestDropboxIgnorerIgnoreFileEdit(t *testing.T) {
	type testInfo struct {
		name string
		edit func(t *testing.T, root string, ft *fileTester)
	}
	tests := []testInfo{
		{
			name: "watch_ignore_file_changes",
			edit: func(t *testing.T, root string, ft *fileTester) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "/my_project")
				ft.Mkdir(filepath.Join(root, "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "my_project"), true)
				ft.Mkdir(filepath.Join(root, "my_project", "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "my_project2"), false)
				ft.Mkdir(filepath.Join(root, "my_project2", "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "my_project3"), false)
				ft.Mkdir(filepath.Join(root, "my_project3", "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "my"), false)
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "/my_project\n/my_project2")
				ft.EditFileStatus(filepath.Join(root, "my_project2"), true)
			},
		},
		{
			name: "watch_ignore_file_changes_slow_write_between lines",
			edit: func(t *testing.T, root string, ft *fileTester) {
				ft.Mkdir(filepath.Join(root, "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "my_project"), false)
				ft.Mkdir(filepath.Join(root, "my_project2"), false)
				ft.Mkdir(filepath.Join(root, "my_project3"), false)
				ft.Mkdir(filepath.Join(root, "my"), false)

				// slow write should no be handled, only after file got closed
				f, err := os.OpenFile(filepath.Join(root, main.DropboxIgnoreFilename), os.O_CREATE|os.O_RDWR|os.O_APPEND, os.ModePerm)
				requireNoError(t, err)
				defer requireCloseFile(t, f)
				requireWriteToFile(t, f, []byte("\nmy_project"))
				err = f.Sync()
				requireNoError(t, err)
				time.Sleep(5 * time.Second)
				requireWriteToFile(t, f, []byte("\nmy_project2"))
				err = f.Close()
				requireNoError(t, err)
				ft.EditFileStatus(filepath.Join(root, "my_project"), true)
				ft.EditFileStatus(filepath.Join(root, "my_project2"), true)
			},
		},
		{
			name: "watch_ignore_file_changes_slow_write_single_line",
			edit: func(t *testing.T, root string, ft *fileTester) {
				t.Skip("not sure if this should be handled or can be handled")
				t.SkipNow()

				ft.Mkdir(filepath.Join(root, "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "my_project"), false)
				ft.Mkdir(filepath.Join(root, "my_project2"), false)
				ft.Mkdir(filepath.Join(root, "my"), false)

				// slow write should no be handled, only after file got closed
				f, err := os.OpenFile(filepath.Join(root, main.DropboxIgnoreFilename), os.O_CREATE|os.O_RDWR|os.O_APPEND, os.ModePerm)
				requireNoError(t, err)
				defer requireCloseFile(t, f)
				requireWriteToFile(t, f, []byte("\nmy"))
				err = f.Sync()
				requireNoError(t, err)
				time.Sleep(5 * time.Second)
				requireWriteToFile(t, f, []byte("_project"))
				err = f.Close()
				requireNoError(t, err)
				ft.EditFileStatus(filepath.Join(root, "my_project"), true)
			},
		},
		{
			name: "ignore_file_is_opened_for_reading",
			edit: func(t *testing.T, root string, ft *fileTester) {
				ft.Mkdir(filepath.Join(root, "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "my_project"), false)
				ft.Mkdir(filepath.Join(root, "my_project2"), false)
				ft.Mkdir(filepath.Join(root, "my"), false)

				var wg sync.WaitGroup
				ft.i.IgnoreFiles().AddAddEventListener(func(s string) {
					wg.Wait()
				})
				wg.Add(1)
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "/node_modules")
				f, err := os.OpenFile(filepath.Join(root, main.DropboxIgnoreFilename), os.O_RDONLY, os.ModePerm)
				requireNoError(t, err)
				defer requireCloseFile(t, f)
				wg.Done()
				ft.EditFileStatus(filepath.Join(root, "node_modules"), true)
				err = f.Close()
				requireNoError(t, err)
			},
		},
		{
			name: "subfolder_ignore_file",
			edit: func(t *testing.T, root string, ft *fileTester) {
				ft.Mkdir(filepath.Join(root, "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "my_project"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "my_project2"), false)
				ft.Mkdir(filepath.Join(root, "my_project2", "node_modules"), false)
				createDropboxignore(t, filepath.Join(root, "my_project2", main.DropboxIgnoreFilename), "/node_modules")
				ft.EditFileStatus(filepath.Join(root, "my_project2", "node_modules"), true)
			},
		},
		{
			name: "subfolder_ignore_file_ignores_subfolder_itself",
			edit: func(t *testing.T, root string, ft *fileTester) {
				createDropboxignore(t, filepath.Join(root, "my_project", main.DropboxIgnoreFilename), "/node_modules")
				ft.Check()
				ft.Mkdir(filepath.Join(root, "my_project", "node_modules"), true)
				ft.Mkdir(filepath.Join(root, "my_project2"), false)
				ft.Mkdir(filepath.Join(root, "my_project2", "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "node_modules"), false)
				createDropboxignore(t, filepath.Join(root, "my_project2", main.DropboxIgnoreFilename), "/")
				ft.EditFileStatus(filepath.Join(root, "my_project2"), true)
			},
		},
		{
			name: "subfolder_ignore_file_ignores_ignore_file",
			edit: func(t *testing.T, root string, ft *fileTester) {
				createDropboxignore(t, filepath.Join(root, "my_project", main.DropboxIgnoreFilename), "/node_modules")
				ft.Check()
				ft.Mkdir(filepath.Join(root, "my_project", "node_modules"), true)
				ft.Mkdir(filepath.Join(root, "my_project2"), false)
				ft.Mkdir(filepath.Join(root, "my_project2", "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "node_modules"), false)
				createDropboxignore(t, filepath.Join(root, "my_project2", main.DropboxIgnoreFilename), "/"+main.DropboxIgnoreFilename)
				ft.EditFileStatus(filepath.Join(root, "my_project2", main.DropboxIgnoreFilename), true)
			},
		},
		{
			name: "ignore_file_removed",
			edit: func(t *testing.T, root string, ft *fileTester) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "node_modules")
				ft.Mkdir(filepath.Join(root, "my_project"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "node_modules"), true)

				var wg sync.WaitGroup
				wg.Add(1)
				called := 0
				ft.i.IgnoreFiles().AddRemoveEventListener(func(s string) {
					require.Equal(t, filepath.Join(root, main.DropboxIgnoreFilename), s)
					require.Equal(t, 0, called)
					called++
					wg.Done()
				})
				requireNoError(t, os.Remove(filepath.Join(root, main.DropboxIgnoreFilename)))
				wg.Wait()

				ft.Mkdir(filepath.Join(root, "my_project2"), false)
				ft.Mkdir(filepath.Join(root, "my_project2", "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "node_modules"), false)
			},
		},
	}

	tmpTestDir := t.TempDir()
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			dropboxDir, err := os.MkdirTemp(tmpTestDir, test.name)
			require.Nil(t, err)
			ctx, ctxCancel := context.WithTimeout(context.Background(), time.Minute)
			defer ctxCancel()

			// TODO: test dependent instead of global log?
			logger := log.Default()

			tryRun := false
			var wg sync.WaitGroup
			ignoredPathsSet := main.NewSortedStringSet()
			ignoreFiles := main.NewSortedStringSet()
			i, err := main.NewDropboxIgnorer(dropboxDir, tryRun, logger, ctx, &wg, ignoredPathsSet, ignoreFiles)
			requireNoError(t, err)
			wg.Wait()

			ft := NewFileTester(t, i)
			test.edit(t, dropboxDir, ft)
			ft.Check()
		})
	}
}
