package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

// var version = "<VERSION>"
// var commit = "<COMMIT>"
// var date = "<DATE>"

func getDropboxFoldersEnsured(cmdFolders []string) ([]string, error) {
	if len(cmdFolders) > 0 {
		return cmdFolders, nil
	}

	folders, err := ParseDropboxInfoPaths()
	if err != nil {
		return nil, fmt.Errorf("error parsing dropbox folders: %s", err)
	}
	if len(folders) == 0 {
		return nil, fmt.Errorf("could not find dropbox folders")
	}
	return folders, nil
}

type stringArrayFlags []string

func (i *stringArrayFlags) String() string {
	return "my string representation"
}
func (i *stringArrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	err := mainWithErr()
	if err != nil {
		log.Fatalf("Error running program %s", err)
		os.Exit(1)
	}
}

func mainWithErr() error {
	var err error
	var logFilename string
	var dropboxFolders stringArrayFlags
	var tryRun bool

	const tryRunAry = "f"
	const dropboxFolderArg = "f"
	const logFilenameArg = "log"
	flag.BoolVar(&tryRun, "t", false, "A try run (does only prints the files, that would get ignored)")
	flag.StringVar(&logFilename, logFilenameArg, "", "The log file location (default: system log/stdout)")
	flag.Var(&dropboxFolders, dropboxFolderArg, "the path to the dropbox root folder (may be specified multiple times)")
	flag.Parse()

	if logFilename != "" {
		absPath, err := filepath.Abs(logFilename)
		if err != nil {
			log.Fatalf("error getting abs path of %s: %s", logFilename, err)
		}
		logFilename = absPath
		logFile, err := os.OpenFile(logFilename, os.O_RDWR|os.O_CREATE|os.O_APPEND, os.ModePerm)
		if err != nil {
			log.Fatalf("Error open log file %s: %s", logFilename, err)
		}
		defer func() {
			err = logFile.Close()
			if err != nil {
				log.Printf("error closing log file: %s", err)
			}
		}()
		bakWriter := log.Writer()
		defer log.SetOutput(bakWriter)
		log.SetOutput(io.MultiWriter(logFile, bakWriter))
	}

	for i, dropboxFolder := range dropboxFolders {
		absPath, err := filepath.Abs(dropboxFolder)
		if err != nil {
			log.Fatalf("error getting abs path of %s: %s", dropboxFolder, err)
		}
		dropboxFolders[i] = absPath
	}

	args := []string{}
	for _, dropboxFolder := range dropboxFolders {
		args = append(args, "-"+dropboxFolderArg, dropboxFolder)
	}
	if tryRun {
		args = append(args, "-"+tryRunAry)
	}
	if logFilename != "" {
		args = append(args, "-"+logFilenameArg, logFilename)
	}
	SetAutoStartArgs(args)

	var wg sync.WaitGroup
	// ctx, ctxStop := context.WithCancel(context.Background())
	ctx, ctxStop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		ctxStop()
		wg.Wait()
	}()

	dropboxFolders, err = getDropboxFoldersEnsured(dropboxFolders)
	if err != nil {
		return err
	}
	log.Printf("handling dropbox folders: %+v", dropboxFolders)

	var m sync.Mutex

	ignoredPathsSet := NewSortedStringSet()
	ignoreFilesSet := NewSortedStringSet()
	dropboxIgnorers := make([]*DropboxIgnorer, len(dropboxFolders))
	var initWg sync.WaitGroup
	for i, dropboxFolder := range dropboxFolders {
		i := i
		dropboxFolder := dropboxFolder

		initWg.Add(1)
		go func() {
			defer initWg.Done()

			m.Lock()
			defer m.Unlock()

			ignorer, err := NewDropboxIgnorer(dropboxFolder, tryRun, log.Default(), ctx, &wg, ignoredPathsSet, ignoreFilesSet)
			if err != nil {
				log.Printf("Error creating dropbox ignorer for %s: %s", dropboxFolder, err)

				return
			}
			dropboxIgnorers[i] = ignorer

			log.Printf("listening for events in dropbox %s", dropboxFolder)
			ignorer.ListenForEvents(nil)
		}()
	}

	initWg.Wait()

	err = ShowGUI(ctx, dropboxIgnorers, ignoredPathsSet, ignoreFilesSet)
	if err != nil {
		return fmt.Errorf("error showing gui: %s", err)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		log.Printf("Warning: exit early: %s", ctxErr)
	}

	return nil

}
