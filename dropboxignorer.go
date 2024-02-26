package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rjeczalik/notify"
)

const DropboxIgnoreFilename = ".dropboxignore"

type DropboxIgnorer struct {
	dropboxPath string
	tryRun      bool

	ignorePatterns   IgnorePattern
	modificationChan chan notify.EventInfo

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

	modificationChan := make(chan notify.EventInfo, 1000)

	i := &DropboxIgnorer{
		dropboxPath:      dropboxPath,
		tryRun:           tryRun,
		logger:           logger,
		ctx:              ctx,
		wg:               wg,
		modificationChan: modificationChan,
		ignoreFiles:      ignoreFiles,
		ignoredPathsSet:  ignoredPathsSet,
	}

	// err := notify.Watch(filepath.Join(i.dropboxPath, "..."), i.modificationChan, notify.Create|notify.Rename)
	err = notify.Watch(filepath.Join(i.dropboxPath, "..."), i.modificationChan, notify.Create|notify.Rename|notify.Remove)
	if err != nil {
		return nil, fmt.Errorf("error watching files: %s", err)
	}

	i.logger.Printf("initial walk started for %s", i.dropboxPath)
	err = filepath.WalkDir(i.dropboxPath, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if i.ctx.Err() != nil {
			return fmt.Errorf("program is shutting down before finish initial file walk")
		}

		if i.ShouldPathGetIgnored(path) {
			err = i.SetIgnoreFlag(path)
			if err != nil {
				i.logger.Printf("Error ignoring dir %s: %s", path, err)
			}

			return filepath.SkipDir
		}

		if filepath.Base(path) == DropboxIgnoreFilename {
			err := i.addIgnoreFile(path)
			if err != nil {
				i.logger.Printf("error adding ignore file: %s", err)
			}
		}

		return nil
	})
	if err != nil {
		i.logger.Printf("Error at initial files walk of folder %s: %s", i.dropboxPath, err)
	}
	i.logger.Printf("initial walk finished for %s", i.dropboxPath)

	return i, nil
}

func (i *DropboxIgnorer) removeIgnoreFile(ignoreFile string) {
	if i.ignoreFiles.Has(ignoreFile) && filepath.Join(i.dropboxPath, DropboxIgnoreFilename) != ignoreFile {
		i.logger.Printf("warning: ignoring dropboxignore in subdirectory, not supported yet (found dropboxignore at %s)", ignoreFile)
	}

	i.ignorePatterns = nil
	i.ignoreFiles.Remove(ignoreFile)
}

func (i *DropboxIgnorer) addIgnoreFile(ignoreFile string) error {
	if filepath.Join(i.dropboxPath, DropboxIgnoreFilename) != ignoreFile {
		i.logger.Printf("warning: ignoring dropboxignore in subdirectory, not supported yet (found dropboxignore at %s)", ignoreFile)
		return nil
	}

	i.ignorePatterns = nil
	i.ignoreFiles.Add(ignoreFile)

	patterns, err := ParseIgnoreFile(ignoreFile)
	if err != nil {
		return fmt.Errorf("error parsing ignore file %s: %w", ignoreFile, err)
	}
	i.ignorePatterns = patterns
	i.logger.Printf("added %s file %s: %+v", DropboxIgnoreFilename, ignoreFile, i.ignorePatterns)

	return nil
}

func (i *DropboxIgnorer) ListenForEvents(cb chan string) {
	i.wg.Add(1)
	go func() {
		defer i.wg.Done()
		defer notify.Stop(i.modificationChan)

		// Block until an event is received.
		for {
			select {
			case <-i.ctx.Done():
				return
			case ei := <-i.modificationChan:
				i.logger.Printf("got event: %s %s", ei.Event().String(), ei.Path())
				path := ei.Path()
				if !strings.HasPrefix(path, i.dropboxPath) {
					_, after, found := strings.Cut(path, i.dropboxPath)
					if found {
						path = filepath.Join(i.dropboxPath, after)
					} else {
						i.logger.Printf("dropbox %s got event not containing dropbox path: %s", i.dropboxPath, path)
					}
				}

				event := ei.Event()
				if event == notify.Create || event == notify.Rename {
					// info: rename event is triggered for both, the new AND old name => stat to check if path exists
					_, err := os.Stat(path)
					if err != nil {
						if !os.IsNotExist(err) {
							i.logger.Printf("stat for path failed: %s", err)
						}
					} else if i.ShouldPathGetIgnored(path) {
						err := i.SetIgnoreFlag(path)
						if err != nil {
							i.logger.Printf("Error ignoring dir %s: %s", path, err)
						} else {
							if cb != nil {
								cb <- path
							}
						}
					}

					if filepath.Base(path) == DropboxIgnoreFilename {
						err := i.addIgnoreFile(path)
						if err != nil {
							i.logger.Printf("Error adding ignore file: %s", err)
						}
					}
				}
				if event == notify.Remove || event == notify.Rename {
					// sometimes event order is incorrect => stat and check if file is created
					// e.g. fast delete file and create is again could swap the order of events
					_, err := os.Stat(path)
					if err != nil {
						if !os.IsNotExist(err) {
							i.logger.Printf("stat for path failed: %s", err)
						} else {
							i.ignoredPathsSet.Remove(path)
							if filepath.Base(path) == DropboxIgnoreFilename {
								i.removeIgnoreFile(path)
							}
						}
					}
				}
			}
		}
	}()
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
	return IsIgnored(i.ignorePatterns, path)
}
