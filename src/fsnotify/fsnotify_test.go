package fsnotify_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anton15x/dropbox_ignore_service/src/fsnotify"
	"github.com/stretchr/testify/require"
)

func WaitForEvent(t *testing.T, w *fsnotify.Watcher, name string, op fsnotify.Op) fsnotify.Event {
	if op == fsnotify.Rename {
		// on windows, if we rename a/b to c we get a create event for c
		op = op | fsnotify.Create
	}

	t.Logf("waiting for event %s %s", name, op.String())
	for {
		e := <-w.Events
		if e.Op.Has(op) && e.Name == name {
			t.Logf("got event %s %s", e.Op.String(), e.Name)
			return e
		}

		t.Logf("got additional event %s %s", e.Op.String(), e.Name)
	}
}

func TestNewWatcherRecursive(t *testing.T) {
	tmpDir := t.TempDir()

	rootWatchDir, err := os.MkdirTemp(tmpDir, "a")
	require.NoError(t, err)

	w, err := fsnotify.NewWatcherRecursive(rootWatchDir)
	require.NoError(t, err)
	require.NotNil(t, w)

	// crete tests

	require.NoError(t, os.Mkdir(filepath.Join(rootWatchDir, "a"), os.ModePerm))
	WaitForEvent(t, w, filepath.Join(rootWatchDir, "a"), fsnotify.Create)

	require.NoError(t, os.Mkdir(filepath.Join(rootWatchDir, "a", "b"), os.ModePerm))
	WaitForEvent(t, w, filepath.Join(rootWatchDir, "a", "b"), fsnotify.Create)

	require.NoError(t, os.Mkdir(filepath.Join(rootWatchDir, "a", "b", "c"), os.ModePerm))
	WaitForEvent(t, w, filepath.Join(rootWatchDir, "a", "b", "c"), fsnotify.Create)

	// rename tests

	require.NoError(t, os.Rename(filepath.Join(rootWatchDir, "a"), filepath.Join(rootWatchDir, "x")))
	WaitForEvent(t, w, filepath.Join(rootWatchDir, "x"), fsnotify.Rename)

	require.NoError(t, os.Rename(filepath.Join(rootWatchDir, "x", "b"), filepath.Join(rootWatchDir, "z")))
	WaitForEvent(t, w, filepath.Join(rootWatchDir, "z"), fsnotify.Rename)

	require.NoError(t, os.Rename(filepath.Join(rootWatchDir, "z"), filepath.Join(rootWatchDir, "a")))
	WaitForEvent(t, w, filepath.Join(rootWatchDir, "a"), fsnotify.Rename)

	require.NoError(t, os.Rename(filepath.Join(rootWatchDir, "a", "c"), filepath.Join(rootWatchDir, "c")))
	WaitForEvent(t, w, filepath.Join(rootWatchDir, "c"), fsnotify.Rename)

	// remove tests

	require.NoError(t, os.Remove(filepath.Join(rootWatchDir, "a")))
	WaitForEvent(t, w, filepath.Join(rootWatchDir, "a"), fsnotify.Remove)

	require.NoError(t, os.Remove(filepath.Join(rootWatchDir, "c")))
	WaitForEvent(t, w, filepath.Join(rootWatchDir, "c"), fsnotify.Remove)

	require.NoError(t, os.Remove(filepath.Join(rootWatchDir, "x")))
	WaitForEvent(t, w, filepath.Join(rootWatchDir, "x"), fsnotify.Remove)

	err = w.Close()
	require.NoError(t, err)

	err, ok := <-w.Errors
	require.NoError(t, err)
	require.False(t, ok)
}
