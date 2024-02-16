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
	err := mainWithErrPanicWrapped()
	if err != nil {
		ShowError(err.Error())
		log.Fatal(err.Error())
		os.Exit(1)
	}
}

func mainWithErrPanicWrapped() (retErr error) {
	panicked := true
	defer func() {
		if panicked {
			r := recover()
			retErr = fmt.Errorf("panicked: %v", r)
		}
	}()

	err := mainWithErr()
	if err != nil {
		retErr = fmt.Errorf("errored: %s", err)
	}
	panicked = false
	return
}

func mainWithErr() error {
	var err error
	var logFilename string
	var dropboxFolders stringArrayFlags
	var tryRun bool
	var hideGUI bool

	const hideGUIArg = "hide-gui"
	const tryRunArg = "f"
	const dropboxFolderArg = "f"
	const logFilenameArg = "log"
	flag.StringVar(&logFilename, logFilenameArg, "", "The log file location (default: no file logging)")
	flag.Var(&dropboxFolders, dropboxFolderArg, "the path to the dropbox root folder, may be specified multiple times (skips reading dropbox config file)")
	flag.BoolVar(&hideGUI, hideGUIArg, false, "If true, the GUI will not get shown at start (used at autostart with the operation system)")
	flag.BoolVar(&tryRun, "t", false, "A try run (does only prints the files, that would get ignored)")
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

	logStringSlice := NewLogStringSlice()
	{
		bakWriter := log.Writer()
		defer log.SetOutput(bakWriter)
		log.SetOutput(io.MultiWriter(logStringSlice, bakWriter))
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
	args = append(args, "-"+hideGUIArg)
	if tryRun {
		args = append(args, "-"+tryRunArg)
	}
	if logFilename != "" {
		args = append(args, "-"+logFilenameArg, logFilename)
	}
	SetAutoStartArgs(args)

	var wg sync.WaitGroup
	// TODO: fyne package seems hide signals from us
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

	ignoredPathsSet := NewSortedStringSet()
	ignoreFilesSet := NewSortedStringSet()
	dropboxIgnorers := make([]*DropboxIgnorer, len(dropboxFolders))

	for i, dropboxFolder := range dropboxFolders {
		ignorer, err := NewDropboxIgnorer(dropboxFolder, tryRun, log.Default(), ctx, &wg, ignoredPathsSet, ignoreFilesSet)
		if err != nil {
			return fmt.Errorf("error creating dropbox ignorer for %s: %w", dropboxFolder, err)
		}
		dropboxIgnorers[i] = ignorer

		log.Printf("listening for events in dropbox %s", dropboxFolder)
		ignorer.ListenForEvents(nil)
	}

	err = ShowGUI(ctx, dropboxIgnorers, hideGUI, ignoredPathsSet, ignoreFilesSet, logStringSlice)
	if err != nil {
		return fmt.Errorf("error showing gui: %w", err)
	}
	log.Printf("gui exited")

	return nil

}
