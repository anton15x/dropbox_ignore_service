/* cSpell:disable */

# development
## building:
```bash
go mod tidy

go generate ./...
# go build
fyne package --release
./dropbox_ignore_service

36846920 go build
18170880 go build -ldflags "-w -s -H=windowsgui"
36870104 fyne package
36870104 fyne package -os windows
18167808 fyne package --release --id=com.github.anton15x.dropbox_ignore_service -icon assets/icon.png
18167808 fyne package --release -os windows
18167808 fyne package --release -os linux
18167808 fyne package --release -os darwin
# fyne package -os windows -icon myapp.png

wc -c dropbox_ignore_service.exe

go build && ./dropbox_ignore_service

// https://github.com/Licoy/fetch-github-hosts/blob/main/.github/workflows/build-linux-windows.yml
go get github.com/fyne-io/fyne-cross && go install github.com/fyne-io/fyne-cross
fyne-cross windows
fyne-cross linux
fyne-cross linux -arch=arm # raspberry
fyne-cross darwin


// https://www.dropbox.com/install-linux
dropbox linux headless installation, auto login = ?

```


mkdir -p test/a/node_modules
mkdir -p test/b/node_modules
mkdir -p test/c/node_modules
mkdir -p test/d/node_modules
mkdir -p test/e/node_modules
mkdir -p test/f/node_modules
mkdir -p test/g/node_modules
mkdir -p test/h/node_modules
mkdir -p test/i/node_modules
mkdir -p test/j/node_modules
mkdir -p test/k/node_modules
mkdir -p test/l/node_modules
mkdir -p test/m/node_modules
mkdir -p test/n/node_modules
mkdir -p test/o/node_modules
mkdir -p test/p/node_modules
mkdir -p test/q/node_modules
mkdir -p test/r/node_modules
mkdir -p test/s/node_modules
mkdir -p test/t/node_modules
mkdir -p test/u/node_modules
mkdir -p test/v/node_modules
mkdir -p test/w/node_modules
mkdir -p test/x/node_modules
mkdir -p test/y/node_modules
mkdir -p test/z/node_modules

mkdir -p test/a/my_ignored_test_filst
mkdir -p test/b/my_ignored_test_filst
mkdir -p test/c/my_ignored_test_filst
mkdir -p test/d/my_ignored_test_filst
mkdir -p test/e/my_ignored_test_filst
mkdir -p test/f/my_ignored_test_filst
mkdir -p test/g/my_ignored_test_filst
mkdir -p test/h/my_ignored_test_filst
mkdir -p test/i/my_ignored_test_filst
mkdir -p test/j/my_ignored_test_filst
mkdir -p test/k/my_ignored_test_filst
mkdir -p test/l/my_ignored_test_filst
mkdir -p test/m/my_ignored_test_filst
mkdir -p test/n/my_ignored_test_filst
mkdir -p test/o/my_ignored_test_filst
mkdir -p test/p/my_ignored_test_filst
mkdir -p test/q/my_ignored_test_filst
mkdir -p test/r/my_ignored_test_filst
mkdir -p test/s/my_ignored_test_filst
mkdir -p test/t/my_ignored_test_filst
mkdir -p test/u/my_ignored_test_filst
mkdir -p test/v/my_ignored_test_filst
mkdir -p test/w/my_ignored_test_filst
mkdir -p test/x/my_ignored_test_filst
mkdir -p test/y/my_ignored_test_filst
mkdir -p test/z/my_ignored_test_filst


go get github.com/jondot/goweight
go install github.com/jondot/goweight
goweight

## running tests:
```bash
go test ./...

go test -run ^TestIgnoreFlagModify$ github.com/anton15x/dropbox_ignore_service
```

```bash
go test -coverprofile cover.out ./... 
go tool cover -html=cover.out
```



https://github.com/fyne-io/fyne





windows:
https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.management/get-content?view=powershell-7.3

linux:
sudo apt-get install attr




mkdir folder
touch folder/file1.txt
touch folder/file2.txt
attr -s com.dropbox.ignored -V 1 folder/file1.txt
attr -s com.dropbox.ignored -V 1 folder/file2.txt

attr -r -g com.dropbox.ignored folder


attr -s com.dropbox.ignored -V 1 file.txt
attr -g com.dropbox.ignored file.txt
attr -r com.dropbox.ignored file.txt
