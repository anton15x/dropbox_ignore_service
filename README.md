<p align="center">
<img alt="application icon" src="assets/icon.png" width="200">
</p>

# Dropbox Ignore Service

[![Go Reference](https://pkg.go.dev/badge/pkg.go.dev/github.com/anton15x/dropbox_ignore_service.svg)](https://pkg.go.dev/github.com/anton15x/dropbox_ignore_service)
[![Test Status](https://github.com/anton15x/dropbox_ignore_service/actions/workflows/main.yml/badge.svg)](https://github.com/anton15x/dropbox_ignore_service/actions/workflows/main.yml)

Dropbox Ignore Service is tool to exclude files or folders from syncing to dropbox. The files gets specified via the `.dropboxignore` file located in the root of a dropbox folder. 

It watches for file/folder changes and notifies the user when a file/folder gets ignored.

It also offers a GUI:
- list all currently ignored files
- list ignored files that are not in the .dropboxignore file specified => button to unignore them
- List .dropboxignore files
- Logs
- Settings: Enable autostart with the operation system

The motivation for development was to exclude node_modules automatically after such folder get created.

- Cross-platform support (Windows, Linux, and macOS)

## How it work?
Dropbox do not offer a functionality to exclude files from syncing automatically yet. But what is does, is checking a file/folder for a ignore flag which this program sets. The dropbox has an article about that: https://help.dropbox.com/sync/ignored-files .

## .dropboxignore example
The `.dropboxignore` currently disallows the use of `*\#[]?!` (exception: # of start of line makes it a comment)

A example for a valid files is this:
```bash
# ignore the node_modules folder located inside any directory
node_modules
# ignores the .git folder located inside the my_project folder
my_project/.git

# ignores my_folder at the dropbox root
/my_folder
# ignores subfolder located inside my_folder at the project dropbox root
/my_folder/subfolder
```

## Installation
You can download the application from the releases, it is a portable single file executable:
```bash
https://github.com/anton15x/dropbox_ignore_service/releases
```

### Building form source
Requirements:
- [go](https://go.dev/dl/) >= 1.21.0
- gcc
- Fyne dependencies: https://docs.fyne.io/started/
  - windows: none
  - linux (ubuntu): `sudo apt-get install -y xorg-dev`
  - macOX: `brew install --cask xquartz`

```bash
go mod download
go generate ./...
go install fyne.io/fyne/v2/cmd/fyne@latest
fyne package --release
```

## Flags
- log
  - The log file location (default: no file logging)
- f
  - the path to the dropbox root folder, may be specified multiple times (skips reading dropbox config file)
- hide-gui
  - If true, the GUI will not get shown at start (used at autostart with the operation system
- t
  - A try run (does only prints the files, that would get ignored)

## Resources:
dropbox documentation about ignoring files:
https://help.dropbox.com/sync/ignored-files
https://help.dropbox.com/de-de/sync/extended-attributes

dropbox documentation about finding pragmatically the dropbox folder(s):
https://help.dropbox.com/de-de/installs/locate-dropbox-folder


## similar projects:
- [kichik/dropbox-ignorer](https://github.com/kichik/dropbox-ignorer)
a cli implementation to specify the dropbox path and ignore files

- [sp1thas/dropboxignore](https://github.com/sp1thas/dropboxignore)
a cli tool that parses .dropboxignore files more featured but without fs watch

## used libraries:
- [rjeczalik/notify](https://github.com/rjeczalik/notify) filesystem change event listener:
- [pkg/xattr](https://github.com/pkg/xattr) read extended file attribute for linux/darwin.
- [fyne.io/fyne](https://github.com/fyne-io/fyne) Cross platform GUI toolkit in Go inspired by Material Design
- [spiretechnology/go-autostart](https://github.com/spiretechnology/go-autostart) for setting the executable to autostart with the system
- [stretchr/testify](https://github.com/stretchr/testify) testing of course

## Limitations/TODO:
- allow patters, to prevent possible braking changes, special characters got disabled for now: `*\#[]?!`
  glob reference: https://globster.xyz/
- nested .dropboxignore files (currently only one file in root dropbox folder allowed)
