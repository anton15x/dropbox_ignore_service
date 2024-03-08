//go:build !windows

package fsnotify

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	Events <-chan Event
	Errors <-chan error

	w *fsnotify.Watcher
}

type Event struct {
	Name string
	Op   Op
}

type Op fsnotify.Op

const (
	Create Op = Op(fsnotify.Create)
	Remove Op = Op(fsnotify.Remove)
	Write  Op = Op(fsnotify.Write)
	Rename Op = Op(fsnotify.Rename)

	All Op = Create | Remove | Write | Rename
)

func (e *Op) String() string {
	return fsnotify.Op(*e).String()
}
func (e *Op) Has(h Op) bool {
	return (*e)&h != 0
}

func GetFileNameEndingWithSeparator(path string) string {
	if !strings.HasSuffix(path, string(filepath.Separator)) {
		path += string(filepath.Separator)
	}
	return path
}

func NewWatcherRecursive(rootPath string) (*Watcher, error) {
	w, err := fsnotify.NewBufferedWatcher(1000)
	if err != nil {
		return nil, fmt.Errorf("error creating watcher: %w", err)
	}

	f := make(chan Event, 1000)
	errChan := make(chan error, 1000)
	var errWg sync.WaitGroup

	watchedPaths := map[string]interface{}{}
	addPath := func(path string) error {
		_, ok := watchedPaths[path]
		if ok {
			return nil
		}

		err := w.Add(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}

			return err
		}
		watchedPaths[path] = nil

		return nil
	}
	addPathRecursive := func(path string) error {
		err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				err = addPath(path)
				if err != nil {
					return fmt.Errorf("error adding path %s to watcher: %w", path, err)
				}
			}

			return nil
		})
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		return nil
	}
	removePathSingle := func(path string) error {
		err := w.Remove(path)
		if err != nil && !errors.Is(err, fsnotify.ErrNonExistentWatch) {
			return err
		}
		watchedPaths[path] = nil
		return nil
	}
	removePathRecursive := func(path string) error {
		_, ok := watchedPaths[path]
		if ok {
			err := removePathSingle(path)
			if err != nil {
				return err
			}
		}

		pathWithSeparator := GetFileNameEndingWithSeparator(path)

		for key := range watchedPaths {
			if strings.HasPrefix(key, pathWithSeparator) {
				err := removePathSingle(key)
				if err != nil {
					return err
				}
			}
		}

		return nil
	}

	err = addPathRecursive(rootPath)
	if err != nil {
		return nil, fmt.Errorf("error walking path %s: %w", rootPath, err)
	}

	rootPathWithSeparator := GetFileNameEndingWithSeparator(rootPath)

	errWg.Add(1)
	go func() {
		defer errWg.Done()
		for {
			val, ok := <-w.Events
			if ok {
				e := Event{
					Name: val.Name,
					Op:   Op(val.Op),
				}

				if e.Op.Has(Create) || e.Op.Has(Rename) || e.Op.Has(Remove) {
					err := removePathRecursive(e.Name)
					if err != nil {
						errChan <- fmt.Errorf("error removing path %s after event %s: %w", e.Name, e.Op.String(), err)
					}
				}
				if (e.Op.Has(Create) || e.Op.Has(Rename) || e.Op.Has(Remove)) && strings.HasPrefix(e.Name, rootPathWithSeparator) {
					// event order cloud be incorrect => try add folder also at remove
					err = addPathRecursive(e.Name)
					if err != nil {
						errChan <- fmt.Errorf("error adding path %s after event %s: %w", e.Name, e.Op.String(), err)
					}
				}
				f <- e
			} else {
				break
			}
		}
		close(f)
	}()

	errWg.Add(1)
	go func() {
		defer errWg.Done()
		for {
			val, ok := <-w.Errors
			if ok {
				errChan <- val
			} else {
				break
			}
		}
	}()
	go func() {
		errWg.Wait()
		close(errChan)
	}()

	return &Watcher{
		Events: f,
		Errors: errChan,

		w: w,
	}, nil
}

func (w *Watcher) Close() error {
	err := w.w.Close()
	if err != nil {
		return fmt.Errorf("error closing watcher: %w", err)
	}

	return nil
}
