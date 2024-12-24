<p align="center">
<img alt="application icon" src="assets/icon.png" width="200">
</p>

# Dropbox Ignore Service

[![Go Reference](https://pkg.go.dev/badge/pkg.go.dev/github.com/anton15x/dropbox_ignore_service.svg)](https://pkg.go.dev/github.com/anton15x/dropbox_ignore_service)
[![Test Status](https://github.com/anton15x/dropbox_ignore_service/actions/workflows/main.yml/badge.svg)](https://github.com/anton15x/dropbox_ignore_service/actions/workflows/main.yml)

Dropbox Ignore Service is tool to exclude files or folders from syncing to dropbox. The files get specified via the `.dropboxignore` file located in the root of a dropbox folder. 

It watches for file/folder changes and notifies the user when a file/folder gets ignored via system notification.

It also offers a GUI:
- list all currently ignored files/folders
- list ignored files that are not in the .dropboxignore file specified => button to unignore them
- List .dropboxignore files
- Logs
- Settings: Enable autostart with the operation system

Cross-platform support (Windows, Linux, and macOS)

## Motivation
The primary motivation behind developing this tool was to automatically exclude the `node_modules` folder after it gets created. While there are similar projects, most of them lack a GUI.

## How it works
Dropbox does not offer a functionality to exclude files from syncing automatically yet. But what is does, is checking a file/folder for a ignore flag which this program sets. The dropbox has an article about that: https://help.dropbox.com/sync/ignored-files .

## .dropboxignore example
The `.dropboxignore` tries to be `.gitignore` compliant.

The only difference is that negations (lines that start with a !) are not allowed.

Here is an example of a valid `.dropboxignore` file:
```bash
# ignore the node_modules folder located inside any directory
node_modules

# lines containing a slash are relative to the ignore file location
my_project/.git
/my_project/.git
/my_folder

# ignore my_project/.git located in any directory
**/my_project/.git

# matches the path "#folder"
\#folder
# matches the path "!folder"
\!folder
```

## Installation
You can download the application from the releases section, it is a portable single file executable:
```bash
https://github.com/anton15x/dropbox_ignore_service/releases
```

### Building form source
Requirements:
- [go](https://go.dev/dl/) >= 1.21.0
- gcc
- Fyne dependencies:
  - windows: No additional dependencies required.
  - linux (ubuntu): Install `xorg-dev` using the command: `sudo apt-get install -y xorg-dev`
  - macOS: Install XQuartz using Homebrew: `brew install --cask xquartz`
  - other: see official documentation: https://docs.fyne.io/started/

```bash
go mod download
go generate ./...
go install fyne.io/fyne/v2/cmd/fyne@v2.4.3
fyne package
```

## Flags
- log
  - The log file location (default: no file logging)
- f
  - the path to the dropbox root folder, may be specified multiple times (skips reading dropbox config file)
- hide-gui
  - If true, the GUI will not get shown at start (used at autostart with the operation system)
- t
  - A try run (does only prints the files, that would get ignored)

## Resources:
dropbox documentation about ignoring files:
https://help.dropbox.com/sync/ignored-files
https://help.dropbox.com/de-de/sync/extended-attributes

dropbox documentation about finding pragmatically the dropbox folder(s):
https://help.dropbox.com/de-de/installs/locate-dropbox-folder

## used libraries:
- [rjeczalik/notify](https://github.com/rjeczalik/notify) filesystem change event listener used for windows (recursive watch)
- [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) filesystem change event listener used for all other platforms (no recursive watch, own implementation, that watches every subfolder separately)
- [pkg/xattr](https://github.com/pkg/xattr) read extended file attribute for linux/darwin.
- [fyne.io/fyne](https://github.com/fyne-io/fyne) Cross platform GUI toolkit in Go inspired by Material Design
- [spiretechnology/go-autostart](https://github.com/spiretechnology/go-autostart) for setting the executable to autostart with the system
- [bmatcuk/doublestar](https://github.com/bmatcuk/doublestar) Path pattern matching and globbing supporting doublestar (**) patterns.
- [stretchr/testify](https://github.com/stretchr/testify) testing of course
