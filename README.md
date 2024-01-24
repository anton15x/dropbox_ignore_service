# Dropbox Ignore Service

[![Go Reference](https://pkg.go.dev/badge/pkg.go.dev/github.com/anton15x/dropbox_ignore_service.svg)](https://pkg.go.dev/pkg.go.dev/github.com/anton15x/dropbox_ignore_service)
[![Test Status](https://github.com/anton15x/dropbox_ignore_service/actions/workflows/test.yml/badge.svg)](https://github.com/anton15x/dropbox_ignore_service/actions?query=workflow%3ATest))


Dropbox Ignore Service is a command-line utility to exclude files or folders from syncing to dropbox. The files gets specified via the `.dropboxignore` file located in the root of a dropbox folder. 

The application makes it possible, to have folders/files on PC, but not in dropbox cloud.

The motivation for development was to exclude node_modules automatically after such folder get created.

- Cross-platform support (Windows, Linux, and macOS)

## How it work?
Dropbox do not offer a functionality to exclude files from syncing automatically yet. But wat is does, is checking a file/folder for a ignore flag which this program sets. The dropbox has an article about that: https://help.dropbox.com/sync/ignored-files .

## .dropboxignore example
The `.dropboxignore` currently disallows the use of `*\#[]?!`

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
You can install the application using [go](https://go.dev/dl/):

```bash
go install "github.com/anton15x/dropbox_ignore_service"
```

## Usage
You can run the Dropbox Ignore Service once without installing as service with:
```bash
dropbox_ignore_service
```

## Resources:
dropbox documentation about ignoring files:
https://help.dropbox.com/sync/ignored-files

dropbox documentation about finding pragmatically the dropbox folder(s):
https://help.dropbox.com/de-de/installs/locate-dropbox-folder


## similar projects:
- [kichik/dropbox-ignorer](https://github.com/kichik/dropbox-ignorer)
a cli implementation to specify the dropbox path and ignore files

- [sp1thas/dropboxignore](https://github.com/sp1thas/dropboxignore)
also parses .dropboxignore files more featured but without fs watch

## used libraries:
- [rjeczalik/notify](https://github.com/rjeczalik/notify) filesystem change event listener:
- [pkg/xattr](https://github.com/pkg/xattr) read extended file attribute for linux/darwin.

## Limitations/TODO:
- allow patters, to prevent possible braking changes, special characters got disabled for now: `*\#[]?!`
  glob reference: https://globster.xyz/
- nested dropboxignore files (ignore fiels in subfolders)
