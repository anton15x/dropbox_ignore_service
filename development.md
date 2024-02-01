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
git tag -a v0.0.1
```

push tag:
```bash
git push origin v0.0.1
```

Remove old tag local:
```bash
git tag -d v0.0.1
```

delete remote tag:
```bash
git push --delete origin v0.0.1
```

all in one, delete tag local and remote and push again:
```bash
MY_GIT_TAG="v0.0.1"

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

go build && ./dropbox_ignore_service
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
go test ./...

go test -run ^TestIgnoreFlagModify$ github.com/anton15x/dropbox_ignore_service
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
