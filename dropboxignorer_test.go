package main_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	main "github.com/anton15x/dropbox_ignore_service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLog struct {
	t *testing.T
}

func NewTestLogger(t *testing.T) *log.Logger {
	// return log.Default()
	return log.New(NewTestLog(t), t.Name(), log.LstdFlags)
}

func NewTestLog(t *testing.T) *testLog {
	return &testLog{
		t: t,
	}
}

func (l *testLog) Write(p []byte) (n int, err error) {
	l.t.Log(strings.TrimSuffix(string(p), "\n"))
	return len(p), nil
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
	t                      *testing.T
	m                      map[string]bool
	i                      *main.DropboxIgnorer
	ignoredPathsChan       <-chan string
	ignoredPathsChanRemove <-chan string
}

func NewFileTester(t *testing.T, i *main.DropboxIgnorer) *fileTester {
	ignoredPathsChan := make(chan string, 1000)
	i.IgnoredPathsSet().AddAddEventListener(func(s string) {
		ignoredPathsChan <- s
	})
	ignoredPathsChanRemove := make(chan string, 1000)
	i.IgnoredPathsSet().AddRemoveEventListener(func(s string) {
		ignoredPathsChanRemove <- s
	})
	i.ListenForEvents()

	time.Sleep(time.Second)

	return &fileTester{
		t:                      t,
		m:                      make(map[string]bool),
		i:                      i,
		ignoredPathsChan:       ignoredPathsChan,
		ignoredPathsChanRemove: ignoredPathsChanRemove,
	}
}

func (f *fileTester) Remove(path string) {
	isIgnored := f.m[path]
	err := os.Remove(path)
	if err != nil {
		f.t.Logf("remove failed, try again after a short sleep: %s", err)
		time.Sleep(5 * time.Second)
		err = os.Remove(path)
	}
	requireNoError(f.t, err)
	delete(f.m, path)

	if isIgnored {
		f.t.Logf("waiting for folder remove event of %s", path)
		val := readChanTimeout(f.t, f.ignoredPathsChanRemove, 10*time.Second, path)
		require.Equal(f.t, path, val)
	} else if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		time.Sleep(5 * time.Second)
	}
}

func (f *fileTester) Mkdir(path string, isIgnored bool) {
	requireMkdir(f.t, path)

	// TODO: fast creating folders lead to missing folder change events
	if !isIgnored && (runtime.GOOS == "darwin" || runtime.GOOS == "linux") {
		time.Sleep(5 * time.Second)
	}

	f.EditFileStatus(path, isIgnored)
}

func (f *fileTester) CheckOfPreInit(path string, isIgnored bool) {
	f.m[path] = isIgnored

	f.checkFile(path, isIgnored)
}

func (f *fileTester) EditFileStatus(path string, isIgnored bool) {
	oldStatus := f.m[path]
	f.m[path] = oldStatus || isIgnored

	if isIgnored && !oldStatus {
		f.t.Logf("waiting for folder add event of %s", path)
		val := readChanTimeout(f.t, f.ignoredPathsChan, 20*time.Second, path)
		require.Equal(f.t, path, val)
	} else if !isIgnored && oldStatus {
		hasFlag, err := main.HasDropboxIgnoreFlag(path)
		requireNoError(f.t, err)
		if hasFlag {
			f.t.Logf("waiting for folder remove event of %s", path)
			val := readChanTimeout(f.t, f.ignoredPathsChanRemove, 20*time.Second, path)
			require.Equal(f.t, path, val)
		}
	}

	f.CheckFile(path)

}

func (f *fileTester) WaitForFileAddEvents(paths []string) {
	for len(paths) > 0 {
		pathsStr := strings.Join(paths, ", ")
		f.t.Logf("waiting for folder add event for any of %s", pathsStr)
		val := readChanTimeout(f.t, f.ignoredPathsChan, 20*time.Second, pathsStr)
		f.t.Logf("got folder add event: %s", val)

		i := slices.Index(paths, val)
		require.True(f.t, i >= 0, val)

		paths = slices.Delete(paths, i, i+1)
	}
}

func (f *fileTester) WaitForFileRemoveEvents(paths []string) {
	for len(paths) > 0 {
		pathsStr := strings.Join(paths, ", ")
		f.t.Logf("waiting for folder remove event for any of %s", pathsStr)
		val := readChanTimeout(f.t, f.ignoredPathsChanRemove, 20*time.Second, pathsStr)
		f.t.Logf("got folder remove event: %s", val)

		i := slices.Index(paths, val)
		require.True(f.t, i >= 0, val)

		paths = slices.Delete(paths, i, i+1)
	}
}

func (f *fileTester) EditFileStatuses(pathIsIgnoredMap map[string]bool) {
	paths := []string{}
	oldPaths := []string{}
	for path, isIgnored := range pathIsIgnoredMap {
		oldIgnored := f.m[path]
		f.m[path] = oldIgnored || isIgnored
		if isIgnored && !oldIgnored {
			paths = append(paths, path)
		} else if !isIgnored && oldIgnored {
			hasFlag, err := main.HasDropboxIgnoreFlag(path)
			requireNoError(f.t, err)
			if hasFlag {
				oldPaths = append(oldPaths, path)
			}
		}
	}

	f.WaitForFileAddEvents(paths)
	f.WaitForFileRemoveEvents(oldPaths)

	for path := range pathIsIgnoredMap {
		f.CheckFile(path)
	}
}

func (f *fileTester) Rename(oldPath, path string, isIgnored bool, subFoldersIsIgnored map[string]bool) {
	err := os.Rename(oldPath, path)
	if err != nil {
		f.t.Logf("rename failed, try again after a short sleep: %s", err)
		time.Sleep(5 * time.Second)
		err = os.Rename(oldPath, path)
	}
	requireNoError(f.t, err)

	m := map[string]bool{}

	waitForRemoveEvents := []string{}
	oldPathWithoutSeparatorSuffix := strings.TrimSuffix(oldPath, string(filepath.Separator))
	oldPathWithSeparatorSuffix := oldPathWithoutSeparatorSuffix + string(filepath.Separator)
	for oldPath, oldPathIsIgnored := range f.m {
		if !strings.HasPrefix(oldPath, oldPathWithSeparatorSuffix) && oldPath != oldPathWithoutSeparatorSuffix {
			continue
		}

		newPath := filepath.Join(path, strings.TrimPrefix(oldPath, oldPathWithoutSeparatorSuffix))

		delete(f.m, oldPath)

		newIgnored, ok := subFoldersIsIgnored[newPath]
		if !ok {
			require.Equal(f.t, path, newPath, "only_path_is_not_in_subFoldersIsIgnored_map")
			newIgnored = isIgnored
		}

		if oldPathIsIgnored {
			waitForRemoveEvents = append(waitForRemoveEvents, oldPath)
			if !newIgnored {
				f.m[newPath] = true
			} else {
				m[newPath] = true
			}
		} else {
			m[newPath] = newIgnored
		}
	}

	f.WaitForFileRemoveEvents(waitForRemoveEvents)
	f.EditFileStatuses(m)

	f.Check()
}

func (f *fileTester) Check() {
	for path, expectedIsIgnored := range f.m {
		f.checkFile(path, expectedIsIgnored)
	}
}

func (f *fileTester) CheckNoPendingEvents() {
	f.checkNoPendingEvents(false)
}

func (f *fileTester) CheckNoPendingEventsAfterCtxCancelWgWait() {
	f.checkNoPendingEvents(true)
}

func (f *fileTester) checkNoPendingEvents(allowRoot bool) {
	// channels should be empty now:
	for ok := true; ok; {
		select {
		case p := <-f.ignoredPathsChan:
			if allowRoot {
				// on macOS the root folder gets notified as an event after closing
				require.Equal(f.t, f.i.DropboxPath(), p)
				continue
			}
			assert.Fail(f.t, "expected no additional events but got:", p)
		case p := <-f.ignoredPathsChanRemove:
			assert.Fail(f.t, "expected no additional remove events but got:", p)
		default:
			ok = false
		}
	}
}

func (f *fileTester) CheckFile(path string) {
	expectedIsIgnored, ok := f.m[path]
	require.True(f.t, ok)
	f.checkFile(path, expectedIsIgnored)
}

func (f *fileTester) checkFile(path string, expectedIsIgnored bool) {
	f.t.Logf("checking dropbox ignore flag for path %s", path)
	isIgnored, err := main.HasDropboxIgnoreFlag(path)
	requireNoError(f.t, err)
	require.Equal(f.t, expectedIsIgnored && !f.i.TryRun(), isIgnored, path)
}

func readChanTimeout[T any](t *testing.T, c <-chan T, duration time.Duration, format string, a ...any) T {
	val, ok := readChanTimeoutWithOk(t, c, duration, format, a...)
	require.True(t, ok)
	return val
}

func readChanTimeoutWithOk[T any](t *testing.T, c <-chan T, duration time.Duration, format string, a ...any) (T, bool) {
	select {
	case val, ok := <-c:
		return val, ok
	case <-time.After(duration):
		if len(a) > 0 {
			format = fmt.Sprintf(format, a...)
		}
		t.Errorf("read chan timeout of %s reached: %s", duration.String(), format)
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
			name:          "initial_create_normal",
			initialCreate: true,
			tryRun:        false,
		},
		{
			name:          "watch_only_normal",
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

				logger := NewTestLogger(t)

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
				i, err := main.NewDropboxIgnorer(dropboxDir, testVariant.tryRun, logger, ctx, &wg, ignoredPathsSet, ignoreFiles)
				requireNoError(t, err)
				wg.Wait()

				ft := NewFileTester(t, i)

				if testVariant.initialCreate {
					for _, folder := range test.folders {
						ft.CheckOfPreInit(folder.path, folder.ignored)
					}
				}

				i.ListenForEvents()

				if testVariant.initialCreate {
					for i := len(test.folders) - 1; i >= 0; i-- {
						folder := test.folders[i]

						ft.Remove(folder.path)
					}
				}

				for _, folder := range test.folders {
					ft.Mkdir(folder.path, folder.ignored)
				}

				ft.Check()

				ft.CheckNoPendingEvents()

				ctxCancel()
				wg.Wait()

				ft.CheckNoPendingEventsAfterCtxCancelWgWait()
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
				ft.Remove(filepath.Join(root, main.DropboxIgnoreFilename))
				wg.Wait()

				ft.Mkdir(filepath.Join(root, "my_project2"), false)
				ft.Mkdir(filepath.Join(root, "my_project2", "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "node_modules"), false)
			},
		},
		{
			name: "ignore_file_in_subfolder_renamed",
			edit: func(t *testing.T, root string, ft *fileTester) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "node_modules")
				ft.Mkdir(filepath.Join(root, "my_project"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "node_modules"), true)
				ft.Mkdir(filepath.Join(root, "my_project", "target"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "src"), false)
				createDropboxignore(t, filepath.Join(root, "my_project", main.DropboxIgnoreFilename), "/target")
				ft.EditFileStatus(filepath.Join(root, "my_project", "target"), true)

				var wg sync.WaitGroup
				wg.Add(1)
				called := 0
				ft.i.IgnoreFiles().AddRemoveEventListener(func(s string) {
					require.Equal(t, filepath.Join(root, "my_project", main.DropboxIgnoreFilename), s)
					require.Equal(t, 0, called)
					called++
					wg.Done()
				})
				ft.Rename(filepath.Join(root, "my_project"), filepath.Join(root, "my_project2"), false, map[string]bool{
					filepath.Join(root, "my_project2", "node_modules"): true,
					filepath.Join(root, "my_project2", "target"):       true,
					filepath.Join(root, "my_project2", "src"):          false,
				})
				wg.Wait()

				// the ignore file should be removed
				// =>  recreate project folder structure without dropboxignore in there should not ignore target
				ft.Mkdir(filepath.Join(root, "my_project"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "node_modules"), true)
				ft.Mkdir(filepath.Join(root, "my_project", "target"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "src"), false)
			},
		},
		{
			name: "path_renamed_gets_ignored",
			edit: func(t *testing.T, root string, ft *fileTester) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "/my_project2")
				ft.Mkdir(filepath.Join(root, "my_project"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "node_modules"), false)
				ft.Rename(filepath.Join(root, "my_project"), filepath.Join(root, "my_project2"), true, map[string]bool{
					filepath.Join(root, "my_project2", "node_modules"): false,
				})
			},
		},
		{
			name: "path_renamed_un_ignored",
			edit: func(t *testing.T, root string, ft *fileTester) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "/my_project*")
				ft.Mkdir(filepath.Join(root, "my_project"), true)
				ft.Mkdir(filepath.Join(root, "my_project", "a1"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "a2"), false)
				ft.Mkdir(filepath.Join(root, "my_project5"), true)
				ft.Mkdir(filepath.Join(root, "my_project5", "a1"), false)
				ft.Mkdir(filepath.Join(root, "my_project5", "a2"), false)
				ft.Rename(filepath.Join(root, "my_project"), filepath.Join(root, "not_my_project"), false, map[string]bool{
					filepath.Join(root, "not_my_project", "a1"): false,
					filepath.Join(root, "not_my_project", "a2"): false,
				})
			},
		},
		{
			name: "path_renamed_sill_ignored",
			edit: func(t *testing.T, root string, ft *fileTester) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "/my_project*")
				ft.Mkdir(filepath.Join(root, "my_project"), true)
				ft.Mkdir(filepath.Join(root, "my_project", "a1"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "a2"), false)
				ft.Mkdir(filepath.Join(root, "my_project5"), true)
				ft.Mkdir(filepath.Join(root, "my_project5", "a1"), false)
				ft.Mkdir(filepath.Join(root, "my_project5", "a2"), false)
				ft.Rename(filepath.Join(root, "my_project"), filepath.Join(root, "my_project2"), true, map[string]bool{
					filepath.Join(root, "my_project2", "a1"): false,
					filepath.Join(root, "my_project2", "a2"): false,
				})
			},
		},
		{
			name: "path_renamed_with_subfolder_gets_ignored",
			edit: func(t *testing.T, root string, ft *fileTester) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "/my_project2/node_modules")
				ft.Mkdir(filepath.Join(root, "my_project"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "node_modules"), false)
				ft.Mkdir(filepath.Join(root, "node_modules"), false)
				ft.Rename(filepath.Join(root, "my_project"), filepath.Join(root, "my_project2"), false, map[string]bool{
					filepath.Join(root, "my_project2", "node_modules"): true,
				})
			},
		},
		{
			name: "path_renamed_with_multiple_subfolder_now_ignored",
			edit: func(t *testing.T, root string, ft *fileTester) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "/my_project2/**/a?")
				ft.Mkdir(filepath.Join(root, "my_project"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "a1"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "a2"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "a3"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "b1"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "b1", "a1"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "b1", "b1"), false)
				ft.Mkdir(filepath.Join(root, "node_modules"), false)
				ft.Rename(filepath.Join(root, "my_project"), filepath.Join(root, "my_project2"), false, map[string]bool{
					filepath.Join(root, "my_project2", "a1"):       true,
					filepath.Join(root, "my_project2", "a2"):       true,
					filepath.Join(root, "my_project2", "a3"):       true,
					filepath.Join(root, "my_project2", "b1"):       false,
					filepath.Join(root, "my_project2", "b1", "a1"): true,
					filepath.Join(root, "my_project2", "b1", "b1"): false,
				})
			},
		},
		{
			name: "path_renamed_with_subfolder_un_ignored",
			edit: func(t *testing.T, root string, ft *fileTester) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "/my_project*/**/a?")
				ft.Mkdir(filepath.Join(root, "my_project"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "a1"), true)
				ft.Mkdir(filepath.Join(root, "my_project", "a2"), true)
				ft.Mkdir(filepath.Join(root, "my_project5"), false)
				ft.Mkdir(filepath.Join(root, "my_project5", "a1"), true)
				ft.Mkdir(filepath.Join(root, "my_project5", "a2"), true)
				ft.Rename(filepath.Join(root, "my_project"), filepath.Join(root, "not_my_project"), false, map[string]bool{
					filepath.Join(root, "not_my_project", "a1"): false,
					filepath.Join(root, "not_my_project", "a2"): false,
				})
			},
		},
		{
			name: "path_renamed_with_subfolder_sill_ignored",
			edit: func(t *testing.T, root string, ft *fileTester) {
				createDropboxignore(t, filepath.Join(root, main.DropboxIgnoreFilename), "/my_project*/**/a?")
				ft.Mkdir(filepath.Join(root, "my_project"), false)
				ft.Mkdir(filepath.Join(root, "my_project", "a1"), true)
				ft.Mkdir(filepath.Join(root, "my_project", "a2"), true)
				ft.Mkdir(filepath.Join(root, "my_project", "b1"), false)
				ft.Mkdir(filepath.Join(root, "my_project5"), false)
				ft.Mkdir(filepath.Join(root, "my_project5", "a1"), true)
				ft.Mkdir(filepath.Join(root, "my_project5", "a2"), true)
				ft.Rename(filepath.Join(root, "my_project"), filepath.Join(root, "my_project2"), false, map[string]bool{
					filepath.Join(root, "my_project2", "a1"): true,
					filepath.Join(root, "my_project2", "a2"): true,
					filepath.Join(root, "my_project2", "b1"): false,
				})
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

			logger := NewTestLogger(t)

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

			ft.CheckNoPendingEvents()

			ctxCancel()
			wg.Wait()

			ft.CheckNoPendingEventsAfterCtxCancelWgWait()
		})
	}
}
