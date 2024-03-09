package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/anton15x/dropbox_ignore_service/src/fsnotify"
)

const DropboxIgnoreFilename = ".dropboxignore"

type DropboxIgnorer struct {
	dropboxPath string
	tryRun      bool

	ignorePatterns map[string]IgnorePattern
	watcher        *fsnotify.Watcher

	ctx    context.Context
	wg     *sync.WaitGroup
	logger *log.Logger

	ignoreFiles     *SortedStringSet
	ignoredPathsSet *SortedStringSet
}

func NewDropboxIgnorer(dropboxPath string, tryRun bool, logger *log.Logger, ctx context.Context, wg *sync.WaitGroup, ignoredPathsSet *SortedStringSet, ignoreFiles *SortedStringSet) (*DropboxIgnorer, error) {
	dropboxPathAbs, err := filepath.Abs(dropboxPath)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path of %s: %w", dropboxPath, err)
	}
	dropboxPath = dropboxPathAbs

	watcher, err := fsnotify.NewWatcherRecursive(dropboxPath)
	if err != nil {
		return nil, fmt.Errorf("error creating file watcher: %w", err)
	}

	i := &DropboxIgnorer{
		dropboxPath:     dropboxPath,
		tryRun:          tryRun,
		ignorePatterns:  map[string]IgnorePattern{},
		logger:          logger,
		ctx:             ctx,
		wg:              wg,
		watcher:         watcher,
		ignoreFiles:     ignoreFiles,
		ignoredPathsSet: ignoredPathsSet,
	}

	i.logger.Printf("initial walk started for %s", i.dropboxPath)
	err = i.checkDirForIgnore(i.dropboxPath, false)
	if err != nil {
		i.logger.Printf("Error at initial files walk of folder %s: %s", i.dropboxPath, err)
	}
	i.logger.Printf("initial walk finished for %s", i.dropboxPath)

	return i, nil
}

func (i *DropboxIgnorer) IgnoredPathsSet() *SortedStringSet {
	return i.ignoredPathsSet
}
func (i *DropboxIgnorer) IgnoreFiles() *SortedStringSet {
	return i.ignoreFiles
}
func (i *DropboxIgnorer) TryRun() bool {
	return i.tryRun
}
func (i *DropboxIgnorer) DropboxPath() string {
	return i.dropboxPath
}
func (i *DropboxIgnorer) Logger() *log.Logger {
	return i.logger
}

func (i *DropboxIgnorer) checkDirForIgnore(rootPath string, skipRootIgnoreFile bool) error {
	err := filepath.WalkDir(rootPath, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		if err := i.ctx.Err(); err != nil {
			return fmt.Errorf("program is shutting down at file walk: %w", err)
		}

		if info.IsDir() {
			if !skipRootIgnoreFile || path != rootPath {
				_, err := i.addIgnoreFileIfExists(filepath.Join(path, DropboxIgnoreFilename))
				if err != nil {
					i.logger.Printf("error adding ignore file: %s", err)
				}
			}
		}

		if i.ShouldPathGetIgnored(path) {
			err = i.SetIgnoreFlag(path)
			if err != nil {
				i.logger.Printf("Error ignoring dir %s: %s", path, err)
			}

			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("error walking dir %s: %w", rootPath, err)
	}

	return nil
}

func (i *DropboxIgnorer) removeIgnoreFile(ignoreFile string) {
	delete(i.ignorePatterns, filepath.Dir(ignoreFile))
	i.ignoreFiles.Remove(ignoreFile)

	for _, path := range i.ignoreFiles.Values() {
		if !i.ShouldPathGetIgnored(path) {
			i.ignoredPathsSet.Remove(path)
		}
	}

	i.logger.Printf("removed %s file %s", DropboxIgnoreFilename, ignoreFile)
}

func (i *DropboxIgnorer) addIgnoreFileIfExists(ignoreFile string) (bool, error) {
	added, err := i.addIgnoreFile(ignoreFile)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("error reading ignore file %s: %w", ignoreFile, err)
	}

	return added, nil
}

func (i *DropboxIgnorer) addIgnoreFile(ignoreFile string) (bool, error) {
	ignoreFileBytes, err := os.ReadFile(ignoreFile)
	if err != nil {
		return false, err
	}
	i.ignoreFiles.Add(ignoreFile)

	patterns, err := ParseIgnoreFileFromBytes(ignoreFile, ignoreFileBytes)
	if err != nil {
		return false, fmt.Errorf("error parsing ignore file %s: %w", ignoreFile, err)
	}

	oldPatterns := i.ignorePatterns[filepath.Dir(ignoreFile)]
	equal := len(patterns) == len(oldPatterns)
	for i := range patterns {
		if !equal {
			break
		}
		equal = patterns[i] == oldPatterns[i]
	}
	if equal {
		return false, nil
	}

	i.ignorePatterns[filepath.Dir(ignoreFile)] = patterns
	i.logger.Printf("added %s file %s: %+v", DropboxIgnoreFilename, ignoreFile, patterns)

	return true, nil
}

func (i *DropboxIgnorer) ListenForEvents() {
	i.wg.Add(1)
	go func() {
		defer i.wg.Done()

		var listenForEventsWg sync.WaitGroup
		defer func() {
			listenForEventsWg.Wait()
			err := i.watcher.Close()
			if err != nil {
				i.logger.Printf("Error closing watcher: %s", err)
			}
		}()

		listenForEventsWg.Add(1)
		go func() {
			defer listenForEventsWg.Done()

			for {
				select {
				case <-i.ctx.Done():
					return
				case err, ok := <-i.watcher.Errors:
					if !ok {
						return
					}
					i.logger.Printf("watcher error: %s", err)
				}
			}
		}()

		// Block until an event is received.
		for {
			select {
			case <-i.ctx.Done():
				return
			case ei := <-i.watcher.Events:
				i.handleEvent(ei)
			}
		}
	}()
}

func (i *DropboxIgnorer) handleEvent(ei fsnotify.Event) {
	i.logger.Printf("got event: %s %s", ei.Op.String(), ei.Name)
	path := ei.Name
	if !strings.HasPrefix(path, i.dropboxPath) {
		_, after, found := strings.Cut(path, i.dropboxPath)
		if found {
			path = filepath.Join(i.dropboxPath, after)
		} else {
			i.logger.Printf("dropbox %s got event not containing dropbox path: %s", i.dropboxPath, path)
		}
	}

	event := ei.Op
	if event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) || (event.Has(fsnotify.Write) && filepath.Base(path) == DropboxIgnoreFilename) {
		// info: rename event is triggered for both, the new AND old name => stat to check if path exists
		info, err := os.Stat(path)
		if err != nil {
			if !os.IsNotExist(err) {
				i.logger.Printf("stat for path failed: %s", err)
			}
		} else {
			if filepath.Base(path) == DropboxIgnoreFilename {
				added, err := i.addIgnoreFile(path)
				if err != nil {
					i.logger.Printf("Error adding ignore file: %s", err)
				}
				if added {
					err = i.checkDirForIgnore(filepath.Dir(path), true)
					if err != nil && !errors.Is(err, i.ctx.Err()) {
						i.logger.Printf("Error handling ignore file subdirectories of %s: %s", path, err)
					}
				}
			} else if i.ShouldPathGetIgnored(path) {
				err := i.SetIgnoreFlag(path)
				if err != nil {
					i.logger.Printf("Error ignoring dir %s: %s", path, err)
				}
			} else if info.IsDir() {
				// created/renamed directory => check for sub directories
				err = i.checkDirForIgnore(path, false)
				if err != nil && !errors.Is(err, i.ctx.Err()) {
					i.logger.Printf("Error handling ignore file subdirectories of %s: %s", path, err)
				}
			}
		}
	}
	if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
		// sometimes event order is incorrect => stat and check if file is created
		// e.g. fast delete file and create is again could swap the order of events
		_, err := os.Stat(path)
		if err != nil {
			if !os.IsNotExist(err) {
				i.logger.Printf("Error stating file: %s", err)
			} else {
				// remove is single element only
				// rename could cause sub directories to get removed
				// but handle both scenarios es they could have subdirectories
				pathWithSeparatorSuffix := path
				if !strings.HasSuffix(path, string(filepath.Separator)) {
					pathWithSeparatorSuffix += string(filepath.Separator)
				}

				i.ignoredPathsSet.Remove(path)
				for _, subFolderPath := range i.ignoredPathsSet.Values() {
					if strings.HasPrefix(subFolderPath, pathWithSeparatorSuffix) {
						i.ignoredPathsSet.Remove(subFolderPath)
					} else {
						i.logger.Printf("path %s is not a prefix of %s", path, subFolderPath)
					}
				}

				if filepath.Base(path) == DropboxIgnoreFilename {
					i.removeIgnoreFile(path)
				}
				for _, ignoreFile := range i.ignoreFiles.Values() {
					if strings.HasPrefix(ignoreFile, pathWithSeparatorSuffix) {
						i.removeIgnoreFile(ignoreFile)
					}
				}
			}
		}
	}
}

func (i *DropboxIgnorer) SetIgnoreFlag(path string) error {
	if i.IsInsideIgnoreDir(path) {
		i.logger.Printf("dir is already inside ignore dir %s", path)
		return nil
	}

	defer i.ignoredPathsSet.Add(path)
	if i.tryRun {
		i.logger.Printf("tryRun: would ignore dir %s", path)
		return nil
	}
	i.logger.Printf("ignoring dir %s", path)

	hasFlag, err := HasDropboxIgnoreFlag(path)
	if err != nil {
		return fmt.Errorf("error checking if path %s already has ignore flag: %w", path, err)
	}
	if hasFlag {
		// already has flag => do not set again
		return nil
	}
	return SetDropboxIgnoreFlag(path)
}

func (i *DropboxIgnorer) IsInsideIgnoreDir(path string) bool {
	currentDir := path
	for {
		if currentDir == i.dropboxPath {
			return false
		}

		newDir := filepath.Dir(currentDir)
		if newDir == currentDir {
			return false
		}
		currentDir = newDir

		if i.ShouldPathGetIgnored(currentDir) {
			return true
		}
	}
}

func (i *DropboxIgnorer) ShouldPathGetIgnored(path string) bool {
	return i.isPathIgnoredByPattern(path) && !i.IsInsideIgnoreDir(path)
}

func (i *DropboxIgnorer) isPathIgnoredByPattern(path string) bool {
	currentDir := path
	for {
		pattern := i.ignorePatterns[currentDir]
		isIgnored := IsIgnored(pattern, path)
		if isIgnored {
			return true
		}

		newDir := filepath.Dir(currentDir)
		if newDir == currentDir {
			return false
		}
		if currentDir == i.dropboxPath {
			return false
		}
		currentDir = newDir
	}
}
