# development

## fyne documentation:
https://docs.fyne.io/

## commit release
list all tags:
```bash
git tag
```

create tag (add -f to overwrite old one):
```bash
git tag -a v1.0.0 -m "Release v1.0.0"
```

push tag:
```bash
git push origin v1.0.0
```

Remove old tag local:
```bash
git tag -d v1.0.0
```

delete remote tag:
```bash
git push --delete origin v1.0.0
```

all in one, delete tag local and remote and push again:
```bash
MY_GIT_TAG="v1.0.0"

git push --delete origin "$MY_GIT_TAG" && git tag -af "$MY_GIT_TAG" && git push origin "$MY_GIT_TAG" && echo "successfully deleted tag $MY_GIT_TAG"
```


delete tags: https://stackoverflow.com/questions/20076233/replace-remote-tag-with-git

## building:
```bash
go mod tidy

go generate ./...
# go build
fyne package --release
./dropbox_ignore_service

fyne package && ./dropbox_ignore_service
```

### cross build:
```bash
go get github.com/fyne-io/fyne-cross && go install github.com/fyne-io/fyne-cross
fyne-cross windows
fyne-cross linux
fyne-cross linux -arch=arm # raspberry
fyne-cross darwin
```

## analyze binary size
```bash
go get github.com/jondot/goweight
go install github.com/jondot/goweight
goweight
```

## running tests:
```bash
go test -v ./...

# disable cache with -count=1
go test -v ./... -count=1
go test -v ./... -count=1 > out.txt 2>&1

# test 10 times to be sure no timing errors occur
go test -v ./... -count=10 > out.txt 2>&1

go test -v -run ^TestIgnoreFlagModify$ github.com/anton15x/dropbox_ignore_service
go test -v -run ^TestNewWatcherRecursive$  github.com/anton15x/dropbox_ignore_service/src/fsnotify

ENABLE_LARGE_TESTS=1 go test -v -count=1 -run ^TestDropboxIgnorerIgnoreFileEdit/big_test$ github.com/anton15x/dropbox_ignore_service > out.txt 2>&1

```

## running linter:
https://golangci-lint.run/usage/install/
```bash
golangci-lint run ./...
```

## read xattr with cli tools:
windows:
https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.management/get-content?view=powershell-7.3

linux:
sudo apt-get install attr
sudo apt-get install xattr
